// 通用中间件：请求日志（环形缓冲区）、panic 恢复、CORS
package handler

import (
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/Mag1cFall/ai-router/api/internal/config"
	"github.com/gin-gonic/gin"
)

const requestLogCapacity = 256

// requestLogEntry 单条请求日志记录
type requestLogEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Protocol   string    `json:"protocol"`
	Model      string    `json:"model,omitempty"`
	Provider   string    `json:"provider,omitempty"`
	StatusCode int       `json:"status_code"`
	LatencyMS  int64     `json:"latency_ms"`
}

// requestLogStore 固定容量环形缓冲区，存储最新的请求日志
type requestLogStore struct {
	mu      sync.Mutex
	entries []requestLogEntry
	next    int
	filled  bool
}

// newRequestLogStore 创建日志存储，capacity <= 0 时使用默认值
func newRequestLogStore(capacity int) *requestLogStore {
	if capacity <= 0 {
		capacity = requestLogCapacity
	}
	return &requestLogStore{entries: make([]requestLogEntry, capacity)}
}

// Add 写入日志条目，满时覆盖最老记录
func (s *requestLogStore) Add(entry requestLogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries[s.next] = entry
	s.next = (s.next + 1) % len(s.entries)
	if s.next == 0 {
		s.filled = true
	}
}

// Snapshot 返回所有日志，按时间倒序（最新在前）
func (s *requestLogStore) Snapshot() []requestLogEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := s.next
	if s.filled {
		count = len(s.entries)
	}
	result := make([]requestLogEntry, 0, count)
	if count == 0 {
		return result
	}

	if !s.filled {
		for idx := s.next - 1; idx >= 0; idx-- {
			result = append(result, s.entries[idx])
		}
		return result
	}

	idx := (s.next - 1 + len(s.entries)) % len(s.entries)
	for range s.entries {
		result = append(result, s.entries[idx])
		idx = (idx - 1 + len(s.entries)) % len(s.entries)
	}
	return result
}

// Capacity 返回缓冲区容量
func (s *requestLogStore) Capacity() int {
	return len(s.entries)
}

// requestLoggingMiddleware 记录代理请求的耗时、协议、模型和 Provider，跳过内部 API 路径
func requestLoggingMiddleware(store *requestLogStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		c.Next()

		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") || path == "/healthz" || path == "/favicon.ico" {
			return
		}

		protocol := protocolForLog(c)
		model := stringValue(c, ctxModelKey)
		provider := providerForLog(c)

		store.Add(requestLogEntry{
			Timestamp:  startedAt.UTC(),
			Method:     c.Request.Method,
			Path:       path,
			Protocol:   protocol,
			Model:      model,
			Provider:   provider,
			StatusCode: c.Writer.Status(),
			LatencyMS:  time.Since(startedAt).Milliseconds(),
		})

		log.Printf("%s %s → %s | model=%s provider=%s status=%d latency=%dms",
			c.Request.Method, path, protocol, model, provider, c.Writer.Status(), time.Since(startedAt).Milliseconds())
	}
}

// recoveryMiddleware 捕获 panic 并返回 500，打印堆栈
func recoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("panic recovered: %v\n%s", recovered, debug.Stack())
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			}
		}()
		c.Next()
	}
}

// corsMiddleware 设置 CORS 头，允许所有来源，OPTIONS 预检直接返回 204
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		headers := c.Writer.Header()
		headers.Set("Access-Control-Allow-Origin", "*")
		headers.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		headers.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, Origin, X-Requested-With")
		headers.Set("Access-Control-Expose-Headers", "Content-Length, Content-Type")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// protocolForLog 从 context 提取协议名用于日志
func protocolForLog(c *gin.Context) string {
	if protocolValue, ok := c.Get(ctxProtocolKey); ok {
		if protocol, ok := protocolValue.(config.ProviderProtocol); ok {
			return string(protocol)
		}
	}
	if strings.HasPrefix(c.Request.URL.Path, "/api/") {
		return "admin"
	}
	return "unknown"
}

// providerForLog 从 context 提取 Provider 名用于日志
func providerForLog(c *gin.Context) string {
	if providerValue, ok := c.Get(ctxProviderKey); ok {
		if provider, ok := providerValue.(config.Provider); ok {
			return provider.Name
		}
	}
	return ""
}

// stringValue 安全读取 gin context 中的字符串值
func stringValue(c *gin.Context, key string) string {
	value, ok := c.Get(key)
	if !ok {
		return ""
	}
	stringValue, ok := value.(string)
	if !ok {
		return ""
	}
	return stringValue
}

func authMiddleware(apiKeys []string) gin.HandlerFunc {
	keySet := make(map[string]struct{}, len(apiKeys))
	for _, k := range apiKeys {
		if k = strings.TrimSpace(k); k != "" {
			keySet[k] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		if len(keySet) == 0 {
			c.Next()
			return
		}

		key := ""
		if auth := c.GetHeader("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			key = strings.TrimPrefix(auth, "Bearer ")
		}
		if key == "" {
			key = c.GetHeader("x-api-key")
		}
		if key == "" {
			key = c.GetHeader("x-goog-api-key")
		}
		if key == "" {
			key = c.Query("key")
		}

		if _, ok := keySet[strings.TrimSpace(key)]; !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
			return
		}
		c.Next()
	}
}
