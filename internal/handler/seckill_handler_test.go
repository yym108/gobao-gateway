//go:build integration

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yym108/gobao-gateway/internal/client"
	"github.com/yym108/gobao-pkg/cache"
	"github.com/yym108/gobao-pkg/mq"
	productv1 "github.com/yym108/gobao-proto/gen/go/gobao/product/v1"
	"google.golang.org/grpc"
)

func init() { gin.SetMode(gin.TestMode) }

// fakeProductService 用函数桩实现测试所需的 Product gRPC 能力。
type fakeProductService struct {
	productv1.UnimplementedProductServiceServer
	getSeckillActivityFn func(ctx context.Context, req *productv1.GetSeckillActivityRequest) (*productv1.GetSeckillActivityResponse, error)
	prewarmSeckillFn     func(ctx context.Context, req *productv1.PrewarmSeckillRequest) (*productv1.PrewarmSeckillResponse, error)
}

// GetSeckillActivity 返回测试桩定义的秒杀活动查询结果。
func (s *fakeProductService) GetSeckillActivity(ctx context.Context, req *productv1.GetSeckillActivityRequest) (*productv1.GetSeckillActivityResponse, error) {
	return s.getSeckillActivityFn(ctx, req)
}

// PrewarmSeckill 返回测试桩定义的预热结果。
func (s *fakeProductService) PrewarmSeckill(ctx context.Context, req *productv1.PrewarmSeckillRequest) (*productv1.PrewarmSeckillResponse, error) {
	return s.prewarmSeckillFn(ctx, req)
}

// seckillHandlerTestEnv 封装秒杀 handler integration test 所需依赖。
type seckillHandlerTestEnv struct {
	handler      *SeckillHandler
	router       *gin.Engine
	rdb          *redis.Client
	bus          *mq.Bus
	subject      string
	stream       string
	token        string
	userID       int64
	productConn  *client.ProductClient
	grpcServer   *grpc.Server
	listener     net.Listener
}

// testRedisAddr 返回 integration 测试使用的 Redis 地址。
func testRedisAddr() string {
	if addr := os.Getenv("SECKILL_TEST_REDIS_ADDR"); addr != "" {
		return addr
	}
	return "127.0.0.1:6379"
}

// testNATSURL 返回 integration 测试使用的 NATS 地址。
func testNATSURL() string {
	if url := os.Getenv("SECKILL_TEST_NATS_URL"); url != "" {
		return url
	}
	return "nats://127.0.0.1:4222"
}

// setupSeckillHandlerTestEnv 创建真 Redis、真 NATS 和假 Product gRPC 服务组成的测试环境。
func setupSeckillHandlerTestEnv(t *testing.T, svc *fakeProductService) *seckillHandlerTestEnv {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	grpcServer := grpc.NewServer()
	productv1.RegisterProductServiceServer(grpcServer, svc)
	go func() { _ = grpcServer.Serve(lis) }()

	productClient, err := client.NewProductClient(lis.Addr().String())
	require.NoError(t, err)

	rdb, err := cache.NewClient(cache.Config{Addr: testRedisAddr()})
	require.NoError(t, err)

	token := strconv.FormatInt(time.Now().UnixNano(), 10)
	stream := "SECKILL_TEST_" + token
	subject := "itest.seckill.order." + token
	userID, err := strconv.ParseInt(token[len(token)-6:], 10, 64)
	require.NoError(t, err)
	bus, err := mq.New(mq.Config{
		URL:      testNATSURL(),
		Stream:   stream,
		Subjects: []string{subject},
	})
	require.NoError(t, err)

	h := NewSeckillHandler(productClient, rdb, bus, subject)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Next()
	})
	router.GET("/seckill/activities/:id", h.GetActivity)
	router.POST("/seckill/activities/:id/prewarm", h.PrewarmActivity)
	router.POST("/seckill/activities/:id/purchase", h.Purchase)

	env := &seckillHandlerTestEnv{
		handler:     h,
		router:      router,
		rdb:         rdb,
		bus:         bus,
		subject:     subject,
		stream:      stream,
		token:       token,
		userID:      userID,
		productConn: productClient,
		grpcServer:  grpcServer,
		listener:    lis,
	}
	t.Cleanup(func() {
		cleanupSeckillTestData(t, env)
		bus.Close()
		_ = rdb.Close()
		_ = productClient.Close()
		grpcServer.Stop()
		_ = lis.Close()
	})
	return env
}

// requestID 为当前测试环境生成唯一请求 ID，避免真 Redis 中的幂等键互相污染。
func (e *seckillHandlerTestEnv) requestID(suffix string) string {
	return suffix + "-" + e.token
}

