//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type smartGroupCreateAPIKeyRepoStub struct {
	APIKeyRepository
	created *APIKey
}

func (s *smartGroupCreateAPIKeyRepoStub) Create(_ context.Context, key *APIKey) error {
	clone := *key
	clone.SmartGroupIDs = append([]int64(nil), key.SmartGroupIDs...)
	s.created = &clone
	return nil
}

type smartGroupCreateUserRepoStub struct {
	UserRepository
	user *User
}

func (s *smartGroupCreateUserRepoStub) GetByID(_ context.Context, _ int64) (*User, error) {
	clone := *s.user
	return &clone, nil
}

type smartGroupCreateGroupRepoStub struct {
	GroupRepository
	groups map[int64]Group
}

func (s *smartGroupCreateGroupRepoStub) GetByID(_ context.Context, id int64) (*Group, error) {
	group, ok := s.groups[id]
	if !ok {
		return nil, ErrGroupNotFound
	}
	clone := group
	return &clone, nil
}

type smartGroupCreateRateRepoStub struct {
	UserGroupRateRepository
	rates map[int64]float64
	err   error
}

func (s *smartGroupCreateRateRepoStub) GetByUserID(_ context.Context, _ int64) (map[int64]float64, error) {
	return s.rates, s.err
}

type smartGroupUpdateAPIKeyRepoStub struct {
	APIKeyRepository
	key     *APIKey
	updated *APIKey
	fields  APIKeyUpdateFields
}

func (s *smartGroupUpdateAPIKeyRepoStub) GetByID(_ context.Context, _ int64) (*APIKey, error) {
	clone := *s.key
	clone.SmartGroupIDs = append([]int64(nil), s.key.SmartGroupIDs...)
	return &clone, nil
}

func (s *smartGroupUpdateAPIKeyRepoStub) Update(_ context.Context, key *APIKey, fields APIKeyUpdateFields) error {
	clone := *key
	clone.SmartGroupIDs = append([]int64(nil), key.SmartGroupIDs...)
	s.updated = &clone
	s.fields = fields
	return nil
}

func TestOrderedSmartGroupProbeCandidatesFailureUsesLowToHighOrder(t *testing.T) {
	candidates := []runtimeSmartGroupCandidate{
		{group: Group{ID: 1}, rate: 0.08},
		{group: Group{ID: 2}, rate: 0.15},
		{group: Group{ID: 3}, rate: 0.30},
	}

	ordered := orderedSmartGroupProbeCandidates(candidates, 2, SmartGroupSwitchReasonFailure)
	// Failure failover advances from the failed route and wraps at the end;
	// it must not immediately retry the cheapest route that just failed.
	require.Equal(t, []int64{3, 1}, []int64{ordered[0].group.ID, ordered[1].group.ID})
}

func TestOrderedSmartGroupProbeCandidatesFailureHandlesUnavailableCurrentGroup(t *testing.T) {
	candidates := []runtimeSmartGroupCandidate{
		{group: Group{ID: 1}, rate: 0.08},
		{group: Group{ID: 2}, rate: 0.15},
	}

	ordered := orderedSmartGroupProbeCandidates(candidates, 99, SmartGroupSwitchReasonFailure)
	require.Equal(t, []int64{1, 2}, []int64{ordered[0].group.ID, ordered[1].group.ID})
	ordered = orderedSmartGroupProbeCandidates(candidates, 99, SmartGroupSwitchReasonCostRecovery)
	require.Equal(t, []int64{1, 2}, []int64{ordered[0].group.ID, ordered[1].group.ID})
}

func TestOrderedSmartGroupFailureCandidatesAdvanceAndWrap(t *testing.T) {
	candidates := []runtimeSmartGroupCandidate{
		{group: Group{ID: 1}, rate: 0.08},
		{group: Group{ID: 2}, rate: 0.15},
		{group: Group{ID: 3}, rate: 0.30},
	}
	ordered := orderedSmartGroupProbeCandidates(candidates, 1, SmartGroupSwitchReasonFailure)
	require.Equal(t, []int64{2, 3}, []int64{ordered[0].group.ID, ordered[1].group.ID})
	ordered = orderedSmartGroupProbeCandidates(candidates, 3, SmartGroupSwitchReasonFailure)
	require.Equal(t, []int64{1, 2}, []int64{ordered[0].group.ID, ordered[1].group.ID})
}

