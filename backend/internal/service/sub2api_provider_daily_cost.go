package service

import (
	"context"
	"fmt"
	"time"
)

type ProviderDailyCost struct {
	Date       string    `json:"date"`
	Requests   int64     `json:"requests"`
	Successes  int64     `json:"successes"`
	Errors     int64     `json:"errors"`
	Tokens     int64     `json:"tokens"`
	Cost       float64   `json:"estimated_cost"`
	Revenue    float64   `json:"revenue"`
	Profit     float64   `json:"gross_profit"`
	P95        float64   `json:"p95_ms"`
	SampledAt  time.Time `json:"sampled_at"`
	CostSource string    `json:"cost_source"`
}

func (s *Sub2APIProviderProbeService) DailyCosts(ctx context.Context, id int64, days int) ([]ProviderDailyCost, error) {
	if days < 1 || days > 93 {
		return nil, invalidProviderProbeConfig("days must be between 1 and 93")
	}
	if _, err := s.providerRepo.GetByID(ctx, id); err != nil {
		return nil, ErrProviderNotFound
	}
	out := make([]ProviderDailyCost, 0)
	if s.db == nil {
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT stat_date::text,SUM(request_count),SUM(success_count),SUM(error_count),SUM(total_tokens),SUM(estimated_upstream_cost),SUM(local_revenue),SUM(gross_profit),MAX(p95_duration_ms),MAX(sampled_at) FROM provider_account_daily_cost_snapshots WHERE provider_id=$1 AND stat_date>=CURRENT_DATE-($2::int-1) GROUP BY stat_date ORDER BY stat_date`, id, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d ProviderDailyCost
		if err = rows.Scan(&d.Date, &d.Requests, &d.Successes, &d.Errors, &d.Tokens, &d.Cost, &d.Revenue, &d.Profit, &d.P95, &d.SampledAt); err != nil {
			return nil, err
		}
		d.CostSource = "local_estimate"
		out = append(out, d)
	}
	return out, rows.Err()
}

// RebuildDailyCosts only reads local raw records. End is exclusive; a maximum
// of 93 days keeps manual backfills bounded. Zero provider is runner-only.
func (s *Sub2APIProviderProbeService) RebuildDailyCosts(ctx context.Context, id int64, start, end time.Time) error {
	if !end.After(start) || end.Sub(start) > 94*24*time.Hour {
		return invalidProviderProbeConfig("rebuild range must be 1 to 93 days")
	}
	if s.db == nil {
		return fmt.Errorf("observability database unavailable")
	}
	q := `WITH events AS (
 SELECT a.provider_id,u.account_id,u.created_at,u.duration_ms::double precision AS latency,true AS success,
 COALESCE(u.input_tokens,0)::bigint AS input_tokens,COALESCE(u.output_tokens,0)::bigint AS output_tokens,
 COALESCE(u.cache_creation_tokens+u.cache_read_tokens,0)::bigint AS cache_tokens,
 COALESCE(u.account_stats_cost,u.total_cost) AS base_cost,
 COALESCE(u.account_stats_cost,u.total_cost)*COALESCE(u.account_rate_multiplier,1) AS cost,u.actual_cost AS revenue,u.account_rate_multiplier AS multiplier
 FROM usage_logs u JOIN accounts a ON a.id=u.account_id
 WHERE a.provider_id IS NOT NULL AND ($1::bigint=0 OR a.provider_id=$1) AND u.created_at >= $2 AND u.created_at < $3
 UNION ALL
 SELECT a.provider_id,o.account_id,MAX(o.created_at),MAX(COALESCE(o.response_latency_ms,o.upstream_latency_ms,o.duration_ms,0))::double precision,false,
 0::bigint,0::bigint,0::bigint,0,0,0,NULL
 FROM ops_error_logs o JOIN accounts a ON a.id=o.account_id
 WHERE a.provider_id IS NOT NULL AND ($1::bigint=0 OR a.provider_id=$1) AND o.created_at >= $2 AND o.created_at < $3
 AND ` + ProviderAttributableErrorSQL + `
 AND NOT EXISTS(SELECT 1 FROM usage_logs u WHERE u.account_id=o.account_id AND u.request_id=o.request_id AND u.created_at >= $2 AND u.created_at < $3)
 GROUP BY a.provider_id,o.account_id,COALESCE(NULLIF(o.request_id,''),o.id::text)
 ) INSERT INTO provider_account_daily_cost_snapshots(provider_id,account_id,stat_date,request_count,success_count,error_count,input_tokens,output_tokens,cache_tokens,total_tokens,estimated_upstream_cost,local_revenue,gross_profit,base_cost,base_cost_available,avg_duration_ms,p50_duration_ms,p95_duration_ms,multiplier_snapshot,cost_source,sampled_at,updated_at)
 SELECT provider_id,account_id,(created_at AT TIME ZONE current_setting('TIMEZONE'))::date,COUNT(*),COUNT(*) FILTER(WHERE success),COUNT(*) FILTER(WHERE NOT success),SUM(input_tokens),SUM(output_tokens),SUM(cache_tokens),SUM(input_tokens+output_tokens+cache_tokens),COALESCE(SUM(cost),0),COALESCE(SUM(revenue),0),COALESCE(SUM(revenue-cost),0),COALESCE(SUM(base_cost),0),TRUE,
 COALESCE(AVG(latency) FILTER(WHERE success),0),COALESCE(percentile_cont(0.50) WITHIN GROUP(ORDER BY latency) FILTER(WHERE success),0),COALESCE(percentile_cont(0.95) WITHIN GROUP(ORDER BY latency) FILTER(WHERE success),0),
 CASE WHEN COUNT(DISTINCT multiplier)=1 THEN MIN(multiplier) ELSE NULL END,'local_estimate',NOW(),NOW()
 FROM events GROUP BY provider_id,account_id,(created_at AT TIME ZONE current_setting('TIMEZONE'))::date
 ON CONFLICT(provider_id,account_id,stat_date) DO UPDATE SET request_count=EXCLUDED.request_count,success_count=EXCLUDED.success_count,error_count=EXCLUDED.error_count,input_tokens=EXCLUDED.input_tokens,output_tokens=EXCLUDED.output_tokens,cache_tokens=EXCLUDED.cache_tokens,total_tokens=EXCLUDED.total_tokens,estimated_upstream_cost=EXCLUDED.estimated_upstream_cost,local_revenue=EXCLUDED.local_revenue,gross_profit=EXCLUDED.gross_profit,base_cost=EXCLUDED.base_cost,base_cost_available=TRUE,avg_duration_ms=EXCLUDED.avg_duration_ms,p50_duration_ms=EXCLUDED.p50_duration_ms,p95_duration_ms=EXCLUDED.p95_duration_ms,multiplier_snapshot=EXCLUDED.multiplier_snapshot,cost_source=EXCLUDED.cost_source,sampled_at=EXCLUDED.sampled_at,updated_at=EXCLUDED.updated_at`
	_, err := s.db.ExecContext(ctx, q, id, start, end)
	return err
}
