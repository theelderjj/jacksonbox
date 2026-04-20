// Package reveal holds the tiny data structures shared between the room
// actor (which emits the per-drawing reveal animation queue) and the game
// modes (which build the queue from stored reveal results). Kept as a
// standalone leaf package so neither side has to depend on the other — the
// room package was originally the home of these types, but putting it here
// breaks the room <-> games/<mode> import cycle that showed up as soon as
// the room integration tests needed to import a game mode (see
// internal/room/integration_test.go).
package reveal

import (
	"time"

	"github.com/jj/trivia/internal/proto"
)

// StepHold is retained for compatibility with older reveal pacing code.
// Leader-driven reveal no longer depends on time-based inter-card gaps, but
// tests and older callers may still reference the symbol.
var StepHold = 1200 * time.Millisecond

// Step is one emission in the per-round reveal animation queue.
//
//	Kind "eliminate" — carries ChoiceID; greys out a fake choice client-side.
//	Kind "final"     — carries Deltas + Awards; flips is_final=true on wire.
//	Kind "gap"       — pure pause between drawings; no broadcast.
//
// Duration overrides the default interval for the NEXT step (0 = use the
// room's configured interval). Primarily used by "gap" to hold longer.
type Step struct {
	Kind      string // "eliminate" | "final" | "gap"
	DrawingID string
	ChoiceID  string
	Deltas    map[string]int
	Awards    []proto.RevealAward
	Duration  time.Duration
}
