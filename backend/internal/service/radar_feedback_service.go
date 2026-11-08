package service

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

const (
	RadarFeedbackMaxContentBytes      = 40000
	RadarFeedbackMaxAttachments       = 3
	RadarFeedbackMaxImageBytes        = 5 << 20
	RadarFeedbackMaxMessageImageBytes = 5 << 20
)

var (
	ErrRadarFeedbackInvalid       = errors.New("invalid radar feedback")
	ErrRadarFeedbackThreadClosed  = errors.New("feedback thread is closed")
	ErrRadarFeedbackThreadMissing = errors.New("feedback thread not found")
)

type RadarFeedbackAttachment struct {
	ID          int64  `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	DataURL     string `json:"data_url,omitempty"`
}

type RadarFeedbackMessage struct {
	ID          int64                     `json:"id"`
	AuthorType  string                    `json:"author_type"`
	AuthorID    int64                     `json:"author_id"`
	Content     string                    `json:"content"`
	CreatedAt   time.Time                 `json:"created_at"`
	Attachments []RadarFeedbackAttachment `json:"attachments"`
}

type RadarFeedbackThread struct {
	ID        int64                  `json:"id"`
	UserID    int64                  `json:"user_id"`
	Username  string                 `json:"username,omitempty"`
	Email     string                 `json:"email,omitempty"`
	Subject   string                 `json:"subject"`
	Status    string                 `json:"status"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	Messages  []RadarFeedbackMessage `json:"messages"`
}

type RadarFeedbackService struct{ db *dbent.Client }

const radarFeedbackPageSize = 10

func NewRadarFeedbackService(db *dbent.Client) *RadarFeedbackService {
	return &RadarFeedbackService{db: db}
}

func (s *RadarFeedbackService) ListMine(ctx context.Context, userID int64) ([]RadarFeedbackThread, error) {
	items, _, err := s.ListMinePage(ctx, userID, radarFeedbackPageSize, 0)
	return items, err
}

