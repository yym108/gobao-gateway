// Package client 封装 Gateway 对后端微服务的 gRPC 调用。
package client

import (
	"context"
	"fmt"

	productv1 "github.com/yym108/gobao-proto/gen/go/gobao/product/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ProductClient 封装对 Product 服务的 gRPC 调用。
// Gateway 通过此 client 将 HTTP 请求转发为商品/类目相关的 gRPC 调用。
type ProductClient struct {
	conn   *grpc.ClientConn               // gRPC 连接
	client productv1.ProductServiceClient // proto 生成的 client 接口
}

// ProductDetailDTO 是 Gateway 内部使用的商品详情传输对象。
// 该结构将“当前商品 + 商品组 + 同组版本列表”从 proto 解耦出来，便于后续 handler 和购物车逻辑只依赖网关本地模型。
type ProductDetailDTO struct {
	Product          ProductDTO       // 当前商品基础信息
	Group            ProductGroupDTO  // 商品组信息
	Variants         []ProductVariant // 同组版本列表
	DefaultProductID int64            // 默认商品版本 ID
	GroupMedias      []ProductMedia   // 商品组公共图片
	ProductMedias    []ProductMedia   // 当前商品专属图片
	ResolvedMedias   []ProductMedia   // 后端拼装后的最终详情图库
}

// ProductDTO 是商品基础信息的网关本地表示。
type ProductDTO struct {
	ID             int64  // 商品 ID
	GroupID        int64  // 所属商品组 ID
	Name           string // 商品名称
	Description    string // 商品描述
	Price          int64  // 当前价格，单位分
	CategoryID     int64  // 类目 ID
	ImageURL       string // 商品主图
	Status         int32  // 商品状态
	StockQuantity  int32  // 商品库存
	SpecLabel      string // 规格展示文案
	SpecValuesJSON string // 结构化规格 JSON
	SortOrder      int32  // 同组内排序权重
}

// ProductGroupDTO 表示一个商品详情页对应的商品组。
type ProductGroupDTO struct {
	ID            int64    // 商品组 ID
	Name          string   // 商品组名称
	Slug          string   // 路由标识
	HeroTitle     string   // 头图标题
	HeroSubtitle  string   // 头图副标题
	HeroImageURL  string   // 头图图片
	CoverImageURL string   // 列表封面图
	CategoryID    int64    // 所属类目
	Status        int32    // 商品组状态
	SortOrder     int32    // 排序权重
	SpecKeys      []string // 规格维度名称，子商品只能填这些维度的值
}

// ProductVariant 表示同组下的一个独立商品版本。
type ProductVariant struct {
	ID             int64  // 独立商品 ID
	SpecLabel      string // 规格文案
	SpecValuesJSON string // 结构化规格 JSON
	ImageURL       string // 版本图片
	Price          int64  // 当前售价
	StockQuantity  int32  // 当前库存
	Status         int32  // 当前状态
}

// ProductMedia 表示商品详情中可直接展示的一张图片。
type ProductMedia struct {
	ID        int64  // 图片资源 ID
	ImageURL  string // 图片地址
	AltText   string // 替代文本
	SortOrder int32  // 排序权重
	IsPrimary bool   // 是否主图
	BindingID int64  // 绑定记录 ID
	UsageType string // 图片用途：cover/hero/gallery
}

// NewProductClient 创建到 Product 服务的 gRPC 连接。
//   - addr: Product 服务的 gRPC 地址，如 "product:9090"（Docker 网络内的服务名）
func NewProductClient(addr string) (*ProductClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial product: %w", err)
	}
	return &ProductClient{
		conn:   conn,
		client: productv1.NewProductServiceClient(conn),
	}, nil
}

// Close 关闭 gRPC 连接。应在程序退出时调用。
func (c *ProductClient) Close() error {
	return c.conn.Close()
}

// CreateProduct 调用 Product 服务的创建商品 RPC。
func (c *ProductClient) CreateProduct(ctx context.Context, req *productv1.CreateProductRequest) (*productv1.CreateProductResponse, error) {
	return c.client.CreateProduct(ctx, req)
}

// GetProduct 调用 Product 服务的查询商品详情 RPC。
func (c *ProductClient) GetProduct(ctx context.Context, req *productv1.GetProductRequest) (*productv1.GetProductResponse, error) {
	return c.client.GetProduct(ctx, req)
}

// GetProductDetail 获取并映射商品详情，供 Gateway 内部业务逻辑使用。
func (c *ProductClient) GetProductDetail(ctx context.Context, productID int64) (*ProductDetailDTO, error) {
	resp, err := c.client.GetProduct(ctx, &productv1.GetProductRequest{Id: productID})
	if err != nil {
		return nil, err
	}
	return mapProductDetail(resp), nil
}

// ListProducts 调用 Product 服务的商品分页查询 RPC。
func (c *ProductClient) ListProducts(ctx context.Context, req *productv1.ListProductsRequest) (*productv1.ListProductsResponse, error) {
	return c.client.ListProducts(ctx, req)
}

