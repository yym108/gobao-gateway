// Package client 封装 Gateway 对后端微服务的 gRPC 调用。
package client

import (
	"context"

	userv1 "github.com/yym108/gobao-proto/gen/go/gobao/user/v1"
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

// FindUserByEmail 调用 User 服务的按邮箱查找用户 RPC。
func (c *UserClient) FindUserByEmail(ctx context.Context, email string) (*userv1.FindUserByEmailResponse, error) {
	return c.client.FindUserByEmail(ctx, &userv1.FindUserByEmailRequest{Email: email})
}

// GetProfile 调用 User 服务的获取当前用户资料 RPC。
func (c *UserClient) GetProfile(ctx context.Context, userID int64) (*userv1.GetProfileResponse, error) {
	return c.client.GetProfile(ctx, &userv1.GetProfileRequest{UserId: userID})
}

// ListAddresses 调用 User 服务的地址簿列表 RPC。
func (c *UserClient) ListAddresses(ctx context.Context, userID int64) (*userv1.ListAddressesResponse, error) {
	return c.client.ListAddresses(ctx, &userv1.ListAddressesRequest{UserId: userID})
}

// GetAddress 调用 User 服务的地址详情 RPC。
func (c *UserClient) GetAddress(ctx context.Context, userID, addressID int64) (*userv1.GetAddressResponse, error) {
	return c.client.GetAddress(ctx, &userv1.GetAddressRequest{
		UserId:    userID,
		AddressId: addressID,
	})
}

// CreateAddress 调用 User 服务的地址创建 RPC。
func (c *UserClient) CreateAddress(ctx context.Context, req *userv1.CreateAddressRequest) (*userv1.CreateAddressResponse, error) {
	return c.client.CreateAddress(ctx, req)
}

// UpdateAddress 调用 User 服务的地址更新 RPC。
func (c *UserClient) UpdateAddress(ctx context.Context, req *userv1.UpdateAddressRequest) (*userv1.UpdateAddressResponse, error) {
	return c.client.UpdateAddress(ctx, req)
}

// DeleteAddress 调用 User 服务的地址删除 RPC。
func (c *UserClient) DeleteAddress(ctx context.Context, userID, addressID int64) (*userv1.DeleteAddressResponse, error) {
	return c.client.DeleteAddress(ctx, &userv1.DeleteAddressRequest{
		UserId:    userID,
		AddressId: addressID,
	})
}

// SetDefaultAddress 调用 User 服务的默认地址切换 RPC。
func (c *UserClient) SetDefaultAddress(ctx context.Context, userID, addressID int64) (*userv1.SetDefaultAddressResponse, error) {
	return c.client.SetDefaultAddress(ctx, &userv1.SetDefaultAddressRequest{
		UserId:    userID,
		AddressId: addressID,
	})
}

// UpdateProfile 调用 User 服务的更新资料 RPC。
func (c *UserClient) UpdateProfile(ctx context.Context, userID int64, nickname, avatarURL string) (*userv1.UpdateProfileResponse, error) {
	return c.client.UpdateProfile(ctx, &userv1.UpdateProfileRequest{
		UserId:    userID,
		Nickname:  nickname,
		AvatarUrl: avatarURL,
	})
}

// UploadAvatar 调用 User 服务的头像上传 RPC，由 user 服务存储文件并回写头像地址。
func (c *UserClient) UploadAvatar(ctx context.Context, userID int64, fileName, mimeType string, content []byte) (*userv1.UploadAvatarResponse, error) {
	return c.client.UploadAvatar(ctx, &userv1.UploadAvatarRequest{
		UserId:   userID,
		FileName: fileName,
		MimeType: mimeType,
		Content:  content,
	})
}

// SendPasswordResetCode 调用 User 服务的验证码发送 RPC。
func (c *UserClient) SendPasswordResetCode(ctx context.Context, userID int64) (*userv1.SendPasswordResetCodeResponse, error) {
	return c.client.SendPasswordResetCode(ctx, &userv1.SendPasswordResetCodeRequest{UserId: userID})
}

// GetPasswordResetCode 读取当前用户待用的改密验证码（仅开发/演示环境）。
func (c *UserClient) GetPasswordResetCode(ctx context.Context, userID int64) (*userv1.GetPasswordResetCodeResponse, error) {
	return c.client.GetPasswordResetCode(ctx, &userv1.GetPasswordResetCodeRequest{UserId: userID})
}

// ChangePassword 调用 User 服务的验证码改密 RPC。
func (c *UserClient) ChangePassword(ctx context.Context, userID int64, code, newPassword string) (*userv1.ChangePasswordResponse, error) {
	return c.client.ChangePassword(ctx, &userv1.ChangePasswordRequest{
		UserId:      userID,
		Code:        code,
		NewPassword: newPassword,
	})
}

// SendPasswordResetCodeByEmail 调用 User 服务的未登录找回密码发码 RPC。
func (c *UserClient) SendPasswordResetCodeByEmail(ctx context.Context, email string) (*userv1.SendPasswordResetCodeByEmailResponse, error) {
	return c.client.SendPasswordResetCodeByEmail(ctx, &userv1.SendPasswordResetCodeByEmailRequest{
		Email: email,
	})
}

// ResetPasswordByEmail 调用 User 服务的未登录找回密码重置 RPC。
func (c *UserClient) ResetPasswordByEmail(ctx context.Context, email, code, newPassword string) (*userv1.ResetPasswordByEmailResponse, error) {
	return c.client.ResetPasswordByEmail(ctx, &userv1.ResetPasswordByEmailRequest{
		Email:       email,
		Code:        code,
		NewPassword: newPassword,
	})
}

// Close 关闭 gRPC 连接。应在程序退出时调用。
func (c *UserClient) Close() error {
	return c.conn.Close()
}
