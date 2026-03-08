package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/Mag1cFall/ai-router/api/internal/config"
	"github.com/Mag1cFall/ai-router/api/internal/proto/detect"
	"github.com/Mag1cFall/ai-router/api/internal/proxy"
	"github.com/gin-gonic/gin"
)

const (
	ctxBodyKey     = "request_body"
	ctxProtocolKey = "request_protocol"
	ctxModelKey    = "request_model"
	ctxStreamKey   = "request_stream"
	ctxProviderKey = "request_provider"
	ctxResponseKey = "proxy_response"
)

func RegisterRoutes(r *gin.Engine, cfg *config.Config) {
	logs := newRequestLogStore(requestLogCapacity)
	r.Use(requestLoggingMiddleware(logs), recoveryMiddleware(), corsMiddleware())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/api/providers", func(c *gin.Context) {
		providers := make([]gin.H, 0, len(cfg.Providers))
		for _, provider := range cfg.Providers {
			providers = append(providers, gin.H{
				"name":     provider.Name,
				"protocol": provider.Protocol,
				"endpoint": provider.Endpoint,
			})
		}
		c.JSON(http.StatusOK, gin.H{"providers": providers})
	})
	r.GET("/api/routes", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"routes": cfg.Routes})
	})
	r.GET("/api/logs", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"capacity": logs.Capacity(),
			"logs":     logs.Snapshot(),
		})
	})
	r.NoRoute(detectMiddleware(), resolveMiddleware(cfg), proxyMiddleware(), respondMiddleware())
}

func detectMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		protocol := detect.FromRequest(c.Request.Method, c.Request.URL.RequestURI(), body)
		if protocol == detect.ProtocolUnknown {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "unsupported request protocol"})
			return
		}
		model := detect.ExtractModelName(c.Request.URL.RequestURI(), body)
		if model == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "model is required"})
			return
		}
		c.Set(ctxBodyKey, body)
		c.Set(ctxProtocolKey, config.ProviderProtocol(protocol))
		c.Set(ctxModelKey, model)
		c.Set(ctxStreamKey, detect.IsStreaming(body))
		c.Next()
	}
}

func resolveMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		model, _ := c.Get(ctxModelKey)
		provider, err := cfg.ResolveProvider(model.(string))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.Set(ctxProviderKey, provider)
		c.Next()
	}
}

func proxyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		body, _ := c.Get(ctxBodyKey)
		protocol, _ := c.Get(ctxProtocolKey)
		model, _ := c.Get(ctxModelKey)
		stream, _ := c.Get(ctxStreamKey)
		provider, _ := c.Get(ctxProviderKey)

		resp, err := proxy.Forward(c.Request.Context(), proxy.Request{
			IncomingProtocol: protocol.(config.ProviderProtocol),
			Provider:         provider.(config.Provider),
			ModelName:        model.(string),
			Body:             body.([]byte),
			Stream:           stream.(bool),
		})
		if err != nil {
			status := http.StatusBadGateway
			var proxyErr *proxy.Error
			if errors.As(err, &proxyErr) {
				switch {
				case errors.Is(proxyErr.Err, context.DeadlineExceeded), errors.Is(proxyErr.Err, context.Canceled):
					status = http.StatusGatewayTimeout
				case proxyErr.UpstreamStatusCode >= http.StatusBadRequest:
					status = proxyErr.UpstreamStatusCode
				}
			}
			c.AbortWithStatusJSON(status, gin.H{"error": err.Error()})
			return
		}
		c.Set(ctxResponseKey, resp)
		c.Next()
	}
}

func respondMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		value, ok := c.Get(ctxResponseKey)
		if !ok {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "missing upstream response"})
			return
		}
		resp := value.(*http.Response)
		defer func() { _ = resp.Body.Close() }()

		for key, values := range resp.Header {
			for _, value := range values {
				c.Writer.Header().Add(key, value)
			}
		}
		c.Status(resp.StatusCode)

		writer := io.Writer(c.Writer)
		if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
			writer = flushWriter{ResponseWriter: c.Writer}
		}
		_, _ = io.Copy(writer, resp.Body)
	}
}

type flushWriter struct {
	gin.ResponseWriter
}

func (w flushWriter) Write(data []byte) (int, error) {
	written, err := w.ResponseWriter.Write(data)
	if err == nil && written > 0 {
		w.Flush()
	}
	return written, err
}
