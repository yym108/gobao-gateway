package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yym108/gobao-gateway/internal/middleware"
	"github.com/yym108/gobao-pkg/authn"
	paymentv1 "github.com/yym108/gobao-proto/gen/go/gobao/payment/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type mockPaymentClient struct {
	getPaymentFn        func(ctx context.Context, req *paymentv1.GetPaymentRequest) (*paymentv1.GetPaymentResponse, error)
	getPaymentByOrderFn func(ctx context.Context, req *paymentv1.GetPaymentByOrderRequest) (*paymentv1.GetPaymentByOrderResponse, error)
	mockConfirmFn       func(ctx context.Context, req *paymentv1.MockConfirmPaymentRequest) (*paymentv1.MockConfirmPaymentResponse, error)
}

func (m *mockPaymentClient) GetPayment(ctx context.Context, req *paymentv1.GetPaymentRequest) (*paymentv1.GetPaymentResponse, error) {
	return m.getPaymentFn(ctx, req)
}
func (m *mockPaymentClient) GetPaymentByOrder(ctx context.Context, req *paymentv1.GetPaymentByOrderRequest) (*paymentv1.GetPaymentByOrderResponse, error) {
	return m.getPaymentByOrderFn(ctx, req)
}
func (m *mockPaymentClient) MockConfirmPayment(ctx context.Context, req *paymentv1.MockConfirmPaymentRequest) (*paymentv1.MockConfirmPaymentResponse, error) {
	return m.mockConfirmFn(ctx, req)
}

func setupPaymentRouter(t *testing.T, client paymentClient) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	jwtMgr := authn.NewJWTManager("payment-handler-test-secret", time.Hour)
	token, _, err := jwtMgr.Sign(1001, "payment@test.com")
	require.NoError(t, err)

	h := NewPaymentHandler(client)
	r := gin.New()
	v1 := r.Group("/api/v1")
	v1.Use(middleware.JWTAuth(jwtMgr))
	v1.GET("/payments/:id", h.GetPayment)
	v1.GET("/payments/by-order/:orderId", h.GetPaymentByOrder)
	v1.POST("/payments/:id/mock-confirm", h.MockConfirmPayment)
	return r, token
}

func TestPaymentHandler_GetPayment_Success(t *testing.T) {
	r, token := setupPaymentRouter(t, &mockPaymentClient{
		getPaymentFn: func(_ context.Context, req *paymentv1.GetPaymentRequest) (*paymentv1.GetPaymentResponse, error) {
			assert.Equal(t, int64(1001), req.GetUserId())
			assert.Equal(t, int64(301), req.GetPaymentId())
			return &paymentv1.GetPaymentResponse{Payment: &paymentv1.Payment{Id: 301, OrderId: 101}}, nil
		},
		getPaymentByOrderFn: func(_ context.Context, _ *paymentv1.GetPaymentByOrderRequest) (*paymentv1.GetPaymentByOrderResponse, error) {
			t.Fatal("unexpected get by order call")
			return nil, nil
		},
		mockConfirmFn: func(_ context.Context, _ *paymentv1.MockConfirmPaymentRequest) (*paymentv1.MockConfirmPaymentResponse, error) {
			t.Fatal("unexpected mock confirm call")
			return nil, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/payments/301", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"id":301`)
}

func TestPaymentHandler_MockConfirm_Conflict(t *testing.T) {
	r, token := setupPaymentRouter(t, &mockPaymentClient{
		getPaymentFn: func(_ context.Context, _ *paymentv1.GetPaymentRequest) (*paymentv1.GetPaymentResponse, error) {
			t.Fatal("unexpected get payment call")
			return nil, nil
		},
		getPaymentByOrderFn: func(_ context.Context, _ *paymentv1.GetPaymentByOrderRequest) (*paymentv1.GetPaymentByOrderResponse, error) {
			t.Fatal("unexpected get by order call")
			return nil, nil
		},
		mockConfirmFn: func(_ context.Context, req *paymentv1.MockConfirmPaymentRequest) (*paymentv1.MockConfirmPaymentResponse, error) {
			assert.Equal(t, "SUCCESS", req.GetResult())
			return nil, status.Error(codes.FailedPrecondition, "当前支付单状态不可重复确认")
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/301/mock-confirm", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Body = stringReadCloser{Reader: strings.NewReader(`{"result":"SUCCESS"}`)}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusPreconditionFailed, w.Code)
	assert.Contains(t, w.Body.String(), "当前支付单状态不可重复确认")
}

type stringReadCloser struct{ *strings.Reader }

func (s stringReadCloser) Close() error { return nil }
