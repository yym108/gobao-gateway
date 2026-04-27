package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yym/gobao-pkg/authn"
)

func init() { gin.SetMode(gin.TestMode) }

// setupRouter 构造带 JWT 中间件的测试路由。
// 受保护路由返回 context 中的 userID，用于验证中间件是否正确写入。
func setupRouter(jwtMgr *authn.JWTManager) *gin.Engine {
	r := gin.New()
	r.GET("/protected", JWTAuth(jwtMgr), func(c *gin.Context) {
		c.JSON(200, gin.H{"user_id": c.GetInt64("userID"), "email": c.GetString("userEmail")})
	})
	return r
}

func TestJWTAuth_ValidToken(t *testing.T) {
	mgr := authn.NewJWTManager("test-secret", time.Hour)
	token, _, err := mgr.Sign(42, "alice@test.com")
	require.NoError(t, err)

	r := setupRouter(mgr)
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"user_id":42`)
	assert.Contains(t, w.Body.String(), `"email":"alice@test.com"`)
}

func TestJWTAuth_MissingHeader(t *testing.T) {
	mgr := authn.NewJWTManager("test-secret", time.Hour)
	r := setupRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Body.String(), "missing authorization header")
}

func TestJWTAuth_InvalidFormat(t *testing.T) {
	mgr := authn.NewJWTManager("test-secret", time.Hour)
	r := setupRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Basic abc123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Body.String(), "invalid authorization format")
}

func TestJWTAuth_ExpiredToken(t *testing.T) {
	mgr := authn.NewJWTManager("test-secret", -time.Hour) // 签发即过期
	token, _, err := mgr.Sign(1, "bob@test.com")
	require.NoError(t, err)

	verifyMgr := authn.NewJWTManager("test-secret", time.Hour) // 用于校验的 mgr
	r := setupRouter(verifyMgr)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Body.String(), "invalid or expired token")
}

func TestJWTAuth_MalformedToken(t *testing.T) {
	mgr := authn.NewJWTManager("test-secret", time.Hour)
	r := setupRouter(mgr)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-jwt")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Body.String(), "invalid or expired token")
}
