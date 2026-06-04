# gobao-gateway

GoBao 的 HTTP 网关仓库，是前端访问后端的统一入口。

## 作用

- 用户认证 HTTP 接口
- 商品与类目 HTTP 接口
- 商品版本购物车接口
- 秒杀入口与预热入口

## 关系

- 依赖 `gobao-user`、`gobao-product`
- 依赖 `gobao-pkg`
- 使用 `gobao-deploy` 提供的 Redis / NATS / MySQL 环境
- 被 `gobao-web` 直接调用

## 独立使用前准备

当前仓库的 `go.mod` 通过本地 `replace` 依赖 `../gobao-pkg` 与 `../gobao-proto`。单独 clone 本仓后，先执行：

```bash
bash scripts/bootstrap-deps.sh
ln -sfn workspace/gobao-pkg ../gobao-pkg
ln -sfn workspace/gobao-proto ../gobao-proto
```

如果你是通过综合部署仓使用，则不需要这一步。

## 环境变量

可参考仓库根目录 `.env.example`：

- `GATEWAY_MYSQL_DSN`
- `GATEWAY_REDIS_ADDR`
- `GATEWAY_NATS_URL`
- `GATEWAY_USER_GRPC_ADDR`
- `GATEWAY_PRODUCT_GRPC_ADDR`
- `GATEWAY_ORDER_GRPC_ADDR`
- `GATEWAY_PAYMENT_GRPC_ADDR`
- `GATEWAY_JWT_SECRET`

## 启动与验证

```bash
go test ./...
go run ./cmd/server
```

如需容器化启动，可直接使用仓库内 `Dockerfile`，或由 `gobao-deploy` / `GoBao` 主仓统一编排。