// cleanupSeckillTestData 删除当前测试环境写入的 Redis 幂等键和 NATS stream，避免污染本地 Docker 数据。
func cleanupSeckillTestData(t *testing.T, env *seckillHandlerTestEnv) {
	t.Helper()

	ctx := context.Background()
	patterns := []string{
		"seckill:req:*" + env.token,
		"seckill:activity:*:stock",
	}
	for _, pattern := range patterns {
		keys, err := env.rdb.Keys(ctx, pattern).Result()
		if err != nil {
			t.Fatalf("list redis keys by pattern %q: %v", pattern, err)
		}
		if len(keys) == 0 {
			continue
		}
		if err := env.rdb.Del(ctx, keys...).Err(); err != nil {
			t.Fatalf("delete redis keys by pattern %q: %v", pattern, err)
		}
	}

	nc, err := nats.Connect(testNATSURL())
	if err != nil {
		t.Fatalf("connect nats for cleanup: %v", err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatalf("create jetstream for cleanup: %v", err)
	}
	if err := js.DeleteStream(ctx, env.stream); err != nil && err != jetstream.ErrStreamNotFound {
		t.Fatalf("delete nats stream %q: %v", env.stream, err)
	}
}

// seedSeckillStock 写入预热后的秒杀库存 key，供 Lua 原子预扣测试使用。
func seedSeckillStock(t *testing.T, rdb *redis.Client, activityID int64, quantity int32) string {
	t.Helper()
	key := "seckill:activity:" + strconv.FormatInt(activityID, 10) + ":stock"
	require.NoError(t, rdb.Set(context.Background(), key, quantity, 5*time.Minute).Err())
	t.Cleanup(func() {
		_ = rdb.Del(context.Background(), key).Err()
	})
	return key
}

// performJSONRequest 发送一个 JSON 请求并返回 recorder。
func performJSONRequest(r http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestSeckillHandler_GetActivity_Success 测试活动查询 HTTP 转发成功。
func TestSeckillHandler_GetActivity_Success(t *testing.T) {
	env := setupSeckillHandlerTestEnv(t, &fakeProductService{
		getSeckillActivityFn: func(_ context.Context, req *productv1.GetSeckillActivityRequest) (*productv1.GetSeckillActivityResponse, error) {
			return &productv1.GetSeckillActivityResponse{
				Activity: &productv1.SeckillActivity{Id: req.GetId(), ProductId: 2001, Title: "活动查询"},
			}, nil
		},
		prewarmSeckillFn: func(_ context.Context, req *productv1.PrewarmSeckillRequest) (*productv1.PrewarmSeckillResponse, error) {
			return &productv1.PrewarmSeckillResponse{ActivityId: req.GetId()}, nil
		},
	})

	w := performJSONRequest(env.router, http.MethodGet, "/seckill/activities/9", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"id":9`)
	assert.Contains(t, w.Body.String(), `"product_id":2001`)
}

// TestSeckillHandler_Purchase_NotPrewarmed 测试库存 key 不存在时返回未预热。
func TestSeckillHandler_Purchase_NotPrewarmed(t *testing.T) {
	now := time.Now()
	env := setupSeckillHandlerTestEnv(t, &fakeProductService{
		getSeckillActivityFn: func(_ context.Context, req *productv1.GetSeckillActivityRequest) (*productv1.GetSeckillActivityResponse, error) {
			return &productv1.GetSeckillActivityResponse{
				Activity: &productv1.SeckillActivity{
					Id: req.GetId(), ProductId: 2002, Title: "未预热活动",
					StartAt: now.Add(-time.Minute).Unix(), EndAt: now.Add(time.Hour).Unix(),
				},
			}, nil
		},
		prewarmSeckillFn: func(_ context.Context, req *productv1.PrewarmSeckillRequest) (*productv1.PrewarmSeckillResponse, error) {
			return &productv1.PrewarmSeckillResponse{ActivityId: req.GetId()}, nil
		},
	})

	w := performJSONRequest(env.router, http.MethodPost, "/seckill/activities/10/purchase", SeckillPurchaseRequest{
		RequestID: env.requestID("req-not-prewarm"),
		Quantity:  1,
	})
	assert.Equal(t, http.StatusPreconditionFailed, w.Code)
	assert.Contains(t, w.Body.String(), "尚未预热")
}

// TestSeckillHandler_Purchase_InsufficientStock 测试库存不足时返回冲突。
func TestSeckillHandler_Purchase_InsufficientStock(t *testing.T) {
	now := time.Now()
	env := setupSeckillHandlerTestEnv(t, &fakeProductService{
		getSeckillActivityFn: func(_ context.Context, req *productv1.GetSeckillActivityRequest) (*productv1.GetSeckillActivityResponse, error) {
			return &productv1.GetSeckillActivityResponse{
				Activity: &productv1.SeckillActivity{
					Id: req.GetId(), ProductId: 2003, Title: "库存不足活动",
					StartAt: now.Add(-time.Minute).Unix(), EndAt: now.Add(time.Hour).Unix(),
				},
			}, nil
		},
		prewarmSeckillFn: func(_ context.Context, req *productv1.PrewarmSeckillRequest) (*productv1.PrewarmSeckillResponse, error) {
			return &productv1.PrewarmSeckillResponse{ActivityId: req.GetId()}, nil
		},
	})
	seedSeckillStock(t, env.rdb, 11, 0)

	w := performJSONRequest(env.router, http.MethodPost, "/seckill/activities/11/purchase", SeckillPurchaseRequest{
		RequestID: env.requestID("req-insufficient"),
		Quantity:  1,
	})
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "库存不足")
}

// TestSeckillHandler_Purchase_SuccessAndDuplicate 测试首次抢购成功、重复请求被幂等守卫拦截。
func TestSeckillHandler_Purchase_SuccessAndDuplicate(t *testing.T) {
	now := time.Now()
	env := setupSeckillHandlerTestEnv(t, &fakeProductService{
		getSeckillActivityFn: func(_ context.Context, req *productv1.GetSeckillActivityRequest) (*productv1.GetSeckillActivityResponse, error) {
			return &productv1.GetSeckillActivityResponse{
				Activity: &productv1.SeckillActivity{
					Id: req.GetId(), ProductId: 2004, Title: "成功活动",
					StartAt: now.Add(-time.Minute).Unix(), EndAt: now.Add(time.Hour).Unix(),
				},
			}, nil
		},
		prewarmSeckillFn: func(_ context.Context, req *productv1.PrewarmSeckillRequest) (*productv1.PrewarmSeckillResponse, error) {
			return &productv1.PrewarmSeckillResponse{ActivityId: req.GetId()}, nil
		},
	})
	stockKey := seedSeckillStock(t, env.rdb, 12, 2)

	first := performJSONRequest(env.router, http.MethodPost, "/seckill/activities/12/purchase", SeckillPurchaseRequest{
		RequestID: env.requestID("req-success"),
		Quantity:  1,
	})
	assert.Equal(t, http.StatusAccepted, first.Code)
	assert.Contains(t, first.Body.String(), `"remaining":1`)

	remaining, err := env.rdb.Get(context.Background(), stockKey).Int()
	require.NoError(t, err)
	assert.Equal(t, 1, remaining)

	second := performJSONRequest(env.router, http.MethodPost, "/seckill/activities/12/purchase", SeckillPurchaseRequest{
		RequestID: env.requestID("req-success"),
		Quantity:  1,
	})
	assert.Equal(t, http.StatusConflict, second.Code)
	assert.Contains(t, second.Body.String(), "重复请求")

	remainingAfterDuplicate, err := env.rdb.Get(context.Background(), stockKey).Int()
	require.NoError(t, err)
	assert.Equal(t, 1, remainingAfterDuplicate)
}

// TestSeckillHandler_Purchase_PublishFailureRestoresStock 测试入队失败后会回补 Redis 库存。
func TestSeckillHandler_Purchase_PublishFailureRestoresStock(t *testing.T) {
	now := time.Now()
	env := setupSeckillHandlerTestEnv(t, &fakeProductService{
		getSeckillActivityFn: func(_ context.Context, req *productv1.GetSeckillActivityRequest) (*productv1.GetSeckillActivityResponse, error) {
			return &productv1.GetSeckillActivityResponse{
				Activity: &productv1.SeckillActivity{
					Id: req.GetId(), ProductId: 2005, Title: "发布失败活动",
					StartAt: now.Add(-time.Minute).Unix(), EndAt: now.Add(time.Hour).Unix(),
				},
			}, nil
		},
		prewarmSeckillFn: func(_ context.Context, req *productv1.PrewarmSeckillRequest) (*productv1.PrewarmSeckillResponse, error) {
			return &productv1.PrewarmSeckillResponse{ActivityId: req.GetId()}, nil
		},
	})
	stockKey := seedSeckillStock(t, env.rdb, 13, 1)
	env.bus.Close()

	w := performJSONRequest(env.router, http.MethodPost, "/seckill/activities/13/purchase", SeckillPurchaseRequest{
		RequestID: env.requestID("req-publish-fail"),
		Quantity:  1,
	})
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	restored, err := env.rdb.Get(context.Background(), stockKey).Int()
	require.NoError(t, err)
	assert.Equal(t, 1, restored)
}
