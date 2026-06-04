package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

// newMetricsEngine 构造一个挂载 Metrics 中间件与一条测试路由的 gin 引擎。
func newMetricsEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Metrics("gateway-test"))
	r.GET("/api/v1/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })
	return r
}

// TestMetrics_countsByRouteAndStatus 验证命中路由会按 route 模板与状态码累加计数。
func TestMetrics_countsByRouteAndStatus(t *testing.T) {
	r := newMetricsEngine()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	got := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("gateway-test", "GET", "/api/v1/ping", "200"))
	assert.Equal(t, float64(1), got, "命中路由应按模板与状态码累加计数")
}

// TestMetrics_unmatchedRoute 验证未匹配路由用占位 route，避免高基数。
func TestMetrics_unmatchedRoute(t *testing.T) {
	r := newMetricsEngine()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/no/such/path/xyz", nil)
	r.ServeHTTP(w, req)

	got := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("gateway-test", "GET", "unmatched", "404"))
	assert.Equal(t, float64(1), got, "未匹配路由应归并到 unmatched")
}

// TestMetrics_duration 验证耗时直方图记录到观测样本。
func TestMetrics_duration(t *testing.T) {
	r := newMetricsEngine()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	r.ServeHTTP(w, req)

	count := testutil.CollectAndCount(httpRequestDuration)
	assert.GreaterOrEqual(t, count, 1, "耗时直方图应记录到观测样本")
}
