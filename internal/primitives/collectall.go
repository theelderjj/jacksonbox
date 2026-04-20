// Package primitives provides the composable phase primitives described in
// docs/DESIGN.md §2. Each primitive is generic over its result type so
// downstream phases read results without any-casting.
package primitives

import (
	"fmt"

	"github.com/jj/trivia/internal/engine"
)

// CollectAllResult is the terminal result of a CollectAll phase:
// one entry per player who submitted. Players absent from the map either
// timed out or were not eligible.
type CollectAllResult[R any] struct {
	Items map[engine.PlayerID]R
}

// CollectAllDecoder extracts and validates a single submission from a raw
// Input. Returning err rejects the submission (gateway has already done
// shape-level validation; this layer handles semantic rules).
type CollectAllDecoder[R any] func(in engine.Input, state *engine.GameState) (R, error)

// CollectAllEligible decides whether a player must/may submit. Players
// filtered out here are excluded from the "all submitted" count as well,
// so drawers (who don't write fakes for their own drawing) don't stall
// the phase.
type CollectAllEligible func(id engine.PlayerID, state *engine.GameState) bool

// CollectAllOnSubmit is an optional hook fired after a successful submission.
// Used by Jrawful to broadcast progress ticks ("3/5 submitted") without
// leaking content.
type CollectAllOnSubmit[R any] func(in engine.Input, item R, state *engine.GameState) []engine.Event

// CollectAll collects one input per eligible player until all have submitted
// or the phase deadline fires. Late submissions after advance are rejected
// by the phase-guard in the room actor (Input.Phase mismatches).
type CollectAll[R any] struct {
	name     string
	decode   CollectAllDecoder[R]
	eligible CollectAllEligible
	onSubmit CollectAllOnSubmit[R]
	items    map[engine.PlayerID]R
}

// NewCollectAll constructs a fresh collector. eligible may be nil (all
// connected players are eligible). onSubmit may be nil.
func NewCollectAll[R any](
	name string,
	decode CollectAllDecoder[R],
	eligible CollectAllEligible,
	onSubmit CollectAllOnSubmit[R],
) *CollectAll[R] {
	if decode == nil {
		panic("primitives: CollectAll requires a decoder")
	}
	if eligible == nil {
		eligible = func(engine.PlayerID, *engine.GameState) bool { return true }
	}
	return &CollectAll[R]{
		name:     name,
		decode:   decode,
		eligible: eligible,
		onSubmit: onSubmit,
		items:    make(map[engine.PlayerID]R),
	}
}

func (c *CollectAll[R]) Name() string { return c.name }

func (c *CollectAll[R]) Start(ctx *engine.PhaseContext) (engine.Decision[CollectAllResult[R]], error) {
	// If no eligible players exist, advance immediately — the phase is a no-op.
	if c.eligibleCount(ctx.State) == 0 {
		return engine.Decision[CollectAllResult[R]]{
			AdvancePhase: true,
			Result:       CollectAllResult[R]{Items: c.items},
		}, nil
	}
	return engine.Decision[CollectAllResult[R]]{}, nil
}

func (c *CollectAll[R]) Handle(ctx *engine.PhaseContext, in engine.Input) (engine.Decision[CollectAllResult[R]], error) {
	if !c.eligible(in.PlayerID, ctx.State) {
		return engine.Decision[CollectAllResult[R]]{}, fmt.Errorf("player %s not eligible for %s", in.PlayerID, c.name)
	}
	if _, already := c.items[in.PlayerID]; already {
		return engine.Decision[CollectAllResult[R]]{}, fmt.Errorf("player %s already submitted for %s", in.PlayerID, c.name)
	}
	item, err := c.decode(in, ctx.State)
	if err != nil {
		return engine.Decision[CollectAllResult[R]]{}, err
	}
	c.items[in.PlayerID] = item

	var events []engine.Event
	if c.onSubmit != nil {
		events = c.onSubmit(in, item, ctx.State)
	}

	if len(c.items) >= c.eligibleCount(ctx.State) {
		return engine.Decision[CollectAllResult[R]]{
			AdvancePhase: true,
			Broadcast:    events,
			Result:       CollectAllResult[R]{Items: c.items},
		}, nil
	}
	return engine.Decision[CollectAllResult[R]]{Broadcast: events}, nil
}

func (c *CollectAll[R]) Timeout(ctx *engine.PhaseContext) (engine.Decision[CollectAllResult[R]], error) {
	// Timeout always advances with whatever we have. Default substitutions
	// for missing submissions are a game-specific concern, applied by the
	// next phase (see drawful.applyDefaultFakes / applyDefaultVotes).
	return engine.Decision[CollectAllResult[R]]{
		AdvancePhase: true,
		Result:       CollectAllResult[R]{Items: c.items},
	}, nil
}

func (c *CollectAll[R]) eligibleCount(state *engine.GameState) int {
	n := 0
	for id, p := range state.Players {
		if p.Connected && c.eligible(id, state) {
			n++
		}
	}
	return n
}
