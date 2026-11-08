package service

import (
	"context"
	"regexp"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestAllowedRadarFeedbackImage(t *testing.T) {
	for _, contentType := range []string{"image/png", "image/jpeg", "image/gif", "image/webp", "IMAGE/PNG"} {
		if !allowedRadarFeedbackImage(contentType) {
			t.Errorf("expected %q to be allowed", contentType)
		}
	}
	for _, contentType := range []string{"text/html", "image/svg+xml", "application/octet-stream", ""} {
		if allowedRadarFeedbackImage(contentType) {
			t.Errorf("expected %q to be rejected", contentType)
		}
	}
}

func TestRadarFeedbackLimits(t *testing.T) {
	if RadarFeedbackMaxAttachments != 3 || RadarFeedbackMaxImageBytes != 5<<20 || RadarFeedbackMaxContentBytes != 40000 {
		t.Fatalf("unexpected feedback limits: attachments=%d image_bytes=%d content_bytes=%d", RadarFeedbackMaxAttachments, RadarFeedbackMaxImageBytes, RadarFeedbackMaxContentBytes)
	}
}

func newRadarFeedbackMock(t *testing.T) (*RadarFeedbackService, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, sqlDB)))
	t.Cleanup(func() { _ = client.Close() })
	return NewRadarFeedbackService(client), mock
}

func TestRadarFeedbackReplyFromUserRejectsOtherOwnersAndClosedThreads(t *testing.T) {
	for _, tc := range []struct {
		name    string
		ownerID int64
		status  string
		wantErr error
	}{
		{name: "other owner", ownerID: 99, status: "open", wantErr: ErrRadarFeedbackThreadMissing},
		{name: "closed thread", ownerID: 7, status: "closed", wantErr: ErrRadarFeedbackThreadClosed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, mock := newRadarFeedbackMock(t)
			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta(`SELECT user_id,status FROM radar_feedback_threads WHERE id=$1 FOR UPDATE`)).
				WithArgs(int64(12)).
				WillReturnRows(sqlmock.NewRows([]string{"user_id", "status"}).AddRow(tc.ownerID, tc.status))
			mock.ExpectRollback()

			_, err := svc.ReplyFromUser(context.Background(), 12, 7, "续问", nil)
			require.ErrorIs(t, err, tc.wantErr)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRadarFeedbackReplyFromUserKeepsTheConversationAndReopensIt(t *testing.T) {
	svc, mock := newRadarFeedbackMock(t)
	now := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT user_id,status FROM radar_feedback_threads WHERE id=$1 FOR UPDATE`)).
		WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "status"}).AddRow(7, "replied"))
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO radar_feedback_messages (thread_id,author_type,author_id,content) VALUES ($1,'user',$2,$3) RETURNING id`)).
		WithArgs(int64(12), int64(7), "续问").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(99))
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE radar_feedback_threads SET status='open',updated_at=NOW() WHERE id=$1`)).
		WithArgs(int64(12)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, user_id, subject, status, created_at, updated_at FROM radar_feedback_threads WHERE user_id=$1 ORDER BY updated_at DESC, id DESC LIMIT $2 OFFSET $3`)).
		WithArgs(int64(7), 11, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "subject", "status", "created_at", "updated_at"}).AddRow(12, 7, "登录问题", "open", now, now))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id,author_type,author_id,content,created_at FROM radar_feedback_messages WHERE thread_id=$1 ORDER BY created_at ASC`)).
		WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "author_type", "author_id", "content", "created_at"}).AddRow(99, "user", 7, "续问", now))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id,filename,content_type,size_bytes,content FROM radar_feedback_attachments WHERE message_id=$1 ORDER BY id`)).
		WithArgs(int64(99)).WillReturnRows(sqlmock.NewRows([]string{"id", "filename", "content_type", "size_bytes", "content"}))

	thread, err := svc.ReplyFromUser(context.Background(), 12, 7, "续问", nil)
	require.NoError(t, err)
	require.Equal(t, int64(12), thread.ID)
	require.Equal(t, "open", thread.Status)
	require.Len(t, thread.Messages, 1)
	require.Empty(t, thread.Messages[0].Attachments)
	require.NoError(t, mock.ExpectationsWereMet())
}
