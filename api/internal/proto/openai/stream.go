package openai

import (
	"fmt"
	"strings"
	"sync"

	"github.com/tidwall/gjson"
)

type openAIClaudeStreamState struct {
	ID               string
	Model            string
	Created          int64
	MessageStarted   bool
	MessageDeltaSent bool
	PendingFinish    string
	SawToolCall      bool
	TextOpen         bool
	TextIndex        int
	ThinkingOpen     bool
	ThinkingIndex    int
	NextContentIndex int
	ToolCallsByIndex map[int]*openAIClaudeToolState
}

type openAIClaudeToolState struct {
	ID         string
	Name       string
	BlockIndex int
	Started    bool
	Pending    strings.Builder
}

type openAIGeminiStreamState struct {
	ToolCallsByIndex map[int]*openAIGeminiToolState
}

type openAIGeminiToolState struct {
	Name string
	Args strings.Builder
}

var (
	openAIClaudeStreamMu  sync.Mutex
	openAIClaudeStreams   = map[string]*openAIClaudeStreamState{}
	openAIClaudeActiveKey string
	openAIGeminiStreamMu  sync.Mutex
	openAIGeminiStreams   = map[string]*openAIGeminiStreamState{}
	openAIGeminiActiveKey string
)

func StreamResponseToClaude(chunk []byte) []byte {
	openAIClaudeStreamMu.Lock()
	defer openAIClaudeStreamMu.Unlock()

	raw, done := normalizeOpenAIStreamChunk(chunk)
	if len(raw) == 0 {
		return nil
	}
	if done {
		key := openAIClaudeActiveKey
		state := openAIClaudeStreams[key]
		if state == nil {
			return nil
		}
		var out strings.Builder
		openAIClaudeCloseThinking(state, &out)
		openAIClaudeCloseText(state, &out)
		openAIClaudeCloseTools(state, &out)
		openAIClaudeEmitMessageDelta(state, gjson.Result{}, &out)
		if state.MessageStarted {
			openAIClaudeEmitEvent(&out, "message_stop", map[string]any{"type": "message_stop"})
		}
		delete(openAIClaudeStreams, key)
		openAIClaudeActiveKey = ""
		return []byte(out.String())
	}

	root := gjson.ParseBytes(raw)
	key := root.Get("id").String()
	if key == "" {
		key = openAIClaudeActiveKey
	}
	if key == "" {
		key = "openai-claude-stream"
	}
	openAIClaudeActiveKey = key

	state := openAIClaudeStreams[key]
	if state == nil {
		state = &openAIClaudeStreamState{
			ID:               key,
			TextIndex:        -1,
			ThinkingIndex:    -1,
			ToolCallsByIndex: map[int]*openAIClaudeToolState{},
		}
		openAIClaudeStreams[key] = state
	}
	if model := root.Get("model").String(); model != "" {
		state.Model = model
	}
	if created := root.Get("created").Int(); created > 0 {
		state.Created = created
	}

	var out strings.Builder
	if !state.MessageStarted {
		openAIClaudeEmitEvent(&out, "message_start", map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"id":            state.ID,
				"type":          "message",
				"role":          "assistant",
				"model":         state.Model,
				"content":       []any{},
				"stop_reason":   nil,
				"stop_sequence": nil,
				"usage": map[string]any{
					"input_tokens":  0,
					"output_tokens": 0,
				},
			},
		})
		state.MessageStarted = true
	}

	delta := root.Get("choices.0.delta")
	for _, text := range streamCollectOpenAIReasoningTexts(delta.Get("reasoning_content")) {
		if text == "" {
			continue
		}
		openAIClaudeCloseText(state, &out)
		openAIClaudeEnsureThinking(state, &out)
		openAIClaudeEmitEvent(&out, "content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": state.ThinkingIndex,
			"delta": map[string]any{
				"type":     "thinking_delta",
				"thinking": text,
			},
		})
	}

	if content := delta.Get("content"); content.Exists() && content.String() != "" {
		openAIClaudeCloseThinking(state, &out)
		openAIClaudeEnsureText(state, &out)
		openAIClaudeEmitEvent(&out, "content_block_delta", map[string]any{
			"type":  "content_block_delta",
			"index": state.TextIndex,
			"delta": map[string]any{
				"type": "text_delta",
				"text": content.String(),
			},
		})
	}

	if toolCalls := delta.Get("tool_calls"); toolCalls.Exists() && toolCalls.IsArray() {
		for _, toolCall := range toolCalls.Array() {
			state.SawToolCall = true
			index := int(toolCall.Get("index").Int())
			toolState := state.ToolCallsByIndex[index]
			if toolState == nil {
				toolState = &openAIClaudeToolState{BlockIndex: state.NextContentIndex}
				state.NextContentIndex++
				state.ToolCallsByIndex[index] = toolState
			}
			if id := toolCall.Get("id").String(); id != "" {
				toolState.ID = id
			}
			if name := toolCall.Get("function.name").String(); name != "" {
				toolState.Name = name
			}
			if !toolState.Started && toolState.Name != "" {
				openAIClaudeCloseThinking(state, &out)
				openAIClaudeCloseText(state, &out)
				openAIClaudeEmitEvent(&out, "content_block_start", map[string]any{
					"type":  "content_block_start",
					"index": toolState.BlockIndex,
					"content_block": map[string]any{
						"type":  "tool_use",
						"id":    streamDefaultString(toolState.ID, fmt.Sprintf("call_%d", index)),
						"name":  toolState.Name,
						"input": map[string]any{},
					},
				})
				toolState.Started = true
				if toolState.Pending.Len() > 0 {
					openAIClaudeEmitEvent(&out, "content_block_delta", map[string]any{
						"type":  "content_block_delta",
						"index": toolState.BlockIndex,
						"delta": map[string]any{
							"type":         "input_json_delta",
							"partial_json": toolState.Pending.String(),
						},
					})
					toolState.Pending.Reset()
				}
			}
			if args := toolCall.Get("function.arguments"); args.Exists() && args.String() != "" {
				if toolState.Started {
					openAIClaudeEmitEvent(&out, "content_block_delta", map[string]any{
						"type":  "content_block_delta",
						"index": toolState.BlockIndex,
						"delta": map[string]any{
							"type":         "input_json_delta",
							"partial_json": args.String(),
						},
					})
				} else {
					toolState.Pending.WriteString(args.String())
				}
			}
		}
	}

	finishReason := root.Get("choices.0.finish_reason").String()
	if finishReason != "" {
		state.PendingFinish = finishReason
		openAIClaudeCloseThinking(state, &out)
		openAIClaudeCloseText(state, &out)
		openAIClaudeCloseTools(state, &out)
		openAIClaudeEmitMessageDelta(state, root.Get("usage"), &out)
	}

	if root.Get("usage").Exists() && state.PendingFinish != "" && !state.MessageDeltaSent {
		openAIClaudeEmitMessageDelta(state, root.Get("usage"), &out)
	}

	return []byte(out.String())
}

