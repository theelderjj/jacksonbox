package primitives

import (
	"fmt"

	"github.com/jj/trivia/internal/engine"
)

// TurnBasedResult holds what each player did during their turn. Absent
// players timed out without a valid input.
type TurnBasedResult[R any] struct {
	ByPlayer map[engine.PlayerID]R
	Order    []engine.PlayerID
}

// TurnBased routes inputs only from the current player. Rotates on valid
// input or per-turn timeout. Phase advances when the queue drains.
// Not used by Jrawful but kept for future game modes (e.g., a Codenames-like
// clue giver).
type TurnBased[R any] struct {
	name    string
	decode  func(engine.Input, *engine.GameState) (R, error)
	order   []engine.PlayerID
	results map[engine.PlayerID]R
	cursor  int
}

func NewTurnBased[R any](
	name string,
	order []engine.PlayerID,
	decode func(engine.Input, *engine.GameState) (R, error),
) *TurnBased[R] {
	if decode == nil {
		panic("primitives: TurnBased requires a decoder")
	}
	return &TurnBased[R]{
		name:    name,
		order:   append([]engine.PlayerID(nil), order...),
		results: make(map[engine.PlayerID]R),
		decode:  decode,
	}
}

func (t *TurnBased[R]) Name() string { return t.name }

func (t *TurnBased[R]) Start(*engine.PhaseContext) (engine.Decision[TurnBasedResult[R]], error) {
	if len(t.order) == 0 {
		return t.done(), nil
	}
	return engine.Decision[TurnBasedResult[R]]{}, nil
}

func (t *TurnBased[R]) Handle(ctx *engine.PhaseContext, in engine.Input) (engine.Decision[TurnBasedResult[R]], error) {
	if t.cursor >= len(t.order) {
		return engine.Decision[TurnBasedResult[R]]{}, fmt.Errorf("no active turn")
	}
	current := t.order[t.cursor]
	if in.PlayerID != current {
		return engine.Decision[TurnBasedResult[R]]{}, fmt.Errorf("not %s's turn", in.PlayerID)
	}
	v, err := t.decode(in, ctx.State)
	if err != nil {
		return engine.Decision[TurnBasedResult[R]]{}, err
	}
	t.results[current] = v
	t.cursor++
	if t.cursor >= len(t.order) {
		return t.done(), nil
	}
	return engine.Decision[TurnBasedResult[R]]{}, nil
}

func (t *TurnBased[R]) Timeout(*engine.PhaseContext) (engine.Decision[TurnBasedResult[R]], error) {
	// Turn-level timeout: skip the current player. Phase deadline is per-turn
	// at the engine level; when the actor fires Timeout for this primitive it
	// means "move on." We advance the whole phase rather than a single turn,
	// matching the simpler spec in §2 where phase deadline = hard cap.
	return t.done(), nil
}

func (t *TurnBased[R]) done() engine.Decision[TurnBasedResult[R]] {
	return engine.Decision[TurnBasedResult[R]]{
		AdvancePhase: true,
		Result:       TurnBasedResult[R]{ByPlayer: t.results, Order: t.order},
	}
}
