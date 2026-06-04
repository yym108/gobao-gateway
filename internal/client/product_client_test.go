package client

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	productv1 "github.com/yym108/gobao-proto/gen/go/gobao/product/v1"
	"google.golang.org/grpc"
)

// fakeProductService 用函数桩实现 Product gRPC 能力，便于验证 Gateway ProductClient 的映射结果。
type fakeProductService struct {
	productv1.UnimplementedProductServiceServer
	getProductFn func(ctx context.Context, req *productv1.GetProductRequest) (*productv1.GetProductResponse, error)
}

// GetProduct 返回测试桩定义的商品详情响应。
func (s *fakeProductService) GetProduct(ctx context.Context, req *productv1.GetProductRequest) (*productv1.GetProductResponse, error) {
	return s.getProductFn(ctx, req)
}

// TestProductClient_GetProductWithVariants 验证 Gateway 能获取商品组详情并映射到本地 DTO。
func TestProductClient_GetProductWithVariants(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	productv1.RegisterProductServiceServer(grpcServer, &fakeProductService{
		getProductFn: func(_ context.Context, req *productv1.GetProductRequest) (*productv1.GetProductResponse, error) {
			return &productv1.GetProductResponse{
				Product: &productv1.Product{
					Id:             req.GetId(),
					GroupId:        10,
					Name:           "MacBook Pro 14",
					Price:          1299900,
					CategoryId:     1,
					ImageUrl:       "https://example.com/macbook-pro-14.png",
					Status:         1,
					StockQuantity:  3,
					SpecLabel:      "18GB / 512GB",
					SpecValuesJson: `{"memory":"18GB","storage":"512GB"}`,
					SortOrder:      1,
				},
				Group: &productv1.ProductGroup{
					Id:            10,
					Name:          "MacBook Pro 14",
					Slug:          "macbook-pro-14",
					HeroTitle:     "更强，更进一步",
					HeroSubtitle:  "专业创作利器",
					HeroImageUrl:  "https://example.com/macbook-pro-hero.png",
					CoverImageUrl: "https://example.com/macbook-pro-cover.png",
					CategoryId:    1,
					Status:        1,
					SortOrder:     1,
				},
				Variants: []*productv1.ProductVariant{
					{
						Id:             2001,
						SpecLabel:      "18GB / 512GB",
						SpecValuesJson: `{"memory":"18GB","storage":"512GB"}`,
						ImageUrl:       "https://example.com/macbook-pro-512.png",
						Price:          1299900,
						StockQuantity:  3,
						Status:         1,
					},
					{
						Id:             2002,
						SpecLabel:      "18GB / 1TB",
						SpecValuesJson: `{"memory":"18GB","storage":"1TB"}`,
						ImageUrl:       "https://example.com/macbook-pro-1tb.png",
						Price:          1499900,
						StockQuantity:  6,
						Status:         1,
					},
				},
				DefaultProductId: 2002,
				GroupMedias: []*productv1.ProductMedia{
					{Id: 9002, ImageUrl: "https://example.com/group-gallery-1.png", AltText: "group-1", SortOrder: 1, BindingId: 8002, UsageType: "gallery"},
					{Id: 9003, ImageUrl: "https://example.com/group-gallery-2.png", AltText: "group-2", SortOrder: 2, BindingId: 8003, UsageType: "hero"},
				},
				ProductMedias: []*productv1.ProductMedia{
					{Id: 9001, ImageUrl: "https://example.com/product-primary.png", AltText: "product-primary", SortOrder: 1, IsPrimary: true, BindingId: 8101, UsageType: "gallery"},
				},
				ResolvedMedias: []*productv1.ProductMedia{
					{Id: 9001, ImageUrl: "https://example.com/product-primary.png", AltText: "product-primary", SortOrder: 1, IsPrimary: true, BindingId: 8101, UsageType: "gallery"},
					{Id: 9002, ImageUrl: "https://example.com/group-gallery-1.png", AltText: "group-1", SortOrder: 2, BindingId: 8002, UsageType: "gallery"},
				},
			}, nil
		},
	})
	go func() { _ = grpcServer.Serve(lis) }()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = lis.Close()
	})

	pc, err := NewProductClient(lis.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = pc.Close() })

	resp, err := pc.GetProductDetail(context.Background(), 9)
	require.NoError(t, err)
	assert.Equal(t, int64(9), resp.Product.ID)
	assert.Equal(t, "MacBook Pro 14", resp.Product.Name)
	assert.Equal(t, int64(10), resp.Group.ID)
	assert.Equal(t, "MacBook Pro 14", resp.Group.Name)
	assert.Equal(t, "https://example.com/macbook-pro-cover.png", resp.Group.CoverImageURL)
	assert.Len(t, resp.Variants, 2)
	assert.Equal(t, int64(2002), resp.Variants[1].ID)
	assert.Equal(t, "18GB / 1TB", resp.Variants[1].SpecLabel)
	assert.Equal(t, int64(2002), resp.DefaultProductID)
	assert.Len(t, resp.GroupMedias, 2)
	assert.Len(t, resp.ProductMedias, 1)
	assert.Len(t, resp.ResolvedMedias, 2)
	assert.Equal(t, int64(9001), resp.ResolvedMedias[0].ID)
	assert.Equal(t, "hero", resp.GroupMedias[1].UsageType)
	assert.Equal(t, int64(8101), resp.ProductMedias[0].BindingID)
}
