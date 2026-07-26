package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// MediaRecord 是 canvas_media 表的一行，对应一份生成内容的元数据。
type MediaRecord struct {
	MediaKey   string
	Kind       string
	S3Key      string
	MimeType   string
	Bytes      int64
	Width      *int
	Height     *int
	DurationMs *int
	Pinned     bool
	CreatedAt  string
}

// MediaRepo 访问 canvas_media 表。所有操作以 userID 为隔离边界。
type MediaRepo struct {
	db *DB
}

// Media 返回媒体仓库。
func (d *DB) Media() *MediaRepo { return &MediaRepo{db: d} }

// Upsert 按 (user_id, media_key) 插入或更新媒体元数据。
// 重新上传同一 key 时刷新 s3_key/元数据，但保留 pinned 与 created_at。
func (r *MediaRepo) Upsert(ctx context.Context, userID int64, m MediaRecord) error {
	const q = `
INSERT INTO canvas_media (user_id, media_key, kind, s3_key, mime_type, bytes, width, height, duration_ms, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, now())
ON CONFLICT (user_id, media_key) DO UPDATE
SET kind = EXCLUDED.kind, s3_key = EXCLUDED.s3_key, mime_type = EXCLUDED.mime_type,
    bytes = EXCLUDED.bytes, width = EXCLUDED.width, height = EXCLUDED.height,
    duration_ms = EXCLUDED.duration_ms`
	_, err := r.db.sql.ExecContext(ctx, q, userID, m.MediaKey, m.Kind, m.S3Key, m.MimeType, m.Bytes, m.Width, m.Height, m.DurationMs)
	if err != nil {
		return fmt.Errorf("upsert media: %w", err)
	}
	return nil
}

// GetByKey 按 media_key 取回一条记录；不存在返回 (nil, nil)。
func (r *MediaRepo) GetByKey(ctx context.Context, userID int64, mediaKey string) (*MediaRecord, error) {
	const q = `SELECT media_key, kind, s3_key, mime_type, bytes, width, height, duration_ms, pinned, created_at
FROM canvas_media WHERE user_id = $1 AND media_key = $2`
	var m MediaRecord
	err := r.db.sql.QueryRowContext(ctx, q, userID, mediaKey).
		Scan(&m.MediaKey, &m.Kind, &m.S3Key, &m.MimeType, &m.Bytes, &m.Width, &m.Height, &m.DurationMs, &m.Pinned, &m.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get media: %w", err)
	}
	return &m, nil
}

// List 按类型分页返回用户的媒体（kind 为空表示全部）。
func (r *MediaRepo) List(ctx context.Context, userID int64, kind string, limit, offset int) ([]MediaRecord, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var (
		rows *sql.Rows
		err  error
	)
	if kind == "" {
		const q = `SELECT media_key, kind, s3_key, mime_type, bytes, width, height, duration_ms, pinned, created_at
FROM canvas_media WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		rows, err = r.db.sql.QueryContext(ctx, q, userID, limit, offset)
	} else {
		const q = `SELECT media_key, kind, s3_key, mime_type, bytes, width, height, duration_ms, pinned, created_at
FROM canvas_media WHERE user_id = $1 AND kind = $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`
		rows, err = r.db.sql.QueryContext(ctx, q, userID, kind, limit, offset)
	}
	if err != nil {
		return nil, fmt.Errorf("list media: %w", err)
	}
	defer rows.Close()

	out := make([]MediaRecord, 0, limit)
	for rows.Next() {
		var m MediaRecord
		if err := rows.Scan(&m.MediaKey, &m.Kind, &m.S3Key, &m.MimeType, &m.Bytes, &m.Width, &m.Height, &m.DurationMs, &m.Pinned, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan media: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate media: %w", err)
	}
	return out, nil
}

// SetPinned 标记/取消收藏。收藏的内容不受生命周期清理影响。
func (r *MediaRepo) SetPinned(ctx context.Context, userID int64, mediaKey string, pinned bool) (bool, error) {
	const q = `UPDATE canvas_media SET pinned = $3 WHERE user_id = $1 AND media_key = $2`
	res, err := r.db.sql.ExecContext(ctx, q, userID, mediaKey, pinned)
	if err != nil {
		return false, fmt.Errorf("set pinned: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Delete 删除一条媒体元数据，返回被删记录的 s3_key（供调用方删对象存储里的字节）。
// 不存在返回 ("", nil)。
func (r *MediaRepo) Delete(ctx context.Context, userID int64, mediaKey string) (string, error) {
	const q = `DELETE FROM canvas_media WHERE user_id = $1 AND media_key = $2 RETURNING s3_key`
	var s3Key string
	err := r.db.sql.QueryRowContext(ctx, q, userID, mediaKey).Scan(&s3Key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("delete media: %w", err)
	}
	return s3Key, nil
}
