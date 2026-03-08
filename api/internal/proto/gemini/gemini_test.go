package gemini_test

import (
	"testing"

	"github.com/Mag1cFall/ai-router/api/internal/proto/gemini"
	"github.com/tidwall/gjson"
)

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

// ===== Request: Gemini → OpenAI =====

func TestGeminiToOpenAI_BasicMessage(t *testing.T) {
	in := []byte(`{
		"contents":[{"role":"user","parts":[{"text":"hello"}]}]
	}`)
	out := gemini.RequestToOpenAI("gpt-5.4", in)

	if gjson.GetBytes(out, "messages.0.role").String() != "user" {
		t.Error("expected messages[0].role=user")
	}
	assertTextContent(t, gjson.GetBytes(out, "messages.0.content"), "hello")
}

func TestGeminiToOpenAI_SystemInstruction(t *testing.T) {
	in := []byte(`{
		"system_instruction":{"role":"user","parts":[{"text":"Be helpful"}]},
		"contents":[{"role":"user","parts":[{"text":"hi"}]}]
	}`)
	out := gemini.RequestToOpenAI("gpt-5.4", in)

	msgs := gjson.GetBytes(out, "messages").Array()
	if len(msgs) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(msgs))
	}
	if msgs[0].Get("role").String() != "system" {
		t.Error("expected first message to be system")
	}
	assertTextContent(t, msgs[0].Get("content"), "Be helpful")
}

func TestGeminiToOpenAI_ModelRole(t *testing.T) {
	in := []byte(`{
		"contents":[
			{"role":"user","parts":[{"text":"hello"}]},
			{"role":"model","parts":[{"text":"world"}]}
		]
	}`)
	out := gemini.RequestToOpenAI("gpt-5.4", in)

	if gjson.GetBytes(out, "messages.1.role").String() != "assistant" {
		t.Error("expected model → assistant")
	}
}

func TestGeminiToOpenAI_GenerationConfig(t *testing.T) {
	in := []byte(`{
		"generationConfig":{"temperature":0.5,"maxOutputTokens":1024},
		"contents":[{"role":"user","parts":[{"text":"hi"}]}]
	}`)
	out := gemini.RequestToOpenAI("gpt-5.4", in)

	if gjson.GetBytes(out, "temperature").Float() != 0.5 {
		t.Error("expected temperature=0.5")
	}
	if gjson.GetBytes(out, "max_tokens").Int() != 1024 {
		t.Error("expected max_tokens=1024")
	}
}

func TestGeminiToOpenAI_FunctionCall(t *testing.T) {
	in := []byte(`{
		"contents":[
			{"role":"model","parts":[{"functionCall":{"name":"search","args":{"q":"go"}}}]}
		]
	}`)
	out := gemini.RequestToOpenAI("gpt-5.4", in)

	tc := gjson.GetBytes(out, "messages.0.tool_calls.0")
	if tc.Get("function.name").String() != "search" {
		t.Error("expected function.name=search")
	}
}

func TestGeminiToOpenAI_FunctionCallAndResponse(t *testing.T) {
	in := []byte(`{
		"contents":[
			{"role":"model","parts":[{"functionCall":{"name":"search","args":{"q":"go"}}}]},
			{"role":"user","parts":[{"functionResponse":{"name":"search","response":{"result":"Go is great"}}}]}
		]
	}`)
	out := gemini.RequestToOpenAI("gpt-5.4", in)

	msgs := gjson.GetBytes(out, "messages").Array()
	foundAssistant := false
	foundTool := false
	for _, m := range msgs {
		if m.Get("role").String() == "assistant" && m.Get("tool_calls").IsArray() {
			foundAssistant = true
		}
		if m.Get("role").String() == "tool" {
			foundTool = true
		}
	}
	if !foundAssistant {
		t.Error("expected assistant message with tool_calls from functionCall")
	}
	if !foundTool {
		t.Error("expected tool message from functionResponse")
	}
}

func TestGeminiToOpenAI_MultiParts(t *testing.T) {
	in := []byte(`{
		"contents":[{"role":"user","parts":[
			{"text":"first"},
			{"text":"second"}
		]}]
	}`)
	out := gemini.RequestToOpenAI("gpt-5.4", in)

	content := gjson.GetBytes(out, "messages.0.content")
	if content.Type == gjson.String {
		if content.String() != "first\nsecond" && content.String() != "first second" && content.String() != "firstsecond" {
			combined := content.String()
			if len(combined) < 5 {
				t.Errorf("expected multi-part text concatenation, got %q", combined)
			}
		}
	} else if content.IsArray() {
		if len(content.Array()) < 2 {
			t.Error("expected at least 2 text parts in content array")
		}
	}
}

// ===== Request: Gemini → Claude =====

func TestGeminiToClaude_BasicMessage(t *testing.T) {
	in := []byte(`{
		"contents":[{"role":"user","parts":[{"text":"hello"}]}]
	}`)
	out := gemini.RequestToClaude("claude-sonnet-4-6", in)

	if gjson.GetBytes(out, "messages.0.role").String() != "user" {
		t.Error("expected messages[0].role=user")
	}
	if gjson.GetBytes(out, "messages.0.content.0.text").String() != "hello" {
		t.Error("expected content[0].text=hello")
	}
}

func TestGeminiToClaude_SystemInstruction(t *testing.T) {
	in := []byte(`{
		"system_instruction":{"parts":[{"text":"Be concise"}]},
		"contents":[{"role":"user","parts":[{"text":"hi"}]}]
	}`)
	out := gemini.RequestToClaude("claude-sonnet-4-6", in)

	sys := gjson.GetBytes(out, "system")
	if !sys.Exists() {
		t.Fatal("expected system field")
	}
	assertTextContent(t, sys, "Be concise")
}

