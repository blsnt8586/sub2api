package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	dbent "github.com/Wei-Shaw/sub2api/ent"
)

const TokenParkCampaignVersion = "v1"

var TokenParkGames = []string{"miner", "ocean", "tower"}

var (
	ErrTokenParkDisabled       = errors.New("token park rewards are disabled")
	ErrTokenParkNotEligible    = errors.New("token park requires cumulative account credits to reach the minimum")
	ErrTokenParkInvalidGame    = errors.New("invalid token park game")
	ErrTokenParkIncomplete     = errors.New("game completion proof is incomplete")
	ErrTokenParkAlreadyClaimed = errors.New("token park reward already claimed")
)

type TokenParkGameStatus struct {
	Claimed   bool    `json:"claimed"`
	AmountUSD float64 `json:"amount_usd"`
}

type TokenParkStatus struct {
	Eligible      bool                           `json:"eligible"`
	RewardEnabled bool                           `json:"reward_enabled"`
	BalanceUSD    float64                        `json:"balance_usd"`
	CreditedUSD   float64                        `json:"credited_usd"`
	ThresholdUSD  float64                        `json:"threshold_usd"`
	Rewards       map[string]TokenParkGameStatus `json:"rewards"`
}

type TokenParkProof struct {
	Level int `json:"level"`
}

type TokenParkProgress struct {
	GameID          string `json:"game_id"`
	CampaignVersion string `json:"campaign_version"`
	State           string `json:"state"`
	UpdatedAt       string `json:"updated_at"`
}

type TokenParkLeaderboardEntry struct {
	Rank            int     `json:"rank"`
	UserID          int64   `json:"user_id,omitempty"`
	DisplayName     string  `json:"display_name"`
	Email           string  `json:"email,omitempty"`
	GamesCompleted  int     `json:"games_completed"`
	TotalRewardUSD  float64 `json:"total_reward_usd"`
	LastCompletedAt string  `json:"last_completed_at"`
	IsCurrentUser   bool    `json:"is_current_user"`
}

type TokenParkLeaderboard struct {
	Entries      []TokenParkLeaderboardEntry `json:"entries"`
	MyRank       int                         `json:"my_rank"`
	TotalPlayers int                         `json:"total_players"`
}

type TokenParkRewardService struct {
	entClient              *dbent.Client
	settingService         *SettingService
	paymentService         *PaymentService
	billingCacheService    *BillingCacheService
	apiKeyCacheInvalidator APIKeyAuthCacheInvalidator
}

func NewTokenParkRewardService(entClient *dbent.Client, settingService *SettingService, paymentService *PaymentService, billingCacheService *BillingCacheService, apiKeyCacheInvalidator APIKeyAuthCacheInvalidator) *TokenParkRewardService {
	return &TokenParkRewardService{entClient: entClient, settingService: settingService, paymentService: paymentService, billingCacheService: billingCacheService, apiKeyCacheInvalidator: apiKeyCacheInvalidator}
}

func (s *TokenParkRewardService) settings(ctx context.Context) (*SystemSettings, error) {
	if s == nil || s.settingService == nil {
		return nil, errors.New("token park service is not ready")
	}
	return s.settingService.GetAllSettings(ctx)
}

