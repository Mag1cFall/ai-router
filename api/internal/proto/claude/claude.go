package claude

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

func RequestToOpenAI(modelName string, input []byte) []byte {
	root := gjson.ParseBytes(input)
	out := map[string]any{
		"model":    modelName,
		"messages": []any{},
	}
	if value := root.Get("max_tokens"); value.Exists() {
		out["max_tokens"] = value.Int()
	}
	if value := root.Get("temperature"); value.Exists() {
		out["temperature"] = value.Float()
	}
	if value := root.Get("stream"); value.Exists() {
		out["stream"] = value.Bool()
	}

	messages := make([]any, 0)
	if system := claudeSystemToOpenAIMessage(root.Get("system")); system != nil {
		messages = append(messages, system)
	}

	for _, message := range root.Get("messages").Array() {
		role := message.Get("role").String()
		content := message.Get("content")
		if content.Type == gjson.String {
			messages = append(messages, map[string]any{"role": role, "content": content.String()})
			continue
		}

		textParts := make([]any, 0)
		texts := make([]string, 0)
		reasoning := make([]string, 0)
		toolCalls := make([]any, 0)
		toolResults := make([]any, 0)

		for _, part := range content.Array() {
			switch part.Get("type").String() {
			case "text":
				text := part.Get("text").String()
				if text == "" {
					continue
				}
				texts = append(texts, text)
				textParts = append(textParts, map[string]any{"type": "text", "text": text})
			case "image":
				if image := claudeImageToOpenAIContent(part); image != nil {
					textParts = append(textParts, image)
				}
			case "thinking":
				if role == "assistant" {
					thinking := strings.TrimSpace(part.Get("thinking").String())
					if thinking != "" {
						reasoning = append(reasoning, thinking)
					}
				}
			case "tool_use":
				if role == "assistant" {
					toolCalls = append(toolCalls, map[string]any{
						"id":   part.Get("id").String(),
						"type": "function",
						"function": map[string]any{
							"name":      part.Get("name").String(),
							"arguments": rawJSONString(part.Get("input")),
						},
					})
				}
			case "tool_result":
				toolResults = append(toolResults, map[string]any{
					"role":         "tool",
					"tool_call_id": part.Get("tool_use_id").String(),
					"content":      claudeToolResultToOpenAIContent(part.Get("content")),
				})
			}
		}

		if role == "assistant" {
			msg := map[string]any{"role": "assistant"}
			if len(textParts) == 0 {
				msg["content"] = ""
			} else if onlyTextParts(textParts) {
				msg["content"] = strings.Join(texts, "\n")
			} else {
				msg["content"] = textParts
			}
			if len(reasoning) > 0 {
				msg["reasoning_content"] = strings.Join(reasoning, "\n\n")
			}
			if len(toolCalls) > 0 {
				msg["tool_calls"] = toolCalls
			}
			messages = append(messages, msg)
		} else {
			if len(textParts) > 0 {
				msg := map[string]any{"role": role}
				if onlyTextParts(textParts) {
					msg["content"] = strings.Join(texts, "\n")
				} else {
					msg["content"] = textParts
				}
				messages = append(messages, msg)
			}
			messages = append(messages, toolResults...)
		}
	}

	out["messages"] = messages
	if tools := claudeToolsToOpenAI(root.Get("tools")); len(tools) > 0 {
		out["tools"] = tools
	}
	return mustJSON(out)
}

func RequestToGemini(modelName string, input []byte) []byte {
	root := gjson.ParseBytes(input)
	out := map[string]any{}
	contents := make([]any, 0)
	toolNames := make(map[string]string)

	if systemParts := claudeSystemToGeminiParts(root.Get("system")); len(systemParts) > 0 {
		out["system_instruction"] = map[string]any{"parts": systemParts}
	}
	if value := root.Get("max_tokens"); value.Exists() {
		out["generationConfig"] = map[string]any{"maxOutputTokens": value.Int()}
	}

	for _, message := range root.Get("messages").Array() {
		role := message.Get("role").String()
		parts := make([]any, 0)
		content := message.Get("content")
		if content.Type == gjson.String {
			text := content.String()
			if text != "" {
				parts = append(parts, map[string]any{"text": text})
			}
		}
		for _, part := range content.Array() {
			switch part.Get("type").String() {
			case "text":
				text := part.Get("text").String()
				if text != "" {
					parts = append(parts, map[string]any{"text": text})
				}
			case "tool_use":
				if role == "assistant" {
					name := part.Get("name").String()
					toolNames[part.Get("id").String()] = name
					parts = append(parts, map[string]any{
						"functionCall": map[string]any{
							"name": name,
							"args": part.Get("input").Value(),
						},
					})
				}
			case "tool_result":
				if role == "user" {
					name := toolNames[part.Get("tool_use_id").String()]
					if name == "" {
						name = part.Get("tool_use_id").String()
					}
					parts = append(parts, map[string]any{
						"functionResponse": map[string]any{
							"name":     name,
							"response": map[string]any{"result": claudeToolResultToOpenAIContent(part.Get("content"))},
						},
					})
				}
			}
		}
		if len(parts) == 0 {
			continue
		}
		geminiRole := "user"
		if role == "assistant" {
			geminiRole = "model"
		}
		contents = append(contents, map[string]any{
			"role":  geminiRole,
			"parts": parts,
		})
	}

	out["contents"] = contents
	_ = modelName
	return mustJSON(out)
}