func (s *RadarFeedbackService) ListMinePage(ctx context.Context, userID int64, limit, offset int) ([]RadarFeedbackThread, bool, error) {
	if userID <= 0 || s == nil || s.db == nil {
		return nil, false, ErrRadarFeedbackInvalid
	}
	if limit <= 0 || limit > 50 {
		limit = radarFeedbackPageSize
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_id, subject, status, created_at, updated_at FROM radar_feedback_threads WHERE user_id=$1 ORDER BY updated_at DESC, id DESC LIMIT $2 OFFSET $3`, userID, limit+1, offset)
	if err != nil {
		return nil, false, fmt.Errorf("list radar feedback: %w", err)
	}
	out := make([]RadarFeedbackThread, 0)
	for rows.Next() {
		var item RadarFeedbackThread
		if err := rows.Scan(&item.ID, &item.UserID, &item.Subject, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, false, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, false, err
	}
	rows.Close()
	hasMore := len(out) > limit
	if hasMore {
		out = out[:limit]
	}
	for i := range out {
		out[i].Messages, err = s.messages(ctx, out[i].ID)
		if err != nil {
			return nil, false, err
		}
	}
	return out, hasMore, nil
}

func (s *RadarFeedbackService) Create(ctx context.Context, userID int64, subject, content string, attachments []RadarFeedbackAttachmentInput) (*RadarFeedbackThread, error) {
	content = strings.TrimSpace(content)
	subject = strings.TrimSpace(subject)
	if userID <= 0 || (content == "" && len(attachments) == 0) || len(content) > RadarFeedbackMaxContentBytes || len(subject) > 120 || len(attachments) > RadarFeedbackMaxAttachments {
		return nil, ErrRadarFeedbackInvalid
	}
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var threadID, messageID int64
	rows, err := tx.Client().QueryContext(ctx, `INSERT INTO radar_feedback_threads (user_id, subject) VALUES ($1,$2) RETURNING id`, userID, subject)
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		rows.Close()
		return nil, sql.ErrNoRows
	}
	err = rows.Scan(&threadID)
	rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = tx.Client().QueryContext(ctx, `INSERT INTO radar_feedback_messages (thread_id, author_type, author_id, content) VALUES ($1,'user',$2,$3) RETURNING id`, threadID, userID, content)
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		rows.Close()
		return nil, sql.ErrNoRows
	}
	err = rows.Scan(&messageID)
	rows.Close()
	if err != nil {
		return nil, err
	}
	if err = insertRadarFeedbackAttachments(ctx, tx, messageID, attachments); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	items, err := s.ListMine(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ID == threadID {
			return &items[i], nil
		}
	}
	return nil, sql.ErrNoRows
}

type RadarFeedbackAttachmentInput struct {
	Filename, ContentType string
	SizeBytes             int64
	Content               []byte
}

func (s *RadarFeedbackService) Reply(ctx context.Context, threadID, adminID int64, content string, attachments []RadarFeedbackAttachmentInput, closeThread bool) (*RadarFeedbackThread, error) {
	content = strings.TrimSpace(content)
	if threadID <= 0 || adminID <= 0 || (content == "" && len(attachments) == 0) || len(content) > RadarFeedbackMaxContentBytes || len(attachments) > RadarFeedbackMaxAttachments {
		return nil, ErrRadarFeedbackInvalid
	}
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var userID int64
	rows, err := tx.Client().QueryContext(ctx, `SELECT user_id FROM radar_feedback_threads WHERE id=$1 FOR UPDATE`, threadID)
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		rows.Close()
		return nil, sql.ErrNoRows
	}
	err = rows.Scan(&userID)
	rows.Close()
	if err != nil {
		return nil, err
	}
	var messageID int64
	rows, err = tx.Client().QueryContext(ctx, `INSERT INTO radar_feedback_messages (thread_id, author_type, author_id, content) VALUES ($1,'admin',$2,$3) RETURNING id`, threadID, adminID, content)
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		rows.Close()
		return nil, sql.ErrNoRows
	}
	err = rows.Scan(&messageID)
	rows.Close()
	if err != nil {
		return nil, err
	}
	if err = insertRadarFeedbackAttachments(ctx, tx, messageID, attachments); err != nil {
		return nil, err
	}
	status := "replied"
	if closeThread {
		status = "closed"
	}
	if _, err = tx.ExecContext(ctx, `UPDATE radar_feedback_threads SET status=$2, updated_at=NOW() WHERE id=$1`, threadID, status); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	items, err := s.ListMine(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ID == threadID {
			return &items[i], nil
		}
	}
	return nil, sql.ErrNoRows
}

func (s *RadarFeedbackService) ReplyFromUser(ctx context.Context, threadID, userID int64, content string, attachments []RadarFeedbackAttachmentInput) (*RadarFeedbackThread, error) {
	content = strings.TrimSpace(content)
	if s == nil || s.db == nil || threadID <= 0 || userID <= 0 || (content == "" && len(attachments) == 0) || len(content) > RadarFeedbackMaxContentBytes || len(attachments) > RadarFeedbackMaxAttachments {
		return nil, ErrRadarFeedbackInvalid
	}
	tx, err := s.db.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var ownerID int64
	var status string
	rows, err := tx.Client().QueryContext(ctx, `SELECT user_id,status FROM radar_feedback_threads WHERE id=$1 FOR UPDATE`, threadID)
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		rows.Close()
		return nil, ErrRadarFeedbackThreadMissing
	}
	err = rows.Scan(&ownerID, &status)
	rows.Close()
	if err != nil {
		return nil, err
	}
	if ownerID != userID {
		return nil, ErrRadarFeedbackThreadMissing
	}
	if status == "closed" {
		return nil, ErrRadarFeedbackThreadClosed
	}
	var messageID int64
	rows, err = tx.Client().QueryContext(ctx, `INSERT INTO radar_feedback_messages (thread_id,author_type,author_id,content) VALUES ($1,'user',$2,$3) RETURNING id`, threadID, userID, content)
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		rows.Close()
		return nil, sql.ErrNoRows
	}
	err = rows.Scan(&messageID)
	rows.Close()
	if err != nil {
		return nil, err
	}
	if err = insertRadarFeedbackAttachments(ctx, tx, messageID, attachments); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE radar_feedback_threads SET status='open',updated_at=NOW() WHERE id=$1`, threadID); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	items, err := s.ListMine(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ID == threadID {
			return &items[i], nil
		}
	}
	return nil, sql.ErrNoRows
}

