package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yym108/gobao-gateway/internal/client"
	"github.com/yym108/gobao-gateway/internal/middleware"
	"github.com/yym108/gobao-pkg/authn"
)

func init() { gin.SetMode(gin.TestMode) }

type stubProductClient struct {
	getProductDetailFn func(ctx context.Context, productID int64) (*client.ProductDetailDTO, error)
}

func (s *stubProductClient) GetProductDetail(ctx context.Context, productID int64) (*client.ProductDetailDTO, error) {
	return s.getProductDetailFn(ctx, productID)
}

func newCartProductStub() *stubProductClient {
	return &stubProductClient{
		getProductDetailFn: func(_ context.Context, productID int64) (*client.ProductDetailDTO, error) {
			switch productID {
			case 1001001, 1001002:
				return &client.ProductDetailDTO{
					Product: client.ProductDTO{
						ID:            productID,
						GroupID:       1001,
						Name:          "MacBook Air",
						ImageURL:      "https://example.com/macbook-air.png",
						Price:         map[int64]int64{1001001: 849900, 1001002: 999900}[productID],
						SpecLabel:     map[int64]string{1001001: "M4 / 16GB / 256GB", 1001002: "M4 / 16GB / 512GB"}[productID],
						Status:        1,
						StockQuantity: 12,
					},
					Group: client.ProductGroupDTO{ID: 1001, Name: "MacBook Air"},
					Variants: []client.ProductVariant{
						{ID: 1001001, Price: 849900, SpecLabel: "M4 / 16GB / 256GB", ImageURL: "https://example.com/macbook-air.png", Status: 1, StockQuantity: 12},
						{ID: 1001002, Price: 999900, SpecLabel: "M4 / 16GB / 512GB", ImageURL: "https://example.com/macbook-air.png", Status: 1, StockQuantity: 9},
					},
					DefaultProductID: 1001001,
				}, nil
			case 1002001, 1002002:
				return &client.ProductDetailDTO{
					Product: client.ProductDTO{
						ID:            productID,
						GroupID:       1002,
						Name:          "iPhone 17 Pro",
						ImageURL:      "https://example.com/iphone17pro.png",
						Price:         map[int64]int64{1002001: 899900, 1002002: 1099900}[productID],
						SpecLabel:     map[int64]string{1002001: "沙漠色 / 256GB", 1002002: "原色 / 512GB"}[productID],
						Status:        1,
						StockQuantity: 8,
					},
					Group: client.ProductGroupDTO{ID: 1002, Name: "iPhone 17 Pro"},
					Variants: []client.ProductVariant{
						{ID: 1002001, Price: 899900, SpecLabel: "沙漠色 / 256GB", ImageURL: "https://example.com/iphone17pro.png", Status: 1, StockQuantity: 8},
						{ID: 1002002, Price: 1099900, SpecLabel: "原色 / 512GB", ImageURL: "https://example.com/iphone17pro.png", Status: 1, StockQuantity: 6},
					},
					DefaultProductID: 1002001,
				}, nil
			}
			return &client.ProductDetailDTO{Product: client.ProductDTO{ID: productID}}, nil
		},
	}
}

// performCartJSONRequest 发送购物车 JSON 请求并返回 recorder。
func performCartJSONRequest(r http.Handler, method, path string, body any, token string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// setupCartTestRouter 构造带 JWT 保护的购物车测试路由。
func setupCartTestRouter(h *CartHandler, jwtMgr *authn.JWTManager) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(middleware.JWTAuth(jwtMgr))
	v1.GET("/cart", h.GetCart)
	v1.POST("/cart/items", h.AddItem)
	v1.PUT("/cart/items/:productId", h.UpdateItem)
	v1.DELETE("/cart/items/:productId", h.DeleteItem)
	return r
}

func TestCartHandler_GetCart_Empty(t *testing.T) {
	jwtMgr := authn.NewJWTManager("cart-test-secret", time.Hour)
	token, _, err := jwtMgr.Sign(101, "cart@test.com")
	require.NoError(t, err)

	h := NewCartHandler(NewMemoryCartStore(), newCartProductStub())
	r := setupCartTestRouter(h, jwtMgr)

	w := performCartJSONRequest(r, http.MethodGet, "/api/v1/cart", nil, token)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"items":[],"total_quantity":0,"total_amount":0}`, w.Body.String())
}

func TestCartHandler_AddItem_Success(t *testing.T) {
	jwtMgr := authn.NewJWTManager("cart-test-secret", time.Hour)
	token, _, err := jwtMgr.Sign(102, "cart@test.com")
	require.NoError(t, err)

	h := NewCartHandler(NewMemoryCartStore(), newCartProductStub())
	r := setupCartTestRouter(h, jwtMgr)

	w := performCartJSONRequest(r, http.MethodPost, "/api/v1/cart/items", map[string]any{
		"product_id": 1001001,
		"quantity":   1,
	}, token)
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), `"product_id":1001001`)
	assert.Contains(t, w.Body.String(), `"cart_item_id":"1001001::`)
	assert.Contains(t, w.Body.String(), `"total_quantity":1`)
	assert.Contains(t, w.Body.String(), `"total_amount":849900`)
	assert.Contains(t, w.Body.String(), `"available":true`)
}

