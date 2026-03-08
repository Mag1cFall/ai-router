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

type providerHTTPPool struct {
	transport *http.Transport
	clients   sync.Map
}

var sharedHTTPPools sync.Map

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

func providerPoolKey(provider config.Provider) string {
	return fmt.Sprintf("%s|%s|%s", provider.Name, provider.Protocol, provider.Endpoint)
}

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
