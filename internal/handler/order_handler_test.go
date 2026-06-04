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
	orderv1 "github.com/yym108/gobao-proto/gen/go/gobao/order/v1"
	userv1 "github.com/yym108/gobao-proto/gen/go/gobao/user/v1"
)

// mockOrderCreator 用函数桩模拟 Gateway 到 Order 服务的 gRPC 调用。
type mockOrderCreator struct {
	createOrderFn func(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error)
	getOrderFn    func(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error)
	listOrdersFn  func(ctx context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error)
	cancelOrderFn func(ctx context.Context, req *orderv1.CancelOrderRequest) (*orderv1.CancelOrderResponse, error)
	adminListFn   func(ctx context.Context, req *orderv1.AdminListOrdersRequest) (*orderv1.ListOrdersResponse, error)
	adminGetFn    func(ctx context.Context, req *orderv1.AdminGetOrderRequest) (*orderv1.GetOrderResponse, error)
	adminCancelFn func(ctx context.Context, req *orderv1.AdminCancelOrderRequest) (*orderv1.CancelOrderResponse, error)
}

// mockOrderAddressReader 模拟下单前向 User 服务查询地址快照与按邮箱解析用户。
type mockOrderAddressReader struct {
	getAddressFn      func(ctx context.Context, userID, addressID int64) (*userv1.GetAddressResponse, error)
	findUserByEmailFn func(ctx context.Context, email string) (*userv1.FindUserByEmailResponse, error)
}

// CreateOrder 执行测试桩定义的下单逻辑。
func (m *mockOrderCreator) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
	return m.createOrderFn(ctx, req)
}

// GetOrder 执行测试桩定义的单笔查单逻辑。
func (m *mockOrderCreator) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
	return m.getOrderFn(ctx, req)
}

// ListOrders 执行测试桩定义的分页列单逻辑。
func (m *mockOrderCreator) ListOrders(ctx context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	return m.listOrdersFn(ctx, req)
}

// CancelOrder 执行测试桩定义的取消订单逻辑。
func (m *mockOrderCreator) CancelOrder(ctx context.Context, req *orderv1.CancelOrderRequest) (*orderv1.CancelOrderResponse, error) {
	return m.cancelOrderFn(ctx, req)
}

// AdminListOrders 执行测试桩定义的管理员列单逻辑。
func (m *mockOrderCreator) AdminListOrders(ctx context.Context, req *orderv1.AdminListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	return m.adminListFn(ctx, req)
}

// AdminGetOrder 执行测试桩定义的管理员查单逻辑。
func (m *mockOrderCreator) AdminGetOrder(ctx context.Context, req *orderv1.AdminGetOrderRequest) (*orderv1.GetOrderResponse, error) {
	return m.adminGetFn(ctx, req)
}

// AdminCancelOrder 执行测试桩定义的管理员关单逻辑。
func (m *mockOrderCreator) AdminCancelOrder(ctx context.Context, req *orderv1.AdminCancelOrderRequest) (*orderv1.CancelOrderResponse, error) {
	return m.adminCancelFn(ctx, req)
}

// FindUserByEmail 执行测试桩定义的按邮箱查找用户逻辑。
func (m *mockOrderAddressReader) FindUserByEmail(ctx context.Context, email string) (*userv1.FindUserByEmailResponse, error) {
	return m.findUserByEmailFn(ctx, email)
}

// GetAddress 执行测试桩定义的地址查询逻辑。
func (m *mockOrderAddressReader) GetAddress(ctx context.Context, userID, addressID int64) (*userv1.GetAddressResponse, error) {
	return m.getAddressFn(ctx, userID, addressID)
}

// performOrderJSONRequest 构造带 JWT 的订单 HTTP 请求。
func performOrderJSONRequest(r http.Handler, method, path string, body any, token string) *httptest.ResponseRecorder {
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

// setupOrderRouter 创建只承载下单入口的测试路由。
func setupOrderRouter(t *testing.T, creator orderCreatorClient, addressReader orderAddressReader) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	jwtMgr := authn.NewJWTManager("order-handler-test-secret", time.Hour)
	token, _, err := jwtMgr.Sign(1001, "order@test.com")
	require.NoError(t, err)

	h := NewOrderHandler(creator, addressReader)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(middleware.JWTAuth(jwtMgr))
	v1.POST("/orders", h.CreateOrder)
	v1.GET("/orders/:id", h.GetOrder)
	v1.GET("/orders", h.ListOrders)
	v1.POST("/orders/:id/cancel", h.CancelOrder)
	return r, token
}