func TestCartHandler_AddItem_ByProductID(t *testing.T) {
	jwtMgr := authn.NewJWTManager("cart-test-secret", time.Hour)
	token, _, err := jwtMgr.Sign(202, "cart@test.com")
	require.NoError(t, err)

	h := NewCartHandler(NewMemoryCartStore(), newCartProductStub())
	r := setupCartTestRouter(h, jwtMgr)

	w := performCartJSONRequest(r, http.MethodPost, "/api/v1/cart/items", map[string]any{
		"product_id": 1002001,
		"quantity":   1,
	}, token)
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), `"product_id":1002001`)
	assert.Contains(t, w.Body.String(), `"price":899900`)
	assert.Contains(t, w.Body.String(), `"option_summary":"沙漠色 / 256GB"`)
}

func TestCartHandler_AddItem_DifferentProductsSeparated(t *testing.T) {
	jwtMgr := authn.NewJWTManager("cart-test-secret", time.Hour)
	token, _, err := jwtMgr.Sign(203, "cart@test.com")
	require.NoError(t, err)

	h := NewCartHandler(NewMemoryCartStore(), newCartProductStub())
	r := setupCartTestRouter(h, jwtMgr)

	performCartJSONRequest(r, http.MethodPost, "/api/v1/cart/items", map[string]any{
		"product_id": 1001001,
		"quantity":   1,
	}, token)

	w := performCartJSONRequest(r, http.MethodPost, "/api/v1/cart/items", map[string]any{
		"product_id": 1001002,
		"quantity":   1,
	}, token)
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), `"total_quantity":2`)
	assert.Contains(t, w.Body.String(), `"product_id":1001001`)
	assert.Contains(t, w.Body.String(), `"product_id":1001002`)
}

func TestCartHandler_AddItem_UsesBackendPriceNotClientPrice(t *testing.T) {
	jwtMgr := authn.NewJWTManager("cart-test-secret", time.Hour)
	token, _, err := jwtMgr.Sign(204, "cart@test.com")
	require.NoError(t, err)

	h := NewCartHandler(NewMemoryCartStore(), newCartProductStub())
	r := setupCartTestRouter(h, jwtMgr)

	w := performCartJSONRequest(r, http.MethodPost, "/api/v1/cart/items", map[string]any{
		"product_id": 1002001,
		"quantity":   1,
		"price":      1,
	}, token)
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), `"price":899900`)
	assert.NotContains(t, w.Body.String(), `"price":1`)
}

func TestCartHandler_AddItem_DifferentOptionsSeparated(t *testing.T) {
	jwtMgr := authn.NewJWTManager("cart-test-secret", time.Hour)
	token, _, err := jwtMgr.Sign(106, "cart@test.com")
	require.NoError(t, err)

	h := NewCartHandler(NewMemoryCartStore(), newCartProductStub())
	r := setupCartTestRouter(h, jwtMgr)

	performCartJSONRequest(r, http.MethodPost, "/api/v1/cart/items", map[string]any{
		"product_id": 1001001,
		"quantity":   1,
	}, token)

	w := performCartJSONRequest(r, http.MethodPost, "/api/v1/cart/items", map[string]any{
		"product_id": 1001002,
		"quantity":   1,
	}, token)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), `"total_quantity":2`)
	assert.Contains(t, w.Body.String(), `"items":[`)
	assert.Contains(t, w.Body.String(), `"cart_item_id":"1001001::TTQgLyAxNkdCIC8gMjU2R0I"`)
	assert.Contains(t, w.Body.String(), `"cart_item_id":"1001002::TTQgLyAxNkdCIC8gNTEyR0I"`)
}

