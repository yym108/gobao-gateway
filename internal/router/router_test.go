package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/yym108/gobao-gateway/internal/handler"
	"github.com/yym108/gobao-pkg/authn"
)

func init() { gin.SetMode(gin.TestMode) }

// TestNew_AllowsCORSPreflight 验证浏览器跨域预检请求能在 Gateway 层直接返回 204 与允许头。
func TestNew_AllowsCORSPreflight(t *testing.T) {
	jwtMgr := authn.NewJWTManager("router-test-secret", time.Hour)
	r := New(
		jwtMgr,
		&handler.AuthHandler{},
		&handler.AdminAuthHandler{},
		&handler.AddressHandler{},
		&handler.ProductHandler{},
		&handler.AdminMediaHandler{},
		&handler.SeckillHandler{},
		handler.NewCartHandler(handler.NewMemoryCartStore(), nil),
		handler.NewOrderHandler(nil, nil),
		handler.NewPaymentHandler(nil),
		handler.NewFavoriteHandler(handler.NewMemoryFavoriteStore(), nil),
		false,
	)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/products?page=1&page_size=4", nil)
	req.Header.Set("Origin", "http://localhost:5174")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "http://localhost:5174", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")
}

// TestNew_AddsCORSHeadersOnPublicAPI 验证公开接口响应带上允许跨域头，供浏览器读取商品列表。
func TestNew_AddsCORSHeadersOnPublicAPI(t *testing.T) {
	jwtMgr := authn.NewJWTManager("router-test-secret", time.Hour)
	r := New(
		jwtMgr,
		&handler.AuthHandler{},
		&handler.AdminAuthHandler{},
		&handler.AddressHandler{},
		&handler.ProductHandler{},
		&handler.AdminMediaHandler{},
		&handler.SeckillHandler{},
		handler.NewCartHandler(handler.NewMemoryCartStore(), nil),
		handler.NewOrderHandler(nil, nil),
		handler.NewPaymentHandler(nil),
		handler.NewFavoriteHandler(handler.NewMemoryFavoriteStore(), nil),
		false,
	)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "http://localhost:5174")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "http://localhost:5174", w.Header().Get("Access-Control-Allow-Origin"))
}
