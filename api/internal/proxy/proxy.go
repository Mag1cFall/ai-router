package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Mag1cFall/ai-router/api/internal/config"
	claudeproto "github.com/Mag1cFall/ai-router/api/internal/proto/claude"
	geminiproto "github.com/Mag1cFall/ai-router/api/internal/proto/gemini"
	openaiproto "github.com/Mag1cFall/ai-router/api/internal/proto/openai"
	"github.com/tidwall/sjson"
)

type Request struct {
	IncomingProtocol config.ProviderProtocol
	Provider         config.Provider
	ModelName        string
	Body             []byte
	Stream           bool
}

func Forward(ctx context.Context, req Request) (*http.Response, error) {
	body, err := translateRequest(req)
	if err != nil {
		return nil, err
	}

	url, err := upstreamURL(req.Provider, req.ModelName, req.Stream)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	applyAuthHeaders(httpReq, req.Provider, req.Stream)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if req.Stream || resp.StatusCode >= http.StatusBadRequest {
		return resp, nil
	}

	translated, err := translateResponse(req, resp)
	if err != nil {
		_ = resp.Body.Close()
		return nil, err
	}
	return translated, nil
}

func translateRequest(req Request) ([]byte, error) {
	switch req.IncomingProtocol {
	case req.Provider.Protocol:
		return overrideModelIfNeeded(req.Provider.Protocol, req.ModelName, req.Body), nil
	case config.ProtocolOpenAI:
		switch req.Provider.Protocol {
		case config.ProtocolClaude:
			return openaiproto.RequestToClaude(req.ModelName, req.Body), nil
		case config.ProtocolGemini:
			return openaiproto.RequestToGemini(req.ModelName, req.Body), nil
		}
	case config.ProtocolClaude:
		switch req.Provider.Protocol {
		case config.ProtocolOpenAI:
			return claudeproto.RequestToOpenAI(req.ModelName, req.Body), nil
		case config.ProtocolGemini:
			return claudeproto.RequestToGemini(req.ModelName, req.Body), nil
		}
	case config.ProtocolGemini:
		switch req.Provider.Protocol {
		case config.ProtocolOpenAI:
			return geminiproto.RequestToOpenAI(req.ModelName, req.Body), nil
		case config.ProtocolClaude:
			return geminiproto.RequestToClaude(req.ModelName, req.Body), nil
		}
	}
	return nil, fmt.Errorf("unsupported request translation: %s -> %s", req.IncomingProtocol, req.Provider.Protocol)
}

func translateResponse(req Request, resp *http.Response) (*http.Response, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	_ = resp.Body.Close()

	translated := body
	switch req.Provider.Protocol {
	case req.IncomingProtocol:
		translated = body
	case config.ProtocolOpenAI:
		switch req.IncomingProtocol {
		case config.ProtocolClaude:
			translated = openaiproto.ResponseToClaude(body)
		case config.ProtocolGemini:
			translated = openaiproto.ResponseToGemini(body)
		}
	case config.ProtocolClaude:
		openAIBody := claudeproto.ResponseToOpenAI(body)
		switch req.IncomingProtocol {
		case config.ProtocolOpenAI:
			translated = openAIBody
		case config.ProtocolGemini:
			translated = openaiproto.ResponseToGemini(openAIBody)
		}
	case config.ProtocolGemini:
		switch req.IncomingProtocol {
		case config.ProtocolOpenAI:
			translated = geminiproto.ResponseToOpenAI(body)
		case config.ProtocolClaude:
			translated = geminiproto.ResponseToClaude(body)
		}
	}

	newResp := *resp
	newResp.Body = io.NopCloser(bytes.NewReader(translated))
	newResp.ContentLength = int64(len(translated))
	newResp.Header = resp.Header.Clone()
	newResp.Header.Set("Content-Type", "application/json")
	newResp.Header.Del("Content-Encoding")
	return &newResp, nil
}

func upstreamURL(provider config.Provider, modelName string, stream bool) (string, error) {
	base := strings.TrimRight(provider.Endpoint, "/")
	if base == "" {
		return "", fmt.Errorf("provider endpoint is empty")
	}
	switch provider.Protocol {
	case config.ProtocolOpenAI:
		return base + "/chat/completions", nil
	case config.ProtocolClaude:
		return base + "/messages", nil
	case config.ProtocolGemini:
		action := "generateContent"
		if stream {
			action = "streamGenerateContent"
		}
		return fmt.Sprintf("%s/models/%s:%s", base, modelName, action), nil
	default:
		return "", fmt.Errorf("unsupported provider protocol %q", provider.Protocol)
	}
}

func applyAuthHeaders(req *http.Request, provider config.Provider, stream bool) {
	if provider.APIKey == "" {
		return
	}
	switch provider.Protocol {
	case config.ProtocolOpenAI:
		req.Header.Set("Authorization", "Bearer "+provider.APIKey)
	case config.ProtocolClaude:
		req.Header.Set("x-api-key", provider.APIKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		if stream {
			req.Header.Set("Accept", "text/event-stream")
		}
	case config.ProtocolGemini:
		q := req.URL.Query()
		q.Set("key", provider.APIKey)
		req.URL.RawQuery = q.Encode()
	}
}

func overrideModelIfNeeded(protocol config.ProviderProtocol, modelName string, body []byte) []byte {
	if len(body) == 0 || modelName == "" {
		return body
	}
	if protocol == config.ProtocolGemini {
		return body
	}
	updated, err := sjson.SetBytes(body, "model", modelName)
	if err != nil {
		return body
	}
	return updated
}
