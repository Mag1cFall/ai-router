package claude_test

import (
	"testing"

	"github.com/Mag1cFall/ai-router/api/internal/proto/claude"
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
			if part.Get("type").String() == "text" && part.Get("text").String() == want {
				return
			}
		}
		t.Errorf("content parts: no text part with %q in %s", want, val.Raw)
		return
	}
	t.Errorf("content: unexpected type %v, raw=%s", val.Type, val.Raw)
}

// ===== Request: Claude → OpenAI =====

func TestClaudeToOpenAI_BasicMessage(t *testing.T) {
	in := []byte(`{
		"model":"claude-sonnet-4-6",
		"max_tokens":1024,
		"messages":[{"role":"user","content":"hello"}]
	}`)
	out := claude.RequestToOpenAI("gpt-5.4", in)

	if gjson.GetBytes(out, "model").String() != "gpt-5.4" {
		t.Error("expected model overridden to gpt-5.4")
	}
	if gjson.GetBytes(out, "messages.0.role").String() != "user" {
		t.Error("expected messages[0].role=user")
	}
}

func TestClaudeToOpenAI_SystemField(t *testing.T) {
	in := []byte(`{
		"model":"x",
		"system":"Be concise.",
		"max_tokens":1024,
		"messages":[{"role":"user","content":"hi"}]
	}`)
	out := claude.RequestToOpenAI("gpt-5.4", in)

	if gjson.GetBytes(out, "messages.0.role").String() != "system" {
		t.Error("expected first message to be system")
	}
	assertTextContent(t, gjson.GetBytes(out, "messages.0.content"), "Be concise.")
	if gjson.GetBytes(out, "messages.1.role").String() != "user" {
		t.Error("expected second message to be user")
	}
}

func TestClaudeToOpenAI_MaxTokens(t *testing.T) {
	in := []byte(`{"model":"x","max_tokens":512,"messages":[{"role":"user","content":"hi"}]}`)
	out := claude.RequestToOpenAI("gpt-5.4", in)

	if gjson.GetBytes(out, "max_tokens").Int() != 512 {
		t.Error("expected max_tokens=512")
	}
}

func TestClaudeToOpenAI_ToolUse(t *testing.T) {
	in := []byte(`{
		"model":"x","max_tokens":1024,
		"tools":[{"name":"search","description":"search web","input_schema":{"type":"object","properties":{"q":{"type":"string"}}}}],
		"messages":[{"role":"user","content":"search for go"}]
	}`)
	out := claude.RequestToOpenAI("gpt-5.4", in)

	tools := gjson.GetBytes(out, "tools")
	if !tools.IsArray() || len(tools.Array()) == 0 {
		t.Fatal("expected tools array")
	}
	if tools.Array()[0].Get("type").String() != "function" {
		t.Error("expected tool type=function")
	}
	if tools.Array()[0].Get("function.name").String() != "search" {
		t.Error("expected function.name=search")
	}
	params := tools.Array()[0].Get("function.parameters")
	if !params.Exists() || !params.IsObject() {
		t.Error("expected function.parameters (from input_schema) to be object")
	}
}

func TestClaudeToOpenAI_ToolResultMessage(t *testing.T) {
	in := []byte(`{
		"model":"x","max_tokens":1024,
		"messages":[
			{"role":"user","content":"search for go"},
			{"role":"assistant","content":[{"type":"tool_use","id":"tu1","name":"search","input":{"q":"go"}}]},
			{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu1","content":"Go is great"}]}
		]
	}`)
	out := claude.RequestToOpenAI("gpt-5.4", in)

	msgs := gjson.GetBytes(out, "messages").Array()
	foundAssistant := false
	toolIndex := -1
	for i, m := range msgs {
		if m.Get("role").String() == "assistant" && m.Get("tool_calls").IsArray() {
			foundAssistant = true
		}
		if m.Get("role").String() == "tool" {
			toolIndex = i
			if m.Get("tool_call_id").String() != "tu1" {
				t.Error("expected tool_call_id=tu1")
			}
		}
	}
	if !foundAssistant {
		t.Error("expected an assistant message with tool_calls")
	}
	if toolIndex == -1 {
		t.Fatal("expected a tool role message")
	}
}

func TestClaudeToOpenAI_ThinkingToReasoningContent(t *testing.T) {
	in := []byte(`{
		"model":"x","max_tokens":1024,
		"messages":[
			{"role":"assistant","content":[
				{"type":"thinking","thinking":"let me think"},
				{"type":"text","text":"answer"}
			]}
		]
	}`)
	out := claude.RequestToOpenAI("gpt-5.4", in)

	msgs := gjson.GetBytes(out, "messages").Array()
	if len(msgs) == 0 {
		t.Fatal("expected messages")
	}
	if msgs[0].Get("reasoning_content").String() != "let me think" {
		t.Error("expected thinking → reasoning_content")
	}
}

