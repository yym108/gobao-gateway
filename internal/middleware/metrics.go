// Package middleware 的本文件提供 Gateway 的 Prometheus HTTP 监控中间件。
// 统一暴露 HTTP RED 指标（请求量 / 错误 / 耗时），经 /metrics 端点供 Prometheus 抓取。
package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// httpRequestsTotal 记录 HTTP 服务端按服务、方法、路由模板与状态码累计处理的请求数。
// route 使用 gin 路由模板（如 /api/v1/orders）而非真实路径，避免路径参数造成标签高基数。
var httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "http_server_requests_total",
	Help: "HTTP 服务端按方法、路由与状态码累计处理的请求数。",
}, []string{"service", "method", "route", "status"})

// httpRequestDuration 记录 HTTP 服务端请求处理耗时分布（秒），使用默认分桶。
var httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "http_server_request_duration_seconds",
	Help:    "HTTP 服务端请求处理耗时分布（秒）。",
	Buckets: prometheus.DefBuckets,
}, []string{"service", "method", "route"})

// Metrics 返回一个 Gin 中间件，记录每个请求的请求量、状态码与耗时。
// 应作为靠前的全局中间件挂载，以覆盖尽量多的请求。
//   - service: 当前服务名，作为指标 service 标签
func Metrics(service string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		// 用路由模板做标签；未匹配任何路由（404）时归并到 unmatched，避免高基数。
		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		status := strconv.Itoa(c.Writer.Status())
		httpRequestsTotal.WithLabelValues(service, c.Request.Method, route, status).Inc()
		httpRequestDuration.WithLabelValues(service, c.Request.Method, route).Observe(time.Since(start).Seconds())
	}
}
