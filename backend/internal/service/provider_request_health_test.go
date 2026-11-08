//go:build unit

package service

import (
	"context"
	"database/sql"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/patrickmn/go-cache"

	"github.com/stretchr/testify/require"
)

func TestProviderHealthFailureClassification(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", want: false},
		{name: "client cancellation", err: context.Canceled, want: false},
		{name: "request timeout", err: context.DeadlineExceeded, want: true},
		{name: "upstream server error", err: &UpstreamFailoverError{StatusCode: 502}, want: true},
		{name: "client error", err: &UpstreamFailoverError{StatusCode: 400}, want: false},
		{name: "account capability mismatch is not an outage", err: newOpenAIResponsesToolsRequireResponsesFailoverError("gpt-6.1-sol"), want: false},
		{name: "rate limit", err: &UpstreamFailoverError{StatusCode: 429}, want: false},
		{name: "request scoped capacity", err: &UpstreamFailoverError{StatusCode: 529, RequestScopedTransient: true}, want: false},
		{name: "provider scoped error", err: &UpstreamFailoverError{StatusCode: 502, Scope: GatewayFailureScopeProvider}, want: false},
		{name: "request scoped error", err: &UpstreamFailoverError{StatusCode: 502, Scope: GatewayFailureScopeRequest}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, providerHealthFailure(tc.err))
		})
	}
}

type providerHealthRecordingCache struct {
	events         []ProviderRequestHealthEvent
	avoidBySession map[string][]ProviderSessionAvoidance
}

func (c *providerHealthRecordingCache) ObserveProviderRequestHealth(_ context.Context, e ProviderRequestHealthEvent) error {
	c.events = append(c.events, e)
	return nil
}
func (c *providerHealthRecordingCache) ProviderSessionAvoidances(_ context.Context, session string, _ time.Time) ([]ProviderSessionAvoidance, error) {
	return c.avoidBySession[session], nil
}

func TestProviderHealthSchedulerSwitchesDegradedStickyAccountOnlyForAffectedSession(t *testing.T) {
	for _, advanced := range []bool{false, true} {
		t.Run("advanced="+strconv.FormatBool(advanced), func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
			groupID, providerID := int64(18), int64(1)
			primary := &Account{ID: 9, ProviderID: &providerID, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 10, Priority: 0, GroupIDs: []int64{groupID}}
			backup := &Account{ID: 10, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Status: StatusActive, Schedulable: true, Concurrency: 10, Priority: 5, GroupIDs: []int64{groupID}}
			cacheState := &providerHealthRecordingCache{avoidBySession: map[string][]ProviderSessionAvoidance{providerSessionKey(1, groupID, "slow-session"): {{AccountID: 9, Generation: "g1"}}}}
			health := &ProviderRequestHealthService{db: &sql.DB{}, cache: cacheState, policies: cache.New(time.Minute, time.Minute)}
			health.policies.SetDefault("9", providerRuntimePolicy{Policy: DefaultProviderAccountHealthPolicy(), Generation: "g1", Since: time.Now().Add(-time.Hour)})
			rate := newOpenAIAdvancedSchedulerRateLimitService(strconv.FormatBool(advanced))
			rate.providerHealth = health
			bindingCache := &schedulerTestGatewayCache{sessionBindings: map[string]int64{"openai:slow-session": 9, "openai:normal-session": 9}}
			svc := &OpenAIGatewayService{
				accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{*primary, *backup}},
				cache:       bindingCache, cfg: &config.Config{}, rateLimitService: rate,
				schedulerSnapshot:  &SchedulerSnapshotService{cache: &openAISnapshotCacheStub{snapshotAccounts: []*Account{primary, backup}, accountsByID: map[int64]*Account{9: primary, 10: backup}}},
				concurrencyService: NewConcurrencyService(schedulerTestConcurrencyCache{}),
			}
			for _, test := range []struct {
				user    int64
				session string
				want    int64
			}{{1, "slow-session", 10}, {1, "normal-session", 9}, {2, "slow-session", 9}} {
				ctx := WithProviderRequestHealth(context.WithValue(context.Background(), ctxkey.UserID, test.user))
				selection, _, err := svc.SelectAccountWithScheduler(ctx, &groupID, "", test.session, "gpt-5.1", nil, OpenAIUpstreamTransportAny, false)
				require.NoError(t, err)
				require.Equal(t, test.want, selection.Account.ID)
				if selection.ReleaseFunc != nil {
					selection.ReleaseFunc()
				}
				// Each user may choose the same client session string. Restore the
				// primary sticky mapping before the next independent user's request.
				bindingCache.sessionBindings["openai:slow-session"] = 9
			}
		})
	}
}

