package drawful

import (
	"encoding/json"
	"sync"

	"github.com/jj/trivia/internal/engine"
)

// Registry keys. Kept as package-level consts so Phase configs and factories
// stay in sync (one place to rename).
const (
	regKeyPromptDistribute = "drawful.prompt_distribute"
	regKeyDrawingCollect   = "drawful.drawing_collect"
	regKeyFakeCollect      = "drawful.fake_collect"
	regKeyVoteCollect      = "drawful.vote_collect"
	regKeyReveal           = "drawful.reveal"
	regKeyScoring          = "drawful.scoring"
	regKeyLeaderboard      = "drawful.leaderboard"
)

var registerOnce sync.Once

// Register wires Jrawful primitives into the engine's factory registry.
// Idempotent; safe to call from tests and from main.
func Register() {
	registerOnce.Do(func() {
		engine.Register(regKeyPromptDistribute, func(_ json.RawMessage) (engine.AnyPrimitive, error) {
			return newPromptDistribute(), nil
		})
		engine.Register(regKeyDrawingCollect, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, _ := readRound(cfg)
			return newDrawingCollect(round), nil
		})
		engine.Register(regKeyFakeCollect, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, _ := readRound(cfg)
			return newFakeCollect(round), nil
		})
		engine.Register(regKeyVoteCollect, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, _ := readRound(cfg)
			return newVoteCollect(round), nil
		})
		engine.Register(regKeyReveal, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, _ := readRound(cfg)
			return newReveal(round), nil
		})
		engine.Register(regKeyScoring, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, _ := readRound(cfg)
			return newScoring(round), nil
		})
		engine.Register(regKeyLeaderboard, func(_ json.RawMessage) (engine.AnyPrimitive, error) {
			return newLeaderboard(), nil
		})
	})
}

func readRound(cfg json.RawMessage) (int, error) {
	var c struct {
		Round int `json:"round"`
	}
	if len(cfg) == 0 {
		return 1, nil
	}
	if err := json.Unmarshal(cfg, &c); err != nil {
		return 1, err
	}
	if c.Round < 1 {
		c.Round = 1
	}
	return c.Round, nil
}