func TestGeminiToClaude_MaxOutputTokens(t *testing.T) {
	in := []byte(`{
		"generationConfig":{"maxOutputTokens":2048},
		"contents":[{"role":"user","parts":[{"text":"hi"}]}]
	}`)
	out := gemini.RequestToClaude("claude-sonnet-4-6", in)

	if gjson.GetBytes(out, "max_tokens").Int() != 2048 {
		t.Error("expected max_tokens=2048")
	}
}

// ===== Response: Gemini → OpenAI (non-stream) =====

func TestGeminiResponseToOpenAI_NonStream(t *testing.T) {
	resp := []byte(`{
		"candidates":[{
			"index":0,
			"content":{"role":"model","parts":[{"text":"hello world"}]},
			"finishReason":"STOP"
		}],
		"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5,"totalTokenCount":15},
		"modelVersion":"gemini-3-pro-preview"
	}`)
	out := gemini.ResponseToOpenAI(resp)

	if gjson.GetBytes(out, "choices.0.message.role").String() != "assistant" {
		t.Error("expected role=assistant")
	}
	if gjson.GetBytes(out, "choices.0.message.content").String() != "hello world" {
		t.Error("expected content=hello world")
	}
	if gjson.GetBytes(out, "choices.0.finish_reason").String() != "stop" {
		t.Error("expected finish_reason=stop")
	}
	if gjson.GetBytes(out, "usage.prompt_tokens").Int() != 10 {
		t.Error("expected prompt_tokens=10")
	}
}

func TestGeminiResponseToOpenAI_FinishReasonMapping(t *testing.T) {
	cases := []struct{ gemini, openai string }{
		{"STOP", "stop"},
		{"MAX_TOKENS", "length"},
		{"SAFETY", "content_filter"},
	}
	for _, c := range cases {
		resp := []byte(`{"candidates":[{"index":0,"content":{"role":"model","parts":[{"text":""}]},"finishReason":"` + c.gemini + `"}],"usageMetadata":{}}`)
		out := gemini.ResponseToOpenAI(resp)
		got := gjson.GetBytes(out, "choices.0.finish_reason").String()
		if got != c.openai {
			t.Errorf("finishReason %q → finish_reason: got %q, want %q", c.gemini, got, c.openai)
		}
	}
}

func TestGeminiResponseToOpenAI_FunctionCall(t *testing.T) {
	resp := []byte(`{
		"candidates":[{
			"content":{"role":"model","parts":[{"functionCall":{"name":"search","args":{"q":"go"}}}]},
			"finishReason":"STOP"
		}],
		"usageMetadata":{}
	}`)
	out := gemini.ResponseToOpenAI(resp)

	tc := gjson.GetBytes(out, "choices.0.message.tool_calls.0")
	if tc.Get("function.name").String() != "search" {
		t.Error("expected function.name=search")
	}
	if gjson.GetBytes(out, "choices.0.finish_reason").String() != "tool_calls" {
		t.Error("expected finish_reason=tool_calls")
	}
}

func TestGeminiResponseToOpenAI_MultipleCandidates(t *testing.T) {
	resp := []byte(`{
		"candidates":[
			{"index":0,"content":{"role":"model","parts":[{"text":"choice A"}]},"finishReason":"STOP"},
			{"index":1,"content":{"role":"model","parts":[{"text":"choice B"}]},"finishReason":"STOP"}
		],
		"usageMetadata":{}
	}`)
	out := gemini.ResponseToOpenAI(resp)

	choices := gjson.GetBytes(out, "choices").Array()
	if len(choices) != 2 {
		t.Errorf("expected 2 choices, got %d", len(choices))
	}
	if choices[0].Get("index").Int() != 0 || choices[1].Get("index").Int() != 1 {
		t.Error("expected index 0 and 1 in choices")
	}
}

// ===== Response: Gemini → Claude (non-stream) =====

func TestGeminiResponseToClaude_NonStream(t *testing.T) {
	resp := []byte(`{
		"candidates":[{
			"content":{"role":"model","parts":[{"text":"hello world"}]},
			"finishReason":"STOP"
		}],
		"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5}
	}`)
	out := gemini.ResponseToClaude(resp)

	if gjson.GetBytes(out, "type").String() != "message" {
		t.Error("expected type=message")
	}
	if gjson.GetBytes(out, "content.0.type").String() != "text" {
		t.Error("expected content[0].type=text")
	}
	if gjson.GetBytes(out, "content.0.text").String() != "hello world" {
		t.Error("expected content[0].text=hello world")
	}
	if gjson.GetBytes(out, "stop_reason").String() != "end_turn" {
		t.Error("expected stop_reason=end_turn")
	}
	if gjson.GetBytes(out, "usage.input_tokens").Int() != 10 {
		t.Error("expected input_tokens=10")
	}
	if gjson.GetBytes(out, "usage.output_tokens").Int() != 5 {
		t.Error("expected output_tokens=5")
	}
}

func TestGeminiResponseToClaude_FinishReasonMapping(t *testing.T) {
	cases := []struct{ gemini, claude string }{
		{"STOP", "end_turn"},
		{"MAX_TOKENS", "max_tokens"},
		{"SAFETY", "end_turn"},
	}
	for _, c := range cases {
		resp := []byte(`{"candidates":[{"content":{"role":"model","parts":[{"text":""}]},"finishReason":"` + c.gemini + `"}],"usageMetadata":{}}`)
		out := gemini.ResponseToClaude(resp)
		got := gjson.GetBytes(out, "stop_reason").String()
		if got != c.claude {
			t.Errorf("finishReason %q → stop_reason: got %q, want %q", c.gemini, got, c.claude)
		}
	}
}