func TestOrderedSmartGroupProbeCandidatesCostRecoveryOnlyUsesCheaperGroups(t *testing.T) {
	candidates := []runtimeSmartGroupCandidate{
		{group: Group{ID: 1}, rate: 0.08},
		{group: Group{ID: 2}, rate: 0.15},
		{group: Group{ID: 3}, rate: 0.30},
	}

	ordered := orderedSmartGroupProbeCandidates(candidates, 3, SmartGroupSwitchReasonCostRecovery)
	require.Equal(t, []int64{1, 2}, []int64{ordered[0].group.ID, ordered[1].group.ID})
	require.Empty(t, orderedSmartGroupProbeCandidates(candidates, 1, SmartGroupSwitchReasonCostRecovery))
}

func TestAPIKeySmartGroupServiceShouldObserveSuccess(t *testing.T) {
	service := &APIKeySmartGroupService{}
	now := time.Now()
	groupID := int64(7)
	key := &APIKey{
		GroupID:                           &groupID,
		SmartGroupEnabled:                 true,
		SmartGroupIDs:                     []int64{7, 8},
		SmartGroupRecoveryIntervalSeconds: 900,
	}

	require.True(t, service.ShouldObserveSuccess(key, now), "first healthy result initializes the stable window")
	healthySince := now.Add(-10 * time.Minute)
	key.SmartGroupHealthySince = &healthySince
	require.False(t, service.ShouldObserveSuccess(key, now))
	key.SmartGroupConsecutiveFailures = 1
	require.True(t, service.ShouldObserveSuccess(key, now), "success after failure must reset the failure counter")
	key.SmartGroupConsecutiveFailures = 0
	healthySince = now.Add(-15 * time.Minute)
	key.SmartGroupHealthySince = &healthySince
	require.True(t, service.ShouldObserveSuccess(key, now), "stable window expiry triggers a cost probe")
}

func TestAPIKeyCreateSmartGroupUsesLowestEffectiveRateAsInitialGroup(t *testing.T) {
	keyRepo := &smartGroupCreateAPIKeyRepoStub{}
	groups := map[int64]Group{
		10: {ID: 10, Platform: PlatformOpenAI, RateMultiplier: 0.08, Status: StatusActive},
		20: {ID: 20, Platform: PlatformOpenAI, RateMultiplier: 0.15, Status: StatusActive},
	}
	svc := NewAPIKeyService(
		keyRepo,
		&smartGroupCreateUserRepoStub{user: &User{ID: 7, Status: StatusActive}},
		&smartGroupCreateGroupRepoStub{groups: groups},
		nil,
		&smartGroupCreateRateRepoStub{rates: map[int64]float64{20: 0.05}},
		nil,
		&config.Config{Default: config.DefaultConfig{APIKeyPrefix: "sk-"}},
	)
	requestedGroupID := int64(10)

	created, err := svc.Create(context.Background(), 7, CreateAPIKeyRequest{
		Name:                              "smart",
		GroupID:                           &requestedGroupID,
		SmartGroupEnabled:                 true,
		SmartGroupIDs:                     []int64{10, 20},
		SmartGroupFailureThreshold:        3,
		SmartGroupRecoveryIntervalSeconds: 900,
	})

	require.NoError(t, err)
	require.NotNil(t, created.GroupID)
	require.Equal(t, int64(20), *created.GroupID)
	require.Equal(t, []int64{20, 10}, created.SmartGroupIDs)
	require.NotNil(t, keyRepo.created)
	require.Equal(t, int64(20), *keyRepo.created.GroupID)
}

