package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// AssetRepo 访问 canvas_assets 表。
type AssetRepo struct {
	db *DB
}

// Assets 返回资产仓库。
func (d *DB) Assets() *AssetRepo { return &AssetRepo{db: d} }

// ListAssets 返回该用户全部资产（完整 data），按 updated_at 倒序，可按 kind 过滤。
func (r *AssetRepo) ListAssets(ctx context.Context, userID int64, kind string) ([]json.RawMessage, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if kind == "" {
		const q = `SELECT data FROM canvas_assets WHERE user_id = $1 ORDER BY updated_at DESC`
		rows, err = r.db.sql.QueryContext(ctx, q, userID)
	} else {
		const q = `SELECT data FROM canvas_assets WHERE user_id = $1 AND kind = $2 ORDER BY updated_at DESC`
		rows, err = r.db.sql.QueryContext(ctx, q, userID, kind)
	}
	if err != nil {
		return nil, fmt.Errorf("list assets: %w", err)
	}
	defer rows.Close()

	out := make([]json.RawMessage, 0, 32)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("scan asset: %w", err)
		}
		out = append(out, json.RawMessage(data))
	}
	return out, rows.Err()
}

// UpsertAsset 按 (user_id, client_id) 插入或更新资产。
func (r *AssetRepo) UpsertAsset(ctx context.Context, userID int64, clientID, kind, title string, data []byte) error {
	const q = `
INSERT INTO canvas_assets (user_id, client_id, kind, title, data, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, now(), now())
ON CONFLICT (user_id, client_id) DO UPDATE
SET kind = EXCLUDED.kind, title = EXCLUDED.title, data = EXCLUDED.data, updated_at = now()`
	if _, err := r.db.sql.ExecContext(ctx, q, userID, clientID, kind, title, data); err != nil {
		return fmt.Errorf("upsert asset: %w", err)
	}
	return nil
}

// DeleteAsset 删除单个资产；返回是否命中。
func (r *AssetRepo) DeleteAsset(ctx context.Context, userID int64, clientID string) (bool, error) {
	const q = `DELETE FROM canvas_assets WHERE user_id = $1 AND client_id = $2`
	res, err := r.db.sql.ExecContext(ctx, q, userID, clientID)
	if err != nil {
		return false, fmt.Errorf("delete asset: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ── Config ────────────────────────────────────────────────────────────────────

// ConfigRepo 访问 canvas_configs 表（每用户一行）。
type ConfigRepo struct {
	db *DB
}

// Config 返回配置仓库。
func (d *DB) Config() *ConfigRepo { return &ConfigRepo{db: d} }

// GetConfig 取用户配置；不存在返回 (nil, nil)。
func (r *ConfigRepo) GetConfig(ctx context.Context, userID int64) (json.RawMessage, error) {
	const q = `SELECT config FROM canvas_configs WHERE user_id = $1`
	var data []byte
	err := r.db.sql.QueryRowContext(ctx, q, userID).Scan(&data)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get config: %w", err)
	}
	return json.RawMessage(data), nil
}

// SaveConfig 插入或覆盖用户配置。
func (r *ConfigRepo) SaveConfig(ctx context.Context, userID int64, data []byte) error {
	const q = `
INSERT INTO canvas_configs (user_id, config, updated_at) VALUES ($1, $2, now())
ON CONFLICT (user_id) DO UPDATE SET config = EXCLUDED.config, updated_at = now()`
	if _, err := r.db.sql.ExecContext(ctx, q, userID, data); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	return nil
}
