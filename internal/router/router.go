// Package router 提供 gateway 的 Gin 路由配置。
package router

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/yym/gobao-pkg/httpx"
)

// New 创建并返回配置好中间件和路由的 Gin 引擎。
func New() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), httpx.Recover(), httpx.RequestID())
	r.GET("/healthz", func(c *gin.Context) { c.String(200, "ok") })
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	v1 := r.Group("/api/v1")
	v1.GET("/ping", func(c *gin.Context) { c.JSON(200, gin.H{"pong": true}) })
	return r
}
