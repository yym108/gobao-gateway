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

/**
 * 收藏商品读取桩复用 Product 详情查询接口，
 * 让收藏后端继续以后端商品快照为准，而不是信任前端透传的名称与价格。
 */
type stubFavoriteProductClient struct {
	getProductDetailFn func(ctx context.Context, productID int64) (*client.ProductDetailDTO, error)
}

func (s *stubFavoriteProductClient) GetProductDetail(ctx context.Context, productID int64) (*client.ProductDetailDTO, error) {
	return s.getProductDetailFn(ctx, productID)
}

/**
 * 收藏测试桩提供两个固定商品，方便验证增删查和列表排序行为。
 */
func newFavoriteProductStub() *stubFavoriteProductClient {
	return &stubFavoriteProductClient{
		getProductDetailFn: func(_ context.Context, productID int64) (*client.ProductDetailDTO, error) {
			switch productID {
			case 1001:
				return &client.ProductDetailDTO{
					Product: client.ProductDTO{
						ID:          1001,
						Name:        "MacBook Air",
						Description: "轻薄高性能笔记本",
						Price:       999900,
						CategoryID:  1,
						ImageURL:    "https://example.com/macbook-air.png",
						Status:      1,
					},
				}, nil
			case 1002:
				return &client.ProductDetailDTO{
					Product: client.ProductDTO{
						ID:          1002,
						Name:        "iPhone 17 Pro",
						Description: "旗舰手机",
						Price:       899900,
						CategoryID:  2,
						ImageURL:    "https://example.com/iphone17pro.png",
						Status:      1,
					},
				}, nil
			case 1003:
				// 通过商品组媒体绑定上图的新商品：自身 image_url 为空，图片在商品组封面上
				return &client.ProductDetailDTO{
					Product: client.ProductDTO{
						ID:          1003,
						Name:        "Studio Display",
						Description: "媒体绑定封面",
						Price:       1299900,
						CategoryID:  1,
						ImageURL:    "",
						Status:      1,
					},
					Group: client.ProductGroupDTO{
						CoverImageURL: "/media/groups/9/cover.jpg",
					},
				}, nil
			default:
				return nil, assert.AnError
			}
		},
	}
}

/**
 * performFavoriteJSONRequest 发送收藏 JSON 请求并返回 recorder。
 */
func performFavoriteJSONRequest(r http.Handler, method, path string, body any, token string) *httptest.ResponseRecorder {
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

/**
 * setupFavoriteTestRouter 构造仅承载收藏接口的受保护测试路由。
 */
func setupFavoriteTestRouter(h *FavoriteHandler, jwtMgr *authn.JWTManager) *gin.Engine {
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(middleware.JWTAuth(jwtMgr))
	v1.GET("/favorites", h.ListFavorites)
	v1.POST("/favorites", h.AddFavorite)
	v1.DELETE("/favorites/:productId", h.DeleteFavorite)
	return r
}

func TestFavoriteHandler_ListFavorites_Empty(t *testing.T) {
	jwtMgr := authn.NewJWTManager("favorite-test-secret", time.Hour)
	token, _, err := jwtMgr.Sign(301, "favorite@test.com")
	require.NoError(t, err)

	h := NewFavoriteHandler(NewMemoryFavoriteStore(), newFavoriteProductStub())
	r := setupFavoriteTestRouter(h, jwtMgr)

	w := performFavoriteJSONRequest(r, http.MethodGet, "/api/v1/favorites", nil, token)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"items":[],"total":0}`, w.Body.String())
}

func TestFavoriteHandler_AddFavorite_Success(t *testing.T) {
	jwtMgr := authn.NewJWTManager("favorite-test-secret", time.Hour)
	token, _, err := jwtMgr.Sign(302, "favorite@test.com")
	require.NoError(t, err)

	h := NewFavoriteHandler(NewMemoryFavoriteStore(), newFavoriteProductStub())
	r := setupFavoriteTestRouter(h, jwtMgr)

	w := performFavoriteJSONRequest(r, http.MethodPost, "/api/v1/favorites", map[string]any{
		"product_id": 1001,
	}, token)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), `"product_id":1001`)
	assert.Contains(t, w.Body.String(), `"name":"MacBook Air"`)
	assert.Contains(t, w.Body.String(), `"total":1`)
}

func TestFavoriteHandler_AddFavorite_DeduplicatesSameProduct(t *testing.T) {
	jwtMgr := authn.NewJWTManager("favorite-test-secret", time.Hour)
	token, _, err := jwtMgr.Sign(303, "favorite@test.com")
	require.NoError(t, err)

	h := NewFavoriteHandler(NewMemoryFavoriteStore(), newFavoriteProductStub())
	r := setupFavoriteTestRouter(h, jwtMgr)

	performFavoriteJSONRequest(r, http.MethodPost, "/api/v1/favorites", map[string]any{"product_id": 1001}, token)
	w := performFavoriteJSONRequest(r, http.MethodPost, "/api/v1/favorites", map[string]any{"product_id": 1001}, token)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), `"total":1`)
}

func TestFavoriteHandler_ListFavorites_FallsBackToGroupCover(t *testing.T) {
	jwtMgr := authn.NewJWTManager("favorite-test-secret", time.Hour)
	token, _, err := jwtMgr.Sign(310, "favorite@test.com")
	require.NoError(t, err)

	h := NewFavoriteHandler(NewMemoryFavoriteStore(), newFavoriteProductStub())
	r := setupFavoriteTestRouter(h, jwtMgr)

	performFavoriteJSONRequest(r, http.MethodPost, "/api/v1/favorites", map[string]any{"product_id": 1003}, token)
	w := performFavoriteJSONRequest(r, http.MethodGet, "/api/v1/favorites", nil, token)

	assert.Equal(t, http.StatusOK, w.Code)
	// 商品自身 image_url 为空时，应回退到商品组封面，保证收藏卡能显示图片
	assert.Contains(t, w.Body.String(), `"image_url":"/media/groups/9/cover.jpg"`)
}

func TestFavoriteHandler_ListFavorites_ReturnsLatestFirst(t *testing.T) {
	jwtMgr := authn.NewJWTManager("favorite-test-secret", time.Hour)
	token, _, err := jwtMgr.Sign(304, "favorite@test.com")
	require.NoError(t, err)

	h := NewFavoriteHandler(NewMemoryFavoriteStore(), newFavoriteProductStub())
	r := setupFavoriteTestRouter(h, jwtMgr)

	performFavoriteJSONRequest(r, http.MethodPost, "/api/v1/favorites", map[string]any{"product_id": 1001}, token)
	performFavoriteJSONRequest(r, http.MethodPost, "/api/v1/favorites", map[string]any{"product_id": 1002}, token)
	w := performFavoriteJSONRequest(r, http.MethodGet, "/api/v1/favorites", nil, token)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"total":2`)
	assert.True(t, bytes.Index(w.Body.Bytes(), []byte(`"product_id":1002`)) < bytes.Index(w.Body.Bytes(), []byte(`"product_id":1001`)))
}

