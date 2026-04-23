package reactionduel

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/jj/trivia/internal/engine"
	"github.com/jj/trivia/internal/proto"
)

const (
	regKeyWait   = "reactionduel.wait"
	regKeyTap    = "reactionduel.tap"
	regKeyReveal = "reactionduel.reveal"

	defaultCountdownSeconds = 3
	defaultTapSeconds       = 4
	revealSeconds           = 4
)

type phaseConfig struct {
	Round int `json:"round"`
}

type WaitResult struct {
	FalseStarts map[engine.PlayerID]bool
}

type TapResult struct {
	Taps map[engine.PlayerID]time.Time
}

type PlayerResult struct {
	PlayerID   string `json:"player_id"`
	PlayerName string `json:"player_name"`
	FalseStart bool   `json:"false_start"`
	ReactionMs int    `json:"reaction_ms,omitempty"`
	Rank       int    `json:"rank,omitempty"`
	Points     int    `json:"points"`
}

type RoundResult struct {
	Round   int            `json:"round"`
	Results []PlayerResult `json:"results"`
}

var registerOnce sync.Once

func Register() {
	registerOnce.Do(func() {
		engine.Register(regKeyWait, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, err := readRound(cfg)
			if err != nil {
				return nil, err
			}
			return engine.Erase(newWait(round)), nil
		})
		engine.Register(regKeyTap, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, err := readRound(cfg)
			if err != nil {
				return nil, err
			}
			return engine.Erase(newTap(round)), nil
		})
		engine.Register(regKeyReveal, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, err := readRound(cfg)
			if err != nil {
				return nil, err
			}
			return engine.Erase(newReveal(round)), nil
		})
	})
}

func BuildPhases(state *engine.GameState) []engine.Phase {
	rounds := state.Settings.RoundCount
	if rounds <= 0 {
		rounds = 3
	}
	countdownSeconds := defaultCountdownSeconds
	if raw, ok := intOption(state.Settings.GameOptions, "countdown_seconds"); ok && raw >= 1 && raw <= 10 {
		countdownSeconds = raw
	}
	phases := make([]engine.Phase, 0, rounds*4)
	for round := 1; round <= rounds; round++ {
		cfg, _ := json.Marshal(phaseConfig{Round: round})
		waitSeconds := 1 + rand.Intn(8)
		phases = append(phases,
			engine.Phase{
				Name:      fmt.Sprintf("reaction_countdown_r%d", round),
				Primitive: regKeyWait,
				Duration:  time.Duration(countdownSeconds) * time.Second,
				Config:    cfg,
			},
			engine.Phase{
				Name:      fmt.Sprintf("reaction_wait_r%d", round),
				Primitive: regKeyWait,
				Duration:  time.Duration(waitSeconds) * time.Second,
				Config:    cfg,
				DependsOn: []string{fmt.Sprintf("reaction_countdown_r%d", round)},
			},
			engine.Phase{
				Name:      fmt.Sprintf("reaction_tap_r%d", round),
				Primitive: regKeyTap,
				Duration:  defaultTapSeconds * time.Second,
				Config:    cfg,
				DependsOn: []string{fmt.Sprintf("reaction_countdown_r%d", round), fmt.Sprintf("reaction_wait_r%d", round)},
			},
			engine.Phase{
				Name:      fmt.Sprintf("reaction_reveal_r%d", round),
				Primitive: regKeyReveal,
				Duration:  revealSeconds * time.Second,
				Config:    cfg,
				DependsOn: []string{fmt.Sprintf("reaction_countdown_r%d", round), fmt.Sprintf("reaction_wait_r%d", round), fmt.Sprintf("reaction_tap_r%d", round)},
			},
		)
	}
	return phases
}

type waitPrimitive struct {
	name        string
	falseStarts map[engine.PlayerID]bool
}

func newWait(round int) *waitPrimitive {
	return &waitPrimitive{
		name:        fmt.Sprintf("reactionduel.wait.r%d", round),
		falseStarts: map[engine.PlayerID]bool{},
	}
}

func (w *waitPrimitive) Name() string { return w.name }

func (w *waitPrimitive) Start(*engine.PhaseContext) (engine.Decision[WaitResult], error) {
	return engine.Decision[WaitResult]{AdvancePhase: false}, nil
}

