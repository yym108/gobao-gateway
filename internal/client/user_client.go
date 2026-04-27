// Package client 封装 Gateway 对后端微服务的 gRPC 调用。
package client

import (
	"context"

	userv1 "github.com/yym/gobao-proto/gen/go/gobao/user/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// UserClient 封装对 User 服务的 gRPC 调用。
// Gateway 通过此 client 将 HTTP 请求转发为 gRPC 调用。
type UserClient struct {
	conn   *grpc.ClientConn         // gRPC 连接
	client userv1.UserServiceClient // proto 生成的 client 接口
}

// NewUserClient 创建到 User 服务的 gRPC 连接。
//   - addr: User 服务的 gRPC 地址，如 "user:9090"（Docker 网络内的服务名）
func NewUserClient(addr string) (*UserClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &UserClient{
		conn:   conn,
		client: userv1.NewUserServiceClient(conn),
	}, nil
}

// Register 调用 User 服务的注册 RPC。
//   - email:    邮箱
//   - password: 明文密码
//   - nickname: 昵称
//
// 返回新用户的 ID。
func (c *UserClient) Register(ctx context.Context, email, password, nickname string) (int64, error) {
	resp, err := c.client.Register(ctx, &userv1.RegisterRequest{
		Email: email, Password: password, Nickname: nickname,
	})
	if err != nil {
		return 0, err
	}
	return resp.GetUserId(), nil
}

// Login 调用 User 服务的登录 RPC。
// 返回 JWT token、过期时间（Unix 秒）和用户 ID。
func (c *UserClient) Login(ctx context.Context, email, password string) (string, int64, int64, error) {
	resp, err := c.client.Login(ctx, &userv1.LoginRequest{
		Email: email, Password: password,
	})
	if err != nil {
		return "", 0, 0, err
	}
	return resp.GetAccessToken(), resp.GetExpiresAt(), resp.GetUserId(), nil
}

// GetUser 调用 User 服务的获取用户信息 RPC。
//   - userID: 用户 ID
func (c *UserClient) GetUser(ctx context.Context, userID int64) (*userv1.GetUserResponse, error) {
	return c.client.GetUser(ctx, &userv1.GetUserRequest{UserId: userID})
}

// Close 关闭 gRPC 连接。应在程序退出时调用。
func (c *UserClient) Close() error {
	return c.conn.Close()
}
