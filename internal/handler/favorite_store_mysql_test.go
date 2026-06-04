package handler

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/**
 * newFavoriteSQLMock 创建收藏 MySQL 仓储测试所需的 sql.DB 与 sqlmock。
 */
func newFavoriteSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db, mock
}

func TestNewMySQLFavoriteStore_EnsuresSchema(t *testing.T) {
	db, mock := newFavoriteSQLMock(t)
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS favorites")).
		WillReturnResult(sqlmock.NewResult(0, 0))

	store, err := NewMySQLFavoriteStore(db)
	require.NoError(t, err)
	require.NotNil(t, store)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLFavoriteStore_List(t *testing.T) {
	db, mock := newFavoriteSQLMock(t)
	store := &MySQLFavoriteStore{db: db}

	rows := sqlmock.NewRows([]string{
		"product_id", "created_at",
	}).AddRow(
		1002, time.Unix(1779000000, 0),
	).AddRow(
		1001, time.Unix(1778990000, 0),
	)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT product_id, created_at")).
		WithArgs(int64(1001)).
		WillReturnRows(rows)

	resp, err := store.List(context.Background(), 1001)
	require.NoError(t, err)
	assert.Equal(t, int64(2), resp.Total)
	assert.Len(t, resp.Items, 2)
	assert.Equal(t, int64(1002), resp.Items[0].ProductID)
	assert.Equal(t, int64(1001), resp.Items[1].ProductID)
	assert.Equal(t, int64(1779000000), resp.Items[0].FavoritedAt)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLFavoriteStore_Add(t *testing.T) {
	db, mock := newFavoriteSQLMock(t)
	store := &MySQLFavoriteStore{db: db}

	item := FavoriteItem{
		ProductID:   1001,
		FavoritedAt: 1779000000,
	}

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO favorites (")).
		WithArgs(
			int64(1001),
			int64(1001),
			item.FavoritedAt,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))
	rows := sqlmock.NewRows([]string{
		"product_id", "created_at",
	}).AddRow(
		1001, time.Unix(item.FavoritedAt, 0),
	)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT product_id, created_at")).
		WithArgs(int64(1001)).
		WillReturnRows(rows)

	resp, err := store.Add(context.Background(), 1001, item)
	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.Total)
	assert.Len(t, resp.Items, 1)
	assert.Equal(t, int64(1001), resp.Items[0].ProductID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLFavoriteStore_Delete(t *testing.T) {
	db, mock := newFavoriteSQLMock(t)
	store := &MySQLFavoriteStore{db: db}

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM favorites WHERE user_id = ? AND product_id = ?")).
		WithArgs(int64(1001), int64(1002)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := store.Delete(context.Background(), 1001, 1002)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
