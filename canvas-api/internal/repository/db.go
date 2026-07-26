package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/lib/pq"

	"github.com/blsnt8586/canvas-api/internal/auth"
)

// DB 封装底层 *sql.DB 连接池。canvas-api 与 sub2api 共享同一个 PostgreSQL 实例，
// 但只写 canvas_* 表；users 表为只读（归 sub2api 所有）。
type DB struct {
	sql *sql.DB
}

// Open 打开 PostgreSQL 连接并验证连通性。
func Open(dsn string) (*DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return &DB{sql: db}, nil
}

// Close 关闭连接池。
func (d *DB) Close() error { return d.sql.Close() }

// SQL 暴露底层连接池，供后续 Phase 的 canvas_* 表访问。
func (d *DB) SQL() *sql.DB { return d.sql }

// GetAuthUser 只读 users 表，返回鉴权所需的最小字段集。
// deleted_at 非空（软删除）视为不存在。
func (d *DB) GetAuthUser(ctx context.Context, id int64) (*auth.AuthUser, error) {
	const q = `SELECT id, email, password_hash, status FROM users WHERE id = $1 AND deleted_at IS NULL`
	var u auth.AuthUser
	err := d.sql.QueryRowContext(ctx, q, id).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("query auth user: %w", err)
	}
	return &u, nil
}
