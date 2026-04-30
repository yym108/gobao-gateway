// Gateway 服务启动入口。
// 负责加载配置、初始化下游 gRPC client、装配 HTTP handler 并启动 HTTP 服务。
package main

import (
	"context"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/yym108/gobao-pkg/authn"
	"github.com/yym108/gobao-pkg/cache"
	pkgcfg "github.com/yym108/gobao-pkg/config"
	"github.com/yym108/gobao-pkg/logger"
	"github.com/yym108/gobao-pkg/mq"

	"github.com/yym108/gobao-gateway/internal/client"
	"github.com/yym108/gobao-gateway/internal/config"
	"github.com/yym108/gobao-gateway/internal/handler"
	"github.com/yym108/gobao-gateway/internal/router"
)

func main() {
	// 1. 加载配置：默认值适用于 Docker Compose 环境，环境变量 GATEWAY_* 覆盖
	cfg := config.Config{
		Addr:            ":8080",
		UserGRPCAddr:    "user:9090",
		ProductGRPCAddr: "product:9090",
		RedisAddr:       "redis:6379",
		RedisDB:         0,
		NATSURL:         "nats://nats:4222",
		NATSStream:      "SECKILL",
		SeckillSubject:  "seckill.order",
		JWTSecret:       "gobao-dev-secret-change-in-prod",
		LogLevel:        "info",
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

	// 4. 创建 Product 服务 gRPC client
	productClient, err := client.NewProductClient(cfg.ProductGRPCAddr)
	if err != nil {
		log.Fatal("failed to create product client: " + err.Error())
	}
	defer func() { _ = productClient.Close() }()

	// 5. 初始化 Redis 与 NATS，供秒杀幂等和异步入队使用
	rdb, err := cache.NewClient(cache.Config{
		Addr: cfg.RedisAddr,
		DB:   cfg.RedisDB,
	})
	if err != nil {
		log.Fatal("failed to create redis client: " + err.Error())
	}
	defer func() { _ = rdb.Close() }()

	bus, err := mq.New(mq.Config{
		URL:      cfg.NATSURL,
		Stream:   cfg.NATSStream,
		Subjects: []string{"seckill.>"},
	})
	if err != nil {
		log.Fatal("failed to create nats bus: " + err.Error())
	}
	defer bus.Close()

	// 6. 创建 JWT 管理器（与 User 服务使用相同的 secret，用于网关本地校验 token）
	jwtMgr := authn.NewJWTManager(cfg.JWTSecret, 24*time.Hour)

	// 7. 创建 HTTP Handler（HTTP → gRPC/Redis/NATS 转发层）
	authHandler := handler.NewAuthHandler(userClient)
	productHandler := handler.NewProductHandler(productClient)
	seckillHandler := handler.NewSeckillHandler(productClient, rdb, bus, cfg.SeckillSubject)

	// 8. 注册信号监听，SIGINT/SIGTERM 触发优雅关停
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 9. 启动 HTTP 服务（Gateway 是纯 HTTP，不使用 gRPC）
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           router.New(jwtMgr, authHandler, productHandler, seckillHandler),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Info("gateway listening " + cfg.Addr)
		_ = srv.ListenAndServe()
	}()

	// 10. 等待退出信号，优雅关停
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
