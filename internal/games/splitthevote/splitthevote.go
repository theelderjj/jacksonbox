package splitthevote

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"slices"
	"sync"
	"time"

	"github.com/jj/trivia/internal/engine"
	"github.com/jj/trivia/internal/proto"
)

const (
	regKeySetup  = "splitthevote.setup"
	regKeyVote   = "splitthevote.vote"
	regKeyReveal = "splitthevote.reveal"

	defaultSetupSeconds  = 45
	defaultRevealSeconds = 5
)

type phaseConfig struct {
	Round int `json:"round"`
}

type SetupSubmission struct {
	Prompt  string `json:"prompt"`
	OptionA string `json:"option_a"`
	OptionB string `json:"option_b"`
}

type SetupResult struct {
	Round         int    `json:"round"`
	SplitterID    string `json:"splitter_id"`
	SplitterName  string `json:"splitter_name"`
	Prompt        string `json:"prompt"`
	OptionA       string `json:"option_a"`
	OptionB       string `json:"option_b"`
	TargetA       int    `json:"target_a"`
	TargetB       int    `json:"target_b"`
	TargetMode    string `json:"target_mode"`
	AuthoringMode string `json:"authoring_mode"`
	ShowTarget    bool   `json:"show_target"`
}

type VoteResult struct {
	ByPlayer map[string]string `json:"by_player"`
	CountA   int               `json:"count_a"`
	CountB   int               `json:"count_b"`
}

type RevealPayload struct {
	Round         int               `json:"round"`
	SplitterID    string            `json:"splitter_id"`
	SplitterName  string            `json:"splitter_name"`
	Prompt        string            `json:"prompt"`
	OptionA       string            `json:"option_a"`
	OptionB       string            `json:"option_b"`
	CountA        int               `json:"count_a"`
	CountB        int               `json:"count_b"`
	TargetA       int               `json:"target_a"`
	TargetB       int               `json:"target_b"`
	TargetMode    string            `json:"target_mode"`
	ShowTarget    bool              `json:"show_target"`
	Achieved      bool              `json:"achieved"`
	PlayerChoices map[string]string `json:"player_choices"`
}

var registerOnce sync.Once

