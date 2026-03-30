// Gemini 协议转换：将 Gemini 请求/响应互转 OpenAI 和 Claude 格式
package gemini

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

// RequestToOpenAI 将 Gemini generateContent 请求转换为 OpenAI Chat Completions 格式
func RequestToOpenAI(modelName string, input []byte) []byte {
	root := gjson.ParseBytes(input)
	out := map[string]any{
		"model":    modelName,
		"messages": []any{},
	}
	if value := root.Get("generationConfig.temperature"); value.Exists() {
		out["temperature"] = value.Float()
	}
	if value := root.Get("generationConfig.maxOutputTokens"); value.Exists() {
		out["max_tokens"] = value.Int()
	}

	messages := make([]any, 0)
	if system := geminiSystemToOpenAI(root.Get("system_instruction")); system != nil {
		messages = append(messages, system)
	}

	nameQueues := make(map[string][]string)
	callCounter := 0

	for _, content := range root.Get("contents").Array() {
		role := content.Get("role").String()
		textParts := make([]any, 0)
		texts := make([]string, 0)
		reasoning := make([]string, 0)
		toolCalls := make([]any, 0)
		toolMessages := make([]any, 0)

		for _, part := range content.Get("parts").Array() {
			if text := part.Get("text"); text.Exists() {
				if role == "model" && part.Get("thought").Bool() {
					reasoning = append(reasoning, text.String())
				} else {
					texts = append(texts, text.String())
					textParts = append(textParts, map[string]any{"type": "text", "text": text.String()})
				}
				continue
			}
			if inlineData := part.Get("inlineData"); inlineData.Exists() || part.Get("inline_data").Exists() {
				dataNode := inlineData
				if !dataNode.Exists() {
					dataNode = part.Get("inline_data")
				}
				mimeType := dataNode.Get("mimeType").String()
				if mimeType == "" {
					mimeType = dataNode.Get("mime_type").String()
				}
				data := dataNode.Get("data").String()
				if data != "" {
					textParts = append(textParts, map[string]any{
						"type": "image_url",
						"image_url": map[string]any{
							"url": fmt.Sprintf("data:%s;base64,%s", defaultString(mimeType, "application/octet-stream"), data),
						},
					})
				}
				continue
			}
			if call := part.Get("functionCall"); call.Exists() {
				callCounter++
				id := fmt.Sprintf("call_%d", callCounter)
				name := call.Get("name").String()
				nameQueues[name] = append(nameQueues[name], id)
				toolCalls = append(toolCalls, map[string]any{
					"id":   id,
					"type": "function",
					"function": map[string]any{
						"name":      name,
						"arguments": rawJSONString(call.Get("args")),
					},
				})
				continue
			}
			if resp := part.Get("functionResponse"); resp.Exists() {
				name := resp.Get("name").String()
				toolCallID := nextCallID(nameQueues, name)
				toolMessages = append(toolMessages, map[string]any{
					"role":         "tool",
					"tool_call_id": toolCallID,
					"content":      geminiFunctionResponseContent(resp.Get("response")),
				})
			}
		}

		switch role {
		case "user":
			if len(textParts) > 0 {
				msg := map[string]any{"role": "user"}
				if onlyTextParts(textParts) {
					msg["content"] = strings.Join(texts, "\n")
				} else {
					msg["content"] = textParts
				}
				messages = append(messages, msg)
			}
			messages = append(messages, toolMessages...)
		case "model":
			msg := map[string]any{"role": "assistant"}
			if len(textParts) == 0 {
				msg["content"] = ""
			} else if onlyTextParts(textParts) {
				msg["content"] = strings.Join(texts, "\n")
			} else {
				msg["content"] = textParts
			}
			if len(toolCalls) > 0 {
				msg["tool_calls"] = toolCalls
			}
			if len(reasoning) > 0 {
				msg["reasoning_content"] = strings.Join(reasoning, "")
			}
			messages = append(messages, msg)
		}
	}

	out["messages"] = messages
	return mustJSON(out)
}

