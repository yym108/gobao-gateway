// Package handler 提供 Gateway 的 HTTP 请求处理器。
package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	paymentv1 "github.com/yym108/gobao-proto/gen/go/gobao/payment/v1"
)

type paymentClient interface {
	GetPayment(ctx context.Context, req *paymentv1.GetPaymentRequest) (*paymentv1.GetPaymentResponse, error)
	GetPaymentByOrder(ctx context.Context, req *paymentv1.GetPaymentByOrderRequest) (*paymentv1.GetPaymentByOrderResponse, error)
	MockConfirmPayment(ctx context.Context, req *paymentv1.MockConfirmPaymentRequest) (*paymentv1.MockConfirmPaymentResponse, error)
}

type mockConfirmPaymentRequest struct {
	Result string `json:"result"` // SUCCESS 或 FAILED
}

// PaymentHandler 处理支付相关 HTTP 请求。
type PaymentHandler struct {
	client paymentClient // Payment 服务 gRPC client
}

// NewPaymentHandler 构造 PaymentHandler。
func NewPaymentHandler(client paymentClient) *PaymentHandler {
	return &PaymentHandler{client: client}
}

// GetPayment 处理 GET /api/v1/payments/:id。
func (h *PaymentHandler) GetPayment(c *gin.Context) {
	paymentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || paymentID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "支付单 ID 必须为正数"})
		return
	}
	resp, grpcErr := h.client.GetPayment(c.Request.Context(), &paymentv1.GetPaymentRequest{
		UserId:    c.GetInt64("userID"),
		PaymentId: paymentID,
	})
	if grpcErr != nil {
		code, msg := grpcErrToHTTP(grpcErr)
		c.JSON(code, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GetPaymentByOrder 处理 GET /api/v1/payments/by-order/:orderId。
func (h *PaymentHandler) GetPaymentByOrder(c *gin.Context) {
	orderID, err := strconv.ParseInt(c.Param("orderId"), 10, 64)
	if err != nil || orderID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "订单 ID 必须为正数"})
		return
	}
	resp, grpcErr := h.client.GetPaymentByOrder(c.Request.Context(), &paymentv1.GetPaymentByOrderRequest{
		UserId:  c.GetInt64("userID"),
		OrderId: orderID,
	})
	if grpcErr != nil {
		code, msg := grpcErrToHTTP(grpcErr)
		c.JSON(code, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// MockConfirmPayment 处理 POST /api/v1/payments/:id/mock-confirm。
func (h *PaymentHandler) MockConfirmPayment(c *gin.Context) {
	paymentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || paymentID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "支付单 ID 必须为正数"})
		return
	}
	var req mockConfirmPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体格式错误"})
		return
	}
	resp, grpcErr := h.client.MockConfirmPayment(c.Request.Context(), &paymentv1.MockConfirmPaymentRequest{
		UserId:    c.GetInt64("userID"),
		PaymentId: paymentID,
		Result:    req.Result,
	})
	if grpcErr != nil {
		code, msg := grpcErrToHTTP(grpcErr)
		c.JSON(code, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusOK, resp)
}
