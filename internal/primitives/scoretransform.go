package primitives

import (
	"fmt"

	"github.com/jj/trivia/internal/engine"
)

// ScoreTransformResult: per-player deltas this round. The caller mutates
// GameState.Scores based on this.
type ScoreTransformResult struct {
	Deltas map[engine.PlayerID]int
}

// ScoreTransform is a pure function over prior PhaseData that produces
// per-player score deltas. Advances immediately like Aggregate.
type ScoreTransform struct {
	name string
	fn   func(state *engine.GameState) (map[engine.PlayerID]int, []engine.Event, error)
}

func NewScoreTransform(
	name string,
	fn func(*engine.GameState) (map[engine.PlayerID]int, []engine.Event, error),
) *ScoreTransform {
	if fn == nil {
		panic("primitives: ScoreTransform requires a function")
	}
	return &ScoreTransform{name: name, fn: fn}
}

func (s *ScoreTransform) Name() string { return s.name }

func (s *ScoreTransform) Start(ctx *engine.PhaseContext) (engine.Decision[ScoreTransformResult], error) {
	deltas, events, err := s.fn(ctx.State)
	if err != nil {
		return engine.Decision[ScoreTransformResult]{}, err
	}
	// Apply deltas. Spec §8: "no negative scores."
	for id, delta := range deltas {
		ctx.State.Scores[id] += delta
		if ctx.State.Scores[id] < 0 {
			ctx.State.Scores[id] = 0
		}
	}
	return engine.Decision[ScoreTransformResult]{
		AdvancePhase: true,
		Broadcast:    events,
		Result:       ScoreTransformResult{Deltas: deltas},
	}, nil
}

func (s *ScoreTransform) Handle(*engine.PhaseContext, engine.Input) (engine.Decision[ScoreTransformResult], error) {
	return engine.Decision[ScoreTransformResult]{}, fmt.Errorf("%s: score-transform accepts no inputs", s.name)
}

func (s *ScoreTransform) Timeout(*engine.PhaseContext) (engine.Decision[ScoreTransformResult], error) {
	return engine.Decision[ScoreTransformResult]{AdvancePhase: true}, nil
}
