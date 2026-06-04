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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/yym108/gobao-gateway/internal/middleware"
	"github.com/yym108/gobao-pkg/authn"
	userv1 "github.com/yym108/gobao-proto/gen/go/gobao/user/v1"
)

// mockAddressClient 用于模拟 Gateway 到 User 地址簿服务的 gRPC 调用。
type mockAddressClient struct {
	listFn       func(ctx context.Context, userID int64) (*userv1.ListAddressesResponse, error)
	getFn        func(ctx context.Context, userID, addressID int64) (*userv1.GetAddressResponse, error)
	createFn     func(ctx context.Context, req *userv1.CreateAddressRequest) (*userv1.CreateAddressResponse, error)
	updateFn     func(ctx context.Context, req *userv1.UpdateAddressRequest) (*userv1.UpdateAddressResponse, error)
	deleteFn     func(ctx context.Context, userID, addressID int64) (*userv1.DeleteAddressResponse, error)
	setDefaultFn func(ctx context.Context, userID, addressID int64) (*userv1.SetDefaultAddressResponse, error)
}

func (m *mockAddressClient) ListAddresses(ctx context.Context, userID int64) (*userv1.ListAddressesResponse, error) {
	return m.listFn(ctx, userID)
}

func (m *mockAddressClient) GetAddress(ctx context.Context, userID, addressID int64) (*userv1.GetAddressResponse, error) {
	return m.getFn(ctx, userID, addressID)
}

func (m *mockAddressClient) CreateAddress(ctx context.Context, req *userv1.CreateAddressRequest) (*userv1.CreateAddressResponse, error) {
	return m.createFn(ctx, req)
}

func (m *mockAddressClient) UpdateAddress(ctx context.Context, req *userv1.UpdateAddressRequest) (*userv1.UpdateAddressResponse, error) {
	return m.updateFn(ctx, req)
}

func (m *mockAddressClient) DeleteAddress(ctx context.Context, userID, addressID int64) (*userv1.DeleteAddressResponse, error) {
	return m.deleteFn(ctx, userID, addressID)
}

func (m *mockAddressClient) SetDefaultAddress(ctx context.Context, userID, addressID int64) (*userv1.SetDefaultAddressResponse, error) {
	return m.setDefaultFn(ctx, userID, addressID)
}

func setupAddressRouter(t *testing.T, client addressReader) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	jwtMgr := authn.NewJWTManager("address-handler-test-secret", time.Hour)
	token, _, err := jwtMgr.Sign(1001, "address@test.com")
	require.NoError(t, err)

	h := NewAddressHandler(client)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(middleware.JWTAuth(jwtMgr))
	v1.GET("/addresses", h.ListAddresses)
	v1.GET("/addresses/:id", h.GetAddress)
	v1.POST("/addresses", h.CreateAddress)
	v1.PUT("/addresses/:id", h.UpdateAddress)
	v1.DELETE("/addresses/:id", h.DeleteAddress)
	v1.POST("/addresses/default", h.SetDefaultAddress)
	return r, token
}

