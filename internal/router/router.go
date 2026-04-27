// Package router 提供 Gateway 的 Gin 路由配置。
// 路由分为公开路由（无需鉴权）和受保护路由（需 JWT 中间件校验），
// 实现 Client → Gateway(JWT) → User Service(gRPC) 的鉴权链路。
package router

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/yym/gobao-gateway/internal/handler"
	"github.com/yym/gobao-gateway/internal/middleware"
	"github.com/yym/gobao-pkg/authn"
	"github.com/yym/gobao-pkg/httpx"
)

// New 创建并返回配置好中间件和路由的 Gin 引擎。
//   - jwtMgr:      JWT 管理器，用于受保护路由的鉴权中间件
//   - authHandler: 认证相关的 HTTP handler（注册/登录/获取当前用户）
func New(jwtMgr *authn.JWTManager, authHandler *handler.AuthHandler) *gin.Engine {
	r := gin.New()

	// 全局中间件：日志、panic 恢复、请求 ID 注入
	r.Use(gin.Logger(), httpx.Recover(), httpx.RequestID())

	// 基础运维路由（不在 /api/v1 下，不受鉴权保护）
	r.GET("/healthz", func(c *gin.Context) { c.String(200, "ok") })
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	v1 := r.Group("/api/v1")

	// ── 公开路由（无需 JWT） ──
	pub := v1.Group("/auth")
	pub.POST("/register", authHandler.Register) // 用户注册
	pub.POST("/login", authHandler.Login)       // 用户登录

	// ── 受保护路由（需 JWT 中间件校验） ──
	protected := v1.Group("")
	protected.Use(middleware.JWTAuth(jwtMgr))
	protected.GET("/auth/me", authHandler.GetMe) // 获取当前用户信息
	protected.GET("/ping", func(c *gin.Context) {
		// 返回 pong + 当前登录用户 ID，用于验证鉴权链路
		c.JSON(200, gin.H{"pong": true, "user_id": c.GetInt64("userID")})
	})

	return r
}
