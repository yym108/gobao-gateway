// Package handler 提供 Gateway 的 HTTP 请求处理器。
// 本文件实现最小购物车 HTTP 接口，并通过可替换的存储接口承接购物车数据。
package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/yym108/gobao-gateway/internal/client"
)

var errCartItemNotFound = errors.New("cart item not found")

// CartItem 表示购物车中的单个商品条目。
type CartItem struct {
	CartItemID    string `json:"cart_item_id"`   // 购物车条目唯一标识，按商品与规格组合生成
	ProductID     int64  `json:"product_id"`     // 商品 ID
	SKUID         int64  `json:"sku_id"`         // SKU ID，作为规格配置的唯一后端标识
	Name          string `json:"name"`           // 商品名称
	Price         int64  `json:"price"`          // 商品单价，单位分
	Quantity      int32  `json:"quantity"`       // 购买数量
	ImageURL      string `json:"image_url"`      // 商品图片 URL
	OptionSummary string `json:"option_summary"` // 当前条目的规格摘要，用于前端展示与分条存储
}

// CartResponse 是购物车查询与变更后的统一响应。
type CartResponse struct {
	Items         []CartItem `json:"items"`          // 当前购物车条目列表
	TotalQuantity int32      `json:"total_quantity"` // 商品总件数
	TotalAmount   int64      `json:"total_amount"`   // 商品总金额，单位分
}

// addCartItemRequest 描述加入购物车的请求体。
type addCartItemRequest struct {
	SKUID    int64 `json:"sku_id"`   // SKU ID，后端据此解析商品、价格和规格
	Quantity int32 `json:"quantity"` // 加入数量
}

// updateCartItemRequest 描述更新购物车数量的请求体。
type updateCartItemRequest struct {
	Quantity int32 `json:"quantity"` // 目标数量
}

// CartStore 抽象购物车数据存储，便于在内存实现与 Redis 实现之间平滑切换。
type CartStore interface {
	Get(ctx context.Context, userID int64) (CartResponse, error)
	Add(ctx context.Context, userID int64, item CartItem) (CartResponse, error)
	Update(ctx context.Context, userID int64, cartItemID string, quantity int32) (CartResponse, error)
	Delete(ctx context.Context, userID int64, cartItemID string) error
}

// MemoryCartStore 是最小购物车内存存储，按 userID 隔离购物车内容。
// 该实现主要用于单元测试与本地降级运行，不适合作为可用系统的最终存储。
type MemoryCartStore struct {
	mu    sync.RWMutex
	carts map[int64]map[string]CartItem
}

// NewMemoryCartStore 构造内存级购物车存储。
func NewMemoryCartStore() *MemoryCartStore {
	return &MemoryCartStore{carts: make(map[int64]map[string]CartItem)}
}

// Get 返回指定用户的购物车快照。
func (s *MemoryCartStore) Get(_ context.Context, userID int64) (CartResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return buildCartResponse(s.carts[userID]), nil
}

// Add 向指定用户购物车新增或累加商品。
func (s *MemoryCartStore) Add(_ context.Context, userID int64, item CartItem) (CartResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	item.CartItemID = buildCartItemID(item.ProductID, item.OptionSummary)
	cart := s.ensureCart(userID)
	if current, ok := cart[item.CartItemID]; ok {
		current.Quantity += item.Quantity
		current.Name = item.Name
		current.Price = item.Price
		current.ImageURL = item.ImageURL
		current.OptionSummary = item.OptionSummary
		cart[item.CartItemID] = current
	} else {
		cart[item.CartItemID] = item
	}
	return buildCartResponse(cart), nil
}

// Update 修改指定购物车条目的数量。
func (s *MemoryCartStore) Update(_ context.Context, userID int64, cartItemID string, quantity int32) (CartResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cart := s.ensureCart(userID)
	item, ok := cart[cartItemID]
	if !ok {
		return CartResponse{}, errCartItemNotFound
	}
	item.Quantity = quantity
	cart[cartItemID] = item
	return buildCartResponse(cart), nil
}

// Delete 删除指定购物车条目。
func (s *MemoryCartStore) Delete(_ context.Context, userID int64, cartItemID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cart, ok := s.carts[userID]; ok {
		delete(cart, cartItemID)
	}
	return nil
}

// ensureCart 确保用户购物车 map 已初始化。
func (s *MemoryCartStore) ensureCart(userID int64) map[string]CartItem {
	cart, ok := s.carts[userID]
	if !ok {
		cart = make(map[string]CartItem)
		s.carts[userID] = cart
	}
	return cart
}

