package client

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	orderv1 "github.com/yym108/gobao-proto/gen/go/gobao/order/v1"
	"google.golang.org/grpc"
)

// fakeOrderService 用函数桩模拟 Order gRPC 服务，验证 Gateway OrderClient 的映射结果。
type fakeOrderService struct {
	orderv1.UnimplementedOrderServiceServer
	createOrderFn func(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error)
	getOrderFn    func(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error)
	listOrdersFn  func(ctx context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error)
	cancelOrderFn func(ctx context.Context, req *orderv1.CancelOrderRequest) (*orderv1.CancelOrderResponse, error)
}

// CreateOrder 返回测试桩定义的订单响应。
func (s *fakeOrderService) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
	return s.createOrderFn(ctx, req)
}

// GetOrder 返回测试桩定义的查单响应。
func (s *fakeOrderService) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
	return s.getOrderFn(ctx, req)
}

// ListOrders 返回测试桩定义的订单列表响应。
func (s *fakeOrderService) ListOrders(ctx context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	return s.listOrdersFn(ctx, req)
}

// CancelOrder 返回测试桩定义的取消订单响应。
func (s *fakeOrderService) CancelOrder(ctx context.Context, req *orderv1.CancelOrderRequest) (*orderv1.CancelOrderResponse, error) {
	return s.cancelOrderFn(ctx, req)
}

// TestOrderClient_CreateOrder 验证 Gateway 可以调用 Order 服务创建订单。
func TestOrderClient_CreateOrder(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	orderv1.RegisterOrderServiceServer(grpcServer, &fakeOrderService{
		createOrderFn: func(_ context.Context, req *orderv1.CreateOrderRequest) (*orderv1.CreateOrderResponse, error) {
			assert.Equal(t, int64(1001), req.GetUserId())
			assert.Equal(t, int64(1001002), req.GetProductId())
			return &orderv1.CreateOrderResponse{
				Order: &orderv1.Order{
					Id:          101,
					OrderNo:     "ORD-20260518141000-1001",
					UserId:      1001,
					RequestId:   "req-001",
					Status:      "CREATED",
					TotalAmount: 1999800,
				},
			}, nil
		},
		getOrderFn: func(_ context.Context, req *orderv1.GetOrderRequest) (*orderv1.GetOrderResponse, error) {
			assert.Equal(t, int64(1001), req.GetUserId())
			assert.Equal(t, int64(101), req.GetOrderId())
			return &orderv1.GetOrderResponse{
				Order: &orderv1.Order{
					Id:      101,
					OrderNo: "ORD-20260518141000-1001",
					UserId:  1001,
					Status:  "CREATED",
				},
			}, nil
		},
		listOrdersFn: func(_ context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
			assert.Equal(t, int64(1001), req.GetUserId())
			assert.Equal(t, int32(1), req.GetPage())
			assert.Equal(t, int32(2), req.GetPageSize())
			return &orderv1.ListOrdersResponse{
				Items: []*orderv1.Order{
					{Id: 103, OrderNo: "ORD-003", UserId: 1001},
					{Id: 102, OrderNo: "ORD-002", UserId: 1001},
				},
				Total: 3,
			}, nil
		},
		cancelOrderFn: func(_ context.Context, req *orderv1.CancelOrderRequest) (*orderv1.CancelOrderResponse, error) {
			assert.Equal(t, int64(1001), req.GetUserId())
			assert.Equal(t, int64(201), req.GetOrderId())
			return &orderv1.CancelOrderResponse{
				Order: &orderv1.Order{
					Id:      201,
					OrderNo: "ORD-CANCEL-201",
					UserId:  1001,
					Status:  "CANCELLED",
				},
			}, nil
		},
	})
	go func() { _ = grpcServer.Serve(lis) }()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = lis.Close()
	})

	client, err := NewOrderClient(lis.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	resp, err := client.CreateOrder(context.Background(), &orderv1.CreateOrderRequest{
		UserId:       1001,
		RequestId:    "req-001",
		ProductId:    1001002,
		Quantity:     2,
		ReceiverName: "张三",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(101), resp.GetOrder().GetId())
	assert.Equal(t, "ORD-20260518141000-1001", resp.GetOrder().GetOrderNo())

	orderResp, err := client.GetOrder(context.Background(), &orderv1.GetOrderRequest{
		UserId:  1001,
		OrderId: 101,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(101), orderResp.GetOrder().GetId())
	assert.Equal(t, "ORD-20260518141000-1001", orderResp.GetOrder().GetOrderNo())

	listResp, err := client.ListOrders(context.Background(), &orderv1.ListOrdersRequest{
		UserId:   1001,
		Page:     1,
		PageSize: 2,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(3), listResp.GetTotal())
	assert.Len(t, listResp.GetItems(), 2)

	cancelResp, err := client.CancelOrder(context.Background(), &orderv1.CancelOrderRequest{
		UserId:  1001,
		OrderId: 201,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(201), cancelResp.GetOrder().GetId())
	assert.Equal(t, "CANCELLED", cancelResp.GetOrder().GetStatus())
}