func TestAPIKeyUpdateSmartGroupReordersCandidatesWithoutProbing(t *testing.T) {
	currentGroupID := int64(10)
	keyRepo := &smartGroupUpdateAPIKeyRepoStub{key: &APIKey{
		ID:                                9,
		UserID:                            7,
		Key:                               "sk-smart",
		GroupID:                           &currentGroupID,
		Status:                            StatusActive,
		SmartGroupEnabled:                 true,
		SmartGroupIDs:                     []int64{10, 30},
		SmartGroupFailureThreshold:        3,
		SmartGroupRecoveryIntervalSeconds: 900,
		SmartGroupConsecutiveFailures:     2,
		SmartGroupLastError:               "old failure",
		SmartGroupLastSwitchReason:        SmartGroupSwitchReasonFailure,
	}}
	groups := map[int64]Group{
		10: {ID: 10, Platform: PlatformOpenAI, RateMultiplier: 0.08, Status: StatusActive},
		20: {ID: 20, Platform: PlatformOpenAI, RateMultiplier: 0.15, Status: StatusActive},
		30: {ID: 30, Platform: PlatformOpenAI, RateMultiplier: 0.30, Status: StatusActive},
	}
	svc := NewAPIKeyService(
		keyRepo,
		&smartGroupCreateUserRepoStub{user: &User{ID: 7, Status: StatusActive}},
		&smartGroupCreateGroupRepoStub{groups: groups},
		nil,
		&smartGroupCreateRateRepoStub{rates: map[int64]float64{20: 0.05}},
		nil,
		&config.Config{},
	)
	enabled := true
	groupIDs := []int64{30, 10, 20}
	preferredGroupID := int64(20)

	updated, err := svc.Update(context.Background(), 9, 7, UpdateAPIKeyRequest{
		SmartGroupEnabled: &enabled,
		SmartGroupIDs:     &groupIDs,
		GroupID:           &preferredGroupID,
	})

	require.NoError(t, err)
	require.Equal(t, []int64{20, 10, 30}, updated.SmartGroupIDs)
	require.NotNil(t, updated.GroupID)
	require.Equal(t, int64(20), *updated.GroupID, "configuration updates make the newly sorted candidate #1 active without probing")
	require.True(t, keyRepo.fields.SmartGroupConfig)
	require.True(t, keyRepo.fields.GroupID)
}

func TestAPIKeyUpdateSmartGroupDoesNotProbeWhenCurrentGroupIsCheapest(t *testing.T) {
	currentGroupID := int64(10)
	keyRepo := &smartGroupUpdateAPIKeyRepoStub{key: &APIKey{
		ID:                                9,
		UserID:                            7,
		Key:                               "sk-smart",
		GroupID:                           &currentGroupID,
		Status:                            StatusActive,
		SmartGroupEnabled:                 true,
		SmartGroupIDs:                     []int64{10, 20},
		SmartGroupFailureThreshold:        3,
		SmartGroupRecoveryIntervalSeconds: 900,
	}}
	svc := NewAPIKeyService(
		keyRepo,
		&smartGroupCreateUserRepoStub{user: &User{ID: 7, Status: StatusActive}},
		&smartGroupCreateGroupRepoStub{groups: map[int64]Group{
			10: {ID: 10, Platform: PlatformOpenAI, RateMultiplier: 0.08, Status: StatusActive},
			20: {ID: 20, Platform: PlatformOpenAI, RateMultiplier: 0.15, Status: StatusActive},
		}},
		nil,
		&smartGroupCreateRateRepoStub{},
		nil,
		&config.Config{},
	)
	groupIDs := []int64{20, 10}

	_, err := svc.Update(context.Background(), 9, 7, UpdateAPIKeyRequest{SmartGroupIDs: &groupIDs})

	require.NoError(t, err)
}

