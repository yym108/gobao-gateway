// Package handler 提供 Gateway 的 HTTP 请求处理器。
// 每个 handler 将 HTTP/JSON 请求转换为 gRPC 调用，并将 gRPC 响应/错误转换回 HTTP。
package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	productv1 "github.com/yym108/gobao-proto/gen/go/gobao/product/v1"

	"github.com/yym108/gobao-gateway/internal/client"
)

// ProductHandler 处理商品与类目相关的 HTTP 请求。
// Gateway 通过此 handler 将 HTTP/JSON 请求转发给 Product 服务的 gRPC 接口。
type ProductHandler struct {
	client *client.ProductClient // Product 服务的 gRPC client
}

// NewProductHandler 构造 ProductHandler。
//   - pc: Product 服务的 gRPC client
func NewProductHandler(pc *client.ProductClient) *ProductHandler {
	return &ProductHandler{client: pc}
}

// grpcErrToHTTP 将 Product 服务返回的 gRPC 错误映射为 HTTP 状态码与错误消息。
func grpcErrToHTTP(err error) (int, string) {
	if err == nil {
		return http.StatusOK, ""
	}
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.InvalidArgument:
			return http.StatusBadRequest, st.Message()
		case codes.NotFound:
			return http.StatusNotFound, st.Message()
		case codes.AlreadyExists:
			return http.StatusConflict, st.Message()
		case codes.FailedPrecondition:
			return http.StatusPreconditionFailed, st.Message()
		case codes.Aborted:
			return http.StatusConflict, st.Message()
		}
	}
	return http.StatusInternalServerError, err.Error()
}

// handleGRPC 将 gRPC 错误写回 HTTP 响应。
// 返回 true 表示已完成错误响应写入，调用方应立即 return。
func (h *ProductHandler) handleGRPC(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	code, msg := grpcErrToHTTP(err)
	c.JSON(code, gin.H{"error": msg})
	return true
}

// ListProducts 处理 GET /api/v1/products。
// 支持按 category_id 过滤，以及 page/page_size 分页参数。
func (h *ProductHandler) ListProducts(c *gin.Context) {
	catID, _ := strconv.ParseInt(c.Query("category_id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	resp, err := h.client.ListProducts(c.Request.Context(), &productv1.ListProductsRequest{
		CategoryId: catID,
		Page:       int32(page),
		PageSize:   int32(pageSize),
	})
	if h.handleGRPC(c, err) {
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GetProduct 处理 GET /api/v1/products/:id。
func (h *ProductHandler) GetProduct(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	resp, err := h.client.GetProduct(c.Request.Context(), &productv1.GetProductRequest{Id: id})
	if h.handleGRPC(c, err) {
		return
	}
	c.JSON(http.StatusOK, resp)
}

// CreateProduct 处理 POST /api/v1/products。
// 请求体按 proto CreateProductRequest 绑定，成功返回 201。
func (h *ProductHandler) CreateProduct(c *gin.Context) {
	var req productv1.CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.client.CreateProduct(c.Request.Context(), &req)
	if h.handleGRPC(c, err) {
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// UpdateProduct 处理 PUT /api/v1/products/:id。
// 路径参数中的商品 ID 会覆盖请求体中的 ID。
func (h *ProductHandler) UpdateProduct(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	var req productv1.UpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Id = id

	resp, err := h.client.UpdateProduct(c.Request.Context(), &req)
	if h.handleGRPC(c, err) {
		return
	}
	c.JSON(http.StatusOK, resp)
}

// DeleteProduct 处理 DELETE /api/v1/products/:id。
// 删除成功返回 204 No Content。
func (h *ProductHandler) DeleteProduct(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	_, err := h.client.DeleteProduct(c.Request.Context(), &productv1.DeleteProductRequest{Id: id})
	if h.handleGRPC(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
}

// ListCategories 处理 GET /api/v1/categories。
func (h *ProductHandler) ListCategories(c *gin.Context) {
	resp, err := h.client.ListCategories(c.Request.Context(), &productv1.ListCategoriesRequest{})
	if h.handleGRPC(c, err) {
		return
	}
	c.JSON(http.StatusOK, resp)
}

// CreateCategory 处理 POST /api/v1/categories。
// 请求体按 proto CreateCategoryRequest 绑定，成功返回 201。
func (h *ProductHandler) CreateCategory(c *gin.Context) {
	var req productv1.CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.client.CreateCategory(c.Request.Context(), &req)
	if h.handleGRPC(c, err) {
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// UpdateCategory 处理 PUT /api/v1/categories/:id。
// 路径参数中的类目 ID 会覆盖请求体中的 ID。
func (h *ProductHandler) UpdateCategory(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	var req productv1.UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Id = id

	resp, err := h.client.UpdateCategory(c.Request.Context(), &req)
	if h.handleGRPC(c, err) {
		return
	}
	c.JSON(http.StatusOK, resp)
}

// DeleteCategory 处理 DELETE /api/v1/categories/:id。
// 删除成功返回 204 No Content。
func (h *ProductHandler) DeleteCategory(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	_, err := h.client.DeleteCategory(c.Request.Context(), &productv1.DeleteCategoryRequest{Id: id})
	if h.handleGRPC(c, err) {
		return
	}
	c.Status(http.StatusNoContent)
}
