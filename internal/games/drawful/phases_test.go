package drawful

import (
	"testing"
	"time"

	"github.com/jj/trivia/internal/engine"
)

func TestBuildPhasesUsesConfiguredTimers(t *testing.T) {
	state := &engine.GameState{
		Settings: engine.GameSettings{
			RoundCount:        2,
			DrawingSeconds:    45,
			FakePromptSeconds: 75,
			VotingSeconds:     30,
		},
	}

	phases := BuildPhases(state)
	if got, want := len(phases), 14; got != want {
		t.Fatalf("phase count: want %d, got %d", want, got)
	}

	check := map[string]time.Duration{
		"drawing_submit_r1":     45 * time.Second,
		"fake_prompt_submit_r1": 75 * time.Second,
		"voting_r1":             30 * time.Second,
		"drawing_submit_r2":     45 * time.Second,
		"fake_prompt_submit_r2": 75 * time.Second,
		"voting_r2":             30 * time.Second,
	}
	for _, phase := range phases {
		want, ok := check[phase.Name]
		if !ok {
			continue
		}
		if phase.Duration != want {
			t.Fatalf("%s duration: want %s, got %s", phase.Name, want, phase.Duration)
		}
	}
}
