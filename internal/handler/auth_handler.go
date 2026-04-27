// Package handler 提供 Gateway 的 HTTP 请求处理器。
// 每个 handler 将 HTTP/JSON 请求转换为 gRPC 调用，并将 gRPC 响应/错误转换回 HTTP。
package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/yym/gobao-gateway/internal/client"
)

// AuthHandler 处理用户认证相关的 HTTP 请求（注册、登录、获取当前用户）。
type AuthHandler struct {
	userClient *client.UserClient // User 服务的 gRPC client
}

// NewAuthHandler 构造 AuthHandler。
//   - uc: User 服务的 gRPC client
func NewAuthHandler(uc *client.UserClient) *AuthHandler {
	return &AuthHandler{userClient: uc}
}

// registerRequest 是 POST /api/v1/auth/register 的请求体。
type registerRequest struct {
	Email    string `json:"email"`    // 邮箱
	Password string `json:"password"` // 密码
	Nickname string `json:"nickname"` // 昵称
}

// loginRequest 是 POST /api/v1/auth/login 的请求体。
type loginRequest struct {
	Email    string `json:"email"`    // 邮箱
	Password string `json:"password"` // 密码
}

// Register 处理 POST /api/v1/auth/register。
// 将 HTTP/JSON 请求转发给 User 服务的 Register RPC。
// 成功返回 201 + user_id，gRPC 错误映射为对应的 HTTP 状态码。
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGUMENT", "message": "invalid request body"})
		return
	}

	userID, err := h.userClient.Register(c.Request.Context(), req.Email, req.Password, req.Nickname)
	if err != nil {
		writeGRPCError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"user_id": userID})
}

// Login 处理 POST /api/v1/auth/login。
// 将 HTTP/JSON 请求转发给 User 服务的 Login RPC。
// 成功返回 200 + access_token / expires_at / user_id。
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGUMENT", "message": "invalid request body"})
		return
	}

	token, expiresAt, userID, err := h.userClient.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		writeGRPCError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token": token,
		"expires_at":   expiresAt,
		"user_id":      userID,
	})
}

// GetMe 处理 GET /api/v1/auth/me（需要 JWT 中间件）。
// 从 gin.Context 获取 JWT 中间件写入的 userID，调用 User 服务的 GetUser RPC。
func (h *AuthHandler) GetMe(c *gin.Context) {
	userID := c.GetInt64("userID")
	resp, err := h.userClient.GetUser(c.Request.Context(), userID)
	if err != nil {
		writeGRPCError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user_id":    resp.GetUserId(),
		"email":      resp.GetEmail(),
		"nickname":   resp.GetNickname(),
		"created_at": resp.GetCreatedAt().AsTime().Format("2006-01-02T15:04:05Z"),
	})
}

// grpcCodeToHTTP 将 gRPC 状态码映射为 HTTP 状态码。
var grpcCodeToHTTP = map[codes.Code]int{
	codes.InvalidArgument:   http.StatusBadRequest,      // 400
	codes.Unauthenticated:   http.StatusUnauthorized,    // 401
	codes.PermissionDenied:  http.StatusForbidden,       // 403
	codes.NotFound:          http.StatusNotFound,        // 404
	codes.AlreadyExists:     http.StatusConflict,        // 409
	codes.ResourceExhausted: http.StatusTooManyRequests, // 429
}

// writeGRPCError 将 gRPC 错误转换为 HTTP JSON 响应。
// 已知的 gRPC 状态码映射为对应 HTTP 状态码，未知的统一返回 500。
func writeGRPCError(c *gin.Context, err error) {
	st, ok := status.FromError(err)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "INTERNAL", "message": "internal server error"})
		return
	}
	httpCode, exists := grpcCodeToHTTP[st.Code()]
	if !exists {
		httpCode = http.StatusInternalServerError
	}
	c.JSON(httpCode, gin.H{"code": st.Code().String(), "message": st.Message()})
}
