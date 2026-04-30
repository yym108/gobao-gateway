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