// RequestToClaude 将 Gemini generateContent 请求转换为 Claude Messages 格式
func RequestToClaude(modelName string, input []byte) []byte {
	root := gjson.ParseBytes(input)
	out := map[string]any{
		"model":    modelName,
		"messages": []any{},
	}
	if value := root.Get("generationConfig.maxOutputTokens"); value.Exists() {
		out["max_tokens"] = value.Int()
	}
	if system := geminiSystemToClaude(root.Get("system_instruction")); system != nil {
		out["system"] = system
	}

	messages := make([]any, 0)
	nameQueues := make(map[string][]string)
	callCounter := 0

	for _, content := range root.Get("contents").Array() {
		role := content.Get("role").String()
		blocks := make([]any, 0)
		for _, part := range content.Get("parts").Array() {
			if text := part.Get("text"); text.Exists() {
				if role == "model" && part.Get("thought").Bool() {
					blocks = append(blocks, map[string]any{"type": "thinking", "thinking": text.String()})
				} else {
					blocks = append(blocks, map[string]any{"type": "text", "text": text.String()})
				}
				continue
			}
			if call := part.Get("functionCall"); call.Exists() {
				callCounter++
				id := fmt.Sprintf("call_%d", callCounter)
				name := call.Get("name").String()
				nameQueues[name] = append(nameQueues[name], id)
				blocks = append(blocks, map[string]any{
					"type":  "tool_use",
					"id":    id,
					"name":  name,
					"input": call.Get("args").Value(),
				})
				continue
			}
			if resp := part.Get("functionResponse"); resp.Exists() {
				name := resp.Get("name").String()
				blocks = append(blocks, map[string]any{
					"type":        "tool_result",
					"tool_use_id": nextCallID(nameQueues, name),
					"content":     geminiFunctionResponseContent(resp.Get("response")),
				})
			}
		}
		if len(blocks) == 0 {
			continue
		}
		claudeRole := "user"
		if role == "model" {
			claudeRole = "assistant"
		}
		messages = append(messages, map[string]any{
			"role":    claudeRole,
			"content": blocks,
		})
	}

	out["messages"] = messages
	return mustJSON(out)
}

// ResponseToOpenAI 将 Gemini generateContent 响应转换为 OpenAI Chat Completions 响应
func ResponseToOpenAI(input []byte) []byte {
	root := gjson.ParseBytes(input)
	choices := make([]any, 0)
	for idx, candidate := range root.Get("candidates").Array() {
		texts := make([]string, 0)
		reasoning := make([]string, 0)
		toolCalls := make([]any, 0)
		for _, part := range candidate.Get("content.parts").Array() {
			if text := part.Get("text"); text.Exists() {
				if part.Get("thought").Bool() {
					reasoning = append(reasoning, text.String())
				} else {
					texts = append(texts, text.String())
				}
				continue
			}
			if call := part.Get("functionCall"); call.Exists() {
				toolCalls = append(toolCalls, map[string]any{
					"id":   fmt.Sprintf("call_%d_%d", idx, len(toolCalls)+1),
					"type": "function",
					"function": map[string]any{
						"name":      call.Get("name").String(),
						"arguments": rawJSONString(call.Get("args")),
					},
				})
			}
		}

		message := map[string]any{
			"role":    "assistant",
			"content": strings.Join(texts, ""),
		}
		if len(reasoning) > 0 {
			message["reasoning_content"] = strings.Join(reasoning, "")
		}
		finishReason := mapGeminiFinishReasonToOpenAI(candidate.Get("finishReason").String())
		if len(toolCalls) > 0 {
			message["tool_calls"] = toolCalls
			finishReason = "tool_calls"
		}
		choiceIndex := candidate.Get("index").Int()
		if !candidate.Get("index").Exists() {
			choiceIndex = int64(idx)
		}
		choices = append(choices, map[string]any{
			"index":         choiceIndex,
			"message":       message,
			"finish_reason": finishReason,
		})
	}

	out := map[string]any{
		"model":   root.Get("modelVersion").String(),
		"choices": choices,
		"usage": map[string]any{
			"prompt_tokens":     root.Get("usageMetadata.promptTokenCount").Int(),
			"completion_tokens": root.Get("usageMetadata.candidatesTokenCount").Int(),
			"total_tokens":      root.Get("usageMetadata.totalTokenCount").Int(),
		},
	}
	return mustJSON(out)
}

