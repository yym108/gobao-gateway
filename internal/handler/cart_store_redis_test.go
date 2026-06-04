package handler

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yym108/gobao-pkg/cache"
)

// TestRedisCartStore_PersistAcrossInstances 验证购物车写入 Redis 后，可被新的 store 实例重新读取。
func TestRedisCartStore_PersistAcrossInstances(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client, err := cache.NewClient(cache.Config{Addr: mr.Addr()})
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	first := NewRedisCartStore(client)
	ctx := context.Background()

	resp, err := first.Add(ctx, 201, CartItem{
		ProductID:     1001,
		Name:          "MacBook Air",
		Price:         899900,
		Quantity:      1,
		ImageURL:      "https://example.com/macbook-air.png",
		OptionSummary: "星光色 / 256GB / 标准版",
	})
	require.NoError(t, err)
	assert.Equal(t, int32(1), resp.TotalQuantity)

	second := NewRedisCartStore(client)
	loaded, err := second.Get(ctx, 201)
	require.NoError(t, err)
	require.Len(t, loaded.Items, 1)
	assert.Equal(t, int64(1001), loaded.Items[0].ProductID)
	assert.Equal(t, "1001::5pif5YWJ6ImyIC8gMjU2R0IgLyDmoIflh4bniYg", loaded.Items[0].CartItemID)
	assert.Equal(t, "MacBook Air", loaded.Items[0].Name)
	assert.Equal(t, int64(899900), loaded.TotalAmount)
}

// TestRedisCartStore_UpdateAndDelete 验证 Redis 购物车支持数量更新与删除。
func TestRedisCartStore_UpdateAndDelete(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client, err := cache.NewClient(cache.Config{Addr: mr.Addr()})
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	store := NewRedisCartStore(client)
	ctx := context.Background()

	_, err = store.Add(ctx, 202, CartItem{
		ProductID:     1002,
		Name:          "iPad Air",
		Price:         459900,
		Quantity:      1,
		ImageURL:      "https://example.com/ipad-air.png",
		OptionSummary: "深空灰 / 256GB / 标准版",
	})
	require.NoError(t, err)

	updated, err := store.Update(ctx, 202, "1002::5rex56m654GwIC8gMjU2R0IgLyDmoIflh4bniYg", 3)
	require.NoError(t, err)
	require.Len(t, updated.Items, 1)
	assert.Equal(t, int32(3), updated.Items[0].Quantity)
	assert.Equal(t, int64(1379700), updated.TotalAmount)

	err = store.Delete(ctx, 202, "1002::5rex56m654GwIC8gMjU2R0IgLyDmoIflh4bniYg")
	require.NoError(t, err)

	empty, err := store.Get(ctx, 202)
	require.NoError(t, err)
	assert.Empty(t, empty.Items)
	assert.Equal(t, int32(0), empty.TotalQuantity)
	assert.Equal(t, int64(0), empty.TotalAmount)
}

// TestRedisCartStore_SeparateSameProductByOption 验证相同商品不同规格在 Redis 中分为独立条目。
func TestRedisCartStore_SeparateSameProductByOption(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client, err := cache.NewClient(cache.Config{Addr: mr.Addr()})
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	store := NewRedisCartStore(client)
	ctx := context.Background()

	_, err = store.Add(ctx, 203, CartItem{
		ProductID:     1003,
		Name:          "iPhone 17 Pro",
		Price:         799900,
		Quantity:      1,
		ImageURL:      "https://example.com/iphone-pro.png",
		OptionSummary: "原色钛金属 / 256GB / 标准版",
	})
	require.NoError(t, err)

	resp, err := store.Add(ctx, 203, CartItem{
		ProductID:     1003,
		Name:          "iPhone 17 Pro",
		Price:         939900,
		Quantity:      1,
		ImageURL:      "https://example.com/iphone-pro.png",
		OptionSummary: "沙金色 / 1TB / AppleCare+ 版",
	})
	require.NoError(t, err)

	require.Len(t, resp.Items, 2)
	assert.Equal(t, int32(2), resp.TotalQuantity)
	assert.Equal(t, int64(1739800), resp.TotalAmount)
	assert.Equal(t, "1003::5Y6f6Imy6ZKb6YeR5bGeIC8gMjU2R0IgLyDmoIflh4bniYg", resp.Items[0].CartItemID)
	assert.Equal(t, "1003::5rKZ6YeR6ImyIC8gMVRCIC8gQXBwbGVDYXJlKyDniYg", resp.Items[1].CartItemID)
}

// TestRedisCartStore_LegacyCartItemIDMigrated 验证 Redis 中旧格式条目 ID 会在读取时迁移为 URL 安全的新格式。
func TestRedisCartStore_LegacyCartItemIDMigrated(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client, err := cache.NewClient(cache.Config{Addr: mr.Addr()})
	require.NoError(t, err)
	defer func() { _ = client.Close() }()

	ctx := context.Background()
	legacyItem := CartItem{
		CartItemID:    "1005::深空灰 / 512GB / 标准版",
		ProductID:     1005,
		Name:          "MacBook Pro",
		Price:         1299900,
		Quantity:      1,
		ImageURL:      "https://example.com/macbook-pro.png",
		OptionSummary: "深空灰 / 512GB / 标准版",
	}
	payload, err := json.Marshal(legacyItem)
	require.NoError(t, err)
	require.NoError(t, client.HSet(ctx, cartRedisKey(205), legacyItem.CartItemID, payload).Err())

	store := NewRedisCartStore(client)
	loaded, err := store.Get(ctx, 205)
	require.NoError(t, err)
	require.Len(t, loaded.Items, 1)
	assert.Equal(t, "1005::5rex56m654GwIC8gNTEyR0IgLyDmoIflh4bniYg", loaded.Items[0].CartItemID)

	fields, err := client.HKeys(ctx, cartRedisKey(205)).Result()
	require.NoError(t, err)
	require.Equal(t, []string{"1005::5rex56m654GwIC8gNTEyR0IgLyDmoIflh4bniYg"}, fields)
}