// UpdateProduct 调用 Product 服务的更新商品 RPC。
func (c *ProductClient) UpdateProduct(ctx context.Context, req *productv1.UpdateProductRequest) (*productv1.UpdateProductResponse, error) {
	return c.client.UpdateProduct(ctx, req)
}

// DeleteProduct 调用 Product 服务的删除商品 RPC。
func (c *ProductClient) DeleteProduct(ctx context.Context, req *productv1.DeleteProductRequest) (*productv1.DeleteProductResponse, error) {
	return c.client.DeleteProduct(ctx, req)
}

// CreateProductGroup 调用 Product 服务的创建商品组 RPC。
func (c *ProductClient) CreateProductGroup(ctx context.Context, req *productv1.CreateProductGroupRequest) (*productv1.CreateProductGroupResponse, error) {
	return c.client.CreateProductGroup(ctx, req)
}

// ListProductGroups 调用 Product 服务的商品组分页查询 RPC。
func (c *ProductClient) ListProductGroups(ctx context.Context, req *productv1.ListProductGroupsRequest) (*productv1.ListProductGroupsResponse, error) {
	return c.client.ListProductGroups(ctx, req)
}

// UpdateProductGroup 调用 Product 服务的更新商品组 RPC。
func (c *ProductClient) UpdateProductGroup(ctx context.Context, req *productv1.UpdateProductGroupRequest) (*productv1.UpdateProductGroupResponse, error) {
	return c.client.UpdateProductGroup(ctx, req)
}

// DeleteProductGroup 调用 Product 服务的删除商品组 RPC。
func (c *ProductClient) DeleteProductGroup(ctx context.Context, req *productv1.DeleteProductGroupRequest) (*productv1.DeleteProductGroupResponse, error) {
	return c.client.DeleteProductGroup(ctx, req)
}

// CreateCategory 调用 Product 服务的创建类目 RPC。
func (c *ProductClient) CreateCategory(ctx context.Context, req *productv1.CreateCategoryRequest) (*productv1.CreateCategoryResponse, error) {
	return c.client.CreateCategory(ctx, req)
}

// ListCategories 调用 Product 服务的类目列表 RPC。
func (c *ProductClient) ListCategories(ctx context.Context, req *productv1.ListCategoriesRequest) (*productv1.ListCategoriesResponse, error) {
	return c.client.ListCategories(ctx, req)
}

// UpdateCategory 调用 Product 服务的更新类目 RPC。
func (c *ProductClient) UpdateCategory(ctx context.Context, req *productv1.UpdateCategoryRequest) (*productv1.UpdateCategoryResponse, error) {
	return c.client.UpdateCategory(ctx, req)
}

// DeleteCategory 调用 Product 服务的删除类目 RPC。
func (c *ProductClient) DeleteCategory(ctx context.Context, req *productv1.DeleteCategoryRequest) (*productv1.DeleteCategoryResponse, error) {
	return c.client.DeleteCategory(ctx, req)
}

// GetSeckillActivity 调用 Product 服务的查询秒杀活动 RPC。
func (c *ProductClient) GetSeckillActivity(ctx context.Context, req *productv1.GetSeckillActivityRequest) (*productv1.GetSeckillActivityResponse, error) {
	return c.client.GetSeckillActivity(ctx, req)
}

// PrewarmSeckill 调用 Product 服务的秒杀预热 RPC。
func (c *ProductClient) PrewarmSeckill(ctx context.Context, req *productv1.PrewarmSeckillRequest) (*productv1.PrewarmSeckillResponse, error) {
	return c.client.PrewarmSeckill(ctx, req)
}

// UpdateStock 调用 Product 服务的后台库存更新 RPC。
func (c *ProductClient) UpdateStock(ctx context.Context, req *productv1.UpdateStockRequest) (*productv1.UpdateStockResponse, error) {
	return c.client.UpdateStock(ctx, req)
}

// UploadMedia 调用 Product 服务的媒体上传 RPC。
func (c *ProductClient) UploadMedia(ctx context.Context, req *productv1.UploadMediaRequest) (*productv1.UploadMediaResponse, error) {
	return c.client.UploadMedia(ctx, req)
}

// BindProductGroupMedia 调用 Product 服务的商品组媒体绑定 RPC。
func (c *ProductClient) BindProductGroupMedia(ctx context.Context, req *productv1.BindProductGroupMediaRequest) (*productv1.BindProductGroupMediaResponse, error) {
	return c.client.BindProductGroupMedia(ctx, req)
}

// BindProductMedia 调用 Product 服务的独立商品媒体绑定 RPC。
func (c *ProductClient) BindProductMedia(ctx context.Context, req *productv1.BindProductMediaRequest) (*productv1.BindProductMediaResponse, error) {
	return c.client.BindProductMedia(ctx, req)
}

// UpdateProductGroupMediaBinding 调用 Product 服务的商品组媒体绑定更新 RPC。
func (c *ProductClient) UpdateProductGroupMediaBinding(ctx context.Context, req *productv1.UpdateProductGroupMediaBindingRequest) (*productv1.UpdateProductGroupMediaBindingResponse, error) {
	return c.client.UpdateProductGroupMediaBinding(ctx, req)
}

