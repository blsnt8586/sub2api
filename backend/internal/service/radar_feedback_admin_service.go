package service

import (
	"context"
	"strings"
	"time"
)

// The inbox is scoped to feedback senders, not the platform's entire user table.
type RadarFeedbackInboxUser struct {
	UserID      int64     `json:"user_id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	ThreadCount int64     `json:"thread_count"`
	UnreadCount int64     `json:"unread_count"`
	UpdatedAt   time.Time `json:"updated_at"`
	Preview     string    `json:"preview"`
}

type RadarFeedbackThreadSummary struct {
	ID            int64     `json:"id"`
	UserID        int64     `json:"user_id"`
	Subject       string    `json:"subject"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	UnreadCount   int64     `json:"unread_count"`
	LastMessageID int64     `json:"last_message_id"`
	Preview       string    `json:"preview"`
}

type RadarFeedbackAdminThread struct {
	RadarFeedbackThread
	UnreadCount       int64 `json:"unread_count"`
	LastUserMessageID int64 `json:"last_user_message_id"`
}

const radarFeedbackInboxUsersSQL = `WITH thread_stats AS (
    SELECT t.id,t.user_id,t.updated_at,
           COUNT(m.id) FILTER (WHERE m.author_type='user' AND m.id>COALESCE(r.last_read_message_id,0)) AS unread_count
    FROM radar_feedback_threads t
    JOIN radar_feedback_messages m ON m.thread_id=t.id
    LEFT JOIN radar_feedback_admin_reads r ON r.thread_id=t.id AND r.admin_id=$1
    GROUP BY t.id,r.last_read_message_id
    HAVING COUNT(m.id) FILTER (WHERE m.author_type='user')>0
), inbox_users AS (
    SELECT user_id,COUNT(*) AS thread_count,SUM(unread_count)::BIGINT AS unread_count,MAX(updated_at) AS updated_at
    FROM thread_stats GROUP BY user_id
)
SELECT i.user_id,COALESCE(u.username,''),COALESCE(u.email,''),i.thread_count,i.unread_count,i.updated_at,
       COALESCE(NULLIF(LEFT(latest.content,160),''),'[图片]')
FROM inbox_users i JOIN users u ON u.id=i.user_id
LEFT JOIN LATERAL (
    SELECT m.content FROM radar_feedback_messages m
    JOIN radar_feedback_threads t ON t.id=m.thread_id
    WHERE t.user_id=i.user_id ORDER BY m.created_at DESC,m.id DESC LIMIT 1
) latest ON true
ORDER BY (i.unread_count>0) DESC,i.updated_at DESC,i.user_id DESC LIMIT $2 OFFSET $3`

const radarFeedbackUserThreadsSQL = `SELECT t.id,t.user_id,t.subject,t.status,t.created_at,t.updated_at,
    (SELECT COUNT(*) FROM radar_feedback_messages m WHERE m.thread_id=t.id AND m.author_type='user' AND m.id>COALESCE(r.last_read_message_id,0)),
    COALESCE(latest.id,0),COALESCE(NULLIF(LEFT(latest.content,160),''),'[图片]')
FROM radar_feedback_threads t
LEFT JOIN radar_feedback_admin_reads r ON r.thread_id=t.id AND r.admin_id=$1
LEFT JOIN LATERAL (
    SELECT id,content FROM radar_feedback_messages WHERE thread_id=t.id ORDER BY id DESC LIMIT 1
) latest ON true
WHERE t.user_id=$2 AND ($3='' OR t.status=$3)
ORDER BY t.updated_at DESC,t.id DESC LIMIT $4 OFFSET $5`

