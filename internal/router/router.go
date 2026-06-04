// Package router 提供 Gateway 的 Gin 路由配置。
// 路由分为公开路由（无需鉴权）和受保护路由（需 JWT 中间件校验），
// 实现 Client → Gateway(JWT) → 后端微服务(gRPC) 的网关转发链路。
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/yym108/gobao-gateway/internal/handler"
	"github.com/yym108/gobao-gateway/internal/middleware"
	"github.com/yym108/gobao-pkg/authn"
	"github.com/yym108/gobao-pkg/httpx"
)

// New 创建并返回配置好中间件和路由的 Gin 引擎。
//   - jwtMgr:      JWT 管理器，用于受保护路由的鉴权中间件
//   - authHandler: 认证相关的 HTTP handler（注册/登录/获取当前用户）
//   - adminAuthHandler: 管理员认证相关的 HTTP handler（后台登录/获取当前管理员）
//   - productHandler: 商品/类目相关的 HTTP handler
//   - seckillHandler: 秒杀活动相关的 HTTP handler
//   - cartHandler: 购物车相关的 HTTP handler
//   - orderHandler: 订单相关的 HTTP handler
//   - favoriteHandler: 收藏相关的 HTTP handler
func New(
	jwtMgr *authn.JWTManager,
	authHandler *handler.AuthHandler,
	adminAuthHandler *handler.AdminAuthHandler,
	addressHandler *handler.AddressHandler,
	productHandler *handler.ProductHandler,
	adminMediaHandler *handler.AdminMediaHandler,
	seckillHandler *handler.SeckillHandler,
	cartHandler *handler.CartHandler,
	orderHandler *handler.OrderHandler,
	paymentHandler *handler.PaymentHandler,
	favoriteHandler *handler.FavoriteHandler,
	exposeDevEndpoints bool,
) *gin.Engine {
	r := gin.New()

	// 全局中间件：Prometheus 指标采集、日志、panic 恢复、请求 ID 注入。
	// Metrics 置于最前，覆盖全部请求并准确记录最终状态码。
	r.Use(middleware.Metrics("gateway"), gin.Logger(), httpx.Recover(), httpx.RequestID())
	r.Use(func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		if c.Request.Method == http.MethodOptions {
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}
		c.Next()
	})

	// 基础运维路由（不在 /api/v1 下，不受鉴权保护）
	r.GET("/healthz", func(c *gin.Context) { c.String(200, "ok") })
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	v1 := r.Group("/api/v1")

	// ── 公开路由（无需 JWT） ──
	pub := v1.Group("")
	pub.POST("/auth/register", authHandler.Register) // 用户注册
	pub.POST("/auth/login", authHandler.Login)       // 用户登录
	pub.POST("/auth/password/code", authHandler.SendPasswordResetCodeByEmail)
	pub.POST("/auth/password/reset", authHandler.ResetPasswordByEmail)
	pub.POST("/admin/auth/login", adminAuthHandler.AdminLogin)
	pub.GET("/products", productHandler.ListProducts)
	pub.GET("/products/:id", productHandler.GetProduct)
	pub.GET("/product-groups", productHandler.ListProductGroups)
	pub.GET("/categories", productHandler.ListCategories)
	pub.GET("/seckill/activities/:id", seckillHandler.GetActivity)

	// ── 受保护路由（需 JWT 中间件校验） ──
	protected := v1.Group("")
	protected.Use(middleware.JWTAuth(jwtMgr))
	protected.GET("/auth/me", authHandler.GetMe) // 获取当前用户信息
	protected.GET("/profile", authHandler.GetProfile)
	protected.PUT("/profile", authHandler.UpdateProfile)
	protected.POST("/profile/avatar", authHandler.UploadAvatar)
	protected.POST("/profile/password/code", authHandler.SendPasswordResetCode)
	// 取回改密验证码属于开发/演示便利能力，仅在显式开启开发接口时注册，生产默认关闭。
	if exposeDevEndpoints {
		protected.GET("/profile/password/code", authHandler.GetPasswordResetCode)
	}
	protected.POST("/profile/password/change", authHandler.ChangePassword)
	protected.GET("/addresses", addressHandler.ListAddresses)
	protected.GET("/addresses/:id", addressHandler.GetAddress)
	protected.POST("/addresses", addressHandler.CreateAddress)
	protected.PUT("/addresses/:id", addressHandler.UpdateAddress)
	protected.DELETE("/addresses/:id", addressHandler.DeleteAddress)
	protected.POST("/addresses/default", addressHandler.SetDefaultAddress)
	protected.GET("/ping", func(c *gin.Context) {
		// 返回 pong + 当前登录用户 ID，用于验证鉴权链路
		c.JSON(200, gin.H{"pong": true, "user_id": c.GetInt64("userID")})
	})
	protected.POST("/seckill/activities/:id/purchase", seckillHandler.Purchase)
	protected.GET("/cart", cartHandler.GetCart)
	protected.POST("/cart/items", cartHandler.AddItem)
	protected.PUT("/cart/items/:productId", cartHandler.UpdateItem)
	protected.DELETE("/cart/items/:productId", cartHandler.DeleteItem)
	protected.GET("/favorites", favoriteHandler.ListFavorites)
	protected.POST("/favorites", favoriteHandler.AddFavorite)
	protected.DELETE("/favorites/:productId", favoriteHandler.DeleteFavorite)
	protected.GET("/orders", orderHandler.ListOrders)
	protected.GET("/orders/:id", orderHandler.GetOrder)
	protected.POST("/orders", orderHandler.CreateOrder)
	protected.POST("/orders/:id/cancel", orderHandler.CancelOrder)
	protected.GET("/payments/:id", paymentHandler.GetPayment)
	protected.GET("/payments/by-order/:orderId", paymentHandler.GetPaymentByOrder)
	protected.POST("/payments/:id/mock-confirm", paymentHandler.MockConfirmPayment)

	adminProtected := v1.Group("/admin")
	adminProtected.Use(middleware.JWTAuthWithRole(jwtMgr, "admin"))
	adminProtected.GET("/auth/me", adminAuthHandler.GetAdminMe)
	adminProtected.POST("/auth/password/change", adminAuthHandler.ChangePassword)
	adminProtected.GET("/accounts", adminAuthHandler.ListAdmins)
	adminProtected.POST("/accounts", adminAuthHandler.CreateAdmin)
	adminProtected.POST("/accounts/:id/password", adminAuthHandler.UpdateAdminPassword)
	adminProtected.GET("/orders", orderHandler.AdminListOrders)
	adminProtected.GET("/orders/:id", orderHandler.AdminGetOrder)
	adminProtected.POST("/orders/:id/cancel", orderHandler.AdminCancelOrder)
	adminProtected.POST("/media/upload", adminMediaHandler.UploadMedia)
	adminProtected.POST("/product-groups", productHandler.CreateProductGroup)
	adminProtected.PUT("/product-groups/item/:id", productHandler.UpdateProductGroup)
	adminProtected.DELETE("/product-groups/item/:id", productHandler.DeleteProductGroup)
	adminProtected.POST("/product-groups/:groupId/media", adminMediaHandler.BindProductGroupMedia)
	adminProtected.POST("/products/:id/media", adminMediaHandler.BindProductMedia)
	adminProtected.PUT("/product-groups/:groupId/media/:bindingId", adminMediaHandler.UpdateProductGroupMediaBinding)
	adminProtected.PUT("/products/:id/media/:bindingId", adminMediaHandler.UpdateProductMediaBinding)
	adminProtected.DELETE("/product-groups/:groupId/media/:bindingId", adminMediaHandler.DeleteProductGroupMediaBinding)
	adminProtected.DELETE("/products/:id/media/:bindingId", adminMediaHandler.DeleteProductMediaBinding)
	adminProtected.POST("/products", productHandler.CreateProduct)
	adminProtected.PUT("/products/:id", productHandler.UpdateProduct)
	adminProtected.PUT("/products/:id/stock", productHandler.UpdateProductStock)
	adminProtected.DELETE("/products/:id", productHandler.DeleteProduct)
	adminProtected.POST("/categories", productHandler.CreateCategory)
	adminProtected.PUT("/categories/:id", productHandler.UpdateCategory)
	adminProtected.DELETE("/categories/:id", productHandler.DeleteCategory)
	adminProtected.POST("/seckill/activities/:id/prewarm", seckillHandler.PrewarmActivity)

	return r
}