func (s *TokenParkRewardService) Status(ctx context.Context, userID int64) (*TokenParkStatus, error) {
	if userID <= 0 || s == nil || s.entClient == nil {
		return nil, errors.New("invalid token park request")
	}
	settings, err := s.settings(ctx)
	if err != nil {
		return nil, err
	}
	currentBalance, credited, err := s.currentUserFunding(ctx, userID)
	if err != nil {
		return nil, err
	}
	status := &TokenParkStatus{RewardEnabled: settings.TokenParkRewardEnabled, BalanceUSD: currentBalance, CreditedUSD: credited, ThresholdUSD: settings.TokenParkMinBalanceUSD, Rewards: map[string]TokenParkGameStatus{}}
	// Unlocking is permanent after the account has received the threshold amount.
	// Current balance is displayed for account context but must not revoke access.
	status.Eligible = status.RewardEnabled && tokenParkUnlocked(credited, settings.TokenParkMinBalanceUSD)
	if status.Eligible {
		for _, game := range TokenParkGames {
			status.Rewards[game] = TokenParkGameStatus{AmountUSD: settings.TokenParkGameRewardsUSD[game]}
		}
	}
	rows, err := s.entClient.QueryContext(ctx, `SELECT game_id FROM token_park_rewards WHERE user_id = $1 AND campaign_version = $2`, userID, TokenParkCampaignVersion)
	if err != nil {
		return nil, fmt.Errorf("query token park rewards: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var game string
		if err := rows.Scan(&game); err != nil {
			return nil, err
		}
		if item, ok := status.Rewards[game]; ok {
			item.Claimed = true
			status.Rewards[game] = item
		}
	}
	return status, rows.Err()
}

func (s *TokenParkRewardService) requireEligible(ctx context.Context, userID int64) error {
	settings, err := s.settings(ctx)
	if err != nil {
		return err
	}
	if !settings.TokenParkRewardEnabled {
		return ErrTokenParkDisabled
	}
	_, credited, err := s.currentUserFunding(ctx, userID)
	if err != nil {
		return err
	}
	if !tokenParkUnlocked(credited, settings.TokenParkMinBalanceUSD) {
		return ErrTokenParkNotEligible
	}
	return nil
}

func (s *TokenParkRewardService) currentUserFunding(ctx context.Context, userID int64) (balance, credited float64, err error) {
	rows, err := s.entClient.QueryContext(ctx, `SELECT balance, total_recharged FROM users WHERE id = $1 AND deleted_at IS NULL`, userID)
	if err != nil {
		return 0, 0, fmt.Errorf("query token park user funding: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return 0, 0, fmt.Errorf("read token park user funding: %w", err)
		}
		return 0, 0, errors.New("token park user not found")
	}
	if err := rows.Scan(&balance, &credited); err != nil {
		return 0, 0, fmt.Errorf("scan token park user funding: %w", err)
	}
	return balance, credited, rows.Err()
}

func (s *TokenParkRewardService) Leaderboard(ctx context.Context, userID int64) (*TokenParkLeaderboard, error) {
	if userID <= 0 || s == nil || s.entClient == nil {
		return nil, errors.New("invalid token park request")
	}
	if err := s.requireEligible(ctx, userID); err != nil {
		return nil, err
	}
	return s.queryLeaderboard(ctx, userID, false)
}

func (s *TokenParkRewardService) AdminLeaderboard(ctx context.Context) (*TokenParkLeaderboard, error) {
	if s == nil || s.entClient == nil {
		return nil, errors.New("token park service is not ready")
	}
	return s.queryLeaderboard(ctx, 0, true)
}

func (s *TokenParkRewardService) queryLeaderboard(ctx context.Context, currentUserID int64, includeEmail bool) (*TokenParkLeaderboard, error) {
	rows, err := s.entClient.QueryContext(ctx, `
		WITH ranked AS (
			SELECT
				r.user_id,
				COUNT(*)::int AS games_completed,
				COALESCE(SUM(r.amount), 0)::float8 AS total_reward_usd,
				MAX(r.created_at)::text AS last_completed_at,
				RANK() OVER (ORDER BY COUNT(*) DESC, MAX(r.created_at) ASC, r.user_id ASC)::int AS rank
			FROM token_park_rewards r
			JOIN users u ON u.id = r.user_id AND u.deleted_at IS NULL
			WHERE r.campaign_version = $1
			GROUP BY r.user_id
		)
		SELECT ranked.rank, ranked.user_id, u.email, ranked.games_completed,
		       ranked.total_reward_usd, ranked.last_completed_at
		FROM ranked
		JOIN users u ON u.id = ranked.user_id
		ORDER BY ranked.rank ASC, ranked.user_id ASC
	`, TokenParkCampaignVersion)
	if err != nil {
		return nil, fmt.Errorf("query token park leaderboard: %w", err)
	}
	defer rows.Close()
	result := &TokenParkLeaderboard{Entries: make([]TokenParkLeaderboardEntry, 0)}
	for rows.Next() {
		var entry TokenParkLeaderboardEntry
		if err := rows.Scan(&entry.Rank, &entry.UserID, &entry.Email, &entry.GamesCompleted, &entry.TotalRewardUSD, &entry.LastCompletedAt); err != nil {
			return nil, fmt.Errorf("scan token park leaderboard: %w", err)
		}
		entry.IsCurrentUser = entry.UserID == currentUserID
		if !includeEmail {
			if entry.IsCurrentUser {
				entry.DisplayName = entry.Email
			} else {
				entry.DisplayName = anonymousTokenParkName(entry.UserID)
			}
			entry.Email = ""
			entry.UserID = 0
		} else {
			entry.DisplayName = entry.Email
		}
		if entry.IsCurrentUser && result.MyRank == 0 {
			result.MyRank = entry.Rank
		}
		result.Entries = append(result.Entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read token park leaderboard: %w", err)
	}
	result.TotalPlayers = len(result.Entries)
	return result, nil
}

func anonymousTokenParkName(userID int64) string {
	digest := sha256.Sum256([]byte(strconv.FormatInt(userID, 10)))
	return "匿名账户-" + fmt.Sprintf("%x", digest[:4])
}

func (s *TokenParkRewardService) Claim(ctx context.Context, userID int64, game string, proof TokenParkProof) (float64, float64, error) {
	if userID <= 0 || s == nil || s.entClient == nil {
		return 0, 0, errors.New("invalid token park request")
	}
	game = strings.TrimSpace(game)
	if !containsTokenParkGame(game) {
		return 0, 0, ErrTokenParkInvalidGame
	}
	settings, err := s.settings(ctx)
	if err != nil {
		return 0, 0, err
	}
	if !settings.TokenParkRewardEnabled {
		return 0, 0, ErrTokenParkDisabled
	}
	if !proofComplete(game, proof) {
		return 0, 0, ErrTokenParkIncomplete
	}
	if err := s.requireStoredCompletion(ctx, userID, game); err != nil {
		return 0, 0, err
	}
	amount := settings.TokenParkGameRewardsUSD[game]
	if amount <= 0 {
		return 0, 0, ErrTokenParkDisabled
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()
	// Recheck cumulative credited balance under the same transaction that records
	// the reward. Spending the balance after unlocking never revokes eligibility.
	rows, err := tx.Client().QueryContext(ctx, `SELECT balance, total_recharged FROM users WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, userID)
	if err != nil {
		return 0, 0, fmt.Errorf("lock token park user balance: %w", err)
	}
	if !rows.Next() {
		rows.Close()
		if err := rows.Err(); err != nil {
			return 0, 0, fmt.Errorf("read token park user balance: %w", err)
		}
		return 0, 0, errors.New("token park user not found")
	}
	var currentBalance, credited float64
	if err := rows.Scan(&currentBalance, &credited); err != nil {
		rows.Close()
		return 0, 0, fmt.Errorf("scan token park user balance: %w", err)
	}
	rows.Close()
	if !tokenParkUnlocked(credited, settings.TokenParkMinBalanceUSD) {
		return 0, 0, ErrTokenParkNotEligible
	}
	rows, err = tx.Client().QueryContext(ctx, `INSERT INTO token_park_rewards (user_id, game_id, campaign_version, amount) VALUES ($1,$2,$3,$4) ON CONFLICT (user_id, game_id, campaign_version) DO NOTHING RETURNING id`, userID, game, TokenParkCampaignVersion, amount)
	if err != nil {
		return 0, 0, fmt.Errorf("record token park reward: %w", err)
	}
	if !rows.Next() {
		_ = rows.Close()
		return 0, 0, ErrTokenParkAlreadyClaimed
	}
	var rewardID int64
	if err := rows.Scan(&rewardID); err != nil {
		_ = rows.Close()
		return 0, 0, err
	}
	_ = rows.Close()
	var balance float64
	rows, err = tx.Client().QueryContext(ctx, `UPDATE users SET balance = balance + $1, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL RETURNING balance`, amount, userID)
	if err != nil {
		_ = rows.Close()
		return 0, 0, fmt.Errorf("credit token park reward: %w", err)
	}
	if !rows.Next() {
		_ = rows.Close()
		return 0, 0, errors.New("credit token park reward: user not found")
	}
	if err := rows.Scan(&balance); err != nil {
		_ = rows.Close()
		return 0, 0, err
	}
	_ = rows.Close()
	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	if s.apiKeyCacheInvalidator != nil {
		s.apiKeyCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCacheService != nil {
		_ = s.billingCacheService.InvalidateUserBalance(ctx, userID)
	}
	return amount, balance, nil
}

func tokenParkUnlocked(totalCredited, threshold float64) bool {
	return totalCredited >= threshold
}

func (s *TokenParkRewardService) requireStoredCompletion(ctx context.Context, userID int64, game string) error {
	var raw []byte
	rows, err := s.entClient.QueryContext(ctx, `SELECT state FROM token_park_progress WHERE user_id = $1 AND game_id = $2 AND campaign_version = $3`, userID, game, TokenParkCampaignVersion)
	if err != nil {
		return ErrTokenParkIncomplete
	}
	defer rows.Close()
	if !rows.Next() || rows.Scan(&raw) != nil {
		return ErrTokenParkIncomplete
	}
	var state struct {
		Level   int    `json:"level"`
		Cleared bool   `json:"cleared"`
		Phase   string `json:"phase"`
		Nested  *struct {
			Phase string `json:"phase"`
		} `json:"state"`
	}
	if json.Unmarshal(raw, &state) != nil {
		return ErrTokenParkIncomplete
	}
	switch game {
	case "miner":
		if state.Level < 99 || !state.Cleared {
			return ErrTokenParkIncomplete
		}
	case "ocean":
		if state.Level < 59 || !state.Cleared {
			return ErrTokenParkIncomplete
		}
	case "tower":
		phase := state.Phase
		if phase == "" && state.Nested != nil {
			phase = state.Nested.Phase
		}
		if phase != "won" {
			return ErrTokenParkIncomplete
		}
	default:
		return ErrTokenParkIncomplete
	}
	return nil
}

func (s *TokenParkRewardService) LoadProgress(ctx context.Context, userID int64, game string) (*TokenParkProgress, error) {
	if userID <= 0 || s == nil || s.entClient == nil || !containsTokenParkGame(game) {
		return nil, errors.New("invalid token park progress request")
	}
	if err := s.requireEligible(ctx, userID); err != nil {
		return nil, err
	}
	var state string
	var updatedAt string
	rows, err := s.entClient.QueryContext(ctx, `
		SELECT state::text, updated_at::text
		FROM token_park_progress
		WHERE user_id = $1 AND game_id = $2 AND campaign_version = $3
	`, userID, strings.TrimSpace(game), TokenParkCampaignVersion)
	if err != nil {
		return nil, fmt.Errorf("load token park progress: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("read token park progress: %w", err)
		}
		return nil, nil
	}
	if err := rows.Scan(&state, &updatedAt); err != nil {
		return nil, fmt.Errorf("scan token park progress: %w", err)
	}
	return &TokenParkProgress{GameID: strings.TrimSpace(game), CampaignVersion: TokenParkCampaignVersion, State: state, UpdatedAt: updatedAt}, rows.Err()
}

func (s *TokenParkRewardService) SaveProgress(ctx context.Context, userID int64, game, state string) (*TokenParkProgress, error) {
	if userID <= 0 || s == nil || s.entClient == nil || !containsTokenParkGame(game) {
		return nil, errors.New("invalid token park progress request")
	}
	if err := s.requireEligible(ctx, userID); err != nil {
		return nil, err
	}
	game = strings.TrimSpace(game)
	state = strings.TrimSpace(state)
	if state == "" || len(state) > 512*1024 {
		return nil, errors.New("invalid token park progress state")
	}
	if isTokenParkCompletion(game, state) {
		previous, err := s.previousProgressState(ctx, userID, game)
		if err != nil || !completionFollowsProgress(game, previous, state) {
			return nil, ErrTokenParkIncomplete
		}
	}
	var updatedAt string
	rows, err := s.entClient.QueryContext(ctx, `
		INSERT INTO token_park_progress (user_id, game_id, campaign_version, state)
		VALUES ($1, $2, $3, $4::jsonb)
		ON CONFLICT (user_id, game_id, campaign_version)
		DO UPDATE SET state = EXCLUDED.state, updated_at = NOW()
		RETURNING updated_at::text
	`, userID, game, TokenParkCampaignVersion, state)
	if err != nil {
		return nil, fmt.Errorf("save token park progress: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("save token park progress: %w", err)
		}
		return nil, errors.New("save token park progress: no row returned")
	}
	if err := rows.Scan(&updatedAt); err != nil {
		return nil, fmt.Errorf("scan token park progress: %w", err)
	}
	return &TokenParkProgress{GameID: game, CampaignVersion: TokenParkCampaignVersion, State: state, UpdatedAt: updatedAt}, nil
}

func (s *TokenParkRewardService) previousProgressState(ctx context.Context, userID int64, game string) (string, error) {
	var state string
	rows, err := s.entClient.QueryContext(ctx, `SELECT state::text FROM token_park_progress WHERE user_id = $1 AND game_id = $2 AND campaign_version = $3`, userID, game, TokenParkCampaignVersion)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return "", err
		}
		return "", nil
	}
	if err := rows.Scan(&state); err != nil {
		return "", err
	}
	return state, rows.Err()
}

func isTokenParkCompletion(game, raw string) bool {
	var state struct {
		Level   int    `json:"level"`
		Cleared bool   `json:"cleared"`
		Phase   string `json:"phase"`
		Nested  *struct {
			Phase string `json:"phase"`
		} `json:"state"`
	}
	if json.Unmarshal([]byte(raw), &state) != nil {
		return false
	}
	switch game {
	case "miner":
		return state.Level >= 99 && state.Cleared
	case "ocean":
		return state.Level >= 59 && state.Cleared
	case "tower":
		phase := state.Phase
		if phase == "" && state.Nested != nil {
			phase = state.Nested.Phase
		}
		return phase == "won"
	default:
		return false
	}
}

func completionFollowsProgress(game, previous, completion string) bool {
	if previous == "" {
		return false
	}
	var state struct {
		Level   int    `json:"level"`
		Cleared bool   `json:"cleared"`
		Floor   int    `json:"floor"`
		Phase   string `json:"phase"`
		Nested  *struct {
			Floor int    `json:"floor"`
			Phase string `json:"phase"`
		} `json:"state"`
	}
	if json.Unmarshal([]byte(previous), &state) != nil {
		return false
	}
	switch game {
	case "miner":
		return state.Level >= 99 && !state.Cleared
	case "ocean":
		return state.Level >= 59 && !state.Cleared
	case "tower":
		return state.Nested != nil && state.Nested.Floor >= 99 && state.Nested.Phase != "won"
	default:
		return false
	}
}

func (s *TokenParkRewardService) DeleteProgress(ctx context.Context, userID int64, game string) error {
	if userID <= 0 || s == nil || s.entClient == nil || !containsTokenParkGame(game) {
		return errors.New("invalid token park progress request")
	}
	if err := s.requireEligible(ctx, userID); err != nil {
		return err
	}
	_, err := s.entClient.ExecContext(ctx, `DELETE FROM token_park_progress WHERE user_id = $1 AND game_id = $2 AND campaign_version = $3`, userID, strings.TrimSpace(game), TokenParkCampaignVersion)
	if err != nil {
		return fmt.Errorf("delete token park progress: %w", err)
	}
	return nil
}

func containsTokenParkGame(game string) bool {
	for _, candidate := range TokenParkGames {
		if candidate == game {
			return true
		}
	}
	return false
}
func proofComplete(game string, proof TokenParkProof) bool {
	switch game {
	case "miner", "tower":
		return proof.Level >= 100
	case "ocean":
		return proof.Level >= 60
	}
	return false
}
