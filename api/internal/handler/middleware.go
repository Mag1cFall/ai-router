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

type requestLogStore struct {
	mu      sync.Mutex
	entries []requestLogEntry
	next    int
	filled  bool
}

func newRequestLogStore(capacity int) *requestLogStore {
	if capacity <= 0 {
		capacity = requestLogCapacity
	}
	return &requestLogStore{entries: make([]requestLogEntry, capacity)}
}

func (s *requestLogStore) Add(entry requestLogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries[s.next] = entry
	s.next = (s.next + 1) % len(s.entries)
	if s.next == 0 {
		s.filled = true
	}
}

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

func (s *requestLogStore) Capacity() int {
	return len(s.entries)
}

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
	}
}

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

func providerForLog(c *gin.Context) string {
	if providerValue, ok := c.Get(ctxProviderKey); ok {
		if provider, ok := providerValue.(config.Provider); ok {
			return provider.Name
		}
	}
	return ""
}

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
