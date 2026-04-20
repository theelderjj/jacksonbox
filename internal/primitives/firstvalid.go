package primitives

import (
	"fmt"

	"github.com/jj/trivia/internal/engine"
)

// FirstValidResult: who won and what they submitted. Empty if Timeout fired
// before any valid input.
type FirstValidResult[R any] struct {
	WinnerID engine.PlayerID
	Value    R
	Won      bool
}

// FirstValid accepts the first valid input and rejects the rest. Advances
// on first accept or on timeout. Not used by Jrawful but registered for
// reuse by future game modes.
type FirstValid[R any] struct {
	name     string
	decode   func(in engine.Input, state *engine.GameState) (R, error)
	eligible func(id engine.PlayerID, state *engine.GameState) bool
	decided  bool
}

func NewFirstValid[R any](
	name string,
	decode func(engine.Input, *engine.GameState) (R, error),
	eligible func(engine.PlayerID, *engine.GameState) bool,
) *FirstValid[R] {
	if decode == nil {
		panic("primitives: FirstValid requires a decoder")
	}
	if eligible == nil {
		eligible = func(engine.PlayerID, *engine.GameState) bool { return true }
	}
	return &FirstValid[R]{name: name, decode: decode, eligible: eligible}
}

func (f *FirstValid[R]) Name() string { return f.name }

func (f *FirstValid[R]) Start(*engine.PhaseContext) (engine.Decision[FirstValidResult[R]], error) {
	return engine.Decision[FirstValidResult[R]]{}, nil
}

func (f *FirstValid[R]) Handle(ctx *engine.PhaseContext, in engine.Input) (engine.Decision[FirstValidResult[R]], error) {
	if f.decided {
		return engine.Decision[FirstValidResult[R]]{}, fmt.Errorf("already decided")
	}
	if !f.eligible(in.PlayerID, ctx.State) {
		return engine.Decision[FirstValidResult[R]]{}, fmt.Errorf("not eligible")
	}
	v, err := f.decode(in, ctx.State)
	if err != nil {
		return engine.Decision[FirstValidResult[R]]{}, err
	}
	f.decided = true
	return engine.Decision[FirstValidResult[R]]{
		AdvancePhase: true,
		Result:       FirstValidResult[R]{WinnerID: in.PlayerID, Value: v, Won: true},
	}, nil
}

func (f *FirstValid[R]) Timeout(*engine.PhaseContext) (engine.Decision[FirstValidResult[R]], error) {
	return engine.Decision[FirstValidResult[R]]{
		AdvancePhase: true,
		Result:       FirstValidResult[R]{Won: false},
	}, nil
}