func StreamResponseToGemini(chunk []byte) []byte {
	openAIGeminiStreamMu.Lock()
	defer openAIGeminiStreamMu.Unlock()

	raw, done := normalizeOpenAIStreamChunk(chunk)
	if len(raw) == 0 || done {
		if done {
			delete(openAIGeminiStreams, openAIGeminiActiveKey)
			openAIGeminiActiveKey = ""
		}
		return nil
	}

	root := gjson.ParseBytes(raw)
	key := root.Get("id").String()
	if key == "" {
		key = openAIGeminiActiveKey
	}
	if key == "" {
		key = "openai-gemini-stream"
	}
	openAIGeminiActiveKey = key

	state := openAIGeminiStreams[key]
	if state == nil {
		state = &openAIGeminiStreamState{ToolCallsByIndex: map[int]*openAIGeminiToolState{}}
		openAIGeminiStreams[key] = state
	}

	out := map[string]any{}
	if model := root.Get("model").String(); model != "" {
		out["modelVersion"] = model
	}

	choice := root.Get("choices.0")
	parts := make([]any, 0)
	delta := choice.Get("delta")
	for _, text := range streamCollectOpenAIReasoningTexts(delta.Get("reasoning_content")) {
		if text == "" {
			continue
		}
		parts = append(parts, map[string]any{"thought": true, "text": text})
	}
	if content := delta.Get("content"); content.Exists() && content.String() != "" {
		parts = append(parts, map[string]any{"text": content.String()})
	}

	if toolCalls := delta.Get("tool_calls"); toolCalls.Exists() && toolCalls.IsArray() {
		for _, toolCall := range toolCalls.Array() {
			index := int(toolCall.Get("index").Int())
			toolState := state.ToolCallsByIndex[index]
			if toolState == nil {
				toolState = &openAIGeminiToolState{}
				state.ToolCallsByIndex[index] = toolState
			}
			if name := toolCall.Get("function.name").String(); name != "" {
				toolState.Name = name
			}
			if args := toolCall.Get("function.arguments"); args.Exists() && args.String() != "" {
				toolState.Args.WriteString(args.String())
			}
		}
	}

	finishReason := choice.Get("finish_reason").String()
	if finishReason != "" && len(state.ToolCallsByIndex) > 0 {
		indexes := streamSortedMapKeys(state.ToolCallsByIndex)
		for _, index := range indexes {
			toolState := state.ToolCallsByIndex[index]
			parts = append(parts, map[string]any{
				"functionCall": map[string]any{
					"name": toolState.Name,
					"args": streamParseJSONObject(toolState.Args.String()),
				},
			})
		}
		state.ToolCallsByIndex = map[int]*openAIGeminiToolState{}
	}

	if len(parts) > 0 || finishReason != "" {
		candidate := map[string]any{
			"index": choice.Get("index").Int(),
			"content": map[string]any{
				"role":  "model",
				"parts": parts,
			},
		}
		if finishReason != "" {
			candidate["finishReason"] = mapOpenAIFinishReasonToGemini(finishReason)
		}
		out["candidates"] = []any{candidate}
	} else if root.Get("usage").Exists() {
		out["candidates"] = []any{}
	}

	if usage := root.Get("usage"); usage.Exists() {
		usageMetadata := map[string]any{
			"promptTokenCount":     usage.Get("prompt_tokens").Int(),
			"candidatesTokenCount": usage.Get("completion_tokens").Int(),
			"totalTokenCount":      usage.Get("total_tokens").Int(),
		}
		if reasoningTokens := streamReasoningTokensFromUsage(usage); reasoningTokens > 0 {
			usageMetadata["thoughtsTokenCount"] = reasoningTokens
		}
		out["usageMetadata"] = usageMetadata
	}

	if len(out) == 0 {
		return nil
	}
	return mustJSON(out)
}

