package drawful

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jj/trivia/internal/engine"
)

// Phase durations — §8.
const (
	DurationDrawing     = 60 * time.Second
	DurationFake        = 90 * time.Second
	DurationVote        = 20 * time.Second
	DurationLeaderboard = 0 * time.Second
	// prompt_distribute / reveal / scoring are Aggregates — Duration=0.
	// leaderboard is a Wait primitive; the room actor animates reveal steps
	// in parallel via its own stepTimer.
)

// phaseName suffixes round number so PhaseData keys stay unique across
// rounds. Example: "drawing_submit_r1".
func phaseName(kind string, round int) string {
	return fmt.Sprintf("%s_r%d", kind, round)
}

// BuildPhases constructs the full per-game phase sequence. Round count
// is ceil(players/2), min 3 (§17). Safe on zero-player state — returns
// an empty list, which the engine treats as "game ends immediately."
func BuildPhases(state *engine.GameState) []engine.Phase {
	playerCount := 0
	for _, p := range state.Players {
		if p.Connected {
			playerCount++
		}
	}
	rounds := (playerCount + 1) / 2
	if rounds < 3 {
		rounds = 3
	}
	if state.Settings.RoundCount > 0 {
		rounds = state.Settings.RoundCount
	}
	drawingDuration := DurationDrawing
	if state.Settings.DrawingSeconds > 0 {
		drawingDuration = time.Duration(state.Settings.DrawingSeconds) * time.Second
	}
	fakeDuration := DurationFake
	if state.Settings.FakePromptSeconds > 0 {
		fakeDuration = time.Duration(state.Settings.FakePromptSeconds) * time.Second
	}
	voteDuration := DurationVote
	if state.Settings.VotingSeconds > 0 {
		voteDuration = time.Duration(state.Settings.VotingSeconds) * time.Second
	}
	phases := make([]engine.Phase, 0, rounds*6)
	for r := 1; r <= rounds; r++ {
		config, _ := json.Marshal(map[string]int{"round": r})
		phases = append(phases,
			engine.Phase{
				Name:      phaseName("prompt_distribute", r),
				Primitive: regKeyPromptDistribute,
				Duration:  0,
				Config:    config,
			},
			engine.Phase{
				Name:      phaseName("drawing_submit", r),
				Primitive: regKeyDrawingCollect,
				Duration:  drawingDuration,
				Config:    config,
				DependsOn: []string{phaseName("prompt_distribute", r)},
			},
			engine.Phase{
				Name:      phaseName("fake_prompt_submit", r),
				Primitive: regKeyFakeCollect,
				Duration:  fakeDuration,
				Config:    config,
				DependsOn: []string{phaseName("drawing_submit", r)},
			},
			engine.Phase{
				Name:      phaseName("voting", r),
				Primitive: regKeyVoteCollect,
				Duration:  voteDuration,
				Config:    config,
				DependsOn: []string{phaseName("drawing_submit", r), phaseName("fake_prompt_submit", r)},
			},
			engine.Phase{
				Name:      phaseName("reveal", r),
				Primitive: regKeyReveal,
				Duration:  0,
				Config:    config,
				DependsOn: []string{phaseName("drawing_submit", r), phaseName("voting", r)},
			},
			engine.Phase{
				Name:      phaseName("scoring", r),
				Primitive: regKeyScoring,
				Duration:  0,
				Config:    config,
				DependsOn: []string{phaseName("reveal", r)},
			},
			engine.Phase{
				Name:      phaseName("leaderboard", r),
				Primitive: regKeyLeaderboard,
				Duration:  DurationLeaderboard,
				Config:    config,
				DependsOn: []string{phaseName("reveal", r), phaseName("scoring", r)},
			},
		)
	}
	return phases
}