func (w *waitPrimitive) Handle(_ *engine.PhaseContext, in engine.Input) (engine.Decision[WaitResult], error) {
	w.falseStarts[in.PlayerID] = true
	return engine.Decision[WaitResult]{AdvancePhase: false}, nil
}

func (w *waitPrimitive) Timeout(*engine.PhaseContext) (engine.Decision[WaitResult], error) {
	return engine.Decision[WaitResult]{
		AdvancePhase: true,
		Result: WaitResult{
			FalseStarts: cloneFalseStarts(w.falseStarts),
		},
	}, nil
}

type tapPrimitive struct {
	name string
	taps map[engine.PlayerID]time.Time
}

func newTap(round int) *tapPrimitive {
	return &tapPrimitive{
		name: fmt.Sprintf("reactionduel.tap.r%d", round),
		taps: map[engine.PlayerID]time.Time{},
	}
}

func (t *tapPrimitive) Name() string { return t.name }

func (t *tapPrimitive) Start(*engine.PhaseContext) (engine.Decision[TapResult], error) {
	return engine.Decision[TapResult]{AdvancePhase: false}, nil
}

func (t *tapPrimitive) Handle(ctx *engine.PhaseContext, in engine.Input) (engine.Decision[TapResult], error) {
	if falseStarts, ok := roundFalseStarts(ctx.State, roundFromPhase(ctx.Phase.Name)); ok && falseStarts[in.PlayerID] {
		return engine.Decision[TapResult]{AdvancePhase: false}, nil
	}
	if _, exists := t.taps[in.PlayerID]; exists {
		return engine.Decision[TapResult]{AdvancePhase: false}, nil
	}
	t.taps[in.PlayerID] = in.Timestamp
	if len(t.taps) >= eligibleTapCount(ctx.State, roundFromPhase(ctx.Phase.Name)) {
		return t.done(ctx), nil
	}
	return engine.Decision[TapResult]{AdvancePhase: false}, nil
}

func (t *tapPrimitive) Timeout(ctx *engine.PhaseContext) (engine.Decision[TapResult], error) {
	return t.done(ctx), nil
}

func (t *tapPrimitive) done(ctx *engine.PhaseContext) engine.Decision[TapResult] {
	return engine.Decision[TapResult]{
		AdvancePhase: true,
		Result: TapResult{
			Taps: cloneTaps(t.taps),
		},
	}
}

type revealPrimitive struct {
	name  string
	round int
}

func newReveal(round int) *revealPrimitive {
	return &revealPrimitive{name: fmt.Sprintf("reactionduel.reveal.r%d", round), round: round}
}

func (r *revealPrimitive) Name() string { return r.name }

func (r *revealPrimitive) Start(ctx *engine.PhaseContext) (engine.Decision[RoundResult], error) {
	results, scores, deltas := scoreRound(ctx.State, r.round)
	wireResults := make([]PlayerResult, len(results))
	copy(wireResults, results)
	resultPayload, _ := json.Marshal(RoundResult{Round: r.round, Results: wireResults})
	roundPayload, _ := json.Marshal(proto.RoundResultPayload{
		Round:  r.round,
		Deltas: deltas,
		Scores: scores,
	})
	return engine.Decision[RoundResult]{
		AdvancePhase: false,
		Broadcast: []engine.Event{
			{Type: proto.S2CReactionResult, Payload: resultPayload},
			{Type: proto.S2CRoundResult, Payload: roundPayload},
		},
	}, nil
}

func (r *revealPrimitive) Handle(*engine.PhaseContext, engine.Input) (engine.Decision[RoundResult], error) {
	return engine.Decision[RoundResult]{AdvancePhase: false}, nil
}

func (r *revealPrimitive) Timeout(ctx *engine.PhaseContext) (engine.Decision[RoundResult], error) {
	results, _, _ := scoreRound(ctx.State, r.round)
	return engine.Decision[RoundResult]{
		AdvancePhase: true,
		Result:       RoundResult{Round: r.round, Results: results},
	}, nil
}

