// 程序入口：加载配置、启动 HTTP 服务并监听优雅关闭信号
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Mag1cFall/ai-router/api/internal/config"
	"github.com/Mag1cFall/ai-router/api/internal/handler"
	"github.com/gin-gonic/gin"
)

func main() {
	// 优先从环境变量读取配置路径，默认 config.yaml
	configPath := strings.TrimSpace(os.Getenv("AI_ROUTER_CONFIG"))
	if configPath == "" {
		configPath = "config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load %s: %v", configPath, err)
	}

	if strings.EqualFold(cfg.Server.LogLevel, "debug") {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	handler.RegisterRoutes(r, cfg)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	if cfg.Server.Host == "" {
		addr = fmt.Sprintf(":%d", cfg.Server.Port)
	}

	// 打印已加载的 provider 列表
	providerNames := make([]string, 0, len(cfg.Providers))
	for _, provider := range cfg.Providers {
		providerNames = append(providerNames, fmt.Sprintf("%s(%s)", provider.Name, provider.Protocol))
	}
	if len(providerNames) == 0 {
		providerNames = append(providerNames, "none")
	}

	server := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	shutdownSignal, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("server listening on %s providers=%s", addr, strings.Join(providerNames, ", "))
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	select {
	case err := <-serverErr:
		if err != nil {
			log.Fatalf("run server: %v", err)
		}
		return
	case <-shutdownSignal.Done():
	}

	// 收到信号后等待 10s 让进行中的请求完成
	log.Printf("shutdown signal received, stopping server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("shutdown server: %v", err)
	}
	if err := <-serverErr; err != nil {
		log.Fatalf("run server: %v", err)
	}
	log.Printf("server stopped")
}
