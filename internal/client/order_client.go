// Package client 封装 Gateway 对后端微服务的 gRPC 调用。
package client

import (
	"context"
	"fmt"

	orderv1 "github.com/yym108/gobao-proto/gen/go/gobao/order/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// OrderClient 封装对 Order 服务的 gRPC 调用。
// Gateway 通过此 client 将前端下单请求转发为内部订单 RPC。
type OrderClient struct {
	conn   *grpc.ClientConn           // gRPC 连接
	client orderv1.OrderServiceClient // proto 生成的 client 接口
}

// NewOrderClient 创建到 Order 服务的 gRPC 连接。
func NewOrderClient(addr string) (*OrderClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial order: %w", err)
	}
	return &OrderClient{
		conn:   conn,
		client: orderv1.NewOrderServiceClient(conn),
	}, nil
}

// Close 关闭 gRPC 连接。
func (c *OrderClient) Close() error {
	return c.conn.Close()
}

// CreateOrder 调用 Order 服务的创建订单 RPC。
func (c *OrderClient) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
	return c.client.CreateOrder(ctx, req)
}

// GetOrder 调用 Order 服务的单笔查单 RPC。
func (c *OrderClient) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
	return c.client.GetOrder(ctx, req)
}

// ListOrders 调用 Order 服务的订单列表 RPC。
func (c *OrderClient) ListOrders(ctx context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	return c.client.ListOrders(ctx, req)
}

// CancelOrder 调用 Order 服务的取消订单 RPC。
func (c *OrderClient) CancelOrder(ctx context.Context, req *orderv1.CancelOrderRequest) (*orderv1.CancelOrderResponse, error) {
	return c.client.CancelOrder(ctx, req)
}

// AdminListOrders 调用 Order 服务的管理员全量订单查询 RPC。
func (c *OrderClient) AdminListOrders(ctx context.Context, req *orderv1.AdminListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	return c.client.AdminListOrders(ctx, req)
}

// AdminGetOrder 调用 Order 服务的管理员任意订单查询 RPC。
func (c *OrderClient) AdminGetOrder(ctx context.Context, req *orderv1.AdminGetOrderRequest) (*orderv1.GetOrderResponse, error) {
	return c.client.AdminGetOrder(ctx, req)
}

// AdminCancelOrder 调用 Order 服务的管理员关闭订单 RPC。
func (c *OrderClient) AdminCancelOrder(ctx context.Context, req *orderv1.AdminCancelOrderRequest) (*orderv1.CancelOrderResponse, error) {
	return c.client.AdminCancelOrder(ctx, req)
}
