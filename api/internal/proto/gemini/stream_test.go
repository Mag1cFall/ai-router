package gemini_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Mag1cFall/ai-router/api/internal/proto/gemini"
	"github.com/tidwall/gjson"
)

func geminiOpenAIPayload(raw []byte) []byte {
	text := strings.TrimSpace(string(raw))
	text = strings.TrimPrefix(text, "data: ")
	text = strings.TrimSpace(text)
	return []byte(text)
}

func TestGeminiStreamResponseToOpenAI_TextReasoningAndTool(t *testing.T) {
	textChunk := []byte(`{"responseId":"resp-stream-text","modelVersion":"gemini-2.5-pro","candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"hello"}]}}]}`)
	textOut := gemini.StreamResponseToOpenAI(textChunk)
	if gjson.GetBytes(geminiOpenAIPayload(textOut), "choices.0.delta.content").String() != "hello" {
		t.Fatalf("expected content chunk, got %s", textOut)
	}

	thinkingChunk := []byte(`{"responseId":"resp-stream-think","modelVersion":"gemini-2.5-pro","candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"先分析","thought":true}]}}]}`)
	thinkingOut := gemini.StreamResponseToOpenAI(thinkingChunk)
	if gjson.GetBytes(geminiOpenAIPayload(thinkingOut), "choices.0.delta.reasoning_content").String() != "先分析" {
		t.Fatalf("expected reasoning_content chunk, got %s", thinkingOut)
	}

	toolChunk := []byte(`{"responseId":"resp-stream-tool","modelVersion":"gemini-2.5-pro","candidates":[{"index":0,"content":{"role":"model","parts":[{"functionCall":{"name":"search","args":{"q":"go"}}}]},"finishReason":"STOP"}]}`)
	toolOut := gemini.StreamResponseToOpenAI(toolChunk)
	if gjson.GetBytes(geminiOpenAIPayload(toolOut), "choices.0.delta.tool_calls.0.function.name").String() != "search" {
		t.Fatalf("expected tool call chunk, got %s", toolOut)
	}
	if gjson.GetBytes(geminiOpenAIPayload(toolOut), "choices.0.finish_reason").String() != "tool_calls" {
		t.Fatalf("expected tool_calls finish_reason, got %s", toolOut)
	}
}

func TestGeminiStreamResponseToClaude_TextAndThinking(t *testing.T) {
	textChunk := []byte(`{"responseId":"resp-claude-text","modelVersion":"gemini-2.5-pro","candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"hello"}]}}]}`)
	textOut := gemini.StreamResponseToClaude(textChunk)
	if !bytes.Contains(textOut, []byte("event: message_start")) || !bytes.Contains(textOut, []byte(`"type":"text_delta"`)) {
		t.Fatalf("expected message_start and text_delta, got %s", textOut)
	}

	thinkingChunk := []byte(`{"responseId":"resp-claude-think","modelVersion":"gemini-2.5-pro","candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"先分析","thought":true}]}}]}`)
	thinkingOut := gemini.StreamResponseToClaude(thinkingChunk)
	if !bytes.Contains(thinkingOut, []byte(`"type":"thinking_delta"`)) || !bytes.Contains(thinkingOut, []byte(`"thinking":"先分析"`)) {
		t.Fatalf("expected thinking_delta, got %s", thinkingOut)
	}
}

func TestGeminiStreamResponseToClaude_ToolAndFinish(t *testing.T) {
	toolChunk := []byte(`{"responseId":"resp-claude-tool","modelVersion":"gemini-2.5-pro","candidates":[{"index":0,"content":{"role":"model","parts":[{"functionCall":{"name":"search","args":{"q":"go"}}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":5}}`)
	out := gemini.StreamResponseToClaude(toolChunk)
	if !bytes.Contains(out, []byte(`"type":"tool_use"`)) || !bytes.Contains(out, []byte(`"name":"search"`)) {
		t.Fatalf("expected tool_use block, got %s", out)
	}
	if !bytes.Contains(out, []byte(`"stop_reason":"tool_use"`)) {
		t.Fatalf("expected tool_use stop_reason, got %s", out)
	}
	if !bytes.Contains(out, []byte("event: message_stop")) {
		t.Fatalf("expected message_stop on terminal chunk, got %s", out)
	}
}

func TestGeminiNonStreamRequestToOpenAI_ThoughtToReasoningContent(t *testing.T) {
	in := []byte(`{"contents":[{"role":"model","parts":[{"text":"先分析","thought":true},{"text":"答案"}]}]}`)
	out := gemini.RequestToOpenAI("gpt-5.4", in)

	if gjson.GetBytes(out, "messages.0.reasoning_content").String() != "先分析" {
		t.Fatalf("expected reasoning_content, got %s", out)
	}
	if gjson.GetBytes(out, "messages.0.content").String() != "答案" {
		t.Fatalf("expected answer content, got %s", out)
	}
}

func TestGeminiNonStreamRequestToClaude_ThoughtToThinking(t *testing.T) {
	in := []byte(`{"contents":[{"role":"model","parts":[{"text":"先分析","thought":true},{"text":"答案"}]}]}`)
	out := gemini.RequestToClaude("claude-sonnet-4-6", in)

	content := gjson.GetBytes(out, "messages.0.content").Array()
	if len(content) < 2 {
		t.Fatalf("expected thinking and text blocks, got %s", out)
	}
	if content[0].Get("type").String() != "thinking" || content[0].Get("thinking").String() != "先分析" {
		t.Fatalf("expected first block thinking, got %s", out)
	}
	if content[1].Get("type").String() != "text" || content[1].Get("text").String() != "答案" {
		t.Fatalf("expected second block text, got %s", out)
	}
}

func TestGeminiNonStreamResponseToClaude_ThoughtToThinking(t *testing.T) {
	resp := []byte(`{"candidates":[{"content":{"role":"model","parts":[{"text":"先分析","thought":true},{"text":"答案"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":2}}`)
	out := gemini.ResponseToClaude(resp)

	content := gjson.GetBytes(out, "content").Array()
	if len(content) < 2 {
		t.Fatalf("expected thinking and text blocks, got %s", out)
	}
	if content[0].Get("type").String() != "thinking" || content[0].Get("thinking").String() != "先分析" {
		t.Fatalf("expected thinking block, got %s", out)
	}
	if content[1].Get("type").String() != "text" || content[1].Get("text").String() != "答案" {
		t.Fatalf("expected text block, got %s", out)
	}
}