func performAddressRequest(r http.Handler, method, path string, body any, token string) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, _ := json.Marshal(body)
		reader = bytes.NewReader(payload)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAddressHandler_ListAddresses_Success(t *testing.T) {
	r, token := setupAddressRouter(t, &mockAddressClient{
		listFn: func(_ context.Context, userID int64) (*userv1.ListAddressesResponse, error) {
			assert.Equal(t, int64(1001), userID)
			return &userv1.ListAddressesResponse{
				Addresses: []*userv1.Address{{Id: 501, ReceiverName: "张三", IsDefault: true}},
			}, nil
		},
		getFn: func(context.Context, int64, int64) (*userv1.GetAddressResponse, error) {
			t.Fatal("unexpected GetAddress call")
			return nil, nil
		},
		createFn: func(context.Context, *userv1.CreateAddressRequest) (*userv1.CreateAddressResponse, error) {
			t.Fatal("unexpected CreateAddress call")
			return nil, nil
		},
		updateFn: func(context.Context, *userv1.UpdateAddressRequest) (*userv1.UpdateAddressResponse, error) {
			t.Fatal("unexpected UpdateAddress call")
			return nil, nil
		},
		deleteFn: func(context.Context, int64, int64) (*userv1.DeleteAddressResponse, error) {
			t.Fatal("unexpected DeleteAddress call")
			return nil, nil
		},
		setDefaultFn: func(context.Context, int64, int64) (*userv1.SetDefaultAddressResponse, error) {
			t.Fatal("unexpected SetDefaultAddress call")
			return nil, nil
		},
	})

	w := performAddressRequest(r, http.MethodGet, "/api/v1/addresses", nil, token)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"receiver_name":"张三"`)
}

func TestAddressHandler_CreateAddress_Success(t *testing.T) {
	r, token := setupAddressRouter(t, &mockAddressClient{
		listFn: func(context.Context, int64) (*userv1.ListAddressesResponse, error) {
			t.Fatal("unexpected ListAddresses call")
			return nil, nil
		},
		getFn: func(context.Context, int64, int64) (*userv1.GetAddressResponse, error) {
			t.Fatal("unexpected GetAddress call")
			return nil, nil
		},
		createFn: func(_ context.Context, req *userv1.CreateAddressRequest) (*userv1.CreateAddressResponse, error) {
			assert.Equal(t, int64(1001), req.GetUserId())
			assert.Equal(t, "张三", req.GetReceiverName())
			return &userv1.CreateAddressResponse{
				Address: &userv1.Address{Id: 501, ReceiverName: "张三"},
			}, nil
		},
		updateFn: func(context.Context, *userv1.UpdateAddressRequest) (*userv1.UpdateAddressResponse, error) {
			t.Fatal("unexpected UpdateAddress call")
			return nil, nil
		},
		deleteFn: func(context.Context, int64, int64) (*userv1.DeleteAddressResponse, error) {
			t.Fatal("unexpected DeleteAddress call")
			return nil, nil
		},
		setDefaultFn: func(context.Context, int64, int64) (*userv1.SetDefaultAddressResponse, error) {
			t.Fatal("unexpected SetDefaultAddress call")
			return nil, nil
		},
	})

	w := performAddressRequest(r, http.MethodPost, "/api/v1/addresses", map[string]any{
		"receiver_name":  "张三",
		"receiver_phone": "13800138000",
		"province":       "上海市",
		"city":           "上海市",
		"district":       "浦东新区",
		"address_line":   "世纪大道1号",
		"postal_code":    "200120",
	}, token)
	require.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), `"id":501`)
}

func TestAddressHandler_SetDefaultAddress_Success(t *testing.T) {
	r, token := setupAddressRouter(t, &mockAddressClient{
		listFn: func(context.Context, int64) (*userv1.ListAddressesResponse, error) {
			t.Fatal("unexpected ListAddresses call")
			return nil, nil
		},
		getFn: func(context.Context, int64, int64) (*userv1.GetAddressResponse, error) {
			t.Fatal("unexpected GetAddress call")
			return nil, nil
		},
		createFn: func(context.Context, *userv1.CreateAddressRequest) (*userv1.CreateAddressResponse, error) {
			t.Fatal("unexpected CreateAddress call")
			return nil, nil
		},
		updateFn: func(context.Context, *userv1.UpdateAddressRequest) (*userv1.UpdateAddressResponse, error) {
			t.Fatal("unexpected UpdateAddress call")
			return nil, nil
		},
		deleteFn: func(context.Context, int64, int64) (*userv1.DeleteAddressResponse, error) {
			t.Fatal("unexpected DeleteAddress call")
			return nil, nil
		},
		setDefaultFn: func(_ context.Context, userID, addressID int64) (*userv1.SetDefaultAddressResponse, error) {
			assert.Equal(t, int64(1001), userID)
			assert.Equal(t, int64(501), addressID)
			return &userv1.SetDefaultAddressResponse{
				Address: &userv1.Address{Id: 501, IsDefault: true},
			}, nil
		},
	})

	w := performAddressRequest(r, http.MethodPost, "/api/v1/addresses/default", map[string]any{
		"address_id": 501,
	}, token)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"is_default":true`)
}

func TestAddressHandler_GetAddress_NotFound(t *testing.T) {
	r, token := setupAddressRouter(t, &mockAddressClient{
		listFn: func(context.Context, int64) (*userv1.ListAddressesResponse, error) {
			t.Fatal("unexpected ListAddresses call")
			return nil, nil
		},
		createFn: func(context.Context, *userv1.CreateAddressRequest) (*userv1.CreateAddressResponse, error) {
			t.Fatal("unexpected CreateAddress call")
			return nil, nil
		},
		updateFn: func(context.Context, *userv1.UpdateAddressRequest) (*userv1.UpdateAddressResponse, error) {
			t.Fatal("unexpected UpdateAddress call")
			return nil, nil
		},
		deleteFn: func(context.Context, int64, int64) (*userv1.DeleteAddressResponse, error) {
			t.Fatal("unexpected DeleteAddress call")
			return nil, nil
		},
		setDefaultFn: func(context.Context, int64, int64) (*userv1.SetDefaultAddressResponse, error) {
			t.Fatal("unexpected SetDefaultAddress call")
			return nil, nil
		},
		getFn: func(_ context.Context, userID, addressID int64) (*userv1.GetAddressResponse, error) {
			assert.Equal(t, int64(1001), userID)
			assert.Equal(t, int64(999), addressID)
			return nil, status.Error(codes.NotFound, "地址不存在")
		},
	})

	w := performAddressRequest(r, http.MethodGet, "/api/v1/addresses/999", nil, token)
	require.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "地址不存在")
}
