package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// ProjectRepo 访问 canvas_projects 表。所有操作都以 userID 为隔离边界，
// 保证一个用户永远看不到另一个用户的画布。
type ProjectRepo struct {
	db *DB
}

// Projects 返回项目仓库。
func (d *DB) Projects() *ProjectRepo { return &ProjectRepo{db: d} }

// ListProjects 返回该用户全部画布的完整 data（每条即一个前端 CanvasProject 对象），
// 按 updated_at 倒序。当前前端把所有画布一次性载入内存，此接口与之 1:1 对齐。
func (r *ProjectRepo) ListProjects(ctx context.Context, userID int64) ([]json.RawMessage, error) {
	const q = `SELECT data FROM canvas_projects WHERE user_id = $1 ORDER BY updated_at DESC`
	rows, err := r.db.sql.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	out := make([]json.RawMessage, 0, 16)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		out = append(out, json.RawMessage(data))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}
	return out, nil
}

// GetProject 返回单个画布的完整 data；不存在返回 (nil, nil)。
func (r *ProjectRepo) GetProject(ctx context.Context, userID int64, clientID string) (json.RawMessage, error) {
	const q = `SELECT data FROM canvas_projects WHERE user_id = $1 AND client_id = $2`
	var data []byte
	err := r.db.sql.QueryRowContext(ctx, q, userID, clientID).Scan(&data)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get project: %w", err)
	}
	return json.RawMessage(data), nil
}

// UpsertProject 按 (user_id, client_id) 插入或更新。首次插入时记录 created_at，
// 之后更新只刷新 title/data/updated_at。
func (r *ProjectRepo) UpsertProject(ctx context.Context, userID int64, clientID, title string, data []byte) error {
	const q = `
INSERT INTO canvas_projects (user_id, client_id, title, data, created_at, updated_at)
VALUES ($1, $2, $3, $4, now(), now())
ON CONFLICT (user_id, client_id) DO UPDATE
SET title = EXCLUDED.title, data = EXCLUDED.data, updated_at = now()`
	if _, err := r.db.sql.ExecContext(ctx, q, userID, clientID, title, data); err != nil {
		return fmt.Errorf("upsert project: %w", err)
	}
	return nil
}

// DeleteProject 删除单个画布；返回是否命中。
func (r *ProjectRepo) DeleteProject(ctx context.Context, userID int64, clientID string) (bool, error) {
	const q = `DELETE FROM canvas_projects WHERE user_id = $1 AND client_id = $2`
	res, err := r.db.sql.ExecContext(ctx, q, userID, clientID)
	if err != nil {
		return false, fmt.Errorf("delete project: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