// UpdateProductMediaBinding 调用 Product 服务的独立商品媒体绑定更新 RPC。
func (c *ProductClient) UpdateProductMediaBinding(ctx context.Context, req *productv1.UpdateProductMediaBindingRequest) (*productv1.UpdateProductMediaBindingResponse, error) {
	return c.client.UpdateProductMediaBinding(ctx, req)
}

// DeleteProductGroupMediaBinding 调用 Product 服务的商品组媒体解绑 RPC。
func (c *ProductClient) DeleteProductGroupMediaBinding(ctx context.Context, req *productv1.DeleteProductGroupMediaBindingRequest) (*productv1.DeleteProductGroupMediaBindingResponse, error) {
	return c.client.DeleteProductGroupMediaBinding(ctx, req)
}

// DeleteProductMediaBinding 调用 Product 服务的独立商品媒体解绑 RPC。
func (c *ProductClient) DeleteProductMediaBinding(ctx context.Context, req *productv1.DeleteProductMediaBindingRequest) (*productv1.DeleteProductMediaBindingResponse, error) {
	return c.client.DeleteProductMediaBinding(ctx, req)
}

// mapProductDetail 将 Product 服务的 proto 响应映射为 Gateway 本地 DTO。
func mapProductDetail(resp *productv1.GetProductResponse) *ProductDetailDTO {
	dto := &ProductDetailDTO{
		Variants:         make([]ProductVariant, 0, len(resp.GetVariants())),
		DefaultProductID: resp.GetDefaultProductId(),
		GroupMedias:      make([]ProductMedia, 0, len(resp.GetGroupMedias())),
		ProductMedias:    make([]ProductMedia, 0, len(resp.GetProductMedias())),
		ResolvedMedias:   make([]ProductMedia, 0, len(resp.GetResolvedMedias())),
	}
	if product := resp.GetProduct(); product != nil {
		dto.Product = ProductDTO{
			ID:             product.GetId(),
			GroupID:        product.GetGroupId(),
			Name:           product.GetName(),
			Description:    product.GetDescription(),
			Price:          product.GetPrice(),
			CategoryID:     product.GetCategoryId(),
			ImageURL:       product.GetImageUrl(),
			Status:         product.GetStatus(),
			StockQuantity:  product.GetStockQuantity(),
			SpecLabel:      product.GetSpecLabel(),
			SpecValuesJSON: product.GetSpecValuesJson(),
			SortOrder:      product.GetSortOrder(),
		}
	}
	if group := resp.GetGroup(); group != nil {
		dto.Group = ProductGroupDTO{
			ID:            group.GetId(),
			Name:          group.GetName(),
			Slug:          group.GetSlug(),
			HeroTitle:     group.GetHeroTitle(),
			HeroSubtitle:  group.GetHeroSubtitle(),
			HeroImageURL:  group.GetHeroImageUrl(),
			CoverImageURL: group.GetCoverImageUrl(),
			CategoryID:    group.GetCategoryId(),
			Status:        group.GetStatus(),
			SortOrder:     group.GetSortOrder(),
			SpecKeys:      group.GetSpecKeys(),
		}
	}
	for _, item := range resp.GetVariants() {
		dto.Variants = append(dto.Variants, ProductVariant{
			ID:             item.GetId(),
			SpecLabel:      item.GetSpecLabel(),
			SpecValuesJSON: item.GetSpecValuesJson(),
			ImageURL:       item.GetImageUrl(),
			Price:          item.GetPrice(),
			StockQuantity:  item.GetStockQuantity(),
			Status:         item.GetStatus(),
		})
	}
	for _, item := range resp.GetGroupMedias() {
		dto.GroupMedias = append(dto.GroupMedias, ProductMedia{
			ID:        item.GetId(),
			ImageURL:  item.GetImageUrl(),
			AltText:   item.GetAltText(),
			SortOrder: item.GetSortOrder(),
			IsPrimary: item.GetIsPrimary(),
			BindingID: item.GetBindingId(),
			UsageType: item.GetUsageType(),
		})
	}
	for _, item := range resp.GetProductMedias() {
		dto.ProductMedias = append(dto.ProductMedias, ProductMedia{
			ID:        item.GetId(),
			ImageURL:  item.GetImageUrl(),
			AltText:   item.GetAltText(),
			SortOrder: item.GetSortOrder(),
			IsPrimary: item.GetIsPrimary(),
			BindingID: item.GetBindingId(),
			UsageType: item.GetUsageType(),
		})
	}
	for _, item := range resp.GetResolvedMedias() {
		dto.ResolvedMedias = append(dto.ResolvedMedias, ProductMedia{
			ID:        item.GetId(),
			ImageURL:  item.GetImageUrl(),
			AltText:   item.GetAltText(),
			SortOrder: item.GetSortOrder(),
			IsPrimary: item.GetIsPrimary(),
			BindingID: item.GetBindingId(),
			UsageType: item.GetUsageType(),
		})
	}
	return dto
}