func Register() {
	registerOnce.Do(func() {
		engine.Register(regKeySetup, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, err := readRound(cfg)
			if err != nil {
				return nil, err
			}
			return engine.Erase(newSetup(round)), nil
		})
		engine.Register(regKeyVote, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, err := readRound(cfg)
			if err != nil {
				return nil, err
			}
			return engine.Erase(newVote(round)), nil
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
	voteSeconds := state.Settings.VotingSeconds
	if voteSeconds <= 0 {
		voteSeconds = 20
	}
	phases := make([]engine.Phase, 0, rounds*3)
	for round := 1; round <= rounds; round++ {
		cfg, _ := json.Marshal(phaseConfig{Round: round})
		phases = append(phases,
			engine.Phase{
				Name:      fmt.Sprintf("split_setup_r%d", round),
				Primitive: regKeySetup,
				Duration:  defaultSetupSeconds * time.Second,
				Config:    cfg,
			},
			engine.Phase{
				Name:      fmt.Sprintf("split_vote_r%d", round),
				Primitive: regKeyVote,
				Duration:  time.Duration(voteSeconds) * time.Second,
				Config:    cfg,
				DependsOn: []string{fmt.Sprintf("split_setup_r%d", round)},
			},
			engine.Phase{
				Name:      fmt.Sprintf("split_reveal_r%d", round),
				Primitive: regKeyReveal,
				Duration:  defaultRevealSeconds * time.Second,
				Config:    cfg,
				DependsOn: []string{fmt.Sprintf("split_setup_r%d", round), fmt.Sprintf("split_vote_r%d", round)},
			},
		)
	}
	return phases
}

type setupPrimitive struct {
	name  string
	round int
}

func newSetup(round int) *setupPrimitive {
	return &setupPrimitive{name: fmt.Sprintf("splitthevote.setup.r%d", round), round: round}
}

func (s *setupPrimitive) Name() string { return s.name }

func (s *setupPrimitive) Start(ctx *engine.PhaseContext) (engine.Decision[SetupResult], error) {
	splitter := splitter(ctx.State)
	setup := buildSetupTemplate(ctx.State, s.round, splitter)
	text := setup.Prompt
	if setup.AuthoringMode == "generated_prompt_splitter_options" {
		text = fmt.Sprintf("Prompt: %s\nSecret target: %d vs %d", setup.Prompt, setup.TargetA, setup.TargetB)
	} else if setup.ShowTarget {
		text = fmt.Sprintf("Secret target: %d vs %d", setup.TargetA, setup.TargetB)
	}
	payload, _ := json.Marshal(proto.PromptIssuedPayload{
		PlayerID: setup.SplitterID,
		Prompt:   text,
	})
	return engine.Decision[SetupResult]{
		AdvancePhase: false,
		Broadcast:    []engine.Event{{Type: proto.S2CPromptIssued, Payload: payload}},
	}, nil
}

func (s *setupPrimitive) Handle(ctx *engine.PhaseContext, in engine.Input) (engine.Decision[SetupResult], error) {
	current := buildSetupTemplate(ctx.State, s.round, splitter(ctx.State))
	if string(in.PlayerID) != current.SplitterID {
		return engine.Decision[SetupResult]{}, fmt.Errorf("only the splitter can submit setup")
	}
	var payload proto.SubmitSplitSetupPayload
	if err := json.Unmarshal(in.Value, &payload); err != nil {
		return engine.Decision[SetupResult]{}, err
	}
	if payload.OptionA == "" || payload.OptionB == "" {
		return engine.Decision[SetupResult]{}, fmt.Errorf("both options are required")
	}
	if current.AuthoringMode == "splitter_prompt_and_options" {
		current.Prompt = payload.Prompt
	}
	current.OptionA = payload.OptionA
	current.OptionB = payload.OptionB
	return engine.Decision[SetupResult]{AdvancePhase: true, Result: current}, nil
}

func (s *setupPrimitive) Timeout(ctx *engine.PhaseContext) (engine.Decision[SetupResult], error) {
	current := buildSetupTemplate(ctx.State, s.round, splitter(ctx.State))
	current.OptionA = "Option A"
	current.OptionB = "Option B"
	return engine.Decision[SetupResult]{AdvancePhase: true, Result: current}, nil
}

type votePrimitive struct {
	name    string
	round   int
	choices map[string]string
}

func newVote(round int) *votePrimitive {
	return &votePrimitive{name: fmt.Sprintf("splitthevote.vote.r%d", round), round: round, choices: map[string]string{}}
}

func (v *votePrimitive) Name() string { return v.name }

func (v *votePrimitive) Start(ctx *engine.PhaseContext) (engine.Decision[VoteResult], error) {
	setup, ok := engine.GetResult[SetupResult](ctx.State, fmt.Sprintf("split_setup_r%d", v.round))
	if !ok {
		return engine.Decision[VoteResult]{}, fmt.Errorf("missing setup result")
	}
	payload, _ := json.Marshal(proto.SplitVotePromptPayload{
		Round:        v.round,
		SplitterID:   setup.SplitterID,
		SplitterName: setup.SplitterName,
		Prompt:       setup.Prompt,
		OptionA:      setup.OptionA,
		OptionB:      setup.OptionB,
		ShowTarget:   false,
	})
	return engine.Decision[VoteResult]{
		AdvancePhase: false,
		Broadcast:    []engine.Event{{Type: proto.S2CSplitVotePrompt, Payload: payload}},
	}, nil
}

func (v *votePrimitive) Handle(ctx *engine.PhaseContext, in engine.Input) (engine.Decision[VoteResult], error) {
	setup, ok := engine.GetResult[SetupResult](ctx.State, fmt.Sprintf("split_setup_r%d", v.round))
	if !ok {
		return engine.Decision[VoteResult]{}, fmt.Errorf("missing setup result")
	}
	if string(in.PlayerID) == setup.SplitterID {
		return engine.Decision[VoteResult]{}, fmt.Errorf("the splitter does not vote")
	}
	var payload proto.SubmitSplitChoicePayload
	if err := json.Unmarshal(in.Value, &payload); err != nil {
		return engine.Decision[VoteResult]{}, err
	}
	if payload.ChoiceID != "A" && payload.ChoiceID != "B" {
		return engine.Decision[VoteResult]{}, fmt.Errorf("choice must be A or B")
	}
	v.choices[string(in.PlayerID)] = payload.ChoiceID
	if len(v.choices) >= voterCount(ctx.State, setup.SplitterID) {
		return v.done(), nil
	}
	return engine.Decision[VoteResult]{AdvancePhase: false}, nil
}

func (v *votePrimitive) Timeout(*engine.PhaseContext) (engine.Decision[VoteResult], error) {
	return v.done(), nil
}

func (v *votePrimitive) done() engine.Decision[VoteResult] {
	countA, countB := 0, 0
	for _, choice := range v.choices {
		if choice == "A" {
			countA++
		} else if choice == "B" {
			countB++
		}
	}
	return engine.Decision[VoteResult]{
		AdvancePhase: true,
		Result: VoteResult{
			ByPlayer: cloneChoices(v.choices),
			CountA:   countA,
			CountB:   countB,
		},
	}
}

type revealPrimitive struct {
	name    string
	round   int
	resolved bool
	cached   RevealPayload
}

func newReveal(round int) *revealPrimitive {
	return &revealPrimitive{name: fmt.Sprintf("splitthevote.reveal.r%d", round), round: round}
}

func (r *revealPrimitive) Name() string { return r.name }

func (r *revealPrimitive) Start(ctx *engine.PhaseContext) (engine.Decision[RevealPayload], error) {
	reveal, scores, deltas, err := resolveRound(ctx.State, r.round)
	if err != nil {
		return engine.Decision[RevealPayload]{}, err
	}
	r.cached = reveal
	r.resolved = true
	revealJSON, _ := json.Marshal(reveal)
	roundJSON, _ := json.Marshal(proto.RoundResultPayload{
		Round:  r.round,
		Deltas: deltas,
		Scores: scores,
	})
	return engine.Decision[RevealPayload]{
		AdvancePhase: false,
		Broadcast: []engine.Event{
			{Type: proto.S2CSplitReveal, Payload: revealJSON},
			{Type: proto.S2CRoundResult, Payload: roundJSON},
		},
	}, nil
}

func (r *revealPrimitive) Handle(*engine.PhaseContext, engine.Input) (engine.Decision[RevealPayload], error) {
	return engine.Decision[RevealPayload]{AdvancePhase: false}, nil
}

func (r *revealPrimitive) Timeout(ctx *engine.PhaseContext) (engine.Decision[RevealPayload], error) {
	if !r.resolved {
		reveal, _, _, err := resolveRound(ctx.State, r.round)
		if err != nil {
			return engine.Decision[RevealPayload]{}, err
		}
		r.cached = reveal
		r.resolved = true
	}
	return engine.Decision[RevealPayload]{AdvancePhase: true, Result: r.cached}, nil
}

func resolveRound(state *engine.GameState, round int) (RevealPayload, map[string]int, map[string]int, error) {
	setup, ok := engine.GetResult[SetupResult](state, fmt.Sprintf("split_setup_r%d", round))
	if !ok {
		return RevealPayload{}, nil, nil, fmt.Errorf("missing setup result")
	}
	votes, ok := engine.GetResult[VoteResult](state, fmt.Sprintf("split_vote_r%d", round))
	if !ok {
		return RevealPayload{}, nil, nil, fmt.Errorf("missing vote result")
	}
	achieved := false
	switch setup.TargetMode {
	case "odd_one":
		achieved = votes.CountA > 0 && votes.CountB > 0 && min(votes.CountA, votes.CountB) == 1
	default:
		achieved = votes.CountA == setup.TargetA && votes.CountB == setup.TargetB
	}
	deltas := map[string]int{}
	if achieved {
		deltas[setup.SplitterID] += 1500
		if pid, ok := state.Players[engine.PlayerID(setup.SplitterID)]; ok {
			state.Scores[pid.ID] += 1500
		}
		for id := range votes.ByPlayer {
			deltas[id] += 500
			state.Scores[engine.PlayerID(id)] += 500
		}
	}
	scores := map[string]int{}
	for pid, score := range state.Scores {
		scores[string(pid)] = score
	}
	return RevealPayload{
		Round:         round,
		SplitterID:    setup.SplitterID,
		SplitterName:  setup.SplitterName,
		Prompt:        setup.Prompt,
		OptionA:       setup.OptionA,
		OptionB:       setup.OptionB,
		CountA:        votes.CountA,
		CountB:        votes.CountB,
		TargetA:       setup.TargetA,
		TargetB:       setup.TargetB,
		TargetMode:    setup.TargetMode,
		ShowTarget:    true,
		Achieved:      achieved,
		PlayerChoices: votes.ByPlayer,
	}, scores, deltas, nil
}

func buildSetupTemplate(state *engine.GameState, round int, splitterID engine.PlayerID) SetupResult {
	mode := stringOption(state.Settings.GameOptions, "target_mode", "strict_split")
	authoringMode := stringOption(state.Settings.GameOptions, "authoring_mode", "splitter_prompt_and_options")
	targetA, targetB := targetCounts(voterCount(state, string(splitterID)), mode)
	prompt := generatedPrompts[(round-1)%len(generatedPrompts)]
	if authoringMode == "splitter_prompt_and_options" {
		prompt = ""
	}
	splitterName := ""
	if splitter, ok := state.Players[splitterID]; ok {
		splitterName = splitter.Name
	}
	return SetupResult{
		Round:         round,
		SplitterID:    string(splitterID),
		SplitterName:  splitterName,
		Prompt:        prompt,
		TargetA:       targetA,
		TargetB:       targetB,
		TargetMode:    mode,
		AuthoringMode: authoringMode,
		ShowTarget:    mode == "variable",
	}
}

func splitter(state *engine.GameState) engine.PlayerID {
	if state.LeaderID != "" {
		if leader, ok := state.Players[state.LeaderID]; ok && leader.Connected {
			return state.LeaderID
		}
	}
	active := state.ActivePlayers()
	slices.Sort(active)
	if len(active) == 0 {
		return ""
	}
	return active[0]
}

func targetCounts(players int, mode string) (int, int) {
	if players <= 0 {
		return 0, 0
	}
	switch mode {
	case "odd_one":
		return players - 1, 1
	case "variable":
		if players == 1 {
			return 0, 1
		}
		if players == 2 {
			return 1, 1
		}
		a := 1 + rand.Intn(players-1)
		b := players - a
		if b == 0 {
			b = 1
			a = players - 1
		}
		return a, b
	default:
		return players / 2, players - (players / 2)
	}
}

func voterCount(state *engine.GameState, splitterID string) int {
	count := 0
	for _, playerID := range state.ActivePlayers() {
		if string(playerID) == splitterID {
			continue
		}
		count++
	}
	return count
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

func stringOption(opts map[string]any, key, fallback string) string {
	if opts == nil {
		return fallback
	}
	raw, ok := opts[key]
	if !ok {
		return fallback
	}
	if value, ok := raw.(string); ok && value != "" {
		return value
	}
	return fallback
}

func cloneChoices(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var generatedPrompts = []string{
	"Which one would make the better mascot for a road trip?",
	"Which one sounds more likely to start drama at a family barbecue?",
	"Which one feels more like the energy of a Friday night?",
	"Which one would survive longer in a haunted mall?",
	"Which one should be the official snack of the room?",
}
