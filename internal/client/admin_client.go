// Package client 封装 Gateway 对后端微服务的 gRPC 调用。
package client

import (
	"context"
	"fmt"

	adminv1 "github.com/yym108/gobao-proto/gen/go/gobao/admin/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// AdminClient 封装对 Admin 服务的 gRPC 调用。
// Gateway 通过此 client 将后台登录请求转发为内部管理员认证 RPC。
type AdminClient struct {
	conn   *grpc.ClientConn           // gRPC 连接
	client adminv1.AdminServiceClient // proto 生成的 client 接口
}

// NewAdminClient 创建到 Admin 服务的 gRPC 连接。
func NewAdminClient(addr string) (*AdminClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("dial admin: %w", err)
	}
	return &AdminClient{
		conn:   conn,
		client: adminv1.NewAdminServiceClient(conn),
	}, nil
}

// Login 调用 Admin 服务的管理员登录 RPC。
func (c *AdminClient) Login(ctx context.Context, email, password string) (string, int64, int64, error) {
	resp, err := c.client.Login(ctx, &adminv1.LoginRequest{
		Email:    email,
		Password: password,
	})
	if err != nil {
		return "", 0, 0, err
	}
	return resp.GetAccessToken(), resp.GetExpiresAt(), resp.GetAdminId(), nil
}

// GetAdmin 调用 Admin 服务的管理员资料 RPC。
func (c *AdminClient) GetAdmin(ctx context.Context, adminID int64) (*adminv1.GetAdminResponse, error) {
	return c.client.GetAdmin(ctx, &adminv1.GetAdminRequest{AdminId: adminID})
}

// ChangePassword 调用 Admin 服务的管理员自改密码 RPC。
func (c *AdminClient) ChangePassword(ctx context.Context, adminID int64, currentPassword, newPassword string) (*adminv1.ChangePasswordResponse, error) {
	return c.client.ChangePassword(ctx, &adminv1.ChangePasswordRequest{
		AdminId:         adminID,
		CurrentPassword: currentPassword,
		NewPassword:     newPassword,
	})
}

// ListAdmins 调用 Admin 服务的后台账号列表 RPC。
func (c *AdminClient) ListAdmins(ctx context.Context, requesterAdminID int64) (*adminv1.ListAdminsResponse, error) {
	return c.client.ListAdmins(ctx, &adminv1.ListAdminsRequest{RequesterAdminId: requesterAdminID})
}

// CreateAdmin 调用 Admin 服务的后台账号创建 RPC。
func (c *AdminClient) CreateAdmin(ctx context.Context, req *adminv1.CreateAdminRequest) (*adminv1.CreateAdminResponse, error) {
	return c.client.CreateAdmin(ctx, req)
}

// UpdateAdminPassword 调用 Admin 服务的后台账号重置密码 RPC。
func (c *AdminClient) UpdateAdminPassword(ctx context.Context, requesterAdminID, targetAdminID int64, newPassword string) (*adminv1.UpdateAdminPasswordResponse, error) {
	return c.client.UpdateAdminPassword(ctx, &adminv1.UpdateAdminPasswordRequest{
		RequesterAdminId: requesterAdminID,
		TargetAdminId:    targetAdminID,
		NewPassword:      newPassword,
	})
}

// Close 关闭 gRPC 连接。应在程序退出时调用。
func (c *AdminClient) Close() error {
	return c.conn.Close()
}
