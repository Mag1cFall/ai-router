// Gemini 流式响应转换：将 Gemini SSE 分片转换为 OpenAI 和 Claude 流式格式
package gemini

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tidwall/gjson"
)

type geminiClaudeStreamState struct {
	ID           string
	Model        string
	MessageSent  bool
	CurrentType  string
	CurrentIndex int
	NextIndex    int
	SawToolCall  bool
}

var (
	geminiClaudeStreamMu  sync.Mutex
	geminiClaudeStreams   = map[string]*geminiClaudeStreamState{}
	geminiClaudeActiveKey string
	geminiOpenAICounter   uint64
)

func StreamResponseToOpenAI(chunk []byte) []byte {
	raw := normalizeGeminiStreamChunk(chunk)
	if len(raw) == 0 || string(raw) == "[DONE]" {
		return nil
	}

	root := gjson.ParseBytes(raw)
	created := time.Now().Unix()
	if createTime := root.Get("createTime"); createTime.Exists() {
		if parsed, err := time.Parse(time.RFC3339Nano, createTime.String()); err == nil {
			created = parsed.Unix()
		}
	}
	model := root.Get("modelVersion").String()
	responseID := root.Get("responseId").String()

	usage := map[string]any{}
	if usageNode := root.Get("usageMetadata"); usageNode.Exists() {
		usage = map[string]any{
			"prompt_tokens":     usageNode.Get("promptTokenCount").Int(),
			"completion_tokens": usageNode.Get("candidatesTokenCount").Int(),
			"total_tokens":      usageNode.Get("totalTokenCount").Int(),
		}
		if reasoningTokens := usageNode.Get("thoughtsTokenCount").Int(); reasoningTokens > 0 {
			usage["completion_tokens_details"] = map[string]any{"reasoning_tokens": reasoningTokens}
		}
	}

	var out strings.Builder
	candidates := root.Get("candidates")
	if candidates.IsArray() && len(candidates.Array()) > 0 {
		for _, candidate := range candidates.Array() {
			delta := map[string]any{"role": "assistant"}
			hasToolCall := false
			for _, part := range candidate.Get("content.parts").Array() {
				if text := part.Get("text"); text.Exists() {
					if part.Get("thought").Bool() {
						delta["reasoning_content"] = text.String()
					} else {
						delta["content"] = text.String()
					}
					continue
				}
				if functionCall := part.Get("functionCall"); functionCall.Exists() {
					hasToolCall = true
					delta["tool_calls"] = []any{map[string]any{
						"index": 0,
						"id":    fmt.Sprintf("call_%d", atomic.AddUint64(&geminiOpenAICounter, 1)),
						"type":  "function",
						"function": map[string]any{
							"name":      functionCall.Get("name").String(),
							"arguments": rawJSONString(functionCall.Get("args")),
						},
					}}
				}
			}
			finishReason := mapGeminiFinishReasonToOpenAI(candidate.Get("finishReason").String())
			if hasToolCall {
				finishReason = "tool_calls"
			}
			payload := map[string]any{
				"id":      responseID,
				"object":  "chat.completion.chunk",
				"created": created,
				"model":   model,
				"choices": []any{map[string]any{
					"index":         candidate.Get("index").Int(),
					"delta":         delta,
					"finish_reason": nil,
				}},
			}
			if finishReason != "" {
				payload["choices"].([]any)[0].(map[string]any)["finish_reason"] = finishReason
			}
			if len(usage) > 0 {
				payload["usage"] = usage
			}
			out.WriteString("data: ")
			out.Write(mustJSON(payload))
			out.WriteString("\n\n")
		}
		return []byte(out.String())
	}

	if len(usage) > 0 {
		payload := map[string]any{
			"id":      responseID,
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   model,
			"choices": []any{},
			"usage":   usage,
		}
		return append([]byte("data: "), append(mustJSON(payload), []byte("\n\n")...)...)
	}

	return nil
}

