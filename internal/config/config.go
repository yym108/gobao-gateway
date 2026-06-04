// Package config 定义 Gateway 服务的配置结构。
// 通过 mapstructure tag 支持 viper 从环境变量加载（前缀 GATEWAY_）。
package config

// Config 是 Gateway 的完整配置。
type Config struct {
	Addr            string `mapstructure:"addr"`              // HTTP 监听地址，如 ":8080"
	UserGRPCAddr    string `mapstructure:"user_grpc_addr"`    // User 服务的 gRPC 地址，如 "user:9090"
	AdminGRPCAddr   string `mapstructure:"admin_grpc_addr"`   // Admin 服务的 gRPC 地址，如 "admin:9090"
	ProductGRPCAddr string `mapstructure:"product_grpc_addr"` // Product 服务的 gRPC 地址，如 "product:9090"
	OrderGRPCAddr   string `mapstructure:"order_grpc_addr"`   // Order 服务的 gRPC 地址，如 "order:9090"
	PaymentGRPCAddr string `mapstructure:"payment_grpc_addr"` // Payment 服务的 gRPC 地址，如 "payment:9090"
	MySQLDSN        string `mapstructure:"mysql_dsn"`         // Gateway 自身 MySQL 地址，用于收藏等持久化数据
	RedisAddr       string `mapstructure:"redis_addr"`        // Redis 地址，用于秒杀幂等与库存预扣
	RedisDB         int    `mapstructure:"redis_db"`          // Redis 数据库编号，默认使用 0
	NATSURL         string `mapstructure:"nats_url"`          // NATS 连接地址，用于投递秒杀下单事件
	NATSStream      string `mapstructure:"nats_stream"`       // JetStream 流名称，用于承载 seckill 主题
	SeckillSubject  string `mapstructure:"seckill_subject"`   // 秒杀下单消息主题，如 "seckill.order"
	JWTSecret       string `mapstructure:"jwt_secret"`        // JWT 签名密钥（需与 User 服务保持一致）
	LogLevel        string `mapstructure:"log_level"`         // 日志级别：debug/info/warn/error

	// ExposeDevEndpoints 控制是否注册开发/演示便利接口（如读回改密验证码）。
	// 默认 false，生产环境保持关闭；仅在本地联调时通过 GATEWAY_EXPOSE_DEV_ENDPOINTS=true 开启。
	ExposeDevEndpoints bool `mapstructure:"expose_dev_endpoints"`
}
