package proxy_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/Mag1cFall/ai-router/api/internal/config"
	"github.com/Mag1cFall/ai-router/api/internal/proxy"
)

type capturedRequest struct {
	Method  string
	Path    string
	Headers http.Header
	Body    []byte
}

func mockProvider(t *testing.T, protocol config.ProviderProtocol, responseBody string, statusCode int) (*httptest.Server, config.Provider, *capturedRequest) {
	t.Helper()
	var mu sync.Mutex
	captured := &capturedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		captured.Method = r.Method
		captured.Path = r.URL.Path
		captured.Headers = r.Header.Clone()
		body, _ := io.ReadAll(r.Body)
		captured.Body = body
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = w.Write([]byte(responseBody))
	}))
	t.Cleanup(srv.Close)
	return srv, config.Provider{
		Name:     "mock",
		Protocol: protocol,
		Endpoint: srv.URL,
		APIKey:   "test-key-123",
	}, captured
}

func TestProxy_OpenAIToOpenAI(t *testing.T) {
	body := `{"id":"x","choices":[{"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8}}`
	_, provider, captured := mockProvider(t, config.ProtocolOpenAI, body, 200)

	req := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}]}`)
	resp, err := proxy.Forward(context.Background(), proxy.Request{
		IncomingProtocol: config.ProtocolOpenAI,
		Provider:         provider,
		ModelName:        "gpt-5.4",
		Body:             req,
		Stream:           false,
	})
	if err != nil {
		t.Fatalf("Forward error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if captured.Method != "POST" {
		t.Errorf("expected POST, got %s", captured.Method)
	}
	if len(captured.Body) == 0 {
		t.Error("expected non-empty request body sent to upstream")
	}
}

func TestProxy_OpenAIToOpenAI_AuthHeader(t *testing.T) {
	_, provider, captured := mockProvider(t, config.ProtocolOpenAI, `{"id":"x","choices":[],"usage":{}}`, 200)

	req := []byte(`{"model":"gpt-5.4","messages":[]}`)
	_, err := proxy.Forward(context.Background(), proxy.Request{
		IncomingProtocol: config.ProtocolOpenAI,
		Provider:         provider,
		ModelName:        "gpt-5.4",
		Body:             req,
		Stream:           false,
	})
	if err != nil {
		t.Fatalf("Forward error: %v", err)
	}
	auth := captured.Headers.Get("Authorization")
	if auth == "" || !strings.Contains(auth, "test-key-123") {
		t.Errorf("expected Authorization header with API key, got %q", auth)
	}
}

func TestProxy_ClaudeToOpenAI_ResponseTranslated(t *testing.T) {
	openaiResp := `{
		"id":"x","model":"gpt-5.4",
		"choices":[{"message":{"role":"assistant","content":"hello from openai"},"finish_reason":"stop"}],
		"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}
	}`
	_, provider, _ := mockProvider(t, config.ProtocolOpenAI, openaiResp, 200)

	claudeReq := []byte(`{
		"model":"claude-sonnet-4-6","max_tokens":1024,
		"messages":[{"role":"user","content":"hi"}]
	}`)
	resp, err := proxy.Forward(context.Background(), proxy.Request{
		IncomingProtocol: config.ProtocolClaude,
		Provider:         provider,
		ModelName:        "gpt-5.4",
		Body:             claudeReq,
		Stream:           false,
	})
	if err != nil {
		t.Fatalf("Forward error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	respBody, _ := io.ReadAll(resp.Body)
	var m map[string]any
	if err := json.Unmarshal(respBody, &m); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if m["type"] != "message" {
		t.Errorf("expected type=message in Claude response, got %v", m["type"])
	}
}

func TestProxy_GeminiToClaude_ResponseTranslated(t *testing.T) {
	claudeResp := `{
		"id":"x","type":"message","role":"assistant","model":"claude-sonnet-4-6",
		"content":[{"type":"text","text":"hello from claude"}],
		"stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":5}
	}`
	_, provider, _ := mockProvider(t, config.ProtocolClaude, claudeResp, 200)

	geminiReq := []byte(`{
		"contents":[{"role":"user","parts":[{"text":"hi"}]}]
	}`)
	resp, err := proxy.Forward(context.Background(), proxy.Request{
		IncomingProtocol: config.ProtocolGemini,
		Provider:         provider,
		ModelName:        "gemini-3-pro-preview",
		Body:             geminiReq,
		Stream:           false,
	})
	if err != nil {
		t.Fatalf("Forward error: %v", err)
	}
	respBody, _ := io.ReadAll(resp.Body)
	var m map[string]any
	if err := json.Unmarshal(respBody, &m); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if _, ok := m["candidates"]; !ok {
		t.Error("expected candidates in Gemini response")
	}
}

func TestProxy_ErrorFromUpstream(t *testing.T) {
	_, provider, _ := mockProvider(t, config.ProtocolOpenAI, `{"error":{"message":"bad request"}}`, 400)

	req := []byte(`{"model":"gpt-5.4","messages":[]}`)
	resp, err := proxy.Forward(context.Background(), proxy.Request{
		IncomingProtocol: config.ProtocolOpenAI,
		Provider:         provider,
		ModelName:        "gpt-5.4",
		Body:             req,
		Stream:           false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("expected upstream 400 passed through, got %d", resp.StatusCode)
	}
}

func TestProxy_StreamingOpenAI(t *testing.T) {
	chunks := "data: {\"id\":\"x\",\"choices\":[{\"delta\":{\"role\":\"assistant\",\"content\":\"hello\"},\"finish_reason\":null}]}\n\ndata: [DONE]\n\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(chunks))
	}))
	defer srv.Close()

	provider := config.Provider{Name: "mock", Protocol: config.ProtocolOpenAI, Endpoint: srv.URL, APIKey: "k"}
	req := []byte(`{"model":"gpt-5.4","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	resp, err := proxy.Forward(context.Background(), proxy.Request{
		IncomingProtocol: config.ProtocolOpenAI,
		Provider:         provider,
		ModelName:        "gpt-5.4",
		Body:             req,
		Stream:           true,
	})
	if err != nil {
		t.Fatalf("Forward error: %v", err)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("expected Content-Type text/event-stream, got %q", ct)
	}
	respBody, _ := io.ReadAll(resp.Body)
	bodyStr := string(respBody)
	if !strings.Contains(bodyStr, "data:") {
		t.Error("expected SSE data in streaming response")
	}
	if !strings.Contains(bodyStr, "[DONE]") {
		t.Error("expected [DONE] marker in streaming response")
	}
}
