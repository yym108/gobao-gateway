# gobao-gateway

GoBao 的 HTTP 网关仓库，是前端访问后端的统一入口。

## 作用

- 用户认证 HTTP 接口
- 商品与类目 HTTP 接口
- SKU 购物车接口
- 秒杀入口与预热入口

## 关系

- 依赖 `gobao-user`、`gobao-product`
- 依赖 `gobao-pkg`
- 使用 `gobao-deploy` 提供的 Redis / NATS / MySQL 环境
- 被 `gobao-web` 直接调用

## 启动

```bash
go test ./...
go run ./cmd/server
```
