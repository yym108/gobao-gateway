// Package handler 提供 Gateway 的 HTTP 请求处理器。
// 本文件实现订单创建入口，将用户态 HTTP 请求转发到 Order gRPC。
package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	orderv1 "github.com/yym108/gobao-proto/gen/go/gobao/order/v1"
	userv1 "github.com/yym108/gobao-proto/gen/go/gobao/user/v1"
)

// orderCreatorClient 抽象 Gateway 到 Order 服务的最小调用接口。
type orderCreatorClient interface {
	// CreateOrder 调用后端 Order 服务创建订单。
	CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error)
	// GetOrder 调用后端 Order 服务查询单笔订单。
	GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error)
	// ListOrders 调用后端 Order 服务分页查询订单列表。
	ListOrders(ctx context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error)
	// CancelOrder 调用后端 Order 服务取消订单。
	CancelOrder(ctx context.Context, req *orderv1.CancelOrderRequest) (*orderv1.CancelOrderResponse, error)
	// AdminListOrders 管理员分页查询全部订单。
	AdminListOrders(ctx context.Context, req *orderv1.AdminListOrdersRequest) (*orderv1.ListOrdersResponse, error)
	// AdminGetOrder 管理员查询任意订单详情。
	AdminGetOrder(ctx context.Context, req *orderv1.AdminGetOrderRequest) (*orderv1.GetOrderResponse, error)
	// AdminCancelOrder 管理员关闭任意未支付订单。
	AdminCancelOrder(ctx context.Context, req *orderv1.AdminCancelOrderRequest) (*orderv1.CancelOrderResponse, error)
}

// orderAddressReader 抽象 Gateway 向 User 服务查询地址快照与按邮箱解析用户的接口。
type orderAddressReader interface {
	GetAddress(ctx context.Context, userID, addressID int64) (*userv1.GetAddressResponse, error)
	// FindUserByEmail 供后台订单按买家邮箱筛选时解析 user_id。
	FindUserByEmail(ctx context.Context, email string) (*userv1.FindUserByEmailResponse, error)
}

// createOrderRequest 描述 Gateway 下单接口的请求体。
type createOrderRequest struct {
	RequestID string `json:"request_id"` // 幂等请求 ID
	ProductID int64  `json:"product_id"` // 后端独立商品 ID
	Quantity  int32  `json:"quantity"`   // 购买数量
	AddressID int64  `json:"address_id"` // 地址簿地址 ID
}

// OrderHandler 处理订单相关的 HTTP 请求。
type OrderHandler struct {
	client        orderCreatorClient // Order 服务 gRPC client
	addressClient orderAddressReader // User 服务地址查询接口
}

// NewOrderHandler 构造 OrderHandler。
func NewOrderHandler(client orderCreatorClient, addressClient orderAddressReader) *OrderHandler {
	return &OrderHandler{client: client, addressClient: addressClient}
}