func TestFavoriteHandler_ListFavorites_MarksUnavailableProduct(t *testing.T) {
	jwtMgr := authn.NewJWTManager("favorite-test-secret", time.Hour)
	token, _, err := jwtMgr.Sign(306, "favorite@test.com")
	require.NoError(t, err)

	productStub := &stubFavoriteProductClient{
		getProductDetailFn: func(_ context.Context, productID int64) (*client.ProductDetailDTO, error) {
			if productID == 1001 {
				return &client.ProductDetailDTO{
					Product: client.ProductDTO{
						ID:          1001,
						Name:        "MacBook Air",
						Description: "轻薄高性能笔记本",
						Price:       999900,
						CategoryID:  1,
						ImageURL:    "https://example.com/macbook-air.png",
						Status:      1,
					},
				}, nil
			}
			return nil, assert.AnError
		},
	}

	h := NewFavoriteHandler(NewMemoryFavoriteStore(), productStub)
	r := setupFavoriteTestRouter(h, jwtMgr)

	performFavoriteJSONRequest(r, http.MethodPost, "/api/v1/favorites", map[string]any{"product_id": 1001}, token)
	store := h.store.(*MemoryFavoriteStore)
	_, err = store.Add(context.Background(), 306, FavoriteItem{ProductID: 9999, FavoritedAt: time.Now().Unix()})
	require.NoError(t, err)

	w := performFavoriteJSONRequest(r, http.MethodGet, "/api/v1/favorites", nil, token)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"product_id":9999`)
	assert.Contains(t, w.Body.String(), `"available":false`)
	assert.Contains(t, w.Body.String(), `"unavailable_reason":"该商品已不存在或已下线"`)
}

func TestFavoriteHandler_DeleteFavorite_Success(t *testing.T) {
	jwtMgr := authn.NewJWTManager("favorite-test-secret", time.Hour)
	token, _, err := jwtMgr.Sign(305, "favorite@test.com")
	require.NoError(t, err)

	h := NewFavoriteHandler(NewMemoryFavoriteStore(), newFavoriteProductStub())
	r := setupFavoriteTestRouter(h, jwtMgr)

	performFavoriteJSONRequest(r, http.MethodPost, "/api/v1/favorites", map[string]any{"product_id": 1001}, token)
	w := performFavoriteJSONRequest(r, http.MethodDelete, "/api/v1/favorites/1001", nil, token)
	assert.Equal(t, http.StatusNoContent, w.Code)

	after := performFavoriteJSONRequest(r, http.MethodGet, "/api/v1/favorites", nil, token)
	assert.JSONEq(t, `{"items":[],"total":0}`, after.Body.String())
}

func TestFavoriteHandler_AddFavorite_RequiresJWT(t *testing.T) {
	jwtMgr := authn.NewJWTManager("favorite-test-secret", time.Hour)
	h := NewFavoriteHandler(NewMemoryFavoriteStore(), newFavoriteProductStub())
	r := setupFavoriteTestRouter(h, jwtMgr)

	w := performFavoriteJSONRequest(r, http.MethodPost, "/api/v1/favorites", map[string]any{"product_id": 1001}, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
