// Package middleware 提供 Gateway 的 Gin 中间件。
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yym108/gobao-pkg/authn"
)

// JWTAuth 返回 Gin 中间件，从 Authorization: Bearer <token> 提取 JWT，
// 用 JWTManager 本地校验（不走 gRPC），成功后将 userID 和 email 写入 gin.Context。
//
// 设计决策：JWT 是自包含的，网关持有与 User 服务相同的 secret 即可本地校验，
// 无需每次请求都调用 User 服务的 VerifyToken RPC，降低延迟和耦合。
//
//   - jwtMgr: JWT 管理器（与 User 服务使用相同的 secret）
func JWTAuth(jwtMgr *authn.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 提取 Authorization 头
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": "UNAUTHENTICATED", "message": "missing authorization header",
			})
			return
		}

		// 2. 校验格式：必须是 "Bearer <token>"
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": "UNAUTHENTICATED", "message": "invalid authorization format",
			})
			return
		}

		// 3. 本地校验 JWT（验签 + 过期检查）
		claims, err := jwtMgr.Verify(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code": "UNAUTHENTICATED", "message": "invalid or expired token",
			})
			return
		}

		// 4. 将用户信息写入 context，后续 handler 通过 c.GetInt64("userID") 获取
		c.Set("userID", claims.UserID)
		c.Set("userEmail", claims.Email)
		c.Next()
	}
}