func TestProviderHealthUsesFirstTokenNotTotalGenerationTime(t *testing.T) {
	recorder := &providerHealthRecordingCache{}
	health := &ProviderRequestHealthService{db: &sql.DB{}, cache: recorder, policies: cache.New(time.Minute, time.Minute)}
	health.policies.SetDefault("9", providerRuntimePolicy{Policy: DefaultProviderAccountHealthPolicy(), Generation: "g1", Since: time.Now().Add(-time.Hour)})
	ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(1))
	providerID := int64(18)
	account := &Account{ID: 9, ProviderID: &providerID}
	for _, ttft := range []int{1000, 32000} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		result := &OpenAIForwardResult{Stream: true, FirstTokenMs: &ttft, Duration: 2 * time.Minute, UpstreamTerminalEvent: "response.completed"}
		gateway := &OpenAIGatewayService{rateLimitService: &RateLimitService{providerHealth: health}}
		gateway.ObserveProviderRequestHealth(ctx, c, account, nil, "s", time.Now(), result, nil)
	}
	require.Len(t, recorder.events, 2)
	require.False(t, recorder.events[0].Degraded)
	require.False(t, recorder.events[0].Failed)
	require.True(t, recorder.events[1].Degraded)
	require.False(t, recorder.events[1].Failed)
}
func (*providerHealthRecordingCache) ProviderRecentFailures(context.Context, int64, string, time.Time, int) (int, error) {
	return 0, nil
}
func (*providerHealthRecordingCache) ProviderRecentDegradations(context.Context, int64, string, time.Time, int) (int, error) {
	return 0, nil
}

func TestProviderHealthRecordsSlowSuccessSeparatelyFromTimeout(t *testing.T) {
	for _, tc := range []struct {
		name              string
		latency           int
		completed, stream bool
		err               error
		wantEvents        int
		failed, degraded  bool
	}{
		{"fast", 1000, true, true, nil, 1, false, false},
		{"at degraded threshold", 25000, true, true, nil, 1, false, false},
		{"slow successful stream", 32000, true, true, nil, 1, false, true},
		{"at timeout threshold", 60000, true, true, nil, 1, false, true},
		{"over timeout successful stream", 62000, true, true, nil, 1, true, false},
		{"non-stream duration", 32000, true, false, nil, 1, false, true},
		{"upstream error", 32000, false, true, &UpstreamFailoverError{StatusCode: 502}, 1, true, false},
		{"client error ignored", 32000, false, true, &UpstreamFailoverError{StatusCode: 400}, 0, false, false},
		{"cancellation ignored", 32000, false, true, context.Canceled, 0, false, false},
		{"incomplete stream ignored", 32000, false, true, nil, 0, false, false},
		{"unknown TTFT ignored", -1, true, true, nil, 0, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &providerHealthRecordingCache{}
			s := &ProviderRequestHealthService{db: &sql.DB{}, cache: recorder, policies: cache.New(time.Minute, time.Minute)}
			s.policies.SetDefault("9", providerRuntimePolicy{Policy: DefaultProviderAccountHealthPolicy(), Generation: "g1", Since: time.Now().Add(-time.Hour)})
			ctx := context.WithValue(context.Background(), ctxkey.UserID, int64(1))
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			providerID := int64(18)
			var latency *int
			if tc.latency >= 0 {
				latency = &tc.latency
			}
			s.Observe(ctx, c, &Account{ID: 9, ProviderID: &providerID}, 5, "conversation", time.Now(), tc.completed, tc.stream, latency, tc.err)
			require.Len(t, recorder.events, tc.wantEvents)
			if tc.wantEvents > 0 {
				require.Equal(t, tc.failed, recorder.events[0].Failed)
				require.Equal(t, tc.degraded, recorder.events[0].Degraded)
			}
		})
	}
}
