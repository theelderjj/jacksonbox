package drawful

import (
	"github.com/jj/trivia/internal/engine"
	"github.com/jj/trivia/internal/primitives"
)

// newLeaderboard builds the per-round display phase that sits between scoring
// and the next round's prompt_distribute. The primitive itself is a pure
// Wait — the room actor drives the reveal-step animation (S2CRevealStep
// broadcasts) in parallel via its own stepTimer, reading RevealResult from
// PhaseData. This keeps scoring correctness isolated from animation timing
// and lets pause/resume freeze both clocks uniformly.
func newLeaderboard() engine.AnyPrimitive {
	return engine.Erase(primitives.NewWait("leaderboard"))
}