func (s *RadarFeedbackService) ListAll(ctx context.Context, status string) ([]RadarFeedbackThread, error) {
	query := `SELECT t.id,t.user_id,COALESCE(u.username,''),COALESCE(u.email,''),t.subject,t.status,t.created_at,t.updated_at FROM radar_feedback_threads t LEFT JOIN users u ON u.id=t.user_id WHERE ($1='' OR t.status=$1) ORDER BY t.updated_at DESC,t.id DESC LIMIT 10`
	rows, err := s.db.QueryContext(ctx, query, strings.TrimSpace(status))
	if err != nil {
		return nil, err
	}
	out := make([]RadarFeedbackThread, 0)
	for rows.Next() {
		var item RadarFeedbackThread
		if err := rows.Scan(&item.ID, &item.UserID, &item.Username, &item.Email, &item.Subject, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for i := range out {
		out[i].Messages, err = s.messages(ctx, out[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *RadarFeedbackService) messages(ctx context.Context, threadID int64) ([]RadarFeedbackMessage, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,author_type,author_id,content,created_at FROM radar_feedback_messages WHERE thread_id=$1 ORDER BY created_at ASC`, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RadarFeedbackMessage, 0)
	for rows.Next() {
		var item RadarFeedbackMessage
		if err := rows.Scan(&item.ID, &item.AuthorType, &item.AuthorID, &item.Content, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Attachments, err = s.attachments(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *RadarFeedbackService) attachments(ctx context.Context, messageID int64) ([]RadarFeedbackAttachment, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,filename,content_type,size_bytes,content FROM radar_feedback_attachments WHERE message_id=$1 ORDER BY id`, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]RadarFeedbackAttachment, 0)
	for rows.Next() {
		var item RadarFeedbackAttachment
		var data []byte
		if err := rows.Scan(&item.ID, &item.Filename, &item.ContentType, &item.SizeBytes, &data); err != nil {
			return nil, err
		}
		item.DataURL = "data:" + item.ContentType + ";base64," + base64.StdEncoding.EncodeToString(data)
		out = append(out, item)
	}
	return out, rows.Err()
}

func insertRadarFeedbackAttachments(ctx context.Context, tx *dbent.Tx, messageID int64, items []RadarFeedbackAttachmentInput) error {
	var total int64
	for _, item := range items {
		if item.SizeBytes <= 0 || item.SizeBytes > RadarFeedbackMaxImageBytes || len(item.Content) == 0 || int64(len(item.Content)) != item.SizeBytes || !allowedRadarFeedbackImage(item.ContentType) {
			return ErrRadarFeedbackInvalid
		}
		total += item.SizeBytes
		if total > RadarFeedbackMaxMessageImageBytes {
			return ErrRadarFeedbackInvalid
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO radar_feedback_attachments (message_id,filename,content_type,size_bytes,content) VALUES ($1,$2,$3,$4,$5)`, messageID, item.Filename, item.ContentType, item.SizeBytes, item.Content); err != nil {
			return err
		}
	}
	return nil
}

func allowedRadarFeedbackImage(contentType string) bool {
	typ, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	switch strings.ToLower(typ) {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return true
	}
	return false
}
