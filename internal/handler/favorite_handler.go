// Package handler 提供 Gateway 的 HTTP 请求处理器。
// 本文件实现最小收藏夹 HTTP 接口，并通过可替换的存储接口承接收藏数据。
package handler

import (
	"context"
	"database/sql"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yym108/gobao-gateway/internal/client"
)

// FavoriteItem 表示用户收藏夹中的单个商品展示条目。
// 持久层只保存收藏关系与时间，名称、价格、图片等字段在查询时由商品服务实时补齐。
type FavoriteItem struct {
	ProductID         int64  `json:"product_id"`         // 商品 ID
	Name              string `json:"name"`               // 商品名称
	Description       string `json:"description"`        // 商品描述
	Price             int64  `json:"price"`              // 当前商品展示价格，单位分
	CategoryID        int64  `json:"category_id"`        // 商品所属类目
	ImageURL          string `json:"image_url"`          // 商品图片
	Status            int32  `json:"status"`             // 商品状态
	FavoritedAt       int64  `json:"favorited_at"`       // 收藏时间，Unix 秒
	Available         bool   `json:"available"`          // 是否仍可从商品服务正常读取
	UnavailableReason string `json:"unavailable_reason"` // 商品失效时给前端的明确提示
}

// FavoriteListResponse 是收藏查询与变更后的统一响应。
type FavoriteListResponse struct {
	Items []FavoriteItem `json:"items"` // 当前收藏商品列表
	Total int64          `json:"total"` // 收藏总数
}

// addFavoriteRequest 描述加入收藏的请求体。
type addFavoriteRequest struct {
	ProductID int64 `json:"product_id"` // 商品 ID，只接受后端主键
}

// FavoriteStore 抽象收藏数据存储。
// 存储层只负责 user-product 关系及收藏时间，不承接商品快照字段。
type FavoriteStore interface {
	List(ctx context.Context, userID int64) (FavoriteListResponse, error)
	Add(ctx context.Context, userID int64, item FavoriteItem) (FavoriteListResponse, error)
	Delete(ctx context.Context, userID int64, productID int64) error
}

// MemoryFavoriteStore 是最小收藏内存存储，适合测试与本地降级运行。
type MemoryFavoriteStore struct {
	mu        sync.RWMutex
	favorites map[int64]map[int64]FavoriteItem
}

// NewMemoryFavoriteStore 构造内存级收藏存储。
func NewMemoryFavoriteStore() *MemoryFavoriteStore {
	return &MemoryFavoriteStore{favorites: make(map[int64]map[int64]FavoriteItem)}
}

// List 返回指定用户的收藏快照。
func (s *MemoryFavoriteStore) List(_ context.Context, userID int64) (FavoriteListResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return buildFavoriteResponse(s.favorites[userID]), nil
}

// Add 向指定用户收藏夹新增或刷新收藏关系。
func (s *MemoryFavoriteStore) Add(_ context.Context, userID int64, item FavoriteItem) (FavoriteListResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	favorites := s.ensureFavorites(userID)
	if current, ok := favorites[item.ProductID]; ok {
		item.FavoritedAt = current.FavoritedAt
	}
	favorites[item.ProductID] = item
	return buildFavoriteResponse(favorites), nil
}

// Delete 删除指定收藏商品。
func (s *MemoryFavoriteStore) Delete(_ context.Context, userID int64, productID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if favorites, ok := s.favorites[userID]; ok {
		delete(favorites, productID)
	}
	return nil
}

// ensureFavorites 确保用户收藏 map 已初始化。
func (s *MemoryFavoriteStore) ensureFavorites(userID int64) map[int64]FavoriteItem {
	favorites, ok := s.favorites[userID]
	if !ok {
		favorites = make(map[int64]FavoriteItem)
		s.favorites[userID] = favorites
	}
	return favorites
}

// favoriteTableDDL 是收藏表的初始化语句。
// 当前以 (user_id, product_id) 唯一键约束单用户不可重复收藏同一商品。
const favoriteTableDDL = `
CREATE TABLE IF NOT EXISTS favorites (
  user_id BIGINT NOT NULL,
  product_id BIGINT NOT NULL,
  created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (user_id, product_id),
  KEY idx_favorites_user_created_at (user_id, created_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
`

