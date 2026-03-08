package claude_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Mag1cFall/ai-router/api/internal/proto/claude"
	"github.com/tidwall/gjson"
)

func claudeOpenAIPayload(raw []byte) []byte {
	text := strings.TrimSpace(string(raw))
	text = strings.TrimPrefix(text, "data: ")
	text = strings.TrimSpace(text)
	return []byte(text)
}

func TestClaudeStreamResponseToOpenAI_TextAndThinking(t *testing.T) {
	start := []byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg-stream-1\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-sonnet-4-6\",\"content\":[]}}\n\n")
	startOut := claude.StreamResponseToOpenAI(start)
	if gjson.GetBytes(claudeOpenAIPayload(startOut), "choices.0.delta.role").String() != "assistant" {
		t.Fatalf("expected assistant role chunk, got %s", startOut)
	}

	thinking := []byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"先分析\"}}\n\n")
	thinkingOut := claude.StreamResponseToOpenAI(thinking)
	if gjson.GetBytes(claudeOpenAIPayload(thinkingOut), "choices.0.delta.reasoning_content").String() != "先分析" {
		t.Fatalf("expected reasoning_content chunk, got %s", thinkingOut)
	}

	text := []byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"text_delta\",\"text\":\"答案\"}}\n\n")
	textOut := claude.StreamResponseToOpenAI(text)
	if gjson.GetBytes(claudeOpenAIPayload(textOut), "choices.0.delta.content").String() != "答案" {
		t.Fatalf("expected content chunk, got %s", textOut)
	}

	_ = claude.StreamResponseToOpenAI([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
}

func TestClaudeStreamResponseToOpenAI_ToolUseAndFinish(t *testing.T) {
	_ = claude.StreamResponseToOpenAI([]byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg-stream-2\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-sonnet-4-6\",\"content\":[]}}\n\n"))

	start := []byte("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"tu_1\",\"name\":\"search\",\"input\":{}}}\n\n")
	if out := claude.StreamResponseToOpenAI(start); len(bytes.TrimSpace(out)) != 0 {
		t.Fatalf("expected no output on tool_use start, got %s", out)
	}

	delta := []byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"q\\\":\\\"go\\\"}\"}}\n\n")
	if out := claude.StreamResponseToOpenAI(delta); len(bytes.TrimSpace(out)) != 0 {
		t.Fatalf("expected no output on input_json_delta, got %s", out)
	}

	stop := []byte("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n")
	stopOut := claude.StreamResponseToOpenAI(stop)
	if gjson.GetBytes(claudeOpenAIPayload(stopOut), "choices.0.delta.tool_calls.0.id").String() != "tu_1" {
		t.Fatalf("expected tool call id, got %s", stopOut)
	}
	if gjson.GetBytes(claudeOpenAIPayload(stopOut), "choices.0.delta.tool_calls.0.function.name").String() != "search" {
		t.Fatalf("expected tool name, got %s", stopOut)
	}
	if gjson.GetBytes(claudeOpenAIPayload(stopOut), "choices.0.delta.tool_calls.0.function.arguments").String() != `{"q":"go"}` {
		t.Fatalf("expected full tool arguments, got %s", stopOut)
	}

	finish := []byte("event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\",\"stop_sequence\":null},\"usage\":{\"input_tokens\":3,\"output_tokens\":5}}\n\n")
	finishOut := claude.StreamResponseToOpenAI(finish)
	if gjson.GetBytes(claudeOpenAIPayload(finishOut), "choices.0.finish_reason").String() != "tool_calls" {
		t.Fatalf("expected tool_calls finish_reason, got %s", finishOut)
	}
	if gjson.GetBytes(claudeOpenAIPayload(finishOut), "usage.prompt_tokens").Int() != 3 || gjson.GetBytes(claudeOpenAIPayload(finishOut), "usage.completion_tokens").Int() != 5 {
		t.Fatalf("expected usage mapping, got %s", finishOut)
	}

	_ = claude.StreamResponseToOpenAI([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
}

func TestClaudeNonStreamResponseToOpenAI_ThinkingToReasoningContent(t *testing.T) {
	resp := []byte(`{"id":"msg-1","type":"message","role":"assistant","model":"claude-sonnet-4-6","content":[{"type":"thinking","thinking":"先分析"},{"type":"text","text":"答案"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":2}}`)
	out := claude.ResponseToOpenAI(resp)

	if gjson.GetBytes(out, "choices.0.message.reasoning_content").String() != "先分析" {
		t.Fatalf("expected reasoning_content, got %s", out)
	}
	if gjson.GetBytes(out, "choices.0.message.content").String() != "答案" {
		t.Fatalf("expected visible answer, got %s", out)
	}
}
