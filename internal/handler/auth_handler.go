// Package handler 提供 Gateway 的 HTTP 请求处理器。
// 每个 handler 将 HTTP/JSON 请求转换为 gRPC 调用，并将 gRPC 响应/错误转换回 HTTP。
package handler

import (
	"encoding/base64"
	"net/http"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/yym108/gobao-gateway/internal/client"
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

// updateProfileRequest 是 PUT /api/v1/profile 的请求体。
type updateProfileRequest struct {
	Nickname  string `json:"nickname"`   // 昵称
	AvatarURL string `json:"avatar_url"` // 头像地址
}

// uploadAvatarRequest 是 POST /api/v1/profile/avatar 的请求体。
// 前端裁剪后的图片以 base64 承载，由 user 服务自行存储并回写 avatar_url。
type uploadAvatarRequest struct {
	FileName      string `json:"file_name"`      // 原始文件名，用于推断扩展名
	MIMEType      string `json:"mime_type"`      // 图片 MIME 类型
	ContentBase64 string `json:"content_base64"` // 裁剪后图片的 base64 内容
}

// changePasswordRequest 是 POST /api/v1/profile/password/change 的请求体。
type changePasswordRequest struct {
	Code        string `json:"code"`         // 邮箱验证码
	NewPassword string `json:"new_password"` // 新密码
}

// forgotPasswordCodeRequest 是 POST /api/v1/auth/password/code 的请求体。
type forgotPasswordCodeRequest struct {
	Email string `json:"email"` // 邮箱
}

// resetPasswordByEmailRequest 是 POST /api/v1/auth/password/reset 的请求体。
type resetPasswordByEmailRequest struct {
	Email       string `json:"email"`        // 邮箱
	Code        string `json:"code"`         // 邮箱验证码
	NewPassword string `json:"new_password"` // 新密码
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

// SendPasswordResetCodeByEmail 处理 POST /api/v1/auth/password/code。
func (h *AuthHandler) SendPasswordResetCodeByEmail(c *gin.Context) {
	var req forgotPasswordCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGUMENT", "message": "invalid request body"})
		return
	}

	resp, err := h.userClient.SendPasswordResetCodeByEmail(c.Request.Context(), req.Email)
	if err != nil {
		writeGRPCError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": resp.GetMessage(),
	})
}

// ResetPasswordByEmail 处理 POST /api/v1/auth/password/reset。
func (h *AuthHandler) ResetPasswordByEmail(c *gin.Context) {
	var req resetPasswordByEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGUMENT", "message": "invalid request body"})
		return
	}

	resp, err := h.userClient.ResetPasswordByEmail(c.Request.Context(), req.Email, req.Code, req.NewPassword)
	if err != nil {
		writeGRPCError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": resp.GetMessage(),
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

// GetProfile 处理 GET /api/v1/profile。
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID := c.GetInt64("userID")
	resp, err := h.userClient.GetProfile(c.Request.Context(), userID)
	if err != nil {
		writeGRPCError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user_id":    resp.GetUserId(),
		"email":      resp.GetEmail(),
		"nickname":   resp.GetNickname(),
		"avatar_url": resp.GetAvatarUrl(),
	})
}

// UpdateProfile 处理 PUT /api/v1/profile。
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	var req updateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGUMENT", "message": "invalid request body"})
		return
	}

	userID := c.GetInt64("userID")
	resp, err := h.userClient.UpdateProfile(c.Request.Context(), userID, req.Nickname, req.AvatarURL)
	if err != nil {
		writeGRPCError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user_id":    resp.GetUserId(),
		"email":      resp.GetEmail(),
		"nickname":   resp.GetNickname(),
		"avatar_url": resp.GetAvatarUrl(),
	})
}

// UploadAvatar 处理 POST /api/v1/profile/avatar。
// 接收前端裁剪后的 base64 图片，转发到 User 服务存储并回写头像地址。
func (h *AuthHandler) UploadAvatar(c *gin.Context) {
	var req uploadAvatarRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGUMENT", "message": "invalid request body"})
		return
	}
	content, err := base64.StdEncoding.DecodeString(req.ContentBase64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGUMENT", "message": "content_base64 非法"})
		return
	}

	userID := c.GetInt64("userID")
	resp, rpcErr := h.userClient.UploadAvatar(c.Request.Context(), userID, req.FileName, req.MIMEType, content)
	if rpcErr != nil {
		writeGRPCError(c, rpcErr)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user_id":    resp.GetUserId(),
		"email":      resp.GetEmail(),
		"nickname":   resp.GetNickname(),
		"avatar_url": resp.GetAvatarUrl(),
	})
}

// SendPasswordResetCode 处理 POST /api/v1/profile/password/code。
func (h *AuthHandler) SendPasswordResetCode(c *gin.Context) {
	userID := c.GetInt64("userID")
	resp, err := h.userClient.SendPasswordResetCode(c.Request.Context(), userID)
	if err != nil {
		writeGRPCError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": resp.GetMessage(),
	})
}

// GetPasswordResetCode 处理 GET /api/v1/profile/password/code。
// 仅供开发/演示环境：把存在 Redis 里的改密验证码读回给当前登录用户，生产环境必须关闭。
func (h *AuthHandler) GetPasswordResetCode(c *gin.Context) {
	userID := c.GetInt64("userID")
	resp, err := h.userClient.GetPasswordResetCode(c.Request.Context(), userID)
	if err != nil {
		writeGRPCError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": resp.GetCode(),
	})
}

// ChangePassword 处理 POST /api/v1/profile/password/change。
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGUMENT", "message": "invalid request body"})
		return
	}

	userID := c.GetInt64("userID")
	resp, err := h.userClient.ChangePassword(c.Request.Context(), userID, req.Code, req.NewPassword)
	if err != nil {
		writeGRPCError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": resp.GetMessage(),
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