func TestCartHandler_UpdateItem_Success(t *testing.T) {
	jwtMgr := authn.NewJWTManager("cart-test-secret", time.Hour)
	token, _, err := jwtMgr.Sign(103, "cart@test.com")
	require.NoError(t, err)

	store := NewMemoryCartStore()
	h := NewCartHandler(store, newCartProductStub())
	r := setupCartTestRouter(h, jwtMgr)

	performCartJSONRequest(r, http.MethodPost, "/api/v1/cart/items", map[string]any{
		"product_id": 1002001,
		"quantity":   1,
	}, token)

	w := performCartJSONRequest(r, http.MethodPut, "/api/v1/cart/items/1002001::5rKZ5ryg6ImyIC8gMjU2R0I", map[string]any{
		"quantity": 3,
	}, token)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"quantity":3`)
	assert.Contains(t, w.Body.String(), `"total_quantity":3`)
	assert.Contains(t, w.Body.String(), `"total_amount":2699700`)
}

func TestCartHandler_DeleteItem_Success(t *testing.T) {
	jwtMgr := authn.NewJWTManager("cart-test-secret", time.Hour)
	token, _, err := jwtMgr.Sign(104, "cart@test.com")
	require.NoError(t, err)

	store := NewMemoryCartStore()
	h := NewCartHandler(store, newCartProductStub())
	r := setupCartTestRouter(h, jwtMgr)

	performCartJSONRequest(r, http.MethodPost, "/api/v1/cart/items", map[string]any{
		"product_id": 1002001,
		"quantity":   1,
	}, token)

	w := performCartJSONRequest(r, http.MethodDelete, "/api/v1/cart/items/1002001::5rKZ5ryg6ImyIC8gMjU2R0I", nil, token)
	assert.Equal(t, http.StatusNoContent, w.Code)

	after := performCartJSONRequest(r, http.MethodGet, "/api/v1/cart", nil, token)
	assert.Equal(t, http.StatusOK, after.Code)
	assert.JSONEq(t, `{"items":[],"total_quantity":0,"total_amount":0}`, after.Body.String())
}

// TestCartHandler_UpdateItem_NotFound 验证更新不存在的购物车商品时返回 404。
func TestCartHandler_UpdateItem_NotFound(t *testing.T) {
	jwtMgr := authn.NewJWTManager("cart-test-secret", time.Hour)
	token, _, err := jwtMgr.Sign(105, "cart@test.com")
	require.NoError(t, err)

	h := NewCartHandler(NewMemoryCartStore(), newCartProductStub())
	r := setupCartTestRouter(h, jwtMgr)

	w := performCartJSONRequest(r, http.MethodPut, "/api/v1/cart/items/9999", map[string]any{
		"quantity": 2,
	}, token)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "购物车商品不存在")
}

// TestCartHandler_GetCart_RefreshesUnavailableState 验证购物车查询会按后端最新状态刷新可售性。
func TestCartHandler_GetCart_RefreshesUnavailableState(t *testing.T) {
	jwtMgr := authn.NewJWTManager("cart-test-secret", time.Hour)
	token, _, err := jwtMgr.Sign(305, "cart@test.com")
	require.NoError(t, err)

	productClient := newCartProductStub()
	h := NewCartHandler(NewMemoryCartStore(), productClient)
	r := setupCartTestRouter(h, jwtMgr)

	addResp := performCartJSONRequest(r, http.MethodPost, "/api/v1/cart/items", map[string]any{
		"product_id": 1001001,
		"quantity":   1,
	}, token)
	assert.Equal(t, http.StatusCreated, addResp.Code)

	productClient.getProductDetailFn = func(_ context.Context, productID int64) (*client.ProductDetailDTO, error) {
		if productID != 1001001 {
			return &client.ProductDetailDTO{Product: client.ProductDTO{ID: productID}}, nil
		}
		return &client.ProductDetailDTO{
			Product: client.ProductDTO{
				ID:            1001001,
				GroupID:       1001,
				Name:          "MacBook Air",
				ImageURL:      "https://example.com/macbook-air.png",
				Price:         849900,
				SpecLabel:     "M4 / 16GB / 256GB",
				Status:        2,
				StockQuantity: 12,
			},
			Group: client.ProductGroupDTO{ID: 1001, Name: "MacBook Air"},
			Variants: []client.ProductVariant{
				{ID: 1001001, Price: 849900, SpecLabel: "M4 / 16GB / 256GB", ImageURL: "https://example.com/macbook-air.png", Status: 2, StockQuantity: 12},
			},
			DefaultProductID: 1001001,
		}, nil
	}

	getResp := performCartJSONRequest(r, http.MethodGet, "/api/v1/cart", nil, token)
	assert.Equal(t, http.StatusOK, getResp.Code)
	assert.Contains(t, getResp.Body.String(), `"available":false`)
	assert.Contains(t, getResp.Body.String(), `"unavailable_reason":"当前商品已下架，暂时无法购买"`)
	assert.Contains(t, getResp.Body.String(), `"status":2`)
}

// TestBuildCartItemID_URLSafe 验证购物车条目 ID 可安全放入 URL 路径中，不包含斜杠等分隔符。
func TestBuildCartItemID_URLSafe(t *testing.T) {
	cartItemID := buildCartItemID(2001, "深空灰 / 512GB / 标准版")
	assert.Equal(t, "2001::5rex56m654GwIC8gNTEyR0IgLyDmoIflh4bniYg", cartItemID)
	assert.NotContains(t, cartItemID, "/")
	assert.NotContains(t, cartItemID, " ")
}
