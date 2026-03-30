// Claude 流式响应转换：将 Claude SSE 事件转换为 OpenAI 流式 chunk 格式
package claude

import (
	"strings"
	"sync"
	"time"

	"github.com/tidwall/gjson"
)

// claudeOpenAIStreamState 跟踪单个 Claude->OpenAI 流的转换状态
type claudeOpenAIStreamState struct {
	ID               string
	Model            string
	Created          int64
	ToolCallsByIndex map[int]*claudeOpenAIToolState
}

// claudeOpenAIToolState 跟踪单个 tool_use block 的参数积累
type claudeOpenAIToolState struct {
	ID   string
	Name string
	Args strings.Builder
}

var (
	claudeOpenAIStreamMu sync.Mutex
	claudeOpenAIState    *claudeOpenAIStreamState
)

// StreamResponseToOpenAI 将 Claude SSE 事件转换为 OpenAI chat.completion.chunk
func StreamResponseToOpenAI(event []byte) []byte {
	claudeOpenAIStreamMu.Lock()
	defer claudeOpenAIStreamMu.Unlock()

	eventName, raw := parseClaudeStreamEvent(event)
	if eventName == "" || len(raw) == 0 {
		return nil
	}

	root := gjson.ParseBytes(raw)
	switch eventName {
	case "message_start":
		message := root.Get("message")
		claudeOpenAIState = &claudeOpenAIStreamState{
			ID:               message.Get("id").String(),
			Model:            message.Get("model").String(),
			Created:          time.Now().Unix(),
			ToolCallsByIndex: map[int]*claudeOpenAIToolState{},
		}
		return claudeOpenAIChunk(claudeOpenAIState, map[string]any{"role": "assistant"}, nil, nil)
	case "content_block_start":
		if claudeOpenAIState == nil {
			claudeOpenAIState = &claudeOpenAIStreamState{Created: time.Now().Unix(), ToolCallsByIndex: map[int]*claudeOpenAIToolState{}}
		}
		block := root.Get("content_block")
		if block.Get("type").String() == "tool_use" {
			index := int(root.Get("index").Int())
			claudeOpenAIState.ToolCallsByIndex[index] = &claudeOpenAIToolState{
				ID:   block.Get("id").String(),
				Name: block.Get("name").String(),
			}
		}
		return nil
	case "content_block_delta":
		delta := root.Get("delta")
		switch delta.Get("type").String() {
		case "text_delta":
			if claudeOpenAIState == nil {
				claudeOpenAIState = &claudeOpenAIStreamState{Created: time.Now().Unix(), ToolCallsByIndex: map[int]*claudeOpenAIToolState{}}
			}
			return claudeOpenAIChunk(claudeOpenAIState, map[string]any{"content": delta.Get("text").String()}, nil, nil)
		case "thinking_delta":
			if claudeOpenAIState == nil {
				claudeOpenAIState = &claudeOpenAIStreamState{Created: time.Now().Unix(), ToolCallsByIndex: map[int]*claudeOpenAIToolState{}}
			}
			return claudeOpenAIChunk(claudeOpenAIState, map[string]any{"reasoning_content": delta.Get("thinking").String()}, nil, nil)
		case "input_json_delta":
			if claudeOpenAIState == nil {
				return nil
			}
			index := int(root.Get("index").Int())
			toolState := claudeOpenAIState.ToolCallsByIndex[index]
			if toolState == nil {
				toolState = &claudeOpenAIToolState{}
				claudeOpenAIState.ToolCallsByIndex[index] = toolState
			}
			toolState.Args.WriteString(delta.Get("partial_json").String())
			return nil
		}
	case "content_block_stop":
		if claudeOpenAIState == nil {
			return nil
		}
		index := int(root.Get("index").Int())
		toolState := claudeOpenAIState.ToolCallsByIndex[index]
		if toolState == nil {
			return nil
		}
		arguments := toolState.Args.String()
		if strings.TrimSpace(arguments) == "" {
			arguments = "{}"
		}
		delete(claudeOpenAIState.ToolCallsByIndex, index)
		return claudeOpenAIChunk(claudeOpenAIState, map[string]any{
			"tool_calls": []any{map[string]any{
				"index": index,
				"id":    toolState.ID,
				"type":  "function",
				"function": map[string]any{
					"name":      toolState.Name,
					"arguments": arguments,
				},
			}},
		}, nil, nil)
	case "message_delta":
		if claudeOpenAIState == nil {
			claudeOpenAIState = &claudeOpenAIStreamState{Created: time.Now().Unix(), ToolCallsByIndex: map[int]*claudeOpenAIToolState{}}
		}
		finishReason := map[string]any{"finish_reason": mapClaudeStopReasonToOpenAI(root.Get("delta.stop_reason").String())}
		usage := map[string]any{}
		if usageNode := root.Get("usage"); usageNode.Exists() {
			usage = map[string]any{
				"prompt_tokens":     usageNode.Get("input_tokens").Int(),
				"completion_tokens": usageNode.Get("output_tokens").Int(),
				"total_tokens":      usageNode.Get("input_tokens").Int() + usageNode.Get("output_tokens").Int(),
			}
		}
		return claudeOpenAIChunk(claudeOpenAIState, map[string]any{}, finishReason, usage)
	case "message_stop":
		claudeOpenAIState = nil
		return nil
	case "ping":
		return nil
	}

	return nil
}

// parseClaudeStreamEvent 解析 Claude SSE 行序，返回事件名和 data 内容
func parseClaudeStreamEvent(event []byte) (string, []byte) {
	raw := strings.TrimSpace(string(event))
	if raw == "" {
		return "", nil
	}
	lines := strings.Split(raw, "\n")
	eventName := ""
	dataLines := make([]string, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "event:"):
			eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if eventName == "" && len(dataLines) > 0 {
		payload := strings.Join(dataLines, "\n")
		return gjson.Get(payload, "type").String(), []byte(payload)
	}
	if len(dataLines) == 0 {
		return "", nil
	}
	return eventName, []byte(strings.Join(dataLines, "\n"))
}

// claudeOpenAIChunk 构建一个 OpenAI SSE chunk 并写入 "data: " 前缀
func claudeOpenAIChunk(state *claudeOpenAIStreamState, delta map[string]any, finish map[string]any, usage map[string]any) []byte {
	if state == nil {
		state = &claudeOpenAIStreamState{Created: time.Now().Unix()}
	}
	payload := map[string]any{
		"id":      state.ID,
		"object":  "chat.completion.chunk",
		"created": state.Created,
		"model":   state.Model,
		"choices": []any{map[string]any{
			"index":         0,
			"delta":         delta,
			"finish_reason": nil,
		}},
	}
	if finish != nil {
		payload["choices"].([]any)[0].(map[string]any)["finish_reason"] = finish["finish_reason"]
	}
	if len(usage) > 0 {
		payload["usage"] = usage
	}
	return append([]byte("data: "), append(mustJSON(payload), []byte("\n\n")...)...)
}
