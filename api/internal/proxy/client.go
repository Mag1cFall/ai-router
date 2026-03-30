// HTTP 客户端池管理：每个 Provider 独立 Transport，根据超时时间复用客户端
package proxy

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Mag1cFall/ai-router/api/internal/config"
)

const (
	requestTimeoutEnv       = "AI_ROUTER_REQUEST_TIMEOUT"
	defaultRequestTimeout   = 60 * time.Second
	defaultMaxIdleConns     = 100
	defaultMaxIdleConnsHost = 20
	defaultIdleConnTimeout  = 90 * time.Second
)

// providerHTTPPool 单个 Provider 的传输层和多超时客户端池
type providerHTTPPool struct {
	transport *http.Transport
	clients   sync.Map
}

// sharedHTTPPools 全局 Provider -> 连接池映射
var sharedHTTPPools sync.Map

// clientForProvider 返回指定 Provider 和超时的 HTTP 客户端，内部共享 Transport
func clientForProvider(provider config.Provider, timeout time.Duration) *http.Client {
	poolValue, _ := sharedHTTPPools.LoadOrStore(providerPoolKey(provider), &providerHTTPPool{
		transport: newProviderTransport(),
	})
	pool := poolValue.(*providerHTTPPool)

	clientKey := timeout.String()
	clientValue, _ := pool.clients.LoadOrStore(clientKey, &http.Client{
		Transport: pool.transport,
		Timeout:   timeout,
	})
	return clientValue.(*http.Client)
}

// requestTimeout 确定请求超时：优先使用 Request.Timeout，其次环境变量，流式请求不超时
func requestTimeout(req Request) time.Duration {
	if req.Timeout > 0 {
		return req.Timeout
	}
	if value := strings.TrimSpace(os.Getenv(requestTimeoutEnv)); value != "" {
		if timeout, err := time.ParseDuration(value); err == nil && timeout > 0 {
			return timeout
		}
	}
	if req.Stream {
		return 0
	}
	return defaultRequestTimeout
}

// providerPoolKey 生成 Provider 的连接池唯一键
func providerPoolKey(provider config.Provider) string {
	return fmt.Sprintf("%s|%s|%s", provider.Name, provider.Protocol, provider.Endpoint)
}

// newProviderTransport 创建适配 AI 接口的 HTTP Transport，支持代理和 HTTP/2
func newProviderTransport() *http.Transport {
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          defaultMaxIdleConns,
		MaxIdleConnsPerHost:   defaultMaxIdleConnsHost,
		IdleConnTimeout:       defaultIdleConnTimeout,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}
