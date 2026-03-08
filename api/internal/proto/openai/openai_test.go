package openai_test

import (
	"testing"

	"github.com/Mag1cFall/ai-router/api/internal/proto/openai"
	"github.com/tidwall/gjson"
)

// assertTextContent 接受纯字符串或 content parts 数组，两种形式均合法
func assertTextContent(t *testing.T, val gjson.Result, want string) {
	t.Helper()
	if val.Type == gjson.String {
		if val.String() != want {
			t.Errorf("content string: got %q, want %q", val.String(), want)
		}
		return
	}
	if val.IsArray() {
		for _, part := range val.Array() {
			if part.Get("text").String() == want {
				return
			}
		}
		t.Errorf("content parts: no part with text=%q in %s", want, val.Raw)
		return
	}
	t.Errorf("content: unexpected type %v, raw=%s", val.Type, val.Raw)
}

// ===== Request: OpenAI → Gemini =====

func TestOpenAIToGemini_BasicMessage(t *testing.T) {
	in := []byte(`{"model":"gemini-2.5-pro","messages":[{"role":"user","content":"hello"}]}`)
	out := openai.RequestToGemini("gemini-2.5-pro", in)

	if !gjson.GetBytes(out, "contents").IsArray() {
		t.Fatal("expected contents array")
	}
	if gjson.GetBytes(out, "contents.0.role").String() != "user" {
		t.Error("expected contents[0].role = user")
	}
	if gjson.GetBytes(out, "contents.0.parts.0.text").String() != "hello" {
		t.Error("expected parts[0].text = hello")
	}
}

func TestOpenAIToGemini_SystemMessage(t *testing.T) {
	in := []byte(`{"model":"gemini-2.5-pro","messages":[
		{"role":"system","content":"You are helpful"},
		{"role":"user","content":"hi"}
	]}`)
	out := openai.RequestToGemini("gemini-2.5-pro", in)

	sysText := gjson.GetBytes(out, "system_instruction.parts.0.text").String()
	if sysText != "You are helpful" {
		t.Errorf("expected system_instruction text, got %q", sysText)
	}
	if gjson.GetBytes(out, "contents.0.role").String() != "user" {
		t.Error("expected first contents entry is user turn, not system")
	}
}

func TestOpenAIToGemini_AssistantMessage(t *testing.T) {
	in := []byte(`{"model":"x","messages":[
		{"role":"user","content":"hello"},
		{"role":"assistant","content":"world"}
	]}`)
	out := openai.RequestToGemini("x", in)

	if gjson.GetBytes(out, "contents.1.role").String() != "model" {
		t.Error("expected assistant → model role")
	}
}

func TestOpenAIToGemini_ToolCall(t *testing.T) {
	in := []byte(`{"model":"x","messages":[
		{"role":"assistant","tool_calls":[
			{"id":"tc1","type":"function","function":{"name":"search","arguments":"{\"q\":\"go\"}"}}
		]}
	]}`)
	out := openai.RequestToGemini("x", in)

	if gjson.GetBytes(out, "contents.0.parts.0.functionCall.name").String() != "search" {
		t.Error("expected functionCall.name = search")
	}
	args := gjson.GetBytes(out, "contents.0.parts.0.functionCall.args")
	if !args.IsObject() {
		t.Error("expected functionCall.args to be an object")
	}
}

func TestOpenAIToGemini_Temperature(t *testing.T) {
	in := []byte(`{"model":"x","temperature":0.7,"messages":[]}`)
	out := openai.RequestToGemini("x", in)

	if gjson.GetBytes(out, "generationConfig.temperature").Float() != 0.7 {
		t.Error("expected temperature passed through")
	}
}

func TestOpenAIToGemini_MixedContent(t *testing.T) {
	in := []byte(`{"model":"x","messages":[
		{"role":"user","content":[
			{"type":"text","text":"describe this"},
			{"type":"image_url","image_url":{"url":"data:image/png;base64,abc123"}}
		]}
	]}`)
	out := openai.RequestToGemini("x", in)

	parts := gjson.GetBytes(out, "contents.0.parts").Array()
	if len(parts) < 2 {
		t.Errorf("expected at least 2 parts (text + inlineData), got %d", len(parts))
	}
}

