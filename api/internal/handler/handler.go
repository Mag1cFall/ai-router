package handler

import (
	"io"
	"net/http"

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
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.Any("/*path", detectMiddleware(), resolveMiddleware(cfg), proxyMiddleware(), respondMiddleware())
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
			c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": err.Error()})
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
		defer resp.Body.Close()

		for key, values := range resp.Header {
			for _, value := range values {
				c.Writer.Header().Add(key, value)
			}
		}
		c.Status(resp.StatusCode)
		_, _ = io.Copy(c.Writer, resp.Body)
	}
}
