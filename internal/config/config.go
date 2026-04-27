// Package config 定义 Gateway 服务的配置结构。
// 通过 mapstructure tag 支持 viper 从环境变量加载（前缀 GATEWAY_）。
package config

// Config 是 Gateway 的完整配置。
type Config struct {
	Addr         string `mapstructure:"addr"`           // HTTP 监听地址，如 ":8080"
	UserGRPCAddr string `mapstructure:"user_grpc_addr"` // User 服务的 gRPC 地址，如 "user:9090"
	JWTSecret    string `mapstructure:"jwt_secret"`     // JWT 签名密钥（需与 User 服务保持一致）
	LogLevel     string `mapstructure:"log_level"`      // 日志级别：debug/info/warn/error
}