// ===== Request: OpenAI → Claude =====

func TestOpenAIToClaude_BasicMessage(t *testing.T) {
	in := []byte(`{"model":"claude-opus-4","messages":[{"role":"user","content":"hello"}],"max_tokens":1024}`)
	out := openai.RequestToClaude("claude-opus-4", in)

	if gjson.GetBytes(out, "model").String() != "claude-opus-4" {
		t.Error("expected model passed through")
	}
	assertTextContent(t, gjson.GetBytes(out, "messages.0.content"), "hello")
}

func TestOpenAIToClaude_MaxTokensRequired(t *testing.T) {
	in := []byte(`{"model":"claude-opus-4","messages":[{"role":"user","content":"hi"}],"max_tokens":512}`)
	out := openai.RequestToClaude("claude-opus-4", in)

	if gjson.GetBytes(out, "max_tokens").Int() != 512 {
		t.Error("expected max_tokens=512 (Claude requires max_tokens)")
	}
}

func TestOpenAIToClaude_MaxTokensDefault(t *testing.T) {
	in := []byte(`{"model":"claude-opus-4","messages":[{"role":"user","content":"hi"}]}`)
	out := openai.RequestToClaude("claude-opus-4", in)

	if gjson.GetBytes(out, "max_tokens").Int() <= 0 {
		t.Error("expected max_tokens to be set with a default when not provided (Claude requires it)")
	}
}

func TestOpenAIToClaude_SystemExtract(t *testing.T) {
	in := []byte(`{"model":"x","messages":[
		{"role":"system","content":"be helpful"},
		{"role":"user","content":"hi"}
	]}`)
	out := openai.RequestToClaude("x", in)

	sys := gjson.GetBytes(out, "system")
	if !sys.Exists() {
		t.Fatal("expected system field")
	}
	assertTextContent(t, sys, "be helpful")

	if gjson.GetBytes(out, "messages.0.role").String() != "user" {
		t.Error("expected messages[0] is user (system stripped from messages)")
	}
}

func TestOpenAIToClaude_ToolCall(t *testing.T) {
	in := []byte(`{"model":"x","messages":[
		{"role":"assistant","content":"","tool_calls":[
			{"id":"tc1","type":"function","function":{"name":"search","arguments":"{\"q\":\"go\"}"}}
		]}
	]}`)
	out := openai.RequestToClaude("x", in)

	content := gjson.GetBytes(out, "messages.0.content")
	if !content.IsArray() {
		t.Fatal("expected content array")
	}
	found := false
	for _, part := range content.Array() {
		if part.Get("type").String() == "tool_use" {
			found = true
			if part.Get("id").String() != "tc1" {
				t.Error("expected tool_use id = tc1")
			}
			if part.Get("name").String() != "search" {
				t.Error("expected tool_use name = search")
			}
			input := part.Get("input")
			if !input.IsObject() {
				t.Error("expected tool_use input to be object")
			}
		}
	}
	if !found {
		t.Error("expected tool_use content block")
	}
}

// ===== Response: OpenAI non-stream → Claude =====

func TestOpenAIResponseToClaude_NonStream(t *testing.T) {
	resp := []byte(`{
		"id":"msg-1","model":"claude-opus-4",
		"choices":[{"index":0,"message":{"role":"assistant","content":"hello world"},"finish_reason":"stop"}],
		"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}
	}`)
	out := openai.ResponseToClaude(resp)

	if gjson.GetBytes(out, "type").String() != "message" {
		t.Error("expected type=message")
	}
	if gjson.GetBytes(out, "role").String() != "assistant" {
		t.Error("expected role=assistant")
	}
	if gjson.GetBytes(out, "content.0.type").String() != "text" {
		t.Error("expected content[0].type=text")
	}
	if gjson.GetBytes(out, "content.0.text").String() != "hello world" {
		t.Error("expected content[0].text=hello world")
	}
	if gjson.GetBytes(out, "stop_reason").String() != "end_turn" {
		t.Errorf("expected stop_reason=end_turn, got %q", gjson.GetBytes(out, "stop_reason").String())
	}
	if gjson.GetBytes(out, "usage.output_tokens").Int() != 5 {
		t.Error("expected output_tokens=5")
	}
}