// MySQLFavoriteStore 使用 MySQL 持久化单用户收藏。
// 当前由 Gateway 直接维护 favorites 表，后续如拆分独立服务可平滑迁移。
type MySQLFavoriteStore struct {
	db *sql.DB
}

// NewMySQLFavoriteStore 构造 MySQL 收藏存储，并在启动时确保表结构存在。
func NewMySQLFavoriteStore(db *sql.DB) (*MySQLFavoriteStore, error) {
	store := &MySQLFavoriteStore{db: db}
	if err := store.ensureSchema(context.Background()); err != nil {
		return nil, err
	}
	return store, nil
}

// ensureSchema 在启动时创建 favorites 表，避免新环境下因遗漏初始化脚本而不可用。
func (s *MySQLFavoriteStore) ensureSchema(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, favoriteTableDDL)
	return err
}

// List 读取指定用户的收藏关系。
func (s *MySQLFavoriteStore) List(ctx context.Context, userID int64) (FavoriteListResponse, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT product_id, created_at
		FROM favorites
		WHERE user_id = ?
		ORDER BY created_at DESC, product_id DESC`,
		userID,
	)
	if err != nil {
		return FavoriteListResponse{}, err
	}
	defer func() {
		_ = rows.Close()
	}()

	items := make([]FavoriteItem, 0)
	for rows.Next() {
		var item FavoriteItem
		var createdAt time.Time
		if err := rows.Scan(
			&item.ProductID,
			&createdAt,
		); err != nil {
			return FavoriteListResponse{}, err
		}
		item.FavoritedAt = createdAt.Unix()
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return FavoriteListResponse{}, err
	}
	return FavoriteListResponse{Items: items, Total: int64(len(items))}, nil
}

// Add 新增或刷新收藏关系。
// 若用户已收藏同一商品，则保留原收藏时间，不再覆盖 created_at。
func (s *MySQLFavoriteStore) Add(ctx context.Context, userID int64, item FavoriteItem) (FavoriteListResponse, error) {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO favorites (user_id, product_id, created_at)
		VALUES (?, ?, FROM_UNIXTIME(?))
		ON DUPLICATE KEY UPDATE product_id = VALUES(product_id)`,
		userID,
		item.ProductID,
		item.FavoritedAt,
	)
	if err != nil {
		return FavoriteListResponse{}, err
	}
	return s.List(ctx, userID)
}