func TestAPIKeyUpdateSmartGroupAllowsRemovingCurrentRouteWithoutProbing(t *testing.T) {
	currentGroupID := int64(10)
	keyRepo := &smartGroupUpdateAPIKeyRepoStub{key: &APIKey{
		ID:                                9,
		UserID:                            7,
		Key:                               "sk-smart",
		GroupID:                           &currentGroupID,
		Status:                            StatusActive,
		SmartGroupEnabled:                 true,
		SmartGroupIDs:                     []int64{10, 30},
		SmartGroupFailureThreshold:        3,
		SmartGroupRecoveryIntervalSeconds: 900,
	}}
	svc := NewAPIKeyService(
		keyRepo,
		&smartGroupCreateUserRepoStub{user: &User{ID: 7, Status: StatusActive}},
		&smartGroupCreateGroupRepoStub{groups: map[int64]Group{
			10: {ID: 10, Platform: PlatformOpenAI, RateMultiplier: 1, Status: StatusActive},
			20: {ID: 20, Platform: PlatformOpenAI, RateMultiplier: 0.1, Status: StatusActive},
			30: {ID: 30, Platform: PlatformOpenAI, RateMultiplier: 0.2, Status: StatusActive},
		}},
		nil,
		&smartGroupCreateRateRepoStub{},
		nil,
		&config.Config{},
	)
	groupIDs := []int64{20, 30}

	updated, err := svc.Update(context.Background(), 9, 7, UpdateAPIKeyRequest{
		SmartGroupIDs: &groupIDs,
	})

	require.NoError(t, err)
	require.Equal(t, []int64{20, 30}, updated.SmartGroupIDs)
	require.NotNil(t, updated.GroupID)
	require.Equal(t, int64(20), *updated.GroupID, "removing the old route makes candidate #1 active without probing")
}

func TestAPIKeyUpdateRejectsDirectSmartGroupRouteSwitch(t *testing.T) {
	currentGroupID := int64(10)
	requestedGroupID := int64(20)
	keyRepo := &smartGroupUpdateAPIKeyRepoStub{key: &APIKey{
		ID:                                9,
		UserID:                            7,
		Key:                               "sk-smart",
		GroupID:                           &currentGroupID,
		Status:                            StatusActive,
		SmartGroupEnabled:                 true,
		SmartGroupIDs:                     []int64{10, 20},
		SmartGroupFailureThreshold:        3,
		SmartGroupRecoveryIntervalSeconds: 900,
	}}
	svc := NewAPIKeyService(
		keyRepo,
		&smartGroupCreateUserRepoStub{user: &User{ID: 7, Status: StatusActive}},
		&smartGroupCreateGroupRepoStub{groups: map[int64]Group{
			10: {ID: 10, Platform: PlatformOpenAI, RateMultiplier: 0.15, Status: StatusActive},
			20: {ID: 20, Platform: PlatformOpenAI, RateMultiplier: 0.08, Status: StatusActive},
		}},
		nil,
		&smartGroupCreateRateRepoStub{},
		nil,
		&config.Config{},
	)

	_, err := svc.Update(context.Background(), 9, 7, UpdateAPIKeyRequest{GroupID: &requestedGroupID})

	require.ErrorIs(t, err, ErrSmartGroupManualSwitchForbidden)
	require.Nil(t, keyRepo.updated)
}

func TestNormalizeSmartGroupCandidatesRejectsCrossPlatformGroups(t *testing.T) {
	svc := &APIKeyService{
		groupRepo: &smartGroupCreateGroupRepoStub{groups: map[int64]Group{
			10: {ID: 10, Platform: PlatformOpenAI, RateMultiplier: 0.08, Status: StatusActive},
			20: {ID: 20, Platform: PlatformAnthropic, RateMultiplier: 0.05, Status: StatusActive},
		}},
	}

	_, err := svc.normalizeSmartGroupCandidates(
		context.Background(),
		&User{ID: 7, Status: StatusActive},
		[]int64{10, 20},
	)

	require.ErrorIs(t, err, ErrSmartGroupPlatformMismatch)
}

func TestNormalizeSmartGroupCandidatesFailsClosedWhenUserRatesCannotBeLoaded(t *testing.T) {
	svc := &APIKeyService{
		groupRepo: &smartGroupCreateGroupRepoStub{groups: map[int64]Group{
			10: {ID: 10, Platform: PlatformOpenAI, RateMultiplier: 0.08, Status: StatusActive},
			20: {ID: 20, Platform: PlatformOpenAI, RateMultiplier: 0.15, Status: StatusActive},
		}},
		userGroupRateRepo: &smartGroupCreateRateRepoStub{err: errors.New("rate store unavailable")},
	}

	_, err := svc.normalizeSmartGroupCandidates(
		context.Background(),
		&User{ID: 7, Status: StatusActive},
		[]int64{10, 20},
	)

	require.ErrorContains(t, err, "load user group rates")
}
