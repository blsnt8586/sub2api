package service

import "testing"

func TestProofCompleteRequiresFinalGameMilestones(t *testing.T) {
	tests := []struct {
		game  string
		proof TokenParkProof
		want  bool
	}{
		{"miner", TokenParkProof{Level: 99}, false},
		{"miner", TokenParkProof{Level: 100}, true},
		{"ocean", TokenParkProof{Level: 60}, true},
		{"tower", TokenParkProof{Level: 100}, true},
		{"2048", TokenParkProof{Level: 100}, false},
		{"fishing", TokenParkProof{Level: 100}, false},
	}
	for _, test := range tests {
		if got := proofComplete(test.game, test.proof); got != test.want {
			t.Errorf("proofComplete(%q, %#v) = %v, want %v", test.game, test.proof, got, test.want)
		}
	}
}

func TestTokenParkUnlockUsesCumulativeCredits(t *testing.T) {
	if tokenParkUnlocked(19.99, 20) {
		t.Fatal("account below the threshold must remain locked")
	}
	if !tokenParkUnlocked(20, 20) || !tokenParkUnlocked(21, 20) {
		t.Fatal("account reaching the threshold must stay unlocked")
	}
}

func TestTokenParkCompletionRequiresPriorProgress(t *testing.T) {
	minerDone := `{"level":99,"cleared":true}`
	if completionFollowsProgress("miner", "", minerDone) || !completionFollowsProgress("miner", `{"level":99,"cleared":false}`, minerDone) {
		t.Fatal("miner completion must follow the final unfinished level")
	}
	if completionFollowsProgress("ocean", `{"level":58,"cleared":false}`, `{"level":59,"cleared":true}`) {
		t.Fatal("ocean completion must not skip the final round")
	}
	if !isTokenParkCompletion("tower", `{"state":{"phase":"won"}}`) {
		t.Fatal("tower completion marker should be recognized")
	}
}

func TestContainsTokenParkGameRejectsUnknownGames(t *testing.T) {
	if len(TokenParkGames) != 3 || TokenParkGames[0] != "miner" || TokenParkGames[1] != "ocean" || TokenParkGames[2] != "tower" {
		t.Fatalf("unexpected token park games: %#v", TokenParkGames)
	}
	if containsTokenParkGame("admin") {
		t.Fatal("unknown game must not be claimable")
	}
	if !containsTokenParkGame("miner") {
		t.Fatal("miner should be claimable")
	}
	if containsTokenParkGame("2048") || containsTokenParkGame("fishing") {
		t.Fatal("retired games must not be claimable")
	}
}

func TestParseTokenParkMinBalanceUsesCurrentBalanceSetting(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]string
		want     float64
	}{
		{
			name:     "current balance setting",
			settings: map[string]string{SettingKeyTokenParkMinBalanceUSD: "75"},
			want:     75,
		},
		{
			name:     "legacy setting fallback",
			settings: map[string]string{SettingKeyTokenParkMinPaidRechargeCNY: "80"},
			want:     80,
		},
		{
			name: "current setting wins over legacy setting",
			settings: map[string]string{
				SettingKeyTokenParkMinBalanceUSD:      "60",
				SettingKeyTokenParkMinPaidRechargeCNY: "90",
			},
			want: 60,
		},
		{
			name:     "invalid settings use default",
			settings: map[string]string{SettingKeyTokenParkMinBalanceUSD: "-1"},
			want:     20,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := parseTokenParkMinBalance(test.settings); got != test.want {
				t.Fatalf("parseTokenParkMinBalance(%v) = %v, want %v", test.settings, got, test.want)
			}
		})
	}
}