func scoreRound(state *engine.GameState, round int) ([]PlayerResult, map[string]int, map[string]int) {
	falseStarts, _ := roundFalseStarts(state, round)
	taps, _ := engine.GetResult[TapResult](state, fmt.Sprintf("reaction_tap_r%d", round))
	type rankedTap struct {
		id   engine.PlayerID
		when time.Time
	}
	var ordered []rankedTap
	for pid, when := range taps.Taps {
		if falseStarts[pid] {
			continue
		}
		ordered = append(ordered, rankedTap{id: pid, when: when})
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].when.Before(ordered[j].when)
	})

	pointsByRank := []int{1000, 500, 250}
	deltas := map[string]int{}
	for idx, tap := range ordered {
		if idx >= len(pointsByRank) {
			break
		}
		deltas[string(tap.id)] = pointsByRank[idx]
		state.Scores[tap.id] += pointsByRank[idx]
	}

	var results []PlayerResult
	rankByID := map[engine.PlayerID]int{}
	for idx, tap := range ordered {
		rankByID[tap.id] = idx + 1
	}
	firstTap := time.Time{}
	if len(ordered) > 0 {
		firstTap = ordered[0].when
	}
	for _, player := range state.Players {
		result := PlayerResult{
			PlayerID:   string(player.ID),
			PlayerName: player.Name,
			FalseStart: falseStarts[player.ID],
			Points:     deltas[string(player.ID)],
		}
		if when, ok := taps.Taps[player.ID]; ok && !result.FalseStart {
			result.Rank = rankByID[player.ID]
			if !firstTap.IsZero() {
				result.ReactionMs = int(when.Sub(firstTap).Milliseconds())
			}
		}
		results = append(results, result)
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].FalseStart != results[j].FalseStart {
			return !results[i].FalseStart
		}
		if results[i].Rank == 0 || results[j].Rank == 0 {
			return results[i].PlayerName < results[j].PlayerName
		}
		return results[i].Rank < results[j].Rank
	})
	scores := map[string]int{}
	for pid, score := range state.Scores {
		scores[string(pid)] = score
	}
	return results, scores, deltas
}

func roundFalseStarts(state *engine.GameState, round int) (map[engine.PlayerID]bool, bool) {
	countdown, okCountdown := engine.GetResult[WaitResult](state, fmt.Sprintf("reaction_countdown_r%d", round))
	wait, okWait := engine.GetResult[WaitResult](state, fmt.Sprintf("reaction_wait_r%d", round))
	if !okCountdown && !okWait {
		return nil, false
	}
	merged := map[engine.PlayerID]bool{}
	for pid, bad := range countdown.FalseStarts {
		merged[pid] = bad
	}
	for pid, bad := range wait.FalseStarts {
		merged[pid] = merged[pid] || bad
	}
	return merged, true
}

func eligibleTapCount(state *engine.GameState, round int) int {
	falseStarts, _ := roundFalseStarts(state, round)
	count := 0
	for _, player := range state.Players {
		if !player.Connected {
			continue
		}
		if falseStarts[player.ID] {
			continue
		}
		count++
	}
	return count
}

func intOption(opts map[string]any, key string) (int, bool) {
	if opts == nil {
		return 0, false
	}
	raw, ok := opts[key]
	if !ok {
		return 0, false
	}
	switch v := raw.(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

func readRound(cfg json.RawMessage) (int, error) {
	if len(cfg) == 0 {
		return 1, nil
	}
	var out phaseConfig
	if err := json.Unmarshal(cfg, &out); err != nil {
		return 0, err
	}
	if out.Round < 1 {
		out.Round = 1
	}
	return out.Round, nil
}

func roundFromPhase(name string) int {
	var round int
	_, _ = fmt.Sscanf(name, "%*[^_]_r%d", &round)
	if round < 1 {
		i := len(name) - 1
		pow := 1
		for i >= 0 && name[i] >= '0' && name[i] <= '9' {
			round += int(name[i]-'0') * pow
			pow *= 10
			i--
		}
	}
	if round < 1 {
		return 1
	}
	return round
}

func cloneFalseStarts(in map[engine.PlayerID]bool) map[engine.PlayerID]bool {
	out := make(map[engine.PlayerID]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneTaps(in map[engine.PlayerID]time.Time) map[engine.PlayerID]time.Time {
	out := make(map[engine.PlayerID]time.Time, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
