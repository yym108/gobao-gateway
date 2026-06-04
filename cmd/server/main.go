// Gateway 服务启动入口。
// 负责加载配置、初始化下游 gRPC client、装配 HTTP handler 并启动 HTTP 服务。
package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
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
		AdminGRPCAddr:   "admin:9090",
		ProductGRPCAddr: "product:9090",
		OrderGRPCAddr:   "order:9090",
		PaymentGRPCAddr: "payment:9090",
		MySQLDSN:        "root:root@tcp(mysql-gateway:3306)/gateway?charset=utf8mb4&parseTime=True&loc=Local",
		RedisAddr:       "redis:6379",
		RedisDB:         0,
		NATSURL:         "nats://nats:4222",
		NATSStream:      "SECKILL",
		SeckillSubject:  "seckill.order",
		JWTSecret:       "gobao-dev-secret-change-in-prod",
		LogLevel:        "info",
	}
	if err := pkgcfg.Load("GATEWAY", "", &cfg); err != nil {
		panic("load gateway config: " + err.Error())
	}

	// 2. 初始化日志
	log := logger.New("gateway", cfg.LogLevel)
	defer func() { _ = log.Sync() }()

	// 3. 创建 User 服务 gRPC client
	userClient, err := client.NewUserClient(cfg.UserGRPCAddr)
	if err != nil {
		log.Fatal("failed to create user client: " + err.Error())
	}
	defer func() { _ = userClient.Close() }()

	adminClient, err := client.NewAdminClient(cfg.AdminGRPCAddr)
	if err != nil {
		log.Fatal("failed to create admin client: " + err.Error())
	}
	defer func() { _ = adminClient.Close() }()

	// 4. 创建 Product 服务 gRPC client
	productClient, err := client.NewProductClient(cfg.ProductGRPCAddr)
	if err != nil {
		log.Fatal("failed to create product client: " + err.Error())
	}
	defer func() { _ = productClient.Close() }()

	orderClient, err := client.NewOrderClient(cfg.OrderGRPCAddr)
	if err != nil {
		log.Fatal("failed to create order client: " + err.Error())
	}
	defer func() { _ = orderClient.Close() }()

	paymentClient, err := client.NewPaymentClient(cfg.PaymentGRPCAddr)
	if err != nil {
		log.Fatal("failed to create payment client: " + err.Error())
	}
	defer func() { _ = paymentClient.Close() }()

	// 5. 创建 Gateway 自身 MySQL 连接，承接收藏等持久化数据
	mysqlDB, err := sql.Open("mysql", cfg.MySQLDSN)
	if err != nil {
		log.Fatal("failed to open gateway mysql: " + err.Error())
	}
	defer func() { _ = mysqlDB.Close() }()
	mysqlDB.SetConnMaxLifetime(30 * time.Minute)
	mysqlDB.SetMaxOpenConns(10)
	mysqlDB.SetMaxIdleConns(5)
	if err := waitForMySQL(context.Background(), 20, 2*time.Second, mysqlDB.PingContext); err != nil {
		log.Fatal("failed to ping gateway mysql: " + err.Error())
	}

	// 6. 初始化 Redis 与 NATS，供秒杀幂等和异步入队使用
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

	// 7. 创建 JWT 管理器（与 User 服务使用相同的 secret，用于网关本地校验 token）
	jwtMgr := authn.NewJWTManager(cfg.JWTSecret, 24*time.Hour)

	// 8. 创建 HTTP Handler（HTTP → gRPC/DB/Redis/NATS 转发层）
	authHandler := handler.NewAuthHandler(userClient)
	adminAuthHandler := handler.NewAdminAuthHandler(adminClient)
	addressHandler := handler.NewAddressHandler(userClient)
	productHandler := handler.NewProductHandler(productClient)
	adminMediaHandler := handler.NewAdminMediaHandler(productClient)
	seckillHandler := handler.NewSeckillHandler(productClient, rdb, bus, cfg.SeckillSubject)
	cartHandler := handler.NewCartHandler(handler.NewRedisCartStore(rdb), productClient)
	orderHandler := handler.NewOrderHandler(orderClient, userClient)
	paymentHandler := handler.NewPaymentHandler(paymentClient)
	favoriteStore, err := handler.NewMySQLFavoriteStore(mysqlDB)
	if err != nil {
		log.Fatal("failed to create favorite store: " + err.Error())
	}
	favoriteHandler := handler.NewFavoriteHandler(favoriteStore, productClient)

	// 9. 注册信号监听，SIGINT/SIGTERM 触发优雅关停
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 10. 启动 HTTP 服务（Gateway 是纯 HTTP，不使用 gRPC）
	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           router.New(jwtMgr, authHandler, adminAuthHandler, addressHandler, productHandler, adminMediaHandler, seckillHandler, cartHandler, orderHandler, paymentHandler, favoriteHandler, cfg.ExposeDevEndpoints),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Info("gateway listening " + cfg.Addr)
		_ = srv.ListenAndServe()
	}()

	// 11. 等待退出信号，优雅关停
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

// waitForMySQL 在服务启动阶段等待 MySQL 就绪。
// 这里显式重试固定次数，避免容器编排中数据库稍慢启动时网关直接退出。
func waitForMySQL(ctx context.Context, maxAttempts int, interval time.Duration, ping func(context.Context) error) error {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ping(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt == maxAttempts {
			break
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			if lastErr != nil {
				return errors.Join(ctx.Err(), lastErr)
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
	return lastErr
}
