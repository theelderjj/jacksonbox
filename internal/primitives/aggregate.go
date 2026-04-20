package primitives

import (
	"fmt"

	"github.com/jj/trivia/internal/engine"
)

// Aggregate is a pure function over prior PhaseData. It consumes no inputs;
// Start runs the function and advances immediately. Use this for reveal,
// tallying, and derivations that don't require player interaction.
type Aggregate[R any] struct {
	name string
	fn   func(state *engine.GameState) (R, []engine.Event, error)
}

func NewAggregate[R any](
	name string,
	fn func(*engine.GameState) (R, []engine.Event, error),
) *Aggregate[R] {
	if fn == nil {
		panic("primitives: Aggregate requires a function")
	}
	return &Aggregate[R]{name: name, fn: fn}
}

func (a *Aggregate[R]) Name() string { return a.name }

func (a *Aggregate[R]) Start(ctx *engine.PhaseContext) (engine.Decision[R], error) {
	v, events, err := a.fn(ctx.State)
	if err != nil {
		return engine.Decision[R]{}, err
	}
	return engine.Decision[R]{AdvancePhase: true, Broadcast: events, Result: v}, nil
}

func (a *Aggregate[R]) Handle(*engine.PhaseContext, engine.Input) (engine.Decision[R], error) {
	return engine.Decision[R]{}, fmt.Errorf("%s: aggregate accepts no inputs", a.name)
}

func (a *Aggregate[R]) Timeout(*engine.PhaseContext) (engine.Decision[R], error) {
	// If we get here, Start's AdvancePhase was ignored — that's a bug.
	return engine.Decision[R]{AdvancePhase: true}, nil
}