func TestOpenAIResponseToClaude_FinishReasonMapping(t *testing.T) {
	cases := []struct{ openai, claude string }{
		{"stop", "end_turn"},
		{"length", "max_tokens"},
		{"tool_calls", "tool_use"},
		{"content_filter", "end_turn"},
		{"unknown_reason", "end_turn"},
	}
	for _, c := range cases {
		resp := []byte(`{"id":"x","choices":[{"message":{"role":"assistant","content":""},"finish_reason":"` + c.openai + `"}],"usage":{}}`)
		out := openai.ResponseToClaude(resp)
		got := gjson.GetBytes(out, "stop_reason").String()
		if got != c.claude {
			t.Errorf("finish_reason %q → stop_reason: got %q, want %q", c.openai, got, c.claude)
		}
	}
}

func TestOpenAIResponseToClaude_ToolUse(t *testing.T) {
	resp := []byte(`{
		"id":"x","choices":[{
			"message":{"role":"assistant","content":"",
				"tool_calls":[{"id":"tc1","type":"function","function":{"name":"search","arguments":"{\"q\":\"go\"}"}}]
			},
			"finish_reason":"tool_calls"
		}],
		"usage":{}
	}`)
	out := openai.ResponseToClaude(resp)

	if gjson.GetBytes(out, "stop_reason").String() != "tool_use" {
		t.Error("expected stop_reason=tool_use")
	}
	if gjson.GetBytes(out, "content.0.type").String() != "tool_use" {
		t.Error("expected content[0].type=tool_use")
	}
	if gjson.GetBytes(out, "content.0.name").String() != "search" {
		t.Error("expected content[0].name=search")
	}
	input := gjson.GetBytes(out, "content.0.input")
	if !input.IsObject() {
		t.Error("expected content[0].input to be an object")
	}
}

func TestOpenAIResponseToClaude_MultipleToolCalls(t *testing.T) {
	resp := []byte(`{
		"id":"x","choices":[{
			"message":{"role":"assistant","content":"","tool_calls":[
				{"id":"tc1","type":"function","function":{"name":"search","arguments":"{}"}},
				{"id":"tc2","type":"function","function":{"name":"lookup","arguments":"{\"id\":1}"}}
			]},
			"finish_reason":"tool_calls"
		}],
		"usage":{}
	}`)
	out := openai.ResponseToClaude(resp)

	content := gjson.GetBytes(out, "content").Array()
	if len(content) != 2 {
		t.Errorf("expected 2 tool_use blocks, got %d", len(content))
	}
}

// ===== Response: OpenAI non-stream → Gemini =====

func TestOpenAIResponseToGemini_NonStream(t *testing.T) {
	resp := []byte(`{
		"id":"x","model":"gemini-2.5-pro",
		"choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],
		"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}
	}`)
	out := openai.ResponseToGemini(resp)

	if !gjson.GetBytes(out, "candidates").IsArray() {
		t.Fatal("expected candidates array")
	}
	if gjson.GetBytes(out, "candidates.0.content.parts.0.text").String() != "hello" {
		t.Error("expected candidates[0].content.parts[0].text=hello")
	}
	if gjson.GetBytes(out, "candidates.0.finishReason").String() != "STOP" {
		t.Errorf("expected finishReason=STOP, got %q", gjson.GetBytes(out, "candidates.0.finishReason").String())
	}
	if gjson.GetBytes(out, "usageMetadata.promptTokenCount").Int() != 10 {
		t.Error("expected promptTokenCount=10")
	}
}

func TestOpenAIResponseToGemini_FinishReasonMapping(t *testing.T) {
	cases := []struct{ openai, gemini string }{
		{"stop", "STOP"},
		{"length", "MAX_TOKENS"},
		{"tool_calls", "STOP"},
	}
	for _, c := range cases {
		resp := []byte(`{"id":"x","choices":[{"index":0,"message":{"role":"assistant","content":""},"finish_reason":"` + c.openai + `"}],"usage":{}}`)
		out := openai.ResponseToGemini(resp)
		got := gjson.GetBytes(out, "candidates.0.finishReason").String()
		if got != c.gemini {
			t.Errorf("finish_reason %q → finishReason: got %q, want %q", c.openai, got, c.gemini)
		}
	}
}
