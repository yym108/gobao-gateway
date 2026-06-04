package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	adminv1 "github.com/yym108/gobao-proto/gen/go/gobao/admin/v1"

	"github.com/yym108/gobao-gateway/internal/client"
)

// AdminAuthHandler 处理管理员认证相关的 HTTP 请求。
// 它与普通用户认证完全分离，专门服务后台管理入口。
type AdminAuthHandler struct {
	adminClient *client.AdminClient // Admin 服务的 gRPC client
}

// NewAdminAuthHandler 构造管理员认证 handler。
func NewAdminAuthHandler(ac *client.AdminClient) *AdminAuthHandler {
	return &AdminAuthHandler{adminClient: ac}
}

type adminLoginRequest struct {
	Email    string `json:"email"`    // 管理员邮箱
	Password string `json:"password"` // 管理员密码
}

type adminChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"` // 当前密码
	NewPassword     string `json:"new_password"`     // 新密码
}

type createAdminRequest struct {
	Email        string `json:"email"`          // 新后台账号邮箱
	Password     string `json:"password"`       // 初始密码
	Nickname     string `json:"nickname"`       // 后台昵称
	AvatarURL    string `json:"avatar_url"`     // 头像地址
	IsSuperAdmin bool   `json:"is_super_admin"` // 是否授予超管权限
}

type updateAdminPasswordRequest struct {
	NewPassword string `json:"new_password"` // 超管为目标账号设置的新密码
}

// AdminLogin 处理 POST /api/v1/admin/auth/login。
func (h *AdminAuthHandler) AdminLogin(c *gin.Context) {
	var req adminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGUMENT", "message": "invalid request body"})
		return
	}

	token, expiresAt, adminID, err := h.adminClient.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		writeGRPCError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"access_token": token,
		"expires_at":   expiresAt,
		"admin_id":     adminID,
	})
}

// GetAdminMe 处理 GET /api/v1/admin/auth/me。
func (h *AdminAuthHandler) GetAdminMe(c *gin.Context) {
	adminID := c.GetInt64("userID")
	resp, err := h.adminClient.GetAdmin(c.Request.Context(), adminID)
	if err != nil {
		writeGRPCError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"admin_id":       resp.GetAdminId(),
		"email":          resp.GetEmail(),
		"nickname":       resp.GetNickname(),
		"avatar_url":     resp.GetAvatarUrl(),
		"is_super_admin": resp.GetIsSuperAdmin(),
	})
}

// ChangePassword 处理 POST /api/v1/admin/auth/password/change。
func (h *AdminAuthHandler) ChangePassword(c *gin.Context) {
	var req adminChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGUMENT", "message": "invalid request body"})
		return
	}
	adminID := c.GetInt64("userID")
	resp, err := h.adminClient.ChangePassword(c.Request.Context(), adminID, req.CurrentPassword, req.NewPassword)
	if err != nil {
		writeGRPCError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": resp.GetMessage()})
}

// ListAdmins 处理 GET /api/v1/admin/accounts，仅超管可用。
func (h *AdminAuthHandler) ListAdmins(c *gin.Context) {
	adminID := c.GetInt64("userID")
	resp, err := h.adminClient.ListAdmins(c.Request.Context(), adminID)
	if err != nil {
		writeGRPCError(c, err)
		return
	}
	items := make([]gin.H, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, gin.H{
			"admin_id":       item.GetAdminId(),
			"email":          item.GetEmail(),
			"nickname":       item.GetNickname(),
			"avatar_url":     item.GetAvatarUrl(),
			"is_super_admin": item.GetIsSuperAdmin(),
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

// CreateAdmin 处理 POST /api/v1/admin/accounts，仅超管可用。
func (h *AdminAuthHandler) CreateAdmin(c *gin.Context) {
	var req createAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGUMENT", "message": "invalid request body"})
		return
	}
	adminID := c.GetInt64("userID")
	resp, err := h.adminClient.CreateAdmin(c.Request.Context(), &adminv1.CreateAdminRequest{
		RequesterAdminId: adminID,
		Email:            req.Email,
		Password:         req.Password,
		Nickname:         req.Nickname,
		AvatarUrl:        req.AvatarURL,
		IsSuperAdmin:     req.IsSuperAdmin,
	})
	if err != nil {
		writeGRPCError(c, err)
		return
	}
	item := resp.GetAdmin()
	c.JSON(http.StatusCreated, gin.H{
		"admin": gin.H{
			"admin_id":       item.GetAdminId(),
			"email":          item.GetEmail(),
			"nickname":       item.GetNickname(),
			"avatar_url":     item.GetAvatarUrl(),
			"is_super_admin": item.GetIsSuperAdmin(),
		},
	})
}

// UpdateAdminPassword 处理 POST /api/v1/admin/accounts/:id/password，仅超管可用。
func (h *AdminAuthHandler) UpdateAdminPassword(c *gin.Context) {
	var req updateAdminPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGUMENT", "message": "invalid request body"})
		return
	}
	targetAdminID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || targetAdminID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ARGUMENT", "message": "invalid admin id"})
		return
	}
	requesterAdminID := c.GetInt64("userID")
	resp, err := h.adminClient.UpdateAdminPassword(c.Request.Context(), requesterAdminID, targetAdminID, req.NewPassword)
	if err != nil {
		writeGRPCError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": resp.GetMessage()})
}
