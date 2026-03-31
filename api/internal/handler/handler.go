// HTTP 路由注册和请求处理链：协议检测→路由匹配→代理转发→响应回写
package handler

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Mag1cFall/ai-router/api/internal/config"
	"github.com/Mag1cFall/ai-router/api/internal/proto/detect"
	"github.com/Mag1cFall/ai-router/api/internal/proxy"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const (
	ctxBodyKey     = "request_body"
	ctxProtocolKey = "request_protocol"
	ctxModelKey    = "request_model"
	ctxStreamKey   = "request_stream"
	ctxProviderKey = "request_provider"
	ctxResponseKey = "proxy_response"
)

// RegisterRoutes 注册健康检查、管理 API 和代理 NoRoute 链路
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

	auth := authMiddleware(cfg.Server.APIKeys)
	models := collectModels(cfg)

	r.GET("/v1/models", auth, func(c *gin.Context) {
		data := make([]gin.H, 0, len(models))
		for _, m := range models {
			data = append(data, gin.H{
				"id":       m.id,
				"object":   "model",
				"created":  m.created,
				"owned_by": m.ownedBy,
			})
		}
		c.JSON(http.StatusOK, gin.H{"object": "list", "data": data})
	})

	r.GET("/v1beta/models", auth, func(c *gin.Context) {
		data := make([]gin.H, 0, len(models))
		for _, m := range models {
			data = append(data, gin.H{
				"name":                        "models/" + m.id,
				"displayName":                 m.id,
				"supportedGenerationMethods": []string{"generateContent", "streamGenerateContent"},
			})
		}
		c.JSON(http.StatusOK, gin.H{"models": data})
	})

	r.NoRoute(auth, detectMiddleware(), resolveMiddleware(cfg), proxyMiddleware(), respondMiddleware())
}

type modelEntry struct {
	id      string
	ownedBy string
	created int64
}

var modelsHTTPClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	},
}

func collectModels(cfg *config.Config) []modelEntry {
	now := time.Now().Unix()
	var models []modelEntry
	for _, p := range cfg.Providers {
		fetched := fetchProviderModels(p)
		if len(fetched) > 0 {
			for _, id := range fetched {
				models = append(models, modelEntry{id: id, ownedBy: p.Name, created: now})
			}
			continue
		}
		for _, m := range p.Models {
			models = append(models, modelEntry{id: m, ownedBy: p.Name, created: now})
		}
	}
	return models
}

func fetchProviderModels(p config.Provider) []string {
	base := strings.TrimRight(p.Endpoint, "/")
	var url string
	var req *http.Request
	var err error

	switch p.Protocol {
	case config.ProtocolOpenAI:
		url = base + "/models"
		req, err = http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil
		}
		if p.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+p.APIKey)
		}
	case config.ProtocolGemini:
		url = fmt.Sprintf("%s/models?key=%s", base, p.APIKey)
		req, err = http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil
		}
	default:
		return nil
	}

	resp, err := modelsHTTPClient.Do(req)
	if err != nil {
		log.Printf("fetch models from %s failed: %v", p.Name, err)
		return nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		log.Printf("fetch models from %s got status %d", p.Name, resp.StatusCode)
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}

	var ids []string
	switch p.Protocol {
	case config.ProtocolOpenAI:
		gjson.GetBytes(body, "data.#.id").ForEach(func(_, v gjson.Result) bool {
			ids = append(ids, v.String())
			return true
		})
	case config.ProtocolGemini:
		gjson.GetBytes(body, "models.#.name").ForEach(func(_, v gjson.Result) bool {
			name := v.String()
			name = strings.TrimPrefix(name, "models/")
			ids = append(ids, name)
			return true
		})
	}

	log.Printf("fetched %d models from %s", len(ids), p.Name)
	return ids
}

// detectMiddleware 读取请求体并检测协议类型、模型名、是否流式，写入 context
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
		stream := detect.IsStreaming(body)
		if !stream && strings.Contains(c.Request.URL.Path, "streamGenerateContent") {
			stream = true
		}
		c.Set(ctxStreamKey, stream)
		c.Next()
	}
}

// resolveMiddleware 根据模型名匹配 Provider 并写入 context
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

// proxyMiddleware 调用 proxy.Forward 将请求转发到上游 Provider，并写入响应对象
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

// respondMiddleware 注入上游响应头并回写身体，流式响应自动 flush
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

// flushWriter 包装每次 Write 后自动 Flush，用于 SSE 流式响应
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
