package openai_test

import (
	"bytes"
	"testing"

	"github.com/Mag1cFall/ai-router/api/internal/proto/openai"
	"github.com/tidwall/gjson"
)

func TestOpenAIStreamResponseToClaude_TextChunk(t *testing.T) {
	chunk := []byte("data: {\"id\":\"chatcmpl-openai-text\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"gpt-5.4\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"hello\"},\"finish_reason\":null}]}")
	out := openai.StreamResponseToClaude(chunk)

	if !bytes.Contains(out, []byte("event: message_start")) {
		t.Fatalf("expected message_start event, got %s", out)
	}
	if !bytes.Contains(out, []byte(`"type":"text_delta"`)) || !bytes.Contains(out, []byte(`"text":"hello"`)) {
		t.Fatalf("expected text_delta hello, got %s", out)
	}

	_ = openai.StreamResponseToClaude([]byte("data: [DONE]"))
}

func TestOpenAIStreamResponseToClaude_ReasoningChunk(t *testing.T) {
	chunk := []byte("data: {\"id\":\"chatcmpl-openai-think\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"gpt-5.4\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"reasoning_content\":\"先分析\"},\"finish_reason\":null}]}")
	out := openai.StreamResponseToClaude(chunk)

	if !bytes.Contains(out, []byte(`"type":"thinking_delta"`)) || !bytes.Contains(out, []byte(`"thinking":"先分析"`)) {
		t.Fatalf("expected thinking_delta, got %s", out)
	}

	_ = openai.StreamResponseToClaude([]byte("data: [DONE]"))
}

func TestOpenAIStreamResponseToClaude_ToolCallAndDone(t *testing.T) {
	chunk := []byte("data: {\"id\":\"chatcmpl-openai-tool\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"gpt-5.4\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"search\",\"arguments\":\"{\\\"q\\\":\\\"go\\\"}\"}}]},\"finish_reason\":\"tool_calls\"}],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":5,\"total_tokens\":8}}")
	out := openai.StreamResponseToClaude(chunk)

	if !bytes.Contains(out, []byte(`"type":"tool_use"`)) || !bytes.Contains(out, []byte(`"name":"search"`)) {
		t.Fatalf("expected tool_use block, got %s", out)
	}
	if !bytes.Contains(out, []byte(`"partial_json":"{\"q\":\"go\"}"`)) {
		t.Fatalf("expected input_json_delta, got %s", out)
	}
	if !bytes.Contains(out, []byte(`"stop_reason":"tool_use"`)) {
		t.Fatalf("expected tool_use stop_reason, got %s", out)
	}

	done := openai.StreamResponseToClaude([]byte("data: [DONE]"))
	if !bytes.Contains(done, []byte("event: message_stop")) {
		t.Fatalf("expected message_stop on done, got %s", done)
	}
}

func TestOpenAIStreamResponseToGemini_ContentReasoningAndTool(t *testing.T) {
	textChunk := []byte("data: {\"id\":\"chatcmpl-openai-gemini-text\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"gpt-5.4\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"hello\"},\"finish_reason\":null}]}")
	textOut := openai.StreamResponseToGemini(textChunk)
	if gjson.GetBytes(textOut, "candidates.0.content.parts.0.text").String() != "hello" {
		t.Fatalf("expected hello text fragment, got %s", textOut)
	}

	thinkingChunk := []byte("data: {\"id\":\"chatcmpl-openai-gemini-think\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"gpt-5.4\",\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"先想想\"},\"finish_reason\":null}]}")
	thinkingOut := openai.StreamResponseToGemini(thinkingChunk)
	if !gjson.GetBytes(thinkingOut, "candidates.0.content.parts.0.thought").Bool() || gjson.GetBytes(thinkingOut, "candidates.0.content.parts.0.text").String() != "先想想" {
		t.Fatalf("expected thought fragment, got %s", thinkingOut)
	}

	toolChunk := []byte("data: {\"id\":\"chatcmpl-openai-gemini-tool\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"gpt-5.4\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"search\",\"arguments\":\"{\\\"q\\\":\\\"go\\\"}\"}}]},\"finish_reason\":\"tool_calls\"}]}")
	toolOut := openai.StreamResponseToGemini(toolChunk)
	if gjson.GetBytes(toolOut, "candidates.0.content.parts.0.functionCall.name").String() != "search" {
		t.Fatalf("expected functionCall search, got %s", toolOut)
	}
	if gjson.GetBytes(toolOut, "candidates.0.finishReason").String() != "STOP" {
		t.Fatalf("expected STOP finishReason, got %s", toolOut)
	}
}

func TestOpenAINonStreamRequestToClaude_ReasoningContent(t *testing.T) {
	in := []byte(`{"model":"claude-sonnet-4-6","messages":[{"role":"assistant","reasoning_content":"先分析","content":"答案"}],"max_tokens":1024}`)
	out := openai.RequestToClaude("claude-sonnet-4-6", in)

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

func TestOpenAINonStreamRequestToGemini_ReasoningContent(t *testing.T) {
	in := []byte(`{"model":"gemini-2.5-pro","messages":[{"role":"assistant","reasoning_content":"先分析","content":"答案"}]}`)
	out := openai.RequestToGemini("gemini-2.5-pro", in)

	parts := gjson.GetBytes(out, "contents.0.parts").Array()
	if len(parts) < 2 {
		t.Fatalf("expected thought and text parts, got %s", out)
	}
	if !parts[0].Get("thought").Bool() || parts[0].Get("text").String() != "先分析" {
		t.Fatalf("expected first part thought text, got %s", out)
	}
	if parts[1].Get("text").String() != "答案" {
		t.Fatalf("expected second part answer text, got %s", out)
	}
}

func TestOpenAINonStreamResponseToClaude_ReasoningContent(t *testing.T) {
	resp := []byte(`{"id":"x","model":"claude-opus-4","choices":[{"index":0,"message":{"role":"assistant","reasoning_content":"先分析","content":"答案"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`)
	out := openai.ResponseToClaude(resp)

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

func TestOpenAINonStreamResponseToGemini_ReasoningContent(t *testing.T) {
	resp := []byte(`{"id":"x","model":"gemini-2.5-pro","choices":[{"index":0,"message":{"role":"assistant","reasoning_content":"先分析","content":"答案"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`)
	out := openai.ResponseToGemini(resp)

	parts := gjson.GetBytes(out, "candidates.0.content.parts").Array()
	if len(parts) < 2 {
		t.Fatalf("expected thought and text parts, got %s", out)
	}
	if !parts[0].Get("thought").Bool() || parts[0].Get("text").String() != "先分析" {
		t.Fatalf("expected thought part, got %s", out)
	}
	if parts[1].Get("text").String() != "答案" {
		t.Fatalf("expected answer text part, got %s", out)
	}
}
