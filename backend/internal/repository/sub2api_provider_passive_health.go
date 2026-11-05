package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ApplyProviderPassiveCooldown atomically preserves administrator-owned state
// and writes the scheduler outbox in the same statement. Only passive-owned
// cooldowns may be extended; no recovery path clears other owners' blocks.
func (r *sub2APIAccountRepository) ApplyProviderPassiveCooldown(ctx context.Context, providerID, accountID int64, now, until time.Time, reason string) error {
	_, err := r.sql.ExecContext(ctx, `WITH changed AS (
 UPDATE accounts SET temp_unschedulable_until=$1,temp_unschedulable_reason=$2,updated_at=NOW()
 WHERE id=$3 AND provider_id=$4 AND deleted_at IS NULL AND status='active' AND schedulable=true
 AND (temp_unschedulable_until IS NULL OR temp_unschedulable_reason LIKE 'passive health:%')
 AND (temp_unschedulable_until IS NULL OR temp_unschedulable_until<$1)
 AND (rate_limit_reset_at IS NULL OR rate_limit_reset_at<=$5)
 AND (overload_until IS NULL OR overload_until<=$5)
 AND (auto_pause_on_expired=false OR expires_at IS NULL OR expires_at>$5)
 RETURNING id)
 INSERT INTO scheduler_outbox(event_type,account_id) SELECT $6,id FROM changed`, until, reason, accountID, providerID, now, service.SchedulerOutboxEventAccountChanged)
	return err
}
