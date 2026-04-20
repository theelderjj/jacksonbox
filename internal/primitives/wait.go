package primitives

import (
	"fmt"

	"github.com/jj/trivia/internal/engine"
)

// WaitResult is an empty sentinel — Wait has nothing to store under PhaseData.
// Kept around only so we can satisfy the generic Primitive[R] contract.
type WaitResult struct{}

func (WaitResult) isPhaseResult() {}

// Wait is a pure timed phase that accepts no inputs and advances on Timeout.
// Use for "hold on this screen for N seconds" phases (leaderboard display,
// intermissions) where the room actor drives side-channel behavior (e.g.,
// the reveal step animator) in parallel with the phase clock.
//
// Rationale for a separate primitive instead of reusing Aggregate: Aggregate
// always advances at Start, which would make the Duration meaningless. Wait
// explicitly models "do nothing, just burn time."
type Wait struct {
	name string
}

func NewWait(name string) *Wait {
	return &Wait{name: name}
}

func (w *Wait) Name() string { return w.name }

func (w *Wait) Start(*engine.PhaseContext) (engine.Decision[WaitResult], error) {
	// Do not advance; the phase timer will fire Timeout when the Duration
	// elapses. No events — the room actor is responsible for any side-channel
	// broadcasts (e.g., reveal step animation) during the wait.
	return engine.Decision[WaitResult]{AdvancePhase: false}, nil
}

func (w *Wait) Handle(*engine.PhaseContext, engine.Input) (engine.Decision[WaitResult], error) {
	return engine.Decision[WaitResult]{}, fmt.Errorf("%s: wait accepts no inputs", w.name)
}

func (w *Wait) Timeout(*engine.PhaseContext) (engine.Decision[WaitResult], error) {
	return engine.Decision[WaitResult]{AdvancePhase: true, Result: WaitResult{}}, nil
}
