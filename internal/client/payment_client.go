// Package client 封装 Gateway 对后端微服务的 gRPC 调用。
package client

import (
	"context"
	"fmt"

	paymentv1 "github.com/yym108/gobao-proto/gen/go/gobao/payment/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// PaymentClient 封装对 Payment 服务的 gRPC 调用。
type PaymentClient struct {
	conn   *grpc.ClientConn               // gRPC 连接
	client paymentv1.PaymentServiceClient // proto 生成的 client 接口
}

// NewPaymentClient 创建到 Payment 服务的 gRPC 连接。
func NewPaymentClient(addr string) (*PaymentClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial payment: %w", err)
	}
	return &PaymentClient{
		conn:   conn,
		client: paymentv1.NewPaymentServiceClient(conn),
	}, nil
}

// GetPayment 调用 Payment 服务的单笔支付单查询 RPC。
func (c *PaymentClient) GetPayment(ctx context.Context, req *paymentv1.GetPaymentRequest) (*paymentv1.GetPaymentResponse, error) {
	return c.client.GetPayment(ctx, req)
}

// GetPaymentByOrder 调用 Payment 服务的按订单查询支付单 RPC。
func (c *PaymentClient) GetPaymentByOrder(ctx context.Context, req *paymentv1.GetPaymentByOrderRequest) (*paymentv1.GetPaymentByOrderResponse, error) {
	return c.client.GetPaymentByOrder(ctx, req)
}

// MockConfirmPayment 调用 Payment 服务的模拟确认支付 RPC。
func (c *PaymentClient) MockConfirmPayment(ctx context.Context, req *paymentv1.MockConfirmPaymentRequest) (*paymentv1.MockConfirmPaymentResponse, error) {
	return c.client.MockConfirmPayment(ctx, req)
}

// Close 关闭 gRPC 连接。
func (c *PaymentClient) Close() error {
	return c.conn.Close()
}