// RedisCartStore 使用 Redis Hash 持久化单用户购物车。
// 每个用户一个 Hash，field 为购物车条目 ID，value 为 CartItem JSON。
type RedisCartStore struct {
	client *redis.Client
}

// NewRedisCartStore 构造 Redis 购物车存储。
func NewRedisCartStore(client *redis.Client) *RedisCartStore {
	return &RedisCartStore{client: client}
}

// Get 读取指定用户购物车快照。
func (s *RedisCartStore) Get(ctx context.Context, userID int64) (CartResponse, error) {
	cart, err := s.loadCart(ctx, userID)
	if err != nil {
		return CartResponse{}, err
	}
	return buildCartResponse(cart), nil
}

// Add 向 Redis 购物车新增或累加商品。
func (s *RedisCartStore) Add(ctx context.Context, userID int64, item CartItem) (CartResponse, error) {
	cart, err := s.loadCart(ctx, userID)
	if err != nil {
		return CartResponse{}, err
	}
	item.CartItemID = buildCartItemID(item.ProductID, item.OptionSummary)
	if current, ok := cart[item.CartItemID]; ok {
		current.Quantity += item.Quantity
		current.Name = item.Name
		current.Price = item.Price
		current.ImageURL = item.ImageURL
		current.OptionSummary = item.OptionSummary
		cart[item.CartItemID] = current
	} else {
		cart[item.CartItemID] = item
	}
	if err := s.saveItem(ctx, userID, cart[item.CartItemID]); err != nil {
		return CartResponse{}, err
	}
	return buildCartResponse(cart), nil
}

// Update 修改 Redis 购物车中指定商品数量。
func (s *RedisCartStore) Update(ctx context.Context, userID int64, cartItemID string, quantity int32) (CartResponse, error) {
	cart, err := s.loadCart(ctx, userID)
	if err != nil {
		return CartResponse{}, err
	}
	item, ok := cart[cartItemID]
	if !ok {
		return CartResponse{}, errCartItemNotFound
	}
	item.Quantity = quantity
	cart[cartItemID] = item
	if err := s.saveItem(ctx, userID, item); err != nil {
		return CartResponse{}, err
	}
	return buildCartResponse(cart), nil
}

// Delete 删除 Redis 购物车中的指定商品条目。
func (s *RedisCartStore) Delete(ctx context.Context, userID int64, cartItemID string) error {
	return s.client.HDel(ctx, cartRedisKey(userID), cartItemID).Err()
}

// loadCart 读取并反序列化当前用户的全部购物车条目。
func (s *RedisCartStore) loadCart(ctx context.Context, userID int64) (map[string]CartItem, error) {
	values, err := s.client.HGetAll(ctx, cartRedisKey(userID)).Result()
	if err != nil {
		return nil, err
	}
	cart := make(map[string]CartItem, len(values))
	for field, raw := range values {
		var item CartItem
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, err
		}
		item.CartItemID = field
		migratedItem, legacyID, migrated := migrateCartItemIDIfNeeded(item)
		if migrated {
			if err := s.client.HDel(ctx, cartRedisKey(userID), legacyID).Err(); err != nil {
				return nil, err
			}
			if err := s.saveItem(ctx, userID, migratedItem); err != nil {
				return nil, err
			}
		}
		cart[migratedItem.CartItemID] = migratedItem
	}
	return cart, nil
}

// saveItem 将单个购物车条目写回 Redis Hash。
func (s *RedisCartStore) saveItem(ctx context.Context, userID int64, item CartItem) error {
	payload, err := json.Marshal(item)
	if err != nil {
		return err
	}
	return s.client.HSet(ctx, cartRedisKey(userID), item.CartItemID, payload).Err()
}

// cartRedisKey 统一约定 Gateway 购物车在 Redis 中的 key。
func cartRedisKey(userID int64) string {
	return "cart:user:" + strconv.FormatInt(userID, 10)
}

// buildCartItemID 根据商品 ID 与规格摘要生成购物车条目 ID。
// 相同商品且规格相同会合并数量，不同规格则拆分为不同条目。
func buildCartItemID(productID int64, optionSummary string) string {
	normalized := strings.TrimSpace(optionSummary)
	if normalized == "" {
		return strconv.FormatInt(productID, 10)
	}
	return strconv.FormatInt(productID, 10) + "::" + base64.RawURLEncoding.EncodeToString([]byte(normalized))
}

// migrateCartItemIDIfNeeded 将旧格式条目 ID 迁移为 URL 安全的新格式。
// 旧格式会直接拼接规格摘要，其中的 "/" 会破坏 HTTP 路由参数解析。
func migrateCartItemIDIfNeeded(item CartItem) (CartItem, string, bool) {
	expectedID := buildCartItemID(item.ProductID, item.OptionSummary)
	currentID := strings.TrimSpace(item.CartItemID)
	if currentID == "" {
		item.CartItemID = expectedID
		return item, "", false
	}
	if currentID == expectedID {
		return item, "", false
	}

	item.CartItemID = expectedID
	return item, currentID, true
}