// TestOrderHandler_CreateOrder_Success 验证已登录用户可以通过 Gateway 创建订单。
func TestOrderHandler_CreateOrder_Success(t *testing.T) {
	r, token := setupOrderRouter(t, &mockOrderCreator{
		createOrderFn: func(_ context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
			assert.Equal(t, int64(1001), req.GetUserId())
			assert.Equal(t, "req-001", req.GetRequestId())
			assert.Equal(t, int64(1001002), req.GetProductId())
			assert.Equal(t, "张三", req.GetReceiverName())
			assert.Equal(t, "浦东新区", req.GetDistrict())
			return &orderv1.CreateOrderResponse{
				Order: &orderv1.Order{
					Id:          101,
					OrderNo:     "ORD-20260518140000-1001",
					UserId:      1001,
					RequestId:   "req-001",
					Status:      "CREATED",
					TotalAmount: 1999800,
					Items: []*orderv1.OrderItem{
						{ProductId: 1001002, Name: "MacBook Air"},
					},
				},
			}, nil
		},
	}, &mockOrderAddressReader{
		getAddressFn: func(_ context.Context, userID, addressID int64) (*userv1.GetAddressResponse, error) {
			assert.Equal(t, int64(1001), userID)
			assert.Equal(t, int64(501), addressID)
			return &userv1.GetAddressResponse{
				Address: &userv1.Address{
					Id:            501,
					UserId:        1001,
					ReceiverName:  "张三",
					ReceiverPhone: "13800138000",
					Province:      "上海市",
					City:          "上海市",
					District:      "浦东新区",
					AddressLine:   "世纪大道 100 号 18 层",
					PostalCode:    "200120",
				},
			}, nil
		},
	})

	w := performOrderJSONRequest(r, http.MethodPost, "/api/v1/orders", map[string]any{
		"request_id": "req-001",
		"product_id": 1001002,
		"quantity":   2,
		"address_id": 501,
	}, token)

	require.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), `"order_no":"ORD-20260518140000-1001"`)
	assert.Contains(t, w.Body.String(), `"product_id":1001002`)
}

// TestOrderHandler_CreateOrder_InvalidBody 验证参数不完整时会返回 400。
func TestOrderHandler_CreateOrder_InvalidBody(t *testing.T) {
	r, token := setupOrderRouter(t, &mockOrderCreator{
		createOrderFn: func(_ context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
			t.Fatal("should not call CreateOrder on invalid body")
			return nil, nil
		},
	}, &mockOrderAddressReader{
		getAddressFn: func(_ context.Context, userID, addressID int64) (*userv1.GetAddressResponse, error) {
			t.Fatal("should not call GetAddress on invalid body")
			return nil, nil
		},
	})

	w := performOrderJSONRequest(r, http.MethodPost, "/api/v1/orders", map[string]any{
		"product_id": 1001002,
		"quantity":   1,
	}, token)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `"error"`)
}