// ResponseToClaude 将 Gemini generateContent 响应转换为 Claude Messages 响应
func ResponseToClaude(input []byte) []byte {
	root := gjson.ParseBytes(input)
	candidate := root.Get("candidates.0")
	content := make([]any, 0)
	for _, part := range candidate.Get("content.parts").Array() {
		if text := part.Get("text"); text.Exists() {
			if part.Get("thought").Bool() {
				content = append(content, map[string]any{"type": "thinking", "thinking": text.String()})
			} else {
				content = append(content, map[string]any{"type": "text", "text": text.String()})
			}
			continue
		}
		if call := part.Get("functionCall"); call.Exists() {
			content = append(content, map[string]any{
				"type":  "tool_use",
				"id":    fmt.Sprintf("call_%s", call.Get("name").String()),
				"name":  call.Get("name").String(),
				"input": call.Get("args").Value(),
			})
		}
	}

	out := map[string]any{
		"type":        "message",
		"role":        "assistant",
		"content":     content,
		"stop_reason": mapGeminiFinishReasonToClaude(candidate.Get("finishReason").String()),
		"usage": map[string]any{
			"input_tokens":  root.Get("usageMetadata.promptTokenCount").Int(),
			"output_tokens": root.Get("usageMetadata.candidatesTokenCount").Int(),
		},
	}
	if model := root.Get("modelVersion").String(); model != "" {
		out["model"] = model
	}
	return mustJSON(out)
}

// geminiSystemToOpenAI 将 Gemini system_instruction 转换为 OpenAI system 消息
func geminiSystemToOpenAI(system gjson.Result) any {
	parts := geminiSystemTextParts(system)
	if len(parts) == 0 {
		return nil
	}
	if len(parts) == 1 {
		if part, ok := parts[0].(map[string]any); ok {
			return map[string]any{"role": "system", "content": part["text"]}
		}
	}
	return map[string]any{"role": "system", "content": parts}
}

// geminiSystemToClaude 将 Gemini system_instruction 转换为 Claude system 字段
func geminiSystemToClaude(system gjson.Result) any {
	parts := geminiSystemTextParts(system)
	if len(parts) == 0 {
		return nil
	}
	if len(parts) == 1 {
		if part, ok := parts[0].(map[string]any); ok {
			return part["text"]
		}
	}
	blocks := make([]any, 0, len(parts))
	for _, part := range parts {
		if block, ok := part.(map[string]any); ok {
			blocks = append(blocks, map[string]any{"type": "text", "text": block["text"]})
		}
	}
	return blocks
}

// geminiSystemTextParts 提取 Gemini system_instruction 中所有文本 parts
func geminiSystemTextParts(system gjson.Result) []any {
	parts := make([]any, 0)
	for _, part := range system.Get("parts").Array() {
		if text := part.Get("text"); text.Exists() {
			parts = append(parts, map[string]any{"type": "text", "text": text.String()})
		}
	}
	return parts
}

// geminiFunctionResponseContent 提取工具返回内容，优先取 result 字段
func geminiFunctionResponseContent(response gjson.Result) any {
	if !response.Exists() {
		return ""
	}
	if result := response.Get("result"); result.Exists() {
		return result.Value()
	}
	return response.Value()
}

// nextCallID 按函数名 FIFO 消费调用 ID，用于 Gemini functionResponse 匹配
func nextCallID(queues map[string][]string, name string) string {
	list := queues[name]
	if len(list) == 0 {
		return fmt.Sprintf("call_%s", name)
	}
	id := list[0]
	queues[name] = list[1:]
	return id
}

// rawJSONString 将 gjson.Result 转为原始 JSON，空时返回 {}
func rawJSONString(value gjson.Result) string {
	if !value.Exists() || strings.TrimSpace(value.Raw) == "" {
		return "{}"
	}
	return value.Raw
}

// mapGeminiFinishReasonToOpenAI 将 Gemini finishReason 映射到 OpenAI finish_reason
func mapGeminiFinishReasonToOpenAI(reason string) string {
	switch strings.ToUpper(reason) {
	case "MAX_TOKENS":
		return "length"
	case "SAFETY":
		return "content_filter"
	case "STOP", "":
		return "stop"
	default:
		return "stop"
	}
}

// mapGeminiFinishReasonToClaude 将 Gemini finishReason 映射到 Claude stop_reason
func mapGeminiFinishReasonToClaude(reason string) string {
	switch strings.ToUpper(reason) {
	case "MAX_TOKENS":
		return "max_tokens"
	case "STOP", "SAFETY", "":
		return "end_turn"
	default:
		return "end_turn"
	}
}

// onlyTextParts 判断 parts 列表是否全部为文本类型
func onlyTextParts(parts []any) bool {
	for _, part := range parts {
		item, ok := part.(map[string]any)
		if !ok {
			return false
		}
		if item["type"] != "text" {
			return false
		}
	}
	return true
}

// defaultString 如果 value 为空返回 fallback
func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func mustJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		return []byte(fmt.Sprintf(`{"error":"%s"}`, err.Error()))
	}
	return data
}
