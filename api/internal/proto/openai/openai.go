// OpenAI 协议转换：将 OpenAI 请求/响应互转 Claude 和 Gemini 格式
package openai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

// RequestToGemini 将 OpenAI Chat Completions 请求转换为 Gemini generateContent 格式
func RequestToGemini(_ string, input []byte) []byte {
	root := gjson.ParseBytes(input)
	out := map[string]any{}
	contents := make([]any, 0)
	systemParts := make([]any, 0)
	toolNames := make(map[string]string)

	for _, message := range root.Get("messages").Array() {
		for _, tc := range message.Get("tool_calls").Array() {
			id := tc.Get("id").String()
			name := tc.Get("function.name").String()
			if id != "" && name != "" {
				toolNames[id] = name
			}
		}
	}

	for _, message := range root.Get("messages").Array() {
		role := message.Get("role").String()
		switch role {
		case "system":
			systemParts = append(systemParts, openAIContentToGeminiParts(message.Get("content"), true)...)
		case "user", "assistant":
			parts := openAIReasoningToGeminiParts(message.Get("reasoning_content"))
			parts = append(parts, openAIContentToGeminiParts(message.Get("content"), false)...)
			if role == "assistant" {
				for _, tc := range message.Get("tool_calls").Array() {
					parts = append(parts, map[string]any{
						"functionCall": map[string]any{
							"name": tc.Get("function.name").String(),
							"args": parseJSONObjectString(tc.Get("function.arguments").String()),
						},
					})
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
		case "tool":
			name := toolNames[message.Get("tool_call_id").String()]
			if name == "" {
				name = message.Get("tool_call_id").String()
			}
			if name == "" {
				name = "tool"
			}
			contents = append(contents, map[string]any{
				"role": "user",
				"parts": []any{map[string]any{
					"functionResponse": map[string]any{
						"name":     name,
						"response": map[string]any{"result": normalizeToolResponse(message.Get("content"))},
					},
				}},
			})
		}
	}

	if len(contents) == 0 {
		out["contents"] = []any{}
	} else {
		out["contents"] = contents
	}
	if len(systemParts) > 0 {
		out["system_instruction"] = map[string]any{"parts": systemParts}
	}

	generationConfig := map[string]any{}
	if value := root.Get("temperature"); value.Exists() {
		generationConfig["temperature"] = value.Float()
	}
	if value := root.Get("max_tokens"); value.Exists() {
		generationConfig["maxOutputTokens"] = value.Int()
	}
	if len(generationConfig) > 0 {
		out["generationConfig"] = generationConfig
	}

	if tools := openAIToolsToGemini(root.Get("tools")); len(tools) > 0 {
		out["tools"] = tools
	}

	return mustJSON(out)
}

// RequestToClaude 将 OpenAI Chat Completions 请求转换为 Claude Messages 格式
func RequestToClaude(modelName string, input []byte) []byte {
	root := gjson.ParseBytes(input)
	out := map[string]any{
		"model":      modelName,
		"max_tokens": int64(1024),
		"messages":   []any{},
	}
	if value := root.Get("max_tokens"); value.Exists() && value.Int() > 0 {
		out["max_tokens"] = value.Int()
	}
	if value := root.Get("temperature"); value.Exists() {
		out["temperature"] = value.Float()
	}
	if value := root.Get("stream"); value.Exists() {
		out["stream"] = value.Bool()
	}

	systemBlocks := make([]any, 0)
	messages := make([]any, 0)

	for _, message := range root.Get("messages").Array() {
		role := message.Get("role").String()
		switch role {
		case "system":
			systemBlocks = append(systemBlocks, openAIContentToClaudeBlocks(message.Get("content"))...)
		case "tool":
			messages = append(messages, map[string]any{
				"role": "user",
				"content": []any{map[string]any{
					"type":        "tool_result",
					"tool_use_id": message.Get("tool_call_id").String(),
					"content":     normalizeToolResponse(message.Get("content")),
				}},
			})
		case "user", "assistant":
			blocks := openAIReasoningToClaudeBlocks(message.Get("reasoning_content"))
			blocks = append(blocks, openAIContentToClaudeBlocks(message.Get("content"))...)
			if role == "assistant" {
				for _, tc := range message.Get("tool_calls").Array() {
					blocks = append(blocks, map[string]any{
						"type":  "tool_use",
						"id":    tc.Get("id").String(),
						"name":  tc.Get("function.name").String(),
						"input": parseJSONObjectString(tc.Get("function.arguments").String()),
					})
				}
			}
			messages = append(messages, map[string]any{
				"role":    role,
				"content": blocks,
			})
		}
	}

	if len(systemBlocks) == 1 {
		if text, ok := systemBlocks[0].(map[string]any); ok && text["type"] == "text" {
			out["system"] = text["text"]
		} else {
			out["system"] = systemBlocks
		}
	} else if len(systemBlocks) > 1 {
		out["system"] = systemBlocks
	}
	out["messages"] = messages

	if tools := openAIToolsToClaude(root.Get("tools")); len(tools) > 0 {
		out["tools"] = tools
	}
	if choice := root.Get("tool_choice"); choice.Exists() {
		out["tool_choice"] = mapOpenAIToolChoiceToClaude(choice)
	}

	return mustJSON(out)
}

// ResponseToClaude 将 OpenAI Chat Completions 响应转换为 Claude Messages 响应
func ResponseToClaude(input []byte) []byte {
	root := gjson.ParseBytes(input)
	choice := root.Get("choices.0")
	content := make([]any, 0)

	content = append(content, openAIReasoningToClaudeBlocks(choice.Get("message.reasoning_content"))...)
	content = append(content, openAIContentToClaudeBlocks(choice.Get("message.content"))...)
	if toolCalls := choice.Get("message.tool_calls"); toolCalls.IsArray() {
		for _, tc := range toolCalls.Array() {
			content = append(content, map[string]any{
				"type":  "tool_use",
				"id":    tc.Get("id").String(),
				"name":  tc.Get("function.name").String(),
				"input": parseJSONObjectString(tc.Get("function.arguments").String()),
			})
		}
	}

	finishReason := mapOpenAIFinishReasonToClaude(choice.Get("finish_reason").String())
	if choice.Get("message.tool_calls").IsArray() && len(choice.Get("message.tool_calls").Array()) > 0 {
		finishReason = "tool_use"
	}

	out := map[string]any{
		"id":          root.Get("id").String(),
		"type":        "message",
		"role":        "assistant",
		"model":       root.Get("model").String(),
		"content":     content,
		"stop_reason": finishReason,
		"usage": map[string]any{
			"input_tokens":  root.Get("usage.prompt_tokens").Int(),
			"output_tokens": root.Get("usage.completion_tokens").Int(),
		},
	}
	return mustJSON(out)
}

// ResponseToGemini 将 OpenAI Chat Completions 响应转换为 Gemini generateContent 响应
func ResponseToGemini(input []byte) []byte {
	root := gjson.ParseBytes(input)
	candidates := make([]any, 0)
	for _, choice := range root.Get("choices").Array() {
		parts := make([]any, 0)
		parts = append(parts, openAIReasoningToGeminiParts(choice.Get("message.reasoning_content"))...)
		parts = append(parts, openAIContentToGeminiParts(choice.Get("message.content"), false)...)
		if toolCalls := choice.Get("message.tool_calls"); toolCalls.IsArray() {
			for _, tc := range toolCalls.Array() {
				parts = append(parts, map[string]any{
					"functionCall": map[string]any{
						"name": tc.Get("function.name").String(),
						"args": parseJSONObjectString(tc.Get("function.arguments").String()),
					},
				})
			}
		}
		candidates = append(candidates, map[string]any{
			"index": choice.Get("index").Int(),
			"content": map[string]any{
				"role":  "model",
				"parts": parts,
			},
			"finishReason": mapOpenAIFinishReasonToGemini(choice.Get("finish_reason").String()),
		})
	}

	out := map[string]any{
		"candidates": candidates,
		"usageMetadata": map[string]any{
			"promptTokenCount":     root.Get("usage.prompt_tokens").Int(),
			"candidatesTokenCount": root.Get("usage.completion_tokens").Int(),
			"totalTokenCount":      root.Get("usage.total_tokens").Int(),
		},
	}
	if model := root.Get("model").String(); model != "" {
		out["modelVersion"] = model
	}
	return mustJSON(out)
}

// openAIContentToGeminiParts 将 OpenAI content 字段转换为 Gemini parts 数组
func openAIContentToGeminiParts(content gjson.Result, systemOnly bool) []any {
	parts := make([]any, 0)
	switch {
	case content.Type == gjson.String:
		text := content.String()
		if strings.TrimSpace(text) != "" {
			parts = append(parts, map[string]any{"text": text})
		}
	case content.IsArray():
		for _, item := range content.Array() {
			typeName := item.Get("type").String()
			switch typeName {
			case "text":
				text := item.Get("text").String()
				if strings.TrimSpace(text) != "" {
					parts = append(parts, map[string]any{"text": text})
				}
			case "image_url":
				if systemOnly {
					continue
				}
				mimeType, data, ok := parseDataURL(item.Get("image_url.url").String())
				if ok {
					parts = append(parts, map[string]any{
						"inlineData": map[string]any{
							"mimeType": mimeType,
							"data":     data,
						},
					})
				}
			}
		}
	case content.IsObject() && content.Get("type").String() == "text":
		text := content.Get("text").String()
		if strings.TrimSpace(text) != "" {
			parts = append(parts, map[string]any{"text": text})
		}
	}
	return parts
}

// openAIContentToClaudeBlocks 将 OpenAI content 转换为 Claude content blocks
func openAIContentToClaudeBlocks(content gjson.Result) []any {
	blocks := make([]any, 0)
	switch {
	case content.Type == gjson.String:
		text := content.String()
		if text != "" {
			blocks = append(blocks, map[string]any{"type": "text", "text": text})
		}
	case content.IsArray():
		for _, item := range content.Array() {
			switch item.Get("type").String() {
			case "text":
				text := item.Get("text").String()
				if text != "" {
					blocks = append(blocks, map[string]any{"type": "text", "text": text})
				}
			case "image_url":
				imageURL := item.Get("image_url.url").String()
				if imageURL == "" {
					continue
				}
				if mimeType, data, ok := parseDataURL(imageURL); ok {
					blocks = append(blocks, map[string]any{
						"type": "image",
						"source": map[string]any{
							"type":       "base64",
							"media_type": mimeType,
							"data":       data,
						},
					})
					continue
				}
				blocks = append(blocks, map[string]any{
					"type": "image",
					"source": map[string]any{
						"type": "url",
						"url":  imageURL,
					},
				})
			}
		}
	case content.IsObject() && content.Get("type").String() == "text":
		text := content.Get("text").String()
		if text != "" {
			blocks = append(blocks, map[string]any{"type": "text", "text": text})
		}
	}
	return blocks
}

// openAIReasoningToGeminiParts 将 reasoning_content 转换为 Gemini thought parts
func openAIReasoningToGeminiParts(reasoning gjson.Result) []any {
	parts := make([]any, 0)
	for _, text := range openAIReasoningTexts(reasoning) {
		parts = append(parts, map[string]any{"thought": true, "text": text})
	}
	return parts
}

// openAIReasoningToClaudeBlocks 将 reasoning_content 转换为 Claude thinking blocks
func openAIReasoningToClaudeBlocks(reasoning gjson.Result) []any {
	blocks := make([]any, 0)
	for _, text := range openAIReasoningTexts(reasoning) {
		blocks = append(blocks, map[string]any{"type": "thinking", "thinking": text})
	}
	return blocks
}

// openAIReasoningTexts 提取 reasoning_content 中所有文本细节
func openAIReasoningTexts(reasoning gjson.Result) []string {
	texts := make([]string, 0)
	if !reasoning.Exists() {
		return texts
	}
	if reasoning.IsArray() {
		for _, item := range reasoning.Array() {
			texts = append(texts, openAIReasoningTexts(item)...)
		}
		return texts
	}
	switch reasoning.Type {
	case gjson.String:
		if text := strings.TrimSpace(reasoning.String()); text != "" {
			texts = append(texts, text)
		}
	case gjson.JSON:
		if text := strings.TrimSpace(reasoning.Get("text").String()); text != "" {
			texts = append(texts, text)
		}
	}
	return texts
}

// openAIToolsToGemini 将 OpenAI tools 转换为 Gemini functionDeclarations
func openAIToolsToGemini(tools gjson.Result) []any {
	declarations := make([]any, 0)
	for _, tool := range tools.Array() {
		if tool.Get("type").String() != "function" {
			continue
		}
		declaration := map[string]any{
			"name":        tool.Get("function.name").String(),
			"description": tool.Get("function.description").String(),
		}
		if params := tool.Get("function.parameters"); params.Exists() {
			declaration["parameters"] = params.Value()
		}
		declarations = append(declarations, declaration)
	}
	if len(declarations) == 0 {
		return nil
	}
	return []any{map[string]any{"functionDeclarations": declarations}}
}

// openAIToolsToClaude 将 OpenAI tools 转换为 Claude tools 格式
func openAIToolsToClaude(tools gjson.Result) []any {
	result := make([]any, 0)
	for _, tool := range tools.Array() {
		if tool.Get("type").String() != "function" {
			continue
		}
		entry := map[string]any{
			"name":        tool.Get("function.name").String(),
			"description": tool.Get("function.description").String(),
		}
		if params := tool.Get("function.parameters"); params.Exists() {
			entry["input_schema"] = params.Value()
		}
		result = append(result, entry)
	}
	return result
}

// mapOpenAIToolChoiceToClaude 将 OpenAI tool_choice 映射到 Claude 格式
func mapOpenAIToolChoiceToClaude(choice gjson.Result) any {
	if choice.Type == gjson.String {
		switch choice.String() {
		case "auto":
			return map[string]any{"type": "auto"}
		case "required":
			return map[string]any{"type": "any"}
		case "none":
			return nil
		}
	}
	if choice.IsObject() && choice.Get("type").String() == "function" {
		return map[string]any{
			"type": "tool",
			"name": choice.Get("function.name").String(),
		}
	}
	return nil
}

// mapOpenAIFinishReasonToClaude 将 OpenAI finish_reason 映射到 Claude stop_reason
func mapOpenAIFinishReasonToClaude(reason string) string {
	switch reason {
	case "stop", "content_filter", "":
		return "end_turn"
	case "length":
		return "max_tokens"
	case "tool_calls", "function_call":
		return "tool_use"
	default:
		return "end_turn"
	}
}

// mapOpenAIFinishReasonToGemini 将 OpenAI finish_reason 映射到 Gemini finishReason
func mapOpenAIFinishReasonToGemini(reason string) string {
	switch reason {
	case "length":
		return "MAX_TOKENS"
	case "stop", "tool_calls", "function_call", "", "content_filter":
		return "STOP"
	default:
		return "STOP"
	}
}

// parseJSONObjectString 将字符串形式的 JSON 对象解析为 map
func parseJSONObjectString(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

// normalizeToolResponse 将工具返回内容标准化为字符串或原始对象
func normalizeToolResponse(content gjson.Result) any {
	if !content.Exists() {
		return ""
	}
	if content.Type == gjson.String {
		return content.String()
	}
	return content.Value()
}

// parseDataURL 解析 data:// URL，返回 MIME 类型、base64 数据和是否成功
func parseDataURL(raw string) (string, string, bool) {
	if !strings.HasPrefix(raw, "data:") {
		return "", "", false
	}
	payload := strings.TrimPrefix(raw, "data:")
	parts := strings.SplitN(payload, ",", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	meta := parts[0]
	data := parts[1]
	metaParts := strings.Split(meta, ";")
	mimeType := metaParts[0]
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return mimeType, data, true
}

// mustJSON 将任意对象序列化为 JSON，失败时返回错误 JSON
func mustJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		return []byte(fmt.Sprintf(`{"error":"%s"}`, err.Error()))
	}
	return data
}