// TestOrderHandler_CreateOrder_RequiresJWT 验证未登录时会被 JWT 中间件拦截。
func TestOrderHandler_CreateOrder_RequiresJWT(t *testing.T) {
	r, _ := setupOrderRouter(t, &mockOrderCreator{
		createOrderFn: func(_ context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
			t.Fatal("should not call CreateOrder without jwt")
			return nil, nil
		},
	}, &mockOrderAddressReader{
		getAddressFn: func(_ context.Context, userID, addressID int64) (*userv1.GetAddressResponse, error) {
			t.Fatal("should not call GetAddress without jwt")
			return nil, nil
		},
	})

	w := performOrderJSONRequest(r, http.MethodPost, "/api/v1/orders", map[string]any{
		"request_id": "req-001",
		"product_id": 1001002,
		"quantity":   1,
		"address_id": 501,
	}, "")

	require.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestOrderHandler_CreateOrder_Conflict 验证 Order gRPC 冲突错误会映射为 HTTP 409。
func TestOrderHandler_CreateOrder_Conflict(t *testing.T) {
	r, token := setupOrderRouter(t, &mockOrderCreator{
		createOrderFn: func(_ context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
			return nil, status.Error(codes.AlreadyExists, "重复下单请求")
		},
		getOrderFn: func(_ context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
			t.Fatal("unexpected GetOrder call")
			return nil, nil
		},
		listOrdersFn: func(_ context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
			t.Fatal("unexpected ListOrders call")
			return nil, nil
		},
	}, &mockOrderAddressReader{
		getAddressFn: func(_ context.Context, userID, addressID int64) (*userv1.GetAddressResponse, error) {
			return &userv1.GetAddressResponse{
				Address: &userv1.Address{
					Id:            addressID,
					UserId:        userID,
					ReceiverName:  "张三",
					ReceiverPhone: "13800138000",
					Province:      "上海市",
					City:          "上海市",
					District:      "浦东新区",
					AddressLine:   "世纪大道 100 号 18 层",
					PostalCode:    "200120",
				},
			}, nil
		},
	})

	w := performOrderJSONRequest(r, http.MethodPost, "/api/v1/orders", map[string]any{
		"request_id": "req-dup",
		"product_id": 1001002,
		"quantity":   1,
		"address_id": 501,
	}, token)

	require.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "重复下单请求")
}

// TestOrderHandler_GetOrder_Success 验证已登录用户可查询单笔订单详情。
func TestOrderHandler_GetOrder_Success(t *testing.T) {
	r, token := setupOrderRouter(t, &mockOrderCreator{
		createOrderFn: func(_ context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
			t.Fatal("unexpected CreateOrder call")
			return nil, nil
		},
		getOrderFn: func(_ context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
			assert.Equal(t, int64(1001), req.GetUserId())
			assert.Equal(t, int64(101), req.GetOrderId())
			return &orderv1.GetOrderResponse{
				Order: &orderv1.Order{
					Id:        101,
					OrderNo:   "ORD-20260518150000-1001",
					UserId:    1001,
					Status:    "CREATED",
					CreatedAt: 1778479800,
				},
			}, nil
		},
		listOrdersFn: func(_ context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
			t.Fatal("unexpected ListOrders call")
			return nil, nil
		},
	}, &mockOrderAddressReader{
		getAddressFn: func(_ context.Context, userID, addressID int64) (*userv1.GetAddressResponse, error) {
			t.Fatal("unexpected GetAddress call")
			return nil, nil
		},
	})

	w := performOrderJSONRequest(r, http.MethodGet, "/api/v1/orders/101", nil, token)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"order_no":"ORD-20260518150000-1001"`)
}

// TestOrderHandler_GetOrder_NotFound 验证订单不存在时会映射为 HTTP 404。
func TestOrderHandler_GetOrder_NotFound(t *testing.T) {
	r, token := setupOrderRouter(t, &mockOrderCreator{
		createOrderFn: func(_ context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
			t.Fatal("unexpected CreateOrder call")
			return nil, nil
		},
		getOrderFn: func(_ context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
			return nil, status.Error(codes.NotFound, "订单不存在")
		},
		listOrdersFn: func(_ context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
			t.Fatal("unexpected ListOrders call")
			return nil, nil
		},
	}, &mockOrderAddressReader{
		getAddressFn: func(_ context.Context, userID, addressID int64) (*userv1.GetAddressResponse, error) {
			t.Fatal("unexpected GetAddress call")
			return nil, nil
		},
	})

	w := performOrderJSONRequest(r, http.MethodGet, "/api/v1/orders/999", nil, token)

	require.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "订单不存在")
}

// TestOrderHandler_ListOrders_Success 验证已登录用户可分页查询订单列表。
func TestOrderHandler_ListOrders_Success(t *testing.T) {
	r, token := setupOrderRouter(t, &mockOrderCreator{
		createOrderFn: func(_ context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
			t.Fatal("unexpected CreateOrder call")
			return nil, nil
		},
		getOrderFn: func(_ context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
			t.Fatal("unexpected GetOrder call")
			return nil, nil
		},
		listOrdersFn: func(_ context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
			assert.Equal(t, int64(1001), req.GetUserId())
			assert.Equal(t, int32(1), req.GetPage())
			assert.Equal(t, int32(2), req.GetPageSize())
			return &orderv1.ListOrdersResponse{
				Items: []*orderv1.Order{
					{Id: 103, OrderNo: "ORD-003", UserId: 1001, TotalAmount: 1999800},
					{Id: 102, OrderNo: "ORD-002", UserId: 1001, TotalAmount: 999900},
				},
				Total: 3,
			}, nil
		},
	}, &mockOrderAddressReader{
		getAddressFn: func(_ context.Context, userID, addressID int64) (*userv1.GetAddressResponse, error) {
			t.Fatal("unexpected GetAddress call")
			return nil, nil
		},
	})

	w := performOrderJSONRequest(r, http.MethodGet, "/api/v1/orders?page=1&page_size=2", nil, token)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"total":3`)
	assert.Contains(t, w.Body.String(), `"order_no":"ORD-003"`)
	assert.Contains(t, w.Body.String(), `"order_no":"ORD-002"`)
}