func TestClaudeToOpenAI_EmptyThinking(t *testing.T) {
	in := []byte(`{
		"model":"x","max_tokens":1024,
		"messages":[
			{"role":"assistant","content":[
				{"type":"thinking","thinking":""},
				{"type":"text","text":"answer"}
			]}
		]
	}`)
	out := claude.RequestToOpenAI("gpt-5.4", in)

	msgs := gjson.GetBytes(out, "messages").Array()
	if len(msgs) == 0 {
		t.Fatal("expected messages")
	}
	rc := msgs[0].Get("reasoning_content")
	if rc.Exists() && rc.String() != "" {
		t.Error("empty thinking should not produce non-empty reasoning_content")
	}
}

// ===== Request: Claude → Gemini =====

func TestClaudeToGemini_BasicMessage(t *testing.T) {
	in := []byte(`{
		"model":"x","max_tokens":1024,
		"messages":[{"role":"user","content":"hello"}]
	}`)
	out := claude.RequestToGemini("gemini-3-pro-preview", in)

	if !gjson.GetBytes(out, "contents").IsArray() {
		t.Fatal("expected contents array")
	}
	if gjson.GetBytes(out, "contents.0.parts.0.text").String() != "hello" {
		t.Error("expected parts[0].text=hello")
	}
}

func TestClaudeToGemini_SystemInstruction(t *testing.T) {
	in := []byte(`{
		"model":"x","max_tokens":1024,
		"system":"You are a router.",
		"messages":[{"role":"user","content":"hi"}]
	}`)
	out := claude.RequestToGemini("gemini-3-pro-preview", in)

	if gjson.GetBytes(out, "system_instruction.parts.0.text").String() != "You are a router." {
		t.Error("expected system_instruction")
	}
}

func TestClaudeToGemini_MaxOutputTokens(t *testing.T) {
	in := []byte(`{"model":"x","max_tokens":2048,"messages":[{"role":"user","content":"hi"}]}`)
	out := claude.RequestToGemini("gemini-3-pro-preview", in)

	if gjson.GetBytes(out, "generationConfig.maxOutputTokens").Int() != 2048 {
		t.Error("expected maxOutputTokens=2048")
	}
}

// ===== Response: Claude → OpenAI (non-stream) =====

func TestClaudeResponseToOpenAI_NonStream(t *testing.T) {
	resp := []byte(`{
		"id":"msg-1","type":"message","role":"assistant","model":"claude-sonnet-4-6",
		"content":[{"type":"text","text":"hello world"}],
		"stop_reason":"end_turn",
		"usage":{"input_tokens":10,"output_tokens":5}
	}`)
	out := claude.ResponseToOpenAI(resp)

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
	if gjson.GetBytes(out, "usage.completion_tokens").Int() != 5 {
		t.Error("expected completion_tokens=5")
	}
}

func TestClaudeResponseToOpenAI_StopReasonMapping(t *testing.T) {
	cases := []struct{ claude, openai string }{
		{"end_turn", "stop"},
		{"max_tokens", "length"},
		{"tool_use", "tool_calls"},
		{"stop_sequence", "stop"},
		{"unknown_value", "stop"},
	}
	for _, c := range cases {
		resp := []byte(`{"id":"x","type":"message","role":"assistant","model":"x","content":[{"type":"text","text":""}],"stop_reason":"` + c.claude + `","usage":{}}`)
		out := claude.ResponseToOpenAI(resp)
		got := gjson.GetBytes(out, "choices.0.finish_reason").String()
		if got != c.openai {
			t.Errorf("stop_reason %q → finish_reason: got %q, want %q", c.claude, got, c.openai)
		}
	}
}

func TestClaudeResponseToOpenAI_ToolUse(t *testing.T) {
	resp := []byte(`{
		"id":"x","type":"message","role":"assistant","model":"x",
		"content":[{"type":"tool_use","id":"tu1","name":"search","input":{"q":"go"}}],
		"stop_reason":"tool_use","usage":{}
	}`)
	out := claude.ResponseToOpenAI(resp)

	tc := gjson.GetBytes(out, "choices.0.message.tool_calls.0")
	if tc.Get("id").String() != "tu1" {
		t.Error("expected tool_calls[0].id=tu1")
	}
	if tc.Get("function.name").String() != "search" {
		t.Error("expected function.name=search")
	}
	args := tc.Get("function.arguments")
	if !args.Exists() || args.String() == "" {
		t.Error("expected function.arguments to be non-empty")
	}
}

func TestClaudeResponseToOpenAI_MultipleContent(t *testing.T) {
	resp := []byte(`{
		"id":"x","type":"message","role":"assistant","model":"x",
		"content":[
			{"type":"text","text":"let me search"},
			{"type":"tool_use","id":"tu1","name":"search","input":{"q":"go"}}
		],
		"stop_reason":"tool_use","usage":{}
	}`)
	out := claude.ResponseToOpenAI(resp)

	content := gjson.GetBytes(out, "choices.0.message.content")
	toolCalls := gjson.GetBytes(out, "choices.0.message.tool_calls")
	if content.String() == "" || !toolCalls.IsArray() {
		t.Error("expected both content and tool_calls to be present")
	}
}