// buildCartResponse 将购物车条目 map 转为前端响应结构。
// 这里按条目 ID 排序输出，保证测试与前端渲染结果稳定。
func buildCartResponse(cart map[string]CartItem) CartResponse {
	resp := CartResponse{Items: make([]CartItem, 0, len(cart))}
	cartItemIDs := make([]string, 0, len(cart))
	for cartItemID := range cart {
		cartItemIDs = append(cartItemIDs, cartItemID)
	}
	sort.Strings(cartItemIDs)
	for _, cartItemID := range cartItemIDs {
		item := cart[cartItemID]
		resp.Items = append(resp.Items, item)
		resp.TotalQuantity += item.Quantity
		resp.TotalAmount += item.Price * int64(item.Quantity)
	}
	return resp
}

// CartHandler 处理购物车 HTTP 请求。
type CartHandler struct {
	store         CartStore           // 购物车存储接口，当前默认接 Redis 持久化实现
	productClient productDetailReader // 商品详情读取接口，用于将 sku_id 解析为后端权威快照
}

// productDetailReader 抽象商品详情读取能力，便于测试时替换为本地 stub。
type productDetailReader interface {
	GetProductDetail(ctx context.Context, productID int64) (*client.ProductDetailDTO, error)
}

// NewCartHandler 构造购物车 handler。
func NewCartHandler(store CartStore, productClient productDetailReader) *CartHandler {
	return &CartHandler{store: store, productClient: productClient}
}

// GetCart 处理 GET /api/v1/cart。
func (h *CartHandler) GetCart(c *gin.Context) {
	resp, err := h.store.Get(c.Request.Context(), c.GetInt64("userID"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "购物车读取失败"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// AddItem 处理 POST /api/v1/cart/items。
func (h *CartHandler) AddItem(c *gin.Context) {
	var req addCartItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.SKUID <= 0 || req.Quantity <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法购物车商品参数"})
		return
	}

	cartItem, err := h.buildCartItemFromSKUID(c.Request.Context(), req.SKUID, req.Quantity)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法购物车商品参数"})
		return
	}

	resp, err := h.store.Add(c.Request.Context(), c.GetInt64("userID"), cartItem)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加入购物车失败"})
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// UpdateItem 处理 PUT /api/v1/cart/items/:itemId。
func (h *CartHandler) UpdateItem(c *gin.Context) {
	cartItemID := strings.TrimSpace(c.Param("productId"))

	var req updateCartItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if cartItemID == "" || req.Quantity <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法购物车更新参数"})
		return
	}

	resp, err := h.store.Update(c.Request.Context(), c.GetInt64("userID"), cartItemID, req.Quantity)
	if err != nil {
		if errors.Is(err, errCartItemNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "购物车商品不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新购物车失败"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// DeleteItem 处理 DELETE /api/v1/cart/items/:itemId。
func (h *CartHandler) DeleteItem(c *gin.Context) {
	cartItemID := strings.TrimSpace(c.Param("productId"))
	if cartItemID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法商品 ID"})
		return
	}
	if err := h.store.Delete(c.Request.Context(), c.GetInt64("userID"), cartItemID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "移出购物车失败"})
		return
	}
	c.Status(http.StatusNoContent)
}

// buildCartItemFromSKUID 根据 SKU ID 查询后端权威快照并构造购物车条目。
// 当前实现通过遍历商品详情中的 SKU 找到目标项，避免前端自行传价格、标题和规格摘要。
func (h *CartHandler) buildCartItemFromSKUID(ctx context.Context, skuID int64, quantity int32) (CartItem, error) {
	if h.productClient == nil {
		return CartItem{}, errors.New("product client not configured")
	}
	productID := skuID / 1000
	if productID <= 0 {
		return CartItem{}, errors.New("invalid sku id")
	}
	detail, err := h.productClient.GetProductDetail(ctx, productID)
	if err != nil {
		return CartItem{}, err
	}
	for _, sku := range detail.SKUs {
		if sku.SKUID != skuID {
			continue
		}
		return CartItem{
			ProductID:     detail.Product.ID,
			SKUID:         sku.SKUID,
			Name:          detail.Product.Name,
			Price:         sku.Price,
			Quantity:      quantity,
			ImageURL:      detail.Product.ImageURL,
			OptionSummary: strings.TrimSpace(sku.OptionSummary),
		}, nil
	}
	return CartItem{}, errors.New("sku not found")
}
