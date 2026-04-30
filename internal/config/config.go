// Package config 定义 Gateway 服务的配置结构。
// 通过 mapstructure tag 支持 viper 从环境变量加载（前缀 GATEWAY_）。
package config

// Config 是 Gateway 的完整配置。
type Config struct {
	Addr            string `mapstructure:"addr"`              // HTTP 监听地址，如 ":8080"
	UserGRPCAddr    string `mapstructure:"user_grpc_addr"`    // User 服务的 gRPC 地址，如 "user:9090"
	ProductGRPCAddr string `mapstructure:"product_grpc_addr"` // Product 服务的 gRPC 地址，如 "product:9090"
	RedisAddr       string `mapstructure:"redis_addr"`        // Redis 地址，用于秒杀幂等与库存预扣
	RedisDB         int    `mapstructure:"redis_db"`          // Redis 数据库编号，默认使用 0
	NATSURL         string `mapstructure:"nats_url"`          // NATS 连接地址，用于投递秒杀下单事件
	NATSStream      string `mapstructure:"nats_stream"`       // JetStream 流名称，用于承载 seckill 主题
	SeckillSubject  string `mapstructure:"seckill_subject"`   // 秒杀下单消息主题，如 "seckill.order"
	JWTSecret       string `mapstructure:"jwt_secret"`        // JWT 签名密钥（需与 User 服务保持一致）
	LogLevel        string `mapstructure:"log_level"`         // 日志级别：debug/info/warn/error
}