func ResponseToOpenAI(input []byte) []byte {
	root := gjson.ParseBytes(input)
	texts := make([]string, 0)
	reasoning := make([]string, 0)
	toolCalls := make([]any, 0)
	for _, part := range root.Get("content").Array() {
		switch part.Get("type").String() {
		case "text":
			texts = append(texts, part.Get("text").String())
		case "thinking":
			if thinking := strings.TrimSpace(part.Get("thinking").String()); thinking != "" {
				reasoning = append(reasoning, thinking)
			}
		case "tool_use":
			toolCalls = append(toolCalls, map[string]any{
				"id":   part.Get("id").String(),
				"type": "function",
				"function": map[string]any{
					"name":      part.Get("name").String(),
					"arguments": rawJSONString(part.Get("input")),
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
	if len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
	}

	finishReason := mapClaudeStopReasonToOpenAI(root.Get("stop_reason").String())
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
	}

	out := map[string]any{
		"id":    root.Get("id").String(),
		"model": root.Get("model").String(),
		"choices": []any{map[string]any{
			"index":         0,
			"message":       message,
			"finish_reason": finishReason,
		}},
		"usage": map[string]any{
			"prompt_tokens":     root.Get("usage.input_tokens").Int(),
			"completion_tokens": root.Get("usage.output_tokens").Int(),
			"total_tokens":      root.Get("usage.input_tokens").Int() + root.Get("usage.output_tokens").Int(),
		},
	}
	return mustJSON(out)
}

func claudeSystemToOpenAIMessage(system gjson.Result) any {
	if !system.Exists() {
		return nil
	}
	if system.Type == gjson.String {
		return map[string]any{"role": "system", "content": system.String()}
	}
	if system.IsArray() {
		content := make([]any, 0)
		for _, part := range system.Array() {
			if part.Get("type").String() == "text" {
				content = append(content, map[string]any{"type": "text", "text": part.Get("text").String()})
			}
		}
		if len(content) > 0 {
			return map[string]any{"role": "system", "content": content}
		}
	}
	return nil
}

func claudeSystemToGeminiParts(system gjson.Result) []any {
	parts := make([]any, 0)
	if !system.Exists() {
		return parts
	}
	if system.Type == gjson.String {
		if system.String() != "" {
			parts = append(parts, map[string]any{"text": system.String()})
		}
		return parts
	}
	if system.IsArray() {
		for _, part := range system.Array() {
			if part.Get("type").String() == "text" {
				parts = append(parts, map[string]any{"text": part.Get("text").String()})
			}
		}
	}
	return parts
}

func claudeImageToOpenAIContent(part gjson.Result) any {
	source := part.Get("source")
	sourceType := source.Get("type").String()
	switch sourceType {
	case "base64":
		mimeType := source.Get("media_type").String()
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		data := source.Get("data").String()
		if data == "" {
			return nil
		}
		return map[string]any{
			"type": "image_url",
			"image_url": map[string]any{
				"url": fmt.Sprintf("data:%s;base64,%s", mimeType, data),
			},
		}
	case "url":
		url := source.Get("url").String()
		if url == "" {
			return nil
		}
		return map[string]any{
			"type": "image_url",
			"image_url": map[string]any{
				"url": url,
			},
		}
	default:
		return nil
	}
}

func claudeToolsToOpenAI(tools gjson.Result) []any {
	result := make([]any, 0)
	for _, tool := range tools.Array() {
		entry := map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        tool.Get("name").String(),
				"description": tool.Get("description").String(),
			},
		}
		if schema := tool.Get("input_schema"); schema.Exists() {
			entry["function"].(map[string]any)["parameters"] = schema.Value()
		}
		result = append(result, entry)
	}
	return result
}

func claudeToolResultToOpenAIContent(content gjson.Result) any {
	if !content.Exists() {
		return ""
	}
	if content.Type == gjson.String {
		return content.String()
	}
	if content.IsArray() {
		texts := make([]string, 0)
		for _, part := range content.Array() {
			if part.Type == gjson.String {
				texts = append(texts, part.String())
				continue
			}
			if part.Get("type").String() == "text" {
				texts = append(texts, part.Get("text").String())
			}
		}
		if len(texts) > 0 {
			return strings.Join(texts, "\n")
		}
	}
	return content.Value()
}

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

func rawJSONString(value gjson.Result) string {
	if !value.Exists() {
		return "{}"
	}
	if strings.TrimSpace(value.Raw) == "" {
		return "{}"
	}
	return value.Raw
}

func mapClaudeStopReasonToOpenAI(reason string) string {
	switch reason {
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	case "end_turn", "stop_sequence", "", "unknown_value":
		return "stop"
	default:
		return "stop"
	}
}

func mustJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		return []byte(fmt.Sprintf(`{"error":"%s"}`, err.Error()))
	}
	return data
}