func feedbackAdminPage(limit, offset int) (int, int) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func (s *RadarFeedbackService) AdminListUsers(ctx context.Context, adminID int64, limit, offset int) ([]RadarFeedbackInboxUser, bool, error) {
	if s == nil || s.db == nil || adminID <= 0 {
		return nil, false, ErrRadarFeedbackInvalid
	}
	limit, offset = feedbackAdminPage(limit, offset)
	rows, err := s.db.QueryContext(ctx, radarFeedbackInboxUsersSQL, adminID, limit+1, offset)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	items := make([]RadarFeedbackInboxUser, 0)
	for rows.Next() {
		var item RadarFeedbackInboxUser
		if err := rows.Scan(&item.UserID, &item.Username, &item.Email, &item.ThreadCount, &item.UnreadCount, &item.UpdatedAt, &item.Preview); err != nil {
			return nil, false, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, nil
}

func (s *RadarFeedbackService) AdminListUserThreads(ctx context.Context, adminID, userID int64, status string, limit, offset int) ([]RadarFeedbackThreadSummary, bool, error) {
	if s == nil || s.db == nil || adminID <= 0 || userID <= 0 {
		return nil, false, ErrRadarFeedbackInvalid
	}
	status = strings.TrimSpace(status)
	if status != "" && status != "open" && status != "replied" && status != "closed" {
		return nil, false, ErrRadarFeedbackInvalid
	}
	limit, offset = feedbackAdminPage(limit, offset)
	rows, err := s.db.QueryContext(ctx, radarFeedbackUserThreadsSQL, adminID, userID, status, limit+1, offset)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	items := make([]RadarFeedbackThreadSummary, 0)
	for rows.Next() {
		var item RadarFeedbackThreadSummary
		if err := rows.Scan(&item.ID, &item.UserID, &item.Subject, &item.Status, &item.CreatedAt, &item.UpdatedAt, &item.UnreadCount, &item.LastMessageID, &item.Preview); err != nil {
			return nil, false, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, nil
}

func (s *RadarFeedbackService) AdminGetThread(ctx context.Context, adminID, threadID int64) (*RadarFeedbackAdminThread, error) {
	if s == nil || s.db == nil || adminID <= 0 || threadID <= 0 {
		return nil, ErrRadarFeedbackInvalid
	}
	rows, err := s.db.QueryContext(ctx, `SELECT t.id,t.user_id,COALESCE(u.username,''),COALESCE(u.email,''),t.subject,t.status,t.created_at,t.updated_at,COALESCE(r.last_read_message_id,0)
        FROM radar_feedback_threads t JOIN users u ON u.id=t.user_id
        LEFT JOIN radar_feedback_admin_reads r ON r.thread_id=t.id AND r.admin_id=$1 WHERE t.id=$2`, adminID, threadID)
	if err != nil {
		return nil, err
	}
	var item RadarFeedbackAdminThread
	var lastReadID int64
	if !rows.Next() {
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		return nil, ErrRadarFeedbackThreadMissing
	}
	err = rows.Scan(&item.ID, &item.UserID, &item.Username, &item.Email, &item.Subject, &item.Status, &item.CreatedAt, &item.UpdatedAt, &lastReadID)
	rows.Close()
	if err != nil {
		return nil, err
	}
	item.Messages, err = s.messages(ctx, threadID)
	if err != nil {
		return nil, err
	}
	for _, m := range item.Messages {
		if m.AuthorType != "user" {
			continue
		}
		if m.ID > lastReadID {
			item.UnreadCount++
		}
		if m.ID > item.LastUserMessageID {
			item.LastUserMessageID = m.ID
		}
	}
	return &item, nil
}

const radarFeedbackMarkReadSQL = `INSERT INTO radar_feedback_admin_reads (thread_id,admin_id,last_read_message_id)
    SELECT thread_id,$2,id FROM radar_feedback_messages WHERE thread_id=$1 AND id=$3 AND author_type='user'
    ON CONFLICT (thread_id,admin_id) DO UPDATE
    SET last_read_message_id=GREATEST(radar_feedback_admin_reads.last_read_message_id,EXCLUDED.last_read_message_id),updated_at=NOW()`

func (s *RadarFeedbackService) AdminMarkRead(ctx context.Context, adminID, threadID, lastReadID int64) error {
	if s == nil || s.db == nil || adminID <= 0 || threadID <= 0 || lastReadID <= 0 {
		return ErrRadarFeedbackInvalid
	}
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Serialize with message writes, so a concurrent message cannot be skipped by a watermark.
	rows, err := tx.Client().QueryContext(ctx, `SELECT id FROM radar_feedback_threads WHERE id=$1 FOR UPDATE`, threadID)
	if err != nil {
		return err
	}
	if !rows.Next() {
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		return ErrRadarFeedbackThreadMissing
	}
	var id int64
	err = rows.Scan(&id)
	rows.Close()
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, radarFeedbackMarkReadSQL, threadID, adminID, lastReadID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrRadarFeedbackInvalid
	}
	return tx.Commit()
}
