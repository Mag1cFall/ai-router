package detect_test

import (
	"testing"

	"github.com/Mag1cFall/ai-router/api/internal/proto/detect"
)

func TestFromRequest_OpenAI(t *testing.T) {
	cases := []struct {
		method, path string
		body         []byte
	}{
		{"POST", "/v1/chat/completions", []byte(`{"model":"gpt-4o","messages":[]}`)},
		{"POST", "/v1/responses", []byte(`{"model":"gpt-4o","input":[]}`)},
	}
	for _, c := range cases {
		if got := detect.FromRequest(c.method, c.path, c.body); got != detect.ProtocolOpenAI {
			t.Errorf("FromRequest(%q,%q) = %v, want OpenAI", c.method, c.path, got)
		}
	}
}

func TestFromRequest_Claude(t *testing.T) {
	cases := []struct {
		method, path string
		body         []byte
	}{
		{"POST", "/v1/messages", []byte(`{"model":"claude-opus-4","max_tokens":1024,"messages":[]}`)},
		{"POST", "/v1/messages", []byte(`{"model":"claude-3-5-sonnet","messages":[]}`)},
		{"POST", "/v1/messages", []byte(`{}`)},
	}
	for _, c := range cases {
		if got := detect.FromRequest(c.method, c.path, c.body); got != detect.ProtocolClaude {
			t.Errorf("FromRequest(%q,%q) = %v, want Claude", c.method, c.path, got)
		}
	}
}

func TestFromRequest_Gemini(t *testing.T) {
	cases := []struct{ method, path string }{
		{"POST", "/v1beta/models/gemini-2.5-pro:generateContent"},
		{"POST", "/v1beta/models/gemini-2.0-flash:streamGenerateContent"},
		{"POST", "/v1/models/gemini-pro:generateContent"},
		{"POST", "/v1beta/models/gemini-2.5-pro:generateContent?key=AIza"},
		{"POST", "/v1beta/models/gemini-2.0-flash-exp:streamGenerateContent/"},
	}
	for _, c := range cases {
		if got := detect.FromRequest(c.method, c.path, nil); got != detect.ProtocolGemini {
			t.Errorf("FromRequest(%q,%q) = %v, want Gemini", c.method, c.path, got)
		}
	}
}

func TestFromRequest_Unknown(t *testing.T) {
	cases := []struct {
		method, path string
		body         []byte
	}{
		{"GET", "/healthz", nil},
		{"POST", "/unknown/path", []byte(`{}`)},
		{"GET", "/", nil},
		{"POST", "/v1/chat/completions", nil},
	}
	for _, c := range cases {
		got := detect.FromRequest(c.method, c.path, c.body)
		if c.path == "/healthz" || c.path == "/unknown/path" || c.path == "/" {
			if got != detect.ProtocolUnknown {
				t.Errorf("FromRequest(%q,%q) = %v, want ProtocolUnknown", c.method, c.path, got)
			}
		}
	}
}

func TestFromRequest_ModelsEndpointAmbiguous(t *testing.T) {
	got := detect.FromRequest("GET", "/v1/models", nil)
	if got != detect.ProtocolOpenAI && got != detect.ProtocolUnknown {
		t.Errorf("GET /v1/models should be OpenAI or Unknown (ambiguous), got %v", got)
	}
}

func TestExtractModelName(t *testing.T) {
	cases := []struct {
		path string
		body []byte
		want string
	}{
		{"/v1/chat/completions", []byte(`{"model":"gpt-4o"}`), "gpt-4o"},
		{"/v1/messages", []byte(`{"model":"claude-opus-4"}`), "claude-opus-4"},
		{"/v1beta/models/gemini-2.5-pro:generateContent", nil, "gemini-2.5-pro"},
		{"/v1beta/models/gemini-2.0-flash:streamGenerateContent", nil, "gemini-2.0-flash"},
		{"/v1beta/models/gemini-2.5-pro:generateContent?key=AIza", nil, "gemini-2.5-pro"},
	}
	for _, c := range cases {
		if got := detect.ExtractModelName(c.path, c.body); got != c.want {
			t.Errorf("ExtractModelName(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

func TestExtractModelName_Fallbacks(t *testing.T) {
	if got := detect.ExtractModelName("/v1/chat/completions", nil); got != "" {
		t.Errorf("nil body should return empty, got %q", got)
	}
	if got := detect.ExtractModelName("/v1/chat/completions", []byte(`not-json`)); got != "" {
		t.Errorf("invalid json body should return empty, got %q", got)
	}
	if got := detect.ExtractModelName("/v1/chat/completions", []byte(`{}`)); got != "" {
		t.Errorf("missing model field should return empty, got %q", got)
	}
}

func TestIsStreaming(t *testing.T) {
	if !detect.IsStreaming([]byte(`{"stream":true}`)) {
		t.Error("expected streaming=true")
	}
	if detect.IsStreaming([]byte(`{"stream":false}`)) {
		t.Error("expected streaming=false")
	}
	if detect.IsStreaming([]byte(`{"model":"x"}`)) {
		t.Error("expected streaming=false when absent")
	}
	if detect.IsStreaming(nil) {
		t.Error("nil body should not be streaming")
	}
	if detect.IsStreaming([]byte(`not-json`)) {
		t.Error("invalid JSON should not be streaming")
	}
}
