// Package handler 提供 Gateway 地址簿 HTTP 接口。
package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	userv1 "github.com/yym108/gobao-proto/gen/go/gobao/user/v1"
)

// addressReader 抽象 Gateway 到 User 服务的地址簿调用接口。
type addressReader interface {
	ListAddresses(ctx context.Context, userID int64) (*userv1.ListAddressesResponse, error)
	GetAddress(ctx context.Context, userID, addressID int64) (*userv1.GetAddressResponse, error)
	CreateAddress(ctx context.Context, req *userv1.CreateAddressRequest) (*userv1.CreateAddressResponse, error)
	UpdateAddress(ctx context.Context, req *userv1.UpdateAddressRequest) (*userv1.UpdateAddressResponse, error)
	DeleteAddress(ctx context.Context, userID, addressID int64) (*userv1.DeleteAddressResponse, error)
	SetDefaultAddress(ctx context.Context, userID, addressID int64) (*userv1.SetDefaultAddressResponse, error)
}

// upsertAddressRequest 描述新增和编辑地址时的公共请求体。
type upsertAddressRequest struct {
	ReceiverName  string `json:"receiver_name"`  // 收货人姓名
	ReceiverPhone string `json:"receiver_phone"` // 收货手机号
	Province      string `json:"province"`       // 省
	City          string `json:"city"`           // 市
	District      string `json:"district"`       // 区
	AddressLine   string `json:"address_line"`   // 详细地址
	PostalCode    string `json:"postal_code"`    // 邮编
	IsDefault     bool   `json:"is_default"`     // 是否设为默认地址
}

// setDefaultAddressRequest 描述默认地址切换请求体。
type setDefaultAddressRequest struct {
	AddressID int64 `json:"address_id"` // 目标地址 ID
}

// AddressHandler 处理地址簿相关 HTTP 请求。
type AddressHandler struct {
	client addressReader // User 服务地址簿 gRPC client
}

// NewAddressHandler 构造地址簿 Handler。
func NewAddressHandler(client addressReader) *AddressHandler {
	return &AddressHandler{client: client}
}

// ListAddresses 处理 GET /api/v1/addresses。
func (h *AddressHandler) ListAddresses(c *gin.Context) {
	resp, err := h.client.ListAddresses(c.Request.Context(), c.GetInt64("userID"))
	if err != nil {
		writeGRPCError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GetAddress 处理 GET /api/v1/addresses/:id。
func (h *AddressHandler) GetAddress(c *gin.Context) {
	addressID, ok := parseAddressID(c)
	if !ok {
		return
	}
	resp, err := h.client.GetAddress(c.Request.Context(), c.GetInt64("userID"), addressID)
	if err != nil {
		writeGRPCError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// CreateAddress 处理 POST /api/v1/addresses。
func (h *AddressHandler) CreateAddress(c *gin.Context) {
	var req upsertAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGUMENT", "message": "invalid request body"})
		return
	}
	resp, err := h.client.CreateAddress(c.Request.Context(), &userv1.CreateAddressRequest{
		UserId:        c.GetInt64("userID"),
		ReceiverName:  req.ReceiverName,
		ReceiverPhone: req.ReceiverPhone,
		Province:      req.Province,
		City:          req.City,
		District:      req.District,
		AddressLine:   req.AddressLine,
		PostalCode:    req.PostalCode,
		IsDefault:     req.IsDefault,
	})
	if err != nil {
		writeGRPCError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// UpdateAddress 处理 PUT /api/v1/addresses/:id。
func (h *AddressHandler) UpdateAddress(c *gin.Context) {
	addressID, ok := parseAddressID(c)
	if !ok {
		return
	}
	var req upsertAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGUMENT", "message": "invalid request body"})
		return
	}
	resp, err := h.client.UpdateAddress(c.Request.Context(), &userv1.UpdateAddressRequest{
		UserId:        c.GetInt64("userID"),
		AddressId:     addressID,
		ReceiverName:  req.ReceiverName,
		ReceiverPhone: req.ReceiverPhone,
		Province:      req.Province,
		City:          req.City,
		District:      req.District,
		AddressLine:   req.AddressLine,
		PostalCode:    req.PostalCode,
		IsDefault:     req.IsDefault,
	})
	if err != nil {
		writeGRPCError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// DeleteAddress 处理 DELETE /api/v1/addresses/:id。
func (h *AddressHandler) DeleteAddress(c *gin.Context) {
	addressID, ok := parseAddressID(c)
	if !ok {
		return
	}
	resp, err := h.client.DeleteAddress(c.Request.Context(), c.GetInt64("userID"), addressID)
	if err != nil {
		writeGRPCError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// SetDefaultAddress 处理 POST /api/v1/addresses/default。
func (h *AddressHandler) SetDefaultAddress(c *gin.Context) {
	var req setDefaultAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGUMENT", "message": "invalid request body"})
		return
	}
	if req.AddressID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGUMENT", "message": "address_id must be positive"})
		return
	}
	resp, err := h.client.SetDefaultAddress(c.Request.Context(), c.GetInt64("userID"), req.AddressID)
	if err != nil {
		writeGRPCError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// parseAddressID 从路径中解析地址 ID。
func parseAddressID(c *gin.Context) (int64, bool) {
	addressID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || addressID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGUMENT", "message": "address id must be positive"})
		return 0, false
	}
	return addressID, true
}
