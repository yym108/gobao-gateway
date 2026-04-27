package main

import (
	"context"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/yym/gobao-pkg/authn"
	pkgcfg "github.com/yym/gobao-pkg/config"
	"github.com/yym/gobao-pkg/logger"

	"github.com/yym/gobao-gateway/internal/client"
	"github.com/yym/gobao-gateway/internal/config"
	"github.com/yym/gobao-gateway/internal/handler"
	"github.com/yym/gobao-gateway/internal/router"
)

func main() {
	// 1. 加载配置：默认值适用于 Docker Compose 环境，环境变量 GATEWAY_* 覆盖
	cfg := config.Config{
		Addr:         ":8080",
		UserGRPCAddr: "user:9090",
		JWTSecret:    "gobao-dev-secret-change-in-prod",
		LogLevel:     "info",
	}
	_ = pkgcfg.Load("GATEWAY", "", &cfg)

	// 2. 初始化日志
	log := logger.New("gateway", cfg.LogLevel)
	defer func() { _ = log.Sync() }()

	// 3. 创建 User 服务 gRPC client
	userClient, err := client.NewUserClient(cfg.UserGRPCAddr)
	if err != nil {
		log.Fatal("failed to create user client: " + err.Error())
	}
	defer func() { _ = userClient.Close() }()

	// 4. 创建 JWT 管理器（与 User 服务使用相同的 secret，用于网关本地校验 token）
	jwtMgr := authn.NewJWTManager(cfg.JWTSecret, 24*time.Hour)

	// 5. 创建 Auth Handler（HTTP → gRPC 转发层）
	authHandler := handler.NewAuthHandler(userClient)

	// 6. 注册信号监听，SIGINT/SIGTERM 触发优雅关停
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 7. 启动 HTTP 服务（Gateway 是纯 HTTP，不使用 gRPC）
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           router.New(jwtMgr, authHandler),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Info("gateway listening " + cfg.Addr)
		_ = srv.ListenAndServe()
	}()

	// 8. 等待退出信号，优雅关停
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