// CreateOrder 处理 POST /api/v1/orders。
// 当前 Gateway 只透传下单请求，不接受前端传入价格、规格摘要或商品快照。
func (h *OrderHandler) CreateOrder(c *gin.Context) {
	var req createOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体格式错误"})
		return
	}
	if req.RequestID == "" || req.ProductID <= 0 || req.Quantity <= 0 || req.AddressID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request_id、product_id、quantity、address_id 必须有效"})
		return
	}

	addressResp, err := h.addressClient.GetAddress(c.Request.Context(), c.GetInt64("userID"), req.AddressID)
	if err != nil {
		code, msg := grpcErrToHTTP(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}
	address := addressResp.GetAddress()

	resp, err := h.client.CreateOrder(c.Request.Context(), &orderv1.CreateOrderRequest{
		UserId:        c.GetInt64("userID"),
		RequestId:     req.RequestID,
		ProductId:     req.ProductID,
		Quantity:      req.Quantity,
		ReceiverName:  address.GetReceiverName(),
		ReceiverPhone: address.GetReceiverPhone(),
		Province:      address.GetProvince(),
		City:          address.GetCity(),
		District:      address.GetDistrict(),
		AddressLine:   address.GetAddressLine(),
		PostalCode:    address.GetPostalCode(),
	})
	if err != nil {
		code, msg := grpcErrToHTTP(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// GetOrder 处理 GET /api/v1/orders/:id。
// Gateway 仅根据当前 JWT 用户查询订单，不接受前端传 user_id。
func (h *OrderHandler) GetOrder(c *gin.Context) {
	orderID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || orderID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "订单 ID 必须为正数"})
		return
	}

	resp, grpcErr := h.client.GetOrder(c.Request.Context(), &orderv1.GetOrderRequest{
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

// ListOrders 处理 GET /api/v1/orders。
// 支持 page/page_size 分页参数，分页归一化逻辑仍由后端 Order 服务统一处理。
func (h *OrderHandler) ListOrders(c *gin.Context) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page 必须为整数"})
		return
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page_size 必须为整数"})
		return
	}

	resp, grpcErr := h.client.ListOrders(c.Request.Context(), &orderv1.ListOrdersRequest{
		UserId:   c.GetInt64("userID"),
		Page:     int32(page),
		PageSize: int32(pageSize),
	})
	if grpcErr != nil {
		code, msg := grpcErrToHTTP(grpcErr)
		c.JSON(code, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// CancelOrder 处理 POST /api/v1/orders/:id/cancel。
// Gateway 仅允许当前 JWT 用户取消自己的订单，不接受前端传 user_id。
func (h *OrderHandler) CancelOrder(c *gin.Context) {
	orderID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || orderID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "订单 ID 必须为正数"})
		return
	}

	resp, grpcErr := h.client.CancelOrder(c.Request.Context(), &orderv1.CancelOrderRequest{
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

// AdminListOrders 处理 GET /api/v1/admin/orders。
// 管理员可分页查询全部订单，并可选按 status 过滤；不限定下单用户。
func (h *OrderHandler) AdminListOrders(c *gin.Context) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page 必须为整数"})
		return
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "page_size 必须为整数"})
		return
	}

	// 按买家邮箱筛选时，先把邮箱解析为 user_id；邮箱不存在则直接返回空列表。
	var userID int64
	if email := strings.TrimSpace(c.Query("email")); email != "" {
		found, lookupErr := h.addressClient.FindUserByEmail(c.Request.Context(), email)
		if lookupErr != nil {
			code, msg := grpcErrToHTTP(lookupErr)
			c.JSON(code, gin.H{"error": msg})
			return
		}
		if !found.GetFound() {
			c.JSON(http.StatusOK, &orderv1.ListOrdersResponse{Items: []*orderv1.Order{}, Total: 0})
			return
		}
		userID = found.GetUserId()
	}

	resp, grpcErr := h.client.AdminListOrders(c.Request.Context(), &orderv1.AdminListOrdersRequest{
		Page:     int32(page),
		PageSize: int32(pageSize),
		Status:   c.Query("status"),
		UserId:   userID,
	})
	if grpcErr != nil {
		code, msg := grpcErrToHTTP(grpcErr)
		c.JSON(code, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// AdminGetOrder 处理 GET /api/v1/admin/orders/:id。
// 管理员可查询任意订单详情，不校验归属。
func (h *OrderHandler) AdminGetOrder(c *gin.Context) {
	orderID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || orderID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "订单 ID 必须为正数"})
		return
	}

	resp, grpcErr := h.client.AdminGetOrder(c.Request.Context(), &orderv1.AdminGetOrderRequest{OrderId: orderID})
	if grpcErr != nil {
		code, msg := grpcErrToHTTP(grpcErr)
		c.JSON(code, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// AdminCancelOrder 处理 POST /api/v1/admin/orders/:id/cancel。
// 管理员关闭任意未支付订单。
func (h *OrderHandler) AdminCancelOrder(c *gin.Context) {
	orderID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || orderID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "订单 ID 必须为正数"})
		return
	}

	resp, grpcErr := h.client.AdminCancelOrder(c.Request.Context(), &orderv1.AdminCancelOrderRequest{OrderId: orderID})
	if grpcErr != nil {
		code, msg := grpcErrToHTTP(grpcErr)
		c.JSON(code, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusOK, resp)
}