func normalizeOpenAIStreamChunk(chunk []byte) ([]byte, bool) {
	raw := strings.TrimSpace(string(chunk))
	if raw == "" {
		return nil, false
	}
	if strings.HasPrefix(raw, "data:") {
		raw = strings.TrimSpace(strings.TrimPrefix(raw, "data:"))
	}
	if raw == "[DONE]" {
		return []byte(raw), true
	}
	return []byte(raw), false
}

func openAIClaudeEnsureText(state *openAIClaudeStreamState, out *strings.Builder) {
	if state.TextOpen {
		return
	}
	state.TextIndex = state.NextContentIndex
	state.NextContentIndex++
	openAIClaudeEmitEvent(out, "content_block_start", map[string]any{
		"type":  "content_block_start",
		"index": state.TextIndex,
		"content_block": map[string]any{
			"type": "text",
			"text": "",
		},
	})
	state.TextOpen = true
}

func openAIClaudeEnsureThinking(state *openAIClaudeStreamState, out *strings.Builder) {
	if state.ThinkingOpen {
		return
	}
	state.ThinkingIndex = state.NextContentIndex
	state.NextContentIndex++
	openAIClaudeEmitEvent(out, "content_block_start", map[string]any{
		"type":  "content_block_start",
		"index": state.ThinkingIndex,
		"content_block": map[string]any{
			"type":     "thinking",
			"thinking": "",
		},
	})
	state.ThinkingOpen = true
}

