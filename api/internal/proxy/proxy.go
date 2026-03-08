package proxy

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

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
	Timeout          time.Duration
}

type Error struct {
	Op                 string
	Provider           string
	UpstreamStatusCode int
	Err                error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.UpstreamStatusCode > 0 {
		return fmt.Sprintf("proxy %s for provider %s failed with upstream status %d: %v", e.Op, e.Provider, e.UpstreamStatusCode, e.Err)
	}
	return fmt.Sprintf("proxy %s for provider %s failed: %v", e.Op, e.Provider, e.Err)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func Forward(ctx context.Context, req Request) (*http.Response, error) {
	body, err := translateRequest(req)
	if err != nil {
		return nil, wrapError("translate request", req.Provider, 0, err)
	}

	url, err := upstreamURL(req.Provider, req.ModelName, req.Stream)
	if err != nil {
		return nil, wrapError("build upstream url", req.Provider, 0, err)
	}

	timeout := requestTimeout(req)
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, wrapError("build upstream request", req.Provider, 0, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	applyAuthHeaders(httpReq, req.Provider, req.Stream)

	resp, err := clientForProvider(req.Provider, timeout).Do(httpReq)
	if err != nil {
		return nil, wrapError("send upstream request", req.Provider, 0, err)
	}
	if isEventStream(resp.Header.Get("Content-Type")) {
		return translateStreamResponse(ctx, req, resp), nil
	}
	if req.Stream || resp.StatusCode >= http.StatusBadRequest {
		return resp, nil
	}

	translated, err := translateResponse(req, resp)
	if err != nil {
		return nil, wrapError("translate response", req.Provider, resp.StatusCode, err)
	}
	return translated, nil
}

func wrapError(op string, provider config.Provider, statusCode int, err error) error {
	if err == nil {
		return nil
	}
	return &Error{
		Op:                 op,
		Provider:           provider.Name,
		UpstreamStatusCode: statusCode,
		Err:                err,
	}
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

type sseChunkTranslator func([]byte) ([]byte, error)

type streamTranslationKey struct {
	from config.ProviderProtocol
	to   config.ProviderProtocol
}

var streamTranslators = map[streamTranslationKey]sseChunkTranslator{
	{from: config.ProtocolOpenAI, to: config.ProtocolClaude}: wrapStreamFn(openaiproto.StreamResponseToClaude),
	{from: config.ProtocolOpenAI, to: config.ProtocolGemini}: wrapStreamFn(openaiproto.StreamResponseToGemini),
	{from: config.ProtocolClaude, to: config.ProtocolOpenAI}: wrapStreamFn(claudeproto.StreamResponseToOpenAI),
	{from: config.ProtocolGemini, to: config.ProtocolOpenAI}: wrapStreamFn(geminiproto.StreamResponseToOpenAI),
	{from: config.ProtocolGemini, to: config.ProtocolClaude}: wrapStreamFn(geminiproto.StreamResponseToClaude),
}

func wrapStreamFn(fn func([]byte) []byte) sseChunkTranslator {
	return func(chunk []byte) ([]byte, error) {
		return fn(chunk), nil
	}
}

func translateStreamResponse(ctx context.Context, req Request, resp *http.Response) *http.Response {
	translated := *resp
	translated.Header = resp.Header.Clone()
	translated.ContentLength = -1
	translated.Header.Del("Content-Length")

	reader, writer := io.Pipe()
	translated.Body = reader

	go func() {
		defer func() { _ = resp.Body.Close() }()
		translator := streamTranslators[streamTranslationKey{from: req.Provider.Protocol, to: req.IncomingProtocol}]
		if err := relaySSE(ctx, resp.Body, writer, translator); err != nil {
			_ = writer.CloseWithError(wrapError("stream response", req.Provider, resp.StatusCode, err))
			return
		}
		_ = writer.Close()
	}()

	return &translated
}

func relaySSE(ctx context.Context, src io.Reader, dst *io.PipeWriter, translator sseChunkTranslator) error {
	reader := bufio.NewReader(src)
	var event bytes.Buffer

	flushEvent := func() error {
		if event.Len() == 0 {
			return nil
		}
		chunk := append([]byte(nil), event.Bytes()...)
		event.Reset()
		if translator != nil {
			translated, err := translator(chunk)
			if err != nil {
				return err
			}
			chunk = translated
		}
		if len(chunk) == 0 {
			return nil
		}
		_, err := dst.Write(chunk)
		return err
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if _, writeErr := event.Write(line); writeErr != nil {
				return writeErr
			}
			if bytes.Equal(line, []byte("\n")) || bytes.Equal(line, []byte("\r\n")) {
				if flushErr := flushEvent(); flushErr != nil {
					return flushErr
				}
			}
		}

		if err == nil {
			continue
		}
		if errors.Is(err, io.EOF) {
			return flushEvent()
		}
		return err
	}
}

func isEventStream(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "text/event-stream")
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
		req.Header.Set("x-api-key", provider.APIKey)
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
