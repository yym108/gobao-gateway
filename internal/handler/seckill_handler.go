// Package handler 提供 Gateway 的 HTTP 请求处理器。
// 本文件实现秒杀活动查询、预热和抢购入口，将 HTTP 请求编排为 Product gRPC、Redis 幂等和 NATS 投递。
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/yym108/gobao-gateway/internal/client"
	"github.com/yym108/gobao-pkg/cache"
	"github.com/yym108/gobao-pkg/idempotency"
	"github.com/yym108/gobao-pkg/mq"
	productv1 "github.com/yym108/gobao-proto/gen/go/gobao/product/v1"
)

// SeckillHandler 处理秒杀活动相关 HTTP 请求。
// 它负责把查询/预热转发到 Product 服务，并把抢购请求做幂等校验后投递到消息总线。
type SeckillHandler struct {
	productClient  *client.ProductClient // Product 服务 gRPC client
	rdb            *redis.Client         // Redis 客户端，用于库存预扣与失败回补
	idemGuard      *idempotency.Guard    // Redis 幂等守卫
	stockScript    *redis.Script         // 秒杀库存原子预扣 Lua 脚本
	bus            *mq.Bus               // NATS JetStream 总线
	seckillSubject string                // 秒杀下单投递主题
}

// seckillStockDeductScript 在 Redis 中原子执行库存预扣。
// 返回值约定：
//   - >= 0: 扣减后剩余库存
//   - -1: 库存不足
//   - -2: 未预热，库存 key 不存在
const seckillStockDeductScript = `
local current = redis.call("GET", KEYS[1])
if not current then
  return -2
end
local remain = tonumber(current)
local deduct = tonumber(ARGV[1])
if remain < deduct then
  return -1
end
return redis.call("DECRBY", KEYS[1], deduct)
`

// SeckillPurchaseRequest 描述抢购入口的请求体。
type SeckillPurchaseRequest struct {
	RequestID string `json:"request_id"` // 幂等请求 ID，同一用户重试时应保持一致
	Quantity  int32  `json:"quantity"`   // 抢购数量，当前基础版只允许为 1
}

// seckillOrderMessage 是投递到 seckill.order 的基础消息体。
// I2b 阶段只要求真实入队，不要求 Order 侧完成真实消费。
type seckillOrderMessage struct {
	RequestID  string `json:"request_id"`  // 幂等请求 ID
	UserID     int64  `json:"user_id"`     // 发起抢购的用户 ID
	ActivityID int64  `json:"activity_id"` // 秒杀活动 ID
	ProductID  int64  `json:"product_id"`  // 关联商品 ID
	Quantity   int32  `json:"quantity"`    // 抢购数量
	QueuedAt   int64  `json:"queued_at"`   // 入队时间戳（Unix 秒）
}

// NewSeckillHandler 构造秒杀 HTTP Handler。
//   - productClient: Product 服务 gRPC client
//   - rdb: Redis 客户端，用于构造幂等守卫
//   - bus: NATS JetStream 总线
//   - seckillSubject: 秒杀下单投递主题
func NewSeckillHandler(
	productClient *client.ProductClient,
	rdb *redis.Client,
	bus *mq.Bus,
	seckillSubject string,
) *SeckillHandler {
	return &SeckillHandler{
		productClient:  productClient,
		rdb:            rdb,
		idemGuard:      idempotency.New(rdb, "seckill:req:"),
		stockScript:    cache.LoadScript(seckillStockDeductScript),
		bus:            bus,
		seckillSubject: seckillSubject,
	}
}

// handleGRPC 将 Product 服务返回的 gRPC 错误写回 HTTP 响应。
// 返回 true 表示已完成错误响应写入，调用方应立即 return。
func (h *SeckillHandler) handleGRPC(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	code, msg := grpcErrToHTTP(err)
	c.JSON(code, gin.H{"error": msg})
	return true
}

// GetActivity 处理 GET /api/v1/seckill/activities/:id。
// 该接口直接透传 Product 服务的秒杀活动查询能力。
func (h *SeckillHandler) GetActivity(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	resp, err := h.productClient.GetSeckillActivity(c.Request.Context(), &productv1.GetSeckillActivityRequest{Id: id})
	if h.handleGRPC(c, err) {
		return
	}
	c.JSON(http.StatusOK, resp)
}

// PrewarmActivity 处理 POST /api/v1/seckill/activities/:id/prewarm。
// 该接口仅供后台或联调用于触发 Redis 预热。
func (h *SeckillHandler) PrewarmActivity(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	resp, err := h.productClient.PrewarmSeckill(c.Request.Context(), &productv1.PrewarmSeckillRequest{Id: id})
	if h.handleGRPC(c, err) {
		return
	}
	c.JSON(http.StatusOK, resp)
}

// Purchase 处理 POST /api/v1/seckill/activities/:id/purchase。
// 当前阶段只做活动校验、幂等去重和下单消息入队，不直接落订单。
func (h *SeckillHandler) Purchase(c *gin.Context) {
	activityID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	var req SeckillPurchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.RequestID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request_id 不能为空"})
		return
	}
	if req.Quantity != 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "quantity 当前仅支持为 1"})
		return
	}

	activityResp, err := h.productClient.GetSeckillActivity(c.Request.Context(), &productv1.GetSeckillActivityRequest{Id: activityID})
	if h.handleGRPC(c, err) {
		return
	}
	activity := activityResp.GetActivity()
	now := time.Now().Unix()
	if activity.GetStartAt() > now {
		c.JSON(http.StatusPreconditionFailed, gin.H{"error": "秒杀活动尚未开始"})
		return
	}
	if activity.GetEndAt() <= now {
		c.JSON(http.StatusPreconditionFailed, gin.H{"error": "秒杀活动已结束"})
		return
	}

	userID := c.GetInt64("userID")
	idemKey := fmt.Sprintf("%d:%d:%s", userID, activityID, req.RequestID)
	acquired, err := h.idemGuard.Acquire(c.Request.Context(), idemKey, 10*time.Minute)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !acquired {
		c.JSON(http.StatusConflict, gin.H{
			"error":       "重复请求",
			"request_id":  req.RequestID,
			"activity_id": activityID,
		})
		return
	}

	stockKey := fmt.Sprintf("seckill:activity:%d:stock", activityID)
	remaining, err := h.stockScript.Run(c.Request.Context(), h.rdb, []string{stockKey}, req.Quantity).Int64()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if remaining == -2 {
		c.JSON(http.StatusPreconditionFailed, gin.H{"error": "秒杀活动尚未预热"})
		return
	}
	if remaining == -1 {
		c.JSON(http.StatusConflict, gin.H{"error": "秒杀库存不足"})
		return
	}

	msg := seckillOrderMessage{
		RequestID:  req.RequestID,
		UserID:     userID,
		ActivityID: activityID,
		ProductID:  activity.GetProductId(),
		Quantity:   req.Quantity,
		QueuedAt:   time.Now().Unix(),
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		_ = h.rdb.IncrBy(c.Request.Context(), stockKey, int64(req.Quantity)).Err()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.bus.Publish(c.Request.Context(), h.seckillSubject, payload); err != nil {
		_ = h.rdb.IncrBy(c.Request.Context(), stockKey, int64(req.Quantity)).Err()
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"queued":      true,
		"request_id":  req.RequestID,
		"activity_id": activityID,
		"product_id":  msg.ProductID,
		"subject":     h.seckillSubject,
		"queued_at":   msg.QueuedAt,
		"quantity":    req.Quantity,
		"remaining":   remaining,
	})
}