func openAIClaudeCloseText(state *openAIClaudeStreamState, out *strings.Builder) {
	if !state.TextOpen {
		return
	}
	openAIClaudeEmitEvent(out, "content_block_stop", map[string]any{"type": "content_block_stop", "index": state.TextIndex})
	state.TextOpen = false
	state.TextIndex = -1
}

func openAIClaudeCloseThinking(state *openAIClaudeStreamState, out *strings.Builder) {
	if !state.ThinkingOpen {
		return
	}
	openAIClaudeEmitEvent(out, "content_block_stop", map[string]any{"type": "content_block_stop", "index": state.ThinkingIndex})
	state.ThinkingOpen = false
	state.ThinkingIndex = -1
}

func openAIClaudeCloseTools(state *openAIClaudeStreamState, out *strings.Builder) {
	for _, index := range streamSortedMapKeys(state.ToolCallsByIndex) {
		toolState := state.ToolCallsByIndex[index]
		if !toolState.Started {
			continue
		}
		openAIClaudeEmitEvent(out, "content_block_stop", map[string]any{"type": "content_block_stop", "index": toolState.BlockIndex})
	}
	state.ToolCallsByIndex = map[int]*openAIClaudeToolState{}
}

func openAIClaudeEmitMessageDelta(state *openAIClaudeStreamState, usage gjson.Result, out *strings.Builder) {
	if state.MessageDeltaSent || state.PendingFinish == "" {
		return
	}
	payload := map[string]any{
		"type": "message_delta",
		"delta": map[string]any{
			"stop_reason":   mapOpenAIFinishReasonToClaude(streamOpenAIEffectiveFinishReason(state)),
			"stop_sequence": nil,
		},
		"usage": map[string]any{
			"input_tokens":  int64(0),
			"output_tokens": int64(0),
		},
	}
	if usage.Exists() {
		payload["usage"] = map[string]any{
			"input_tokens":  usage.Get("prompt_tokens").Int(),
			"output_tokens": usage.Get("completion_tokens").Int(),
		}
	}
	openAIClaudeEmitEvent(out, "message_delta", payload)
	state.MessageDeltaSent = true
}

func openAIClaudeEmitEvent(out *strings.Builder, name string, payload any) {
	out.WriteString("event: ")
	out.WriteString(name)
	out.WriteString("\n")
	out.WriteString("data: ")
	out.Write(mustJSON(payload))
	out.WriteString("\n\n")
}

func streamOpenAIEffectiveFinishReason(state *openAIClaudeStreamState) string {
	if state.SawToolCall {
		return "tool_calls"
	}
	return state.PendingFinish
}

func streamCollectOpenAIReasoningTexts(node gjson.Result) []string {
	texts := make([]string, 0)
	if !node.Exists() {
		return texts
	}
	if node.IsArray() {
		for _, item := range node.Array() {
			texts = append(texts, streamCollectOpenAIReasoningTexts(item)...)
		}
		return texts
	}
	switch node.Type {
	case gjson.String:
		if text := node.String(); text != "" {
			texts = append(texts, text)
		}
	case gjson.JSON:
		if text := node.Get("text"); text.Exists() && text.String() != "" {
			texts = append(texts, text.String())
		}
	}
	return texts
}

func streamParseJSONObject(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}
	}
	parsed := gjson.Parse(raw)
	if parsed.Exists() && parsed.IsObject() {
		if value, ok := parsed.Value().(map[string]any); ok && value != nil {
			return value
		}
	}
	return map[string]any{}
}

func streamReasoningTokensFromUsage(usage gjson.Result) int64 {
	if value := usage.Get("completion_tokens_details.reasoning_tokens"); value.Exists() {
		return value.Int()
	}
	if value := usage.Get("output_tokens_details.reasoning_tokens"); value.Exists() {
		return value.Int()
	}
	return 0
}

func streamDefaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func streamSortedMapKeys[T any](m map[int]T) []int {
	keys := make([]int, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}
