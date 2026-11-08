//go:build integration

package service

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// Run only against an explicitly supplied local test database; all writes use a disposable schema.
func TestRadarFeedbackAdminInbox(t *testing.T) {
	dsn := os.Getenv("FEEDBACK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("FEEDBACK_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	schema := fmt.Sprintf("feedback_test_%d", time.Now().UnixNano())
	_, err = db.ExecContext(ctx, `CREATE SCHEMA `+schema)
	require.NoError(t, err)
	defer func() { _, _ = db.ExecContext(ctx, `DROP SCHEMA `+schema+` CASCADE`) }()
	parsed, err := url.Parse(dsn)
	require.NoError(t, err)
	q := parsed.Query()
	q.Set("search_path", schema)
	parsed.RawQuery = q.Encode()
	scoped, err := sql.Open("postgres", parsed.String())
	require.NoError(t, err)
	defer scoped.Close()
	_, err = scoped.ExecContext(ctx, `CREATE TABLE users(id BIGINT PRIMARY KEY,username TEXT,email TEXT);
        INSERT INTO users VALUES (1,'admin1','a1@example.test'),(2,'admin2','a2@example.test'),
        (7,'Alice','alice@example.test'),(8,'Bob','bob@example.test'),(9,'No messages','none@example.test')`)
	require.NoError(t, err)
	for _, name := range []string{"265_radar_feedback.sql", "267_feedback_admin_reads.sql"} {
		content, readErr := migrations.FS.ReadFile(name)
		require.NoError(t, readErr)
		_, err = scoped.ExecContext(ctx, string(content))
		require.NoError(t, err)
	}
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, scoped)))
	defer client.Close()
	svc := NewRadarFeedbackService(client)
	a, err := svc.Create(ctx, 7, "Alice first", "question A", nil)
	require.NoError(t, err)
	a2, err := svc.Create(ctx, 7, "Alice second", "question A2", nil)
	require.NoError(t, err)
	b, err := svc.Create(ctx, 8, "Bob first", "question B", nil)
	require.NoError(t, err)

	users, more, err := svc.AdminListUsers(ctx, 1, 20, 0)
	require.NoError(t, err)
	require.False(t, more)
	require.Len(t, users, 2, "do not list admins or users who have never sent a message")
	require.Equal(t, int64(8), users[0].UserID)
	require.Equal(t, int64(2), users[1].ThreadCount)
	require.Equal(t, int64(2), users[1].UnreadCount)

	// Reading details alone is side-effect free; acknowledgement records the displayed message only.
	detail, err := svc.AdminGetThread(ctx, 1, b.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), detail.UnreadCount)
	again, err := svc.AdminGetThread(ctx, 1, b.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), again.UnreadCount)
	require.NoError(t, svc.AdminMarkRead(ctx, 1, b.ID, detail.LastUserMessageID))
	users, _, err = svc.AdminListUsers(ctx, 1, 20, 0)
	require.NoError(t, err)
	require.Equal(t, int64(7), users[0].UserID, "unread sender ranks before a more recent read sender")
	require.Zero(t, users[1].UnreadCount)
	detail, err = svc.AdminGetThread(ctx, 1, b.ID)
	require.NoError(t, err)
	require.Zero(t, detail.UnreadCount)
	require.Equal(t, "open", detail.Status, "reading is not replying")
	detail2, err := svc.AdminGetThread(ctx, 2, b.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), detail2.UnreadCount, "read state is scoped to the administrator")

	// A later incoming message remains unread even if an older display is acknowledged concurrently.
	oldID := detail.LastUserMessageID
	_, err = svc.ReplyFromUser(ctx, b.ID, 8, "new message", nil)
	require.NoError(t, err)
	require.NoError(t, svc.AdminMarkRead(ctx, 1, b.ID, oldID))
	detail, err = svc.AdminGetThread(ctx, 1, b.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), detail.UnreadCount)
	require.NoError(t, svc.AdminMarkRead(ctx, 1, b.ID, detail.LastUserMessageID))
	require.NoError(t, svc.AdminMarkRead(ctx, 1, b.ID, oldID))
	detail, err = svc.AdminGetThread(ctx, 1, b.ID)
	require.NoError(t, err)
	require.Zero(t, detail.UnreadCount, "late acknowledgements must not move the watermark backwards")
	require.ErrorIs(t, svc.AdminMarkRead(ctx, 1, b.ID, a.Messages[0].ID), ErrRadarFeedbackInvalid)
	require.ErrorIs(t, svc.AdminMarkRead(ctx, 1, 999999, oldID), ErrRadarFeedbackThreadMissing)

	summaries, more, err := svc.AdminListUserThreads(ctx, 1, 7, "", 1, 0)
	require.NoError(t, err)
	require.True(t, more)
	require.Len(t, summaries, 1)
	require.Equal(t, a2.ID, summaries[0].ID)
	second, more, err := svc.AdminListUserThreads(ctx, 1, 7, "", 1, 1)
	require.NoError(t, err)
	require.False(t, more)
	require.Len(t, second, 1)
	require.Equal(t, a.ID, second[0].ID)
	_, err = svc.Reply(ctx, a.ID, 1, "answer", nil, true)
	require.NoError(t, err)
	summaries, _, err = svc.AdminListUserThreads(ctx, 1, 7, "closed", 20, 0)
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	require.Equal(t, a.ID, summaries[0].ID)
	require.NotNil(t, detail.Messages[0].Attachments)

	// More than the old ten-thread cap remains accessible through pages and direct detail reads.
	for i := 0; i < 20; i++ {
		_, err = svc.Create(ctx, 7, fmt.Sprintf("history %d", i), "content", nil)
		require.NoError(t, err)
	}
	summaries, more, err = svc.AdminListUserThreads(ctx, 1, 7, "", 20, 0)
	require.NoError(t, err)
	require.True(t, more)
	require.Len(t, summaries, 20)
	summaries, more, err = svc.AdminListUserThreads(ctx, 1, 7, "", 20, 20)
	require.NoError(t, err)
	require.False(t, more)
	require.Len(t, summaries, 2)
	oldest, err := svc.AdminGetThread(ctx, 1, a.ID)
	require.NoError(t, err)
	require.Len(t, oldest.Messages, 2)
}