// TestOrderHandler_CancelOrder_Success 验证已登录用户可取消自己的订单。
func TestOrderHandler_CancelOrder_Success(t *testing.T) {
	r, token := setupOrderRouter(t, &mockOrderCreator{
		createOrderFn: func(_ context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
			t.Fatal("unexpected CreateOrder call")
			return nil, nil
		},
		getOrderFn: func(_ context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
			t.Fatal("unexpected GetOrder call")
			return nil, nil
		},
		listOrdersFn: func(_ context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
			t.Fatal("unexpected ListOrders call")
			return nil, nil
		},
		cancelOrderFn: func(_ context.Context, req *orderv1.CancelOrderRequest) (*orderv1.CancelOrderResponse, error) {
			assert.Equal(t, int64(1001), req.GetUserId())
			assert.Equal(t, int64(201), req.GetOrderId())
			return &orderv1.CancelOrderResponse{
				Order: &orderv1.Order{
					Id:      201,
					OrderNo: "ORD-CANCEL-201",
					UserId:  1001,
					Status:  "CANCELLED",
				},
			}, nil
		},
	}, &mockOrderAddressReader{
		getAddressFn: func(_ context.Context, userID, addressID int64) (*userv1.GetAddressResponse, error) {
			t.Fatal("unexpected GetAddress call")
			return nil, nil
		},
	})

	w := performOrderJSONRequest(r, http.MethodPost, "/api/v1/orders/201/cancel", nil, token)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"CANCELLED"`)
}

// TestOrderHandler_CancelOrder_FailedPrecondition 验证不可取消状态会映射为 HTTP 412。
func TestOrderHandler_CancelOrder_FailedPrecondition(t *testing.T) {
	r, token := setupOrderRouter(t, &mockOrderCreator{
		createOrderFn: func(_ context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
			t.Fatal("unexpected CreateOrder call")
			return nil, nil
		},
		getOrderFn: func(_ context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
			t.Fatal("unexpected GetOrder call")
			return nil, nil
		},
		listOrdersFn: func(_ context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
			t.Fatal("unexpected ListOrders call")
			return nil, nil
		},
		cancelOrderFn: func(_ context.Context, req *orderv1.CancelOrderRequest) (*orderv1.CancelOrderResponse, error) {
			return nil, status.Error(codes.FailedPrecondition, "当前订单状态不可取消")
		},
	}, &mockOrderAddressReader{
		getAddressFn: func(_ context.Context, userID, addressID int64) (*userv1.GetAddressResponse, error) {
			t.Fatal("unexpected GetAddress call")
			return nil, nil
		},
	})

	w := performOrderJSONRequest(r, http.MethodPost, "/api/v1/orders/202/cancel", nil, token)

	require.Equal(t, http.StatusPreconditionFailed, w.Code)
	assert.Contains(t, w.Body.String(), "当前订单状态不可取消")
}
