package reactionduel

import (
	"testing"
	"time"

	"github.com/jj/trivia/internal/engine"
)

func TestBuildPhasesUsesCountdownOptionAndRoundCount(t *testing.T) {
	t.Parallel()

	state := &engine.GameState{
		Players: map[engine.PlayerID]*engine.Player{
			"a": {ID: "a", Connected: true},
			"b": {ID: "b", Connected: true},
		},
		Settings: engine.GameSettings{
			GameID:     "reaction_duel",
			RoundCount: 2,
			GameOptions: map[string]any{
				"countdown_seconds": 5,
			},
		},
	}

	phases := BuildPhases(state)
	if len(phases) != 8 {
		t.Fatalf("want 8 phases for 2 rounds, got %d", len(phases))
	}
	if phases[0].Name != "reaction_countdown_r1" {
		t.Fatalf("unexpected first phase %q", phases[0].Name)
	}
	if phases[0].Duration != 5*time.Second {
		t.Fatalf("want countdown duration 5s, got %s", phases[0].Duration)
	}
	if phases[2].Name != "reaction_tap_r1" {
		t.Fatalf("unexpected tap phase name %q", phases[2].Name)
	}
	if phases[3].Name != "reaction_reveal_r1" {
		t.Fatalf("unexpected reveal phase name %q", phases[3].Name)
	}
}

func TestScoreRoundAwardsFastestLegalTaps(t *testing.T) {
	t.Parallel()

	base := time.Unix(100, 0)
	state := &engine.GameState{
		Players: map[engine.PlayerID]*engine.Player{
			"a": {ID: "a", Name: "Alice", Connected: true},
			"b": {ID: "b", Name: "Bob", Connected: true},
			"c": {ID: "c", Name: "Carol", Connected: true},
		},
		PhaseData: map[string]engine.PhaseResult{},
		Scores:    map[engine.PlayerID]int{},
	}
	engine.PutResult(state, "reaction_countdown_r1", "reactionduel.wait.r1", WaitResult{
		FalseStarts: map[engine.PlayerID]bool{"c": true},
	})
	engine.PutResult(state, "reaction_wait_r1", "reactionduel.wait.r1", WaitResult{
		FalseStarts: map[engine.PlayerID]bool{},
	})
	engine.PutResult(state, "reaction_tap_r1", "reactionduel.tap.r1", TapResult{
		Taps: map[engine.PlayerID]time.Time{
			"b": base.Add(20 * time.Millisecond),
			"a": base.Add(40 * time.Millisecond),
		},
	})

	results, scores, deltas := scoreRound(state, 1)
	if deltas["b"] != 1000 {
		t.Fatalf("expected Bob to get 1000, got %d", deltas["b"])
	}
	if deltas["a"] != 500 {
		t.Fatalf("expected Alice to get 500, got %d", deltas["a"])
	}
	if deltas["c"] != 0 {
		t.Fatalf("expected Carol false start to get 0, got %d", deltas["c"])
	}
	if scores["b"] != 1000 || scores["a"] != 500 {
		t.Fatalf("unexpected scores: %+v", scores)
	}

	foundCarol := false
	for _, result := range results {
		if result.PlayerID != "c" {
			continue
		}
		foundCarol = true
		if !result.FalseStart {
			t.Fatal("expected Carol false_start=true")
		}
	}
	if !foundCarol {
		t.Fatal("expected Carol in results")
	}
}