func StreamResponseToClaude(chunk []byte) []byte {
	geminiClaudeStreamMu.Lock()
	defer geminiClaudeStreamMu.Unlock()

	raw := normalizeGeminiStreamChunk(chunk)
	if len(raw) == 0 || string(raw) == "[DONE]" {
		return nil
	}

	root := gjson.ParseBytes(raw)
	key := root.Get("responseId").String()
	if key == "" {
		key = geminiClaudeActiveKey
	}
	if key == "" {
		key = "gemini-claude-stream"
	}
	geminiClaudeActiveKey = key

	state := geminiClaudeStreams[key]
	if state == nil {
		state = &geminiClaudeStreamState{ID: key, CurrentIndex: -1}
		geminiClaudeStreams[key] = state
	}
	if model := root.Get("modelVersion").String(); model != "" {
		state.Model = model
	}

	var out strings.Builder
	if !state.MessageSent {
		streamGeminiClaudeEmitEvent(&out, "message_start", map[string]any{
			"type": "message_start",
			"message": map[string]any{
				"id":            state.ID,
				"type":          "message",
				"role":          "assistant",
				"content":       []any{},
				"model":         state.Model,
				"stop_reason":   nil,
				"stop_sequence": nil,
				"usage": map[string]any{
					"input_tokens":  0,
					"output_tokens": 0,
				},
			},
		})
		state.MessageSent = true
	}

	for _, part := range root.Get("candidates.0.content.parts").Array() {
		if text := part.Get("text"); text.Exists() && text.String() != "" {
			blockType := "text"
			deltaType := "text_delta"
			fieldName := "text"
			if part.Get("thought").Bool() {
				blockType = "thinking"
				deltaType = "thinking_delta"
				fieldName = "thinking"
			}
			streamGeminiClaudeEnsureBlock(state, blockType, &out)
			streamGeminiClaudeEmitEvent(&out, "content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": state.CurrentIndex,
				"delta": map[string]any{
					"type":    deltaType,
					fieldName: text.String(),
				},
			})
			continue
		}
		if functionCall := part.Get("functionCall"); functionCall.Exists() {
			streamGeminiClaudeCloseCurrent(state, &out)
			toolIndex := state.NextIndex
			state.NextIndex++
			state.SawToolCall = true
			streamGeminiClaudeEmitEvent(&out, "content_block_start", map[string]any{
				"type":  "content_block_start",
				"index": toolIndex,
				"content_block": map[string]any{
					"type":  "tool_use",
					"id":    fmt.Sprintf("call_%d", atomic.AddUint64(&geminiOpenAICounter, 1)),
					"name":  functionCall.Get("name").String(),
					"input": map[string]any{},
				},
			})
			streamGeminiClaudeEmitEvent(&out, "content_block_delta", map[string]any{
				"type":  "content_block_delta",
				"index": toolIndex,
				"delta": map[string]any{
					"type":         "input_json_delta",
					"partial_json": rawJSONString(functionCall.Get("args")),
				},
			})
			streamGeminiClaudeEmitEvent(&out, "content_block_stop", map[string]any{"type": "content_block_stop", "index": toolIndex})
		}
	}

	finishReason := root.Get("candidates.0.finishReason").String()
	if finishReason != "" {
		streamGeminiClaudeCloseCurrent(state, &out)
		usage := map[string]any{
			"input_tokens":  root.Get("usageMetadata.promptTokenCount").Int(),
			"output_tokens": root.Get("usageMetadata.candidatesTokenCount").Int() + root.Get("usageMetadata.thoughtsTokenCount").Int(),
		}
		stopReason := mapGeminiFinishReasonToClaude(finishReason)
		if state.SawToolCall {
			stopReason = "tool_use"
		}
		streamGeminiClaudeEmitEvent(&out, "message_delta", map[string]any{
			"type": "message_delta",
			"delta": map[string]any{
				"stop_reason":   stopReason,
				"stop_sequence": nil,
			},
			"usage": usage,
		})
		streamGeminiClaudeEmitEvent(&out, "message_stop", map[string]any{"type": "message_stop"})
		delete(geminiClaudeStreams, key)
		if geminiClaudeActiveKey == key {
			geminiClaudeActiveKey = ""
		}
	}

	return []byte(out.String())
}

func normalizeGeminiStreamChunk(chunk []byte) []byte {
	raw := strings.TrimSpace(string(chunk))
	if strings.HasPrefix(raw, "data:") {
		raw = strings.TrimSpace(strings.TrimPrefix(raw, "data:"))
	}
	if raw == "" {
		return nil
	}
	return []byte(raw)
}

func streamGeminiClaudeEnsureBlock(state *geminiClaudeStreamState, blockType string, out *strings.Builder) {
	if state.CurrentType == blockType && state.CurrentIndex >= 0 {
		return
	}
	streamGeminiClaudeCloseCurrent(state, out)
	state.CurrentType = blockType
	state.CurrentIndex = state.NextIndex
	state.NextIndex++
	block := map[string]any{"type": blockType}
	if blockType == "thinking" {
		block["thinking"] = ""
	} else {
		block["text"] = ""
	}
	streamGeminiClaudeEmitEvent(out, "content_block_start", map[string]any{
		"type":          "content_block_start",
		"index":         state.CurrentIndex,
		"content_block": block,
	})
}

func streamGeminiClaudeCloseCurrent(state *geminiClaudeStreamState, out *strings.Builder) {
	if state.CurrentIndex < 0 {
		return
	}
	streamGeminiClaudeEmitEvent(out, "content_block_stop", map[string]any{"type": "content_block_stop", "index": state.CurrentIndex})
	state.CurrentIndex = -1
	state.CurrentType = ""
}

func streamGeminiClaudeEmitEvent(out *strings.Builder, name string, payload any) {
	out.WriteString("event: ")
	out.WriteString(name)
	out.WriteString("\n")
	out.WriteString("data: ")
	out.Write(mustJSON(payload))
	out.WriteString("\n\n")
}
