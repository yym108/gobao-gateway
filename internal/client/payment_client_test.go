package client

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	paymentv1 "github.com/yym108/gobao-proto/gen/go/gobao/payment/v1"
	"google.golang.org/grpc"
)

type fakePaymentService struct {
	paymentv1.UnimplementedPaymentServiceServer
	getPaymentFn        func(ctx context.Context, req *paymentv1.GetPaymentRequest) (*paymentv1.GetPaymentResponse, error)
	getPaymentByOrderFn func(ctx context.Context, req *paymentv1.GetPaymentByOrderRequest) (*paymentv1.GetPaymentByOrderResponse, error)
	mockConfirmFn       func(ctx context.Context, req *paymentv1.MockConfirmPaymentRequest) (*paymentv1.MockConfirmPaymentResponse, error)
}

func (s *fakePaymentService) GetPayment(ctx context.Context, req *paymentv1.GetPaymentRequest) (*paymentv1.GetPaymentResponse, error) {
	return s.getPaymentFn(ctx, req)
}
func (s *fakePaymentService) GetPaymentByOrder(ctx context.Context, req *paymentv1.GetPaymentByOrderRequest) (*paymentv1.GetPaymentByOrderResponse, error) {
	return s.getPaymentByOrderFn(ctx, req)
}
func (s *fakePaymentService) MockConfirmPayment(ctx context.Context, req *paymentv1.MockConfirmPaymentRequest) (*paymentv1.MockConfirmPaymentResponse, error) {
	return s.mockConfirmFn(ctx, req)
}

func TestPaymentClient_BasicCalls(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	paymentv1.RegisterPaymentServiceServer(grpcServer, &fakePaymentService{
		getPaymentFn: func(_ context.Context, req *paymentv1.GetPaymentRequest) (*paymentv1.GetPaymentResponse, error) {
			assert.Equal(t, int64(1001), req.GetUserId())
			assert.Equal(t, int64(301), req.GetPaymentId())
			return &paymentv1.GetPaymentResponse{Payment: &paymentv1.Payment{Id: 301, OrderId: 101}}, nil
		},
		getPaymentByOrderFn: func(_ context.Context, req *paymentv1.GetPaymentByOrderRequest) (*paymentv1.GetPaymentByOrderResponse, error) {
			assert.Equal(t, int64(1001), req.GetUserId())
			assert.Equal(t, int64(101), req.GetOrderId())
			return &paymentv1.GetPaymentByOrderResponse{Payment: &paymentv1.Payment{Id: 301, OrderId: 101}}, nil
		},
		mockConfirmFn: func(_ context.Context, req *paymentv1.MockConfirmPaymentRequest) (*paymentv1.MockConfirmPaymentResponse, error) {
			assert.Equal(t, int64(1001), req.GetUserId())
			assert.Equal(t, int64(301), req.GetPaymentId())
			assert.Equal(t, "SUCCESS", req.GetResult())
			return &paymentv1.MockConfirmPaymentResponse{Payment: &paymentv1.Payment{Id: 301, Status: "SUCCEEDED"}}, nil
		},
	})
	go func() { _ = grpcServer.Serve(lis) }()
	t.Cleanup(func() {
		grpcServer.Stop()
		_ = lis.Close()
	})

	client, err := NewPaymentClient(lis.Addr().String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	getResp, err := client.GetPayment(context.Background(), &paymentv1.GetPaymentRequest{UserId: 1001, PaymentId: 301})
	require.NoError(t, err)
	assert.Equal(t, int64(301), getResp.GetPayment().GetId())

	orderResp, err := client.GetPaymentByOrder(context.Background(), &paymentv1.GetPaymentByOrderRequest{UserId: 1001, OrderId: 101})
	require.NoError(t, err)
	assert.Equal(t, int64(301), orderResp.GetPayment().GetId())

	mockResp, err := client.MockConfirmPayment(context.Background(), &paymentv1.MockConfirmPaymentRequest{UserId: 1001, PaymentId: 301, Result: "SUCCESS"})
	require.NoError(t, err)
	assert.Equal(t, "SUCCEEDED", mockResp.GetPayment().GetStatus())
}