// Delete 删除 MySQL 中的指定收藏商品。
func (s *MySQLFavoriteStore) Delete(ctx context.Context, userID int64, productID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM favorites WHERE user_id = ? AND product_id = ?`, userID, productID)
	return err
}

// buildFavoriteResponse 将收藏 map 转为前端响应结构。
// 这里按收藏时间倒序输出，保证用户看到最近收藏的商品排在前面。
func buildFavoriteResponse(favorites map[int64]FavoriteItem) FavoriteListResponse {
	resp := FavoriteListResponse{Items: make([]FavoriteItem, 0, len(favorites))}
	for _, item := range favorites {
		resp.Items = append(resp.Items, item)
	}
	sort.Slice(resp.Items, func(i, j int) bool {
		if resp.Items[i].FavoritedAt == resp.Items[j].FavoritedAt {
			return resp.Items[i].ProductID > resp.Items[j].ProductID
		}
		return resp.Items[i].FavoritedAt > resp.Items[j].FavoritedAt
	})
	resp.Total = int64(len(resp.Items))
	return resp
}

// FavoriteHandler 处理收藏 HTTP 请求。
type FavoriteHandler struct {
	store         FavoriteStore       // 收藏存储接口，当前默认接 Redis 持久化实现
	productClient favoriteProductInfo // 商品详情读取接口，用于补收藏商品快照
	now           func() time.Time    // 当前时间提供器，便于测试替换
}

// favoriteProductInfo 抽象商品详情读取能力，便于测试时替换为本地 stub。
type favoriteProductInfo interface {
	GetProductDetail(ctx context.Context, productID int64) (*client.ProductDetailDTO, error)
}

// NewFavoriteHandler 构造收藏 handler。
func NewFavoriteHandler(store FavoriteStore, productClient favoriteProductInfo) *FavoriteHandler {
	return &FavoriteHandler{
		store:         store,
		productClient: productClient,
		now:           time.Now,
	}
}

// ListFavorites 处理 GET /api/v1/favorites。
func (h *FavoriteHandler) ListFavorites(c *gin.Context) {
	resp, err := h.store.List(c.Request.Context(), c.GetInt64("userID"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "收藏列表读取失败"})
		return
	}
	c.JSON(http.StatusOK, h.enrichFavoriteItems(c.Request.Context(), resp))
}

// AddFavorite 处理 POST /api/v1/favorites。
// 当前只接受 product_id，由后端 Product 服务回填商品名称、价格与图片快照。
func (h *FavoriteHandler) AddFavorite(c *gin.Context) {
	var req addFavoriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体格式错误"})
		return
	}
	if req.ProductID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法商品 ID"})
		return
	}

	item, err := h.buildFavoriteItem(c.Request.Context(), req.ProductID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法商品 ID"})
		return
	}

	resp, err := h.store.Add(c.Request.Context(), c.GetInt64("userID"), item)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "加入收藏失败"})
		return
	}
	c.JSON(http.StatusCreated, h.enrichFavoriteItems(c.Request.Context(), resp))
}

// DeleteFavorite 处理 DELETE /api/v1/favorites/:productId。
func (h *FavoriteHandler) DeleteFavorite(c *gin.Context) {
	productID, err := strconv.ParseInt(strings.TrimSpace(c.Param("productId")), 10, 64)
	if err != nil || productID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法商品 ID"})
		return
	}
	if err := h.store.Delete(c.Request.Context(), c.GetInt64("userID"), productID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "取消收藏失败"})
		return
	}
	c.Status(http.StatusNoContent)
}

// buildFavoriteItem 根据商品 ID 构造收藏关系条目。
// 收藏写入时只保存关系和时间，不再持久化商品快照字段。
func (h *FavoriteHandler) buildFavoriteItem(ctx context.Context, productID int64) (FavoriteItem, error) {
	if h.productClient == nil {
		return FavoriteItem{}, context.Canceled
	}
	detail, err := h.productClient.GetProductDetail(ctx, productID)
	if err != nil {
		return FavoriteItem{}, err
	}
	return FavoriteItem{
		ProductID:   detail.Product.ID,
		FavoritedAt: h.now().Unix(),
	}, nil
}

// resolveFavoriteImageURL 解析收藏项应展示的图片地址。
// 与用户端商品卡片一致地做回退：优先商品自身主图，其次商品组封面，最后取详情图库首图；
// 这样通过商品组媒体绑定上图、自身 image_url 为空的新商品也能在收藏页正常显示图片。
func resolveFavoriteImageURL(detail *client.ProductDetailDTO) string {
	if detail == nil {
		return ""
	}
	if detail.Product.ImageURL != "" {
		return detail.Product.ImageURL
	}
	if detail.Group.CoverImageURL != "" {
		return detail.Group.CoverImageURL
	}
	for _, media := range detail.ResolvedMedias {
		if media.ImageURL != "" {
			return media.ImageURL
		}
	}
	return ""
}

// enrichFavoriteItems 将收藏关系实时补齐为前端可直接展示的结构。
// 若商品已删除或已下线，则保留收藏记录并返回显式的失效提示，便于用户主动清理。
func (h *FavoriteHandler) enrichFavoriteItems(ctx context.Context, resp FavoriteListResponse) FavoriteListResponse {
	if h.productClient == nil || len(resp.Items) == 0 {
		return resp
	}

	items := make([]FavoriteItem, 0, len(resp.Items))
	for _, item := range resp.Items {
		detail, err := h.productClient.GetProductDetail(ctx, item.ProductID)
		if err != nil {
			item.Name = "商品已下线"
			item.Description = ""
			item.Price = 0
			item.CategoryID = 0
			item.ImageURL = ""
			item.Status = 0
			item.Available = false
			item.UnavailableReason = "该商品已不存在或已下线"
			items = append(items, item)
			continue
		}

		item.Name = detail.Product.Name
		item.Description = detail.Product.Description
		item.Price = detail.Product.Price
		item.CategoryID = detail.Product.CategoryID
		item.ImageURL = resolveFavoriteImageURL(detail)
		item.Status = detail.Product.Status
		item.Available = true
		item.UnavailableReason = ""
		items = append(items, item)
	}
	resp.Items = items
	resp.Total = int64(len(items))
	return resp
}
