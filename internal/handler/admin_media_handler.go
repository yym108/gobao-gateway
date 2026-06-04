package handler

import (
	"encoding/base64"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	productv1 "github.com/yym108/gobao-proto/gen/go/gobao/product/v1"

	"github.com/yym108/gobao-gateway/internal/client"
)

// AdminMediaHandler 提供后台媒体管理的最小 HTTP 入口。
// 当前阶段仅负责将受保护的 HTTP 请求转发为 Product gRPC 调用。
type AdminMediaHandler struct {
	client *client.ProductClient // Product 服务 gRPC client
}

// NewAdminMediaHandler 构造后台媒体管理 handler。
func NewAdminMediaHandler(pc *client.ProductClient) *AdminMediaHandler {
	return &AdminMediaHandler{client: pc}
}

type uploadMediaRequest struct {
	Folder        string `json:"folder"`
	FileName      string `json:"file_name"`
	AltText       string `json:"alt_text"`
	MIMEType      string `json:"mime_type"`
	ContentBase64 string `json:"content_base64"`
}

type bindMediaRequest struct {
	MediaID   int64  `json:"media_id"`
	UsageType string `json:"usage_type"`
	SortOrder int32  `json:"sort_order"`
	IsPrimary bool   `json:"is_primary"`
}

// UploadMedia 处理 POST /api/v1/admin/media/upload。
func (h *AdminMediaHandler) UploadMedia(c *gin.Context) {
	var req uploadMediaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	payload, err := base64.StdEncoding.DecodeString(req.ContentBase64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content_base64 非法"})
		return
	}
	resp, rpcErr := h.client.UploadMedia(c.Request.Context(), &productv1.UploadMediaRequest{
		Folder:   req.Folder,
		FileName: req.FileName,
		AltText:  req.AltText,
		MimeType: req.MIMEType,
		Content:  payload,
	})
	if code, msg := grpcErrToHTTP(rpcErr); rpcErr != nil {
		c.JSON(code, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// BindProductGroupMedia 处理 POST /api/v1/admin/product-groups/:groupId/media。
func (h *AdminMediaHandler) BindProductGroupMedia(c *gin.Context) {
	groupID, _ := strconv.ParseInt(c.Param("groupId"), 10, 64)
	var req bindMediaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, rpcErr := h.client.BindProductGroupMedia(c.Request.Context(), &productv1.BindProductGroupMediaRequest{
		GroupId:   groupID,
		MediaId:   req.MediaID,
		UsageType: req.UsageType,
		SortOrder: req.SortOrder,
		IsPrimary: req.IsPrimary,
	})
	if code, msg := grpcErrToHTTP(rpcErr); rpcErr != nil {
		c.JSON(code, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// BindProductMedia 处理 POST /api/v1/admin/products/:id/media。
func (h *AdminMediaHandler) BindProductMedia(c *gin.Context) {
	productID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req bindMediaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, rpcErr := h.client.BindProductMedia(c.Request.Context(), &productv1.BindProductMediaRequest{
		ProductId: productID,
		MediaId:   req.MediaID,
		UsageType: req.UsageType,
		SortOrder: req.SortOrder,
		IsPrimary: req.IsPrimary,
	})
	if code, msg := grpcErrToHTTP(rpcErr); rpcErr != nil {
		c.JSON(code, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// UpdateProductGroupMediaBinding 处理 PUT /api/v1/admin/product-groups/:groupId/media/:bindingId。
func (h *AdminMediaHandler) UpdateProductGroupMediaBinding(c *gin.Context) {
	groupID, _ := strconv.ParseInt(c.Param("groupId"), 10, 64)
	bindingID, _ := strconv.ParseInt(c.Param("bindingId"), 10, 64)
	var req bindMediaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, rpcErr := h.client.UpdateProductGroupMediaBinding(c.Request.Context(), &productv1.UpdateProductGroupMediaBindingRequest{
		GroupId:   groupID,
		BindingId: bindingID,
		UsageType: req.UsageType,
		SortOrder: req.SortOrder,
		IsPrimary: req.IsPrimary,
	})
	if code, msg := grpcErrToHTTP(rpcErr); rpcErr != nil {
		c.JSON(code, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateProductMediaBinding 处理 PUT /api/v1/admin/products/:id/media/:bindingId。
func (h *AdminMediaHandler) UpdateProductMediaBinding(c *gin.Context) {
	productID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	bindingID, _ := strconv.ParseInt(c.Param("bindingId"), 10, 64)
	var req bindMediaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, rpcErr := h.client.UpdateProductMediaBinding(c.Request.Context(), &productv1.UpdateProductMediaBindingRequest{
		ProductId: productID,
		BindingId: bindingID,
		UsageType: req.UsageType,
		SortOrder: req.SortOrder,
		IsPrimary: req.IsPrimary,
	})
	if code, msg := grpcErrToHTTP(rpcErr); rpcErr != nil {
		c.JSON(code, gin.H{"error": msg})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// DeleteProductGroupMediaBinding 处理 DELETE /api/v1/admin/product-groups/:groupId/media/:bindingId。
func (h *AdminMediaHandler) DeleteProductGroupMediaBinding(c *gin.Context) {
	groupID, _ := strconv.ParseInt(c.Param("groupId"), 10, 64)
	bindingID, _ := strconv.ParseInt(c.Param("bindingId"), 10, 64)
	_, rpcErr := h.client.DeleteProductGroupMediaBinding(c.Request.Context(), &productv1.DeleteProductGroupMediaBindingRequest{
		GroupId:   groupID,
		BindingId: bindingID,
	})
	if code, msg := grpcErrToHTTP(rpcErr); rpcErr != nil {
		c.JSON(code, gin.H{"error": msg})
		return
	}
	c.Status(http.StatusNoContent)
}

// DeleteProductMediaBinding 处理 DELETE /api/v1/admin/products/:id/media/:bindingId。
func (h *AdminMediaHandler) DeleteProductMediaBinding(c *gin.Context) {
	productID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	bindingID, _ := strconv.ParseInt(c.Param("bindingId"), 10, 64)
	_, rpcErr := h.client.DeleteProductMediaBinding(c.Request.Context(), &productv1.DeleteProductMediaBindingRequest{
		ProductId: productID,
		BindingId: bindingID,
	})
	if code, msg := grpcErrToHTTP(rpcErr); rpcErr != nil {
		c.JSON(code, gin.H{"error": msg})
		return
	}
	c.Status(http.StatusNoContent)
}
