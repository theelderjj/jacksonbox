package fakeartist

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jj/trivia/internal/engine"
	"github.com/jj/trivia/internal/games/drawful"
	"github.com/jj/trivia/internal/proto"
)

const (
	regKeyRole      = "fakeartist.role"
	regKeyCountdown = "fakeartist.countdown"
	regKeyDraw      = "fakeartist.draw"
	regKeyReplay    = "fakeartist.replay"
	regKeyVote      = "fakeartist.vote"
	regKeyGuess     = "fakeartist.guess"
	regKeyReveal    = "fakeartist.reveal"

	countdownSeconds = 3
	defaultDrawSecs  = 10
	defaultRevealSec = 5
)

type phaseConfig struct {
	Round int `json:"round"`
	Turn  int `json:"turn,omitempty"`
}

type RoleResult struct {
	Round         int               `json:"round"`
	Prompt        string            `json:"prompt"`
	FakeID        string            `json:"fake_id"`
	FakeName      string            `json:"fake_name"`
	Order         []string          `json:"order"`
	DrawOrder     []string          `json:"draw_order"`
	PlayerNames   map[string]string `json:"player_names"`
	ColorByPlayer map[string]string `json:"color_by_player"`
}

type TurnDrawingResult struct {
	Turn       int    `json:"turn"`
	DrawerID   string `json:"drawer_id"`
	DrawerName string `json:"drawer_name"`
	Color      string `json:"color"`
	Format     string `json:"format"`
	Data       string `json:"data"`
}

type VoteResult struct {
	Votes map[string]string `json:"votes"`
}

type GuessResult struct {
	Needed  bool   `json:"needed"`
	Guess   string `json:"guess"`
	Correct bool   `json:"correct"`
}

var registerOnce sync.Once

func Register() {
	registerOnce.Do(func() {
		engine.Register(regKeyRole, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, _, err := readConfig(cfg)
			if err != nil {
				return nil, err
			}
			return engine.Erase(newRole(round)), nil
		})
		engine.Register(regKeyCountdown, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, turn, err := readConfig(cfg)
			if err != nil {
				return nil, err
			}
			return engine.Erase(newCountdown(round, turn)), nil
		})
		engine.Register(regKeyDraw, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, turn, err := readConfig(cfg)
			if err != nil {
				return nil, err
			}
			return engine.Erase(newDraw(round, turn)), nil
		})
		engine.Register(regKeyReplay, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, _, err := readConfig(cfg)
			if err != nil {
				return nil, err
			}
			return engine.Erase(newReplay(round)), nil
		})
		engine.Register(regKeyVote, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, _, err := readConfig(cfg)
			if err != nil {
				return nil, err
			}
			return engine.Erase(newVote(round)), nil
		})
		engine.Register(regKeyGuess, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, _, err := readConfig(cfg)
			if err != nil {
				return nil, err
			}
			return engine.Erase(newGuess(round)), nil
		})
		engine.Register(regKeyReveal, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, _, err := readConfig(cfg)
			if err != nil {
				return nil, err
			}
			return engine.Erase(newReveal(round)), nil
		})
	})
}

func BuildPhases(state *engine.GameState) []engine.Phase {
	players := sortedActivePlayers(state)
	if len(players) == 0 {
		return nil
	}
	rounds := state.Settings.RoundCount
	if rounds <= 0 {
		rounds = 3
	}
	drawSecs := state.Settings.DrawingSeconds
	if drawSecs <= 0 {
		drawSecs = defaultDrawSecs
	}
	voteSecs := state.Settings.VotingSeconds
	if voteSecs <= 0 {
		voteSecs = 30
	}
	totalTurns := len(players) * 2
	phases := make([]engine.Phase, 0, rounds*(totalTurns*2+4))
	for round := 1; round <= rounds; round++ {
		roleCfg, _ := json.Marshal(phaseConfig{Round: round})
		phases = append(phases, engine.Phase{
			Name:      phaseName("fake_artist_role", round, 0),
			Primitive: regKeyRole,
			Config:    roleCfg,
		})
		for turn := 1; turn <= totalTurns; turn++ {
			turnCfg, _ := json.Marshal(phaseConfig{Round: round, Turn: turn})
			phases = append(phases,
				engine.Phase{
					Name:      phaseName("fake_artist_countdown", round, turn),
					Primitive: regKeyCountdown,
					Duration:  countdownSeconds * time.Second,
					Config:    turnCfg,
					DependsOn: []string{phaseName("fake_artist_role", round, 0)},
				},
				engine.Phase{
					Name:      phaseName("fake_artist_draw", round, turn),
					Primitive: regKeyDraw,
					Duration:  time.Duration(drawSecs) * time.Second,
					Config:    turnCfg,
					DependsOn: []string{phaseName("fake_artist_role", round, 0)},
				},
			)
		}
		phases = append(phases,
			engine.Phase{
				Name:      phaseName("fake_artist_replay", round, 0),
				Primitive: regKeyReplay,
				Duration:  replayDuration(state, totalTurns),
				Config:    roleCfg,
				DependsOn: drawPhaseDependencies(round, totalTurns),
			},
			engine.Phase{
				Name:      phaseName("fake_artist_vote", round, 0),
				Primitive: regKeyVote,
				Duration:  time.Duration(voteSecs) * time.Second,
				Config:    roleCfg,
				DependsOn: []string{phaseName("fake_artist_role", round, 0), phaseName("fake_artist_replay", round, 0)},
			},
			engine.Phase{
				Name:      phaseName("fake_artist_guess", round, 0),
				Primitive: regKeyGuess,
				Duration:  time.Duration(voteSecs) * time.Second,
				Config:    roleCfg,
				DependsOn: []string{phaseName("fake_artist_role", round, 0), phaseName("fake_artist_vote", round, 0)},
			},
			engine.Phase{
				Name:      phaseName("fake_artist_reveal", round, 0),
				Primitive: regKeyReveal,
				Duration:  defaultRevealSec * time.Second,
				Config:    roleCfg,
				DependsOn: []string{phaseName("fake_artist_role", round, 0), phaseName("fake_artist_vote", round, 0), phaseName("fake_artist_guess", round, 0)},
			},
		)
	}
	return phases
}

type rolePrimitive struct {
	round int
}

func newRole(round int) *rolePrimitive { return &rolePrimitive{round: round} }
func (p *rolePrimitive) Name() string  { return fmt.Sprintf("fakeartist.role.r%d", p.round) }

func (p *rolePrimitive) Start(ctx *engine.PhaseContext) (engine.Decision[RoleResult], error) {
	result := buildRoleResult(ctx.State, p.round)
	broadcasts := make([]engine.Event, 0, len(result.Order))
	for _, playerID := range result.Order {
		payload, _ := json.Marshal(proto.PromptIssuedPayload{
			PlayerID: playerID,
			Prompt:   promptForPlayer(result, playerID),
		})
		broadcasts = append(broadcasts, engine.Event{Type: proto.S2CPromptIssued, Payload: payload})
	}
	return engine.Decision[RoleResult]{AdvancePhase: true, Broadcast: broadcasts, Result: result}, nil
}

func (*rolePrimitive) Handle(*engine.PhaseContext, engine.Input) (engine.Decision[RoleResult], error) {
	return engine.Decision[RoleResult]{AdvancePhase: false}, nil
}

func (p *rolePrimitive) Timeout(ctx *engine.PhaseContext) (engine.Decision[RoleResult], error) {
	return p.Start(ctx)
}

type countdownPrimitive struct {
	round int
	turn  int
}

func newCountdown(round, turn int) *countdownPrimitive {
	return &countdownPrimitive{round: round, turn: turn}
}
func (p *countdownPrimitive) Name() string {
	return fmt.Sprintf("fakeartist.countdown.r%d.t%d", p.round, p.turn)
}

func (p *countdownPrimitive) Start(ctx *engine.PhaseContext) (engine.Decision[struct{}], error) {
	role, ok := engine.GetResult[RoleResult](ctx.State, phaseName("fake_artist_role", p.round, 0))
	if !ok {
		return engine.Decision[struct{}]{}, fmt.Errorf("missing role result")
	}
	drawerID, drawerName := currentDrawer(role, p.turn)
	payload, _ := json.Marshal(proto.FakeArtistTurnPayload{
		Round:            p.round,
		Turn:             p.turn,
		DrawerID:         drawerID,
		DrawerName:       drawerName,
		Color:            role.ColorByPlayer[drawerID],
		Phase:            "countdown",
		CountdownSeconds: countdownSeconds,
		Format:           "strokes",
		Data:             combinedDataForTurn(ctx.State, p.round, p.turn-1),
	})
	return engine.Decision[struct{}]{
		Broadcast: []engine.Event{{Type: proto.S2CFakeArtistTurn, Payload: payload}},
	}, nil
}

func (*countdownPrimitive) Handle(*engine.PhaseContext, engine.Input) (engine.Decision[struct{}], error) {
	return engine.Decision[struct{}]{AdvancePhase: false}, nil
}

func (*countdownPrimitive) Timeout(*engine.PhaseContext) (engine.Decision[struct{}], error) {
	return engine.Decision[struct{}]{AdvancePhase: true}, nil
}

type drawPrimitive struct {
	round  int
	turn   int
	latest proto.SubmitDrawingPayload
}

func newDraw(round, turn int) *drawPrimitive { return &drawPrimitive{round: round, turn: turn} }
func (p *drawPrimitive) Name() string        { return fmt.Sprintf("fakeartist.draw.r%d.t%d", p.round, p.turn) }

func (p *drawPrimitive) Start(ctx *engine.PhaseContext) (engine.Decision[TurnDrawingResult], error) {
	role, ok := engine.GetResult[RoleResult](ctx.State, phaseName("fake_artist_role", p.round, 0))
	if !ok {
		return engine.Decision[TurnDrawingResult]{}, fmt.Errorf("missing role result")
	}
	drawerID, drawerName := currentDrawer(role, p.turn)
	payload, _ := json.Marshal(proto.FakeArtistTurnPayload{
		Round:      p.round,
		Turn:       p.turn,
		DrawerID:   drawerID,
		DrawerName: drawerName,
		Color:      role.ColorByPlayer[drawerID],
		Phase:      "draw",
		Format:     "strokes",
		Data:       combinedDataForTurn(ctx.State, p.round, p.turn-1),
	})
	return engine.Decision[TurnDrawingResult]{
		Broadcast: []engine.Event{{Type: proto.S2CFakeArtistTurn, Payload: payload}},
	}, nil
}

func (p *drawPrimitive) Handle(ctx *engine.PhaseContext, in engine.Input) (engine.Decision[TurnDrawingResult], error) {
	role, ok := engine.GetResult[RoleResult](ctx.State, phaseName("fake_artist_role", p.round, 0))
	if !ok {
		return engine.Decision[TurnDrawingResult]{}, fmt.Errorf("missing role result")
	}
	drawerID, drawerName := currentDrawer(role, p.turn)
	if string(in.PlayerID) != drawerID {
		return engine.Decision[TurnDrawingResult]{}, fmt.Errorf("only %s can draw right now", drawerName)
	}
	var payload proto.SubmitDrawingPayload
	if err := json.Unmarshal(in.Value, &payload); err != nil {
		return engine.Decision[TurnDrawingResult]{}, err
	}
	if err := drawful.ValidateDrawing(payload.Format, payload.Data); err != nil {
		return engine.Decision[TurnDrawingResult]{}, err
	}
	p.latest = payload
	canvas, _ := json.Marshal(proto.FakeArtistCanvasPayload{
		Round:      p.round,
		Turn:       p.turn,
		DrawerID:   drawerID,
		DrawerName: drawerName,
		Color:      role.ColorByPlayer[drawerID],
		Format:     "strokes",
		Data:       mergeStrokeData(combinedDataForTurn(ctx.State, p.round, p.turn-1), payload.Data),
	})
	return engine.Decision[TurnDrawingResult]{
		Broadcast: []engine.Event{{Type: proto.S2CFakeArtistCanvas, Payload: canvas}},
	}, nil
}

func (p *drawPrimitive) Timeout(ctx *engine.PhaseContext) (engine.Decision[TurnDrawingResult], error) {
	role, ok := engine.GetResult[RoleResult](ctx.State, phaseName("fake_artist_role", p.round, 0))
	if !ok {
		return engine.Decision[TurnDrawingResult]{}, fmt.Errorf("missing role result")
	}
	drawerID, drawerName := currentDrawer(role, p.turn)
	data := p.latest.Data
	format := p.latest.Format
	if strings.TrimSpace(data) == "" {
		format = "strokes"
		data = `{"strokes":[]}`
	}
	return engine.Decision[TurnDrawingResult]{
		AdvancePhase: true,
		Result: TurnDrawingResult{
			Turn:       p.turn,
			DrawerID:   drawerID,
			DrawerName: drawerName,
			Color:      role.ColorByPlayer[drawerID],
			Format:     format,
			Data:       data,
		},
	}, nil
}

type replayPrimitive struct {
	round int
}

func newReplay(round int) *replayPrimitive { return &replayPrimitive{round: round} }
func (p *replayPrimitive) Name() string    { return fmt.Sprintf("fakeartist.replay.r%d", p.round) }

func (p *replayPrimitive) Start(ctx *engine.PhaseContext) (engine.Decision[proto.FakeArtistReplayPayload], error) {
	payload := replayPayload(ctx.State, p.round)
	raw, _ := json.Marshal(payload)
	return engine.Decision[proto.FakeArtistReplayPayload]{
		Broadcast: []engine.Event{{Type: proto.S2CFakeArtistReplay, Payload: raw}},
	}, nil
}

func (*replayPrimitive) Handle(*engine.PhaseContext, engine.Input) (engine.Decision[proto.FakeArtistReplayPayload], error) {
	return engine.Decision[proto.FakeArtistReplayPayload]{AdvancePhase: false}, nil
}

func (p *replayPrimitive) Timeout(ctx *engine.PhaseContext) (engine.Decision[proto.FakeArtistReplayPayload], error) {
	return engine.Decision[proto.FakeArtistReplayPayload]{AdvancePhase: true, Result: replayPayload(ctx.State, p.round)}, nil
}

type votePrimitive struct {
	round int
	votes map[string]string
}

func newVote(round int) *votePrimitive {
	return &votePrimitive{round: round, votes: map[string]string{}}
}
func (p *votePrimitive) Name() string { return fmt.Sprintf("fakeartist.vote.r%d", p.round) }

func (*votePrimitive) Start(*engine.PhaseContext) (engine.Decision[VoteResult], error) {
	return engine.Decision[VoteResult]{AdvancePhase: false}, nil
}

func (p *votePrimitive) Handle(ctx *engine.PhaseContext, in engine.Input) (engine.Decision[VoteResult], error) {
	role, ok := engine.GetResult[RoleResult](ctx.State, phaseName("fake_artist_role", p.round, 0))
	if !ok {
		return engine.Decision[VoteResult]{}, fmt.Errorf("missing role result")
	}
	var payload proto.SubmitVotePayload
	if err := json.Unmarshal(in.Value, &payload); err != nil {
		return engine.Decision[VoteResult]{}, err
	}
	if _, ok := role.PlayerNames[payload.ChoiceID]; !ok {
		return engine.Decision[VoteResult]{}, fmt.Errorf("unknown player vote")
	}
	p.votes[string(in.PlayerID)] = payload.ChoiceID
	if len(p.votes) >= len(sortedActivePlayers(ctx.State)) {
		return engine.Decision[VoteResult]{AdvancePhase: true, Result: VoteResult{Votes: cloneMap(p.votes)}}, nil
	}
	return engine.Decision[VoteResult]{AdvancePhase: false}, nil
}

func (p *votePrimitive) Timeout(*engine.PhaseContext) (engine.Decision[VoteResult], error) {
	return engine.Decision[VoteResult]{AdvancePhase: true, Result: VoteResult{Votes: cloneMap(p.votes)}}, nil
}

type guessPrimitive struct {
	round int
}

func newGuess(round int) *guessPrimitive { return &guessPrimitive{round: round} }
func (p *guessPrimitive) Name() string   { return fmt.Sprintf("fakeartist.guess.r%d", p.round) }

func (p *guessPrimitive) Start(ctx *engine.PhaseContext) (engine.Decision[GuessResult], error) {
	role, ok := engine.GetResult[RoleResult](ctx.State, phaseName("fake_artist_role", p.round, 0))
	if !ok {
		return engine.Decision[GuessResult]{}, fmt.Errorf("missing role result")
	}
	voteResult, ok := engine.GetResult[VoteResult](ctx.State, phaseName("fake_artist_vote", p.round, 0))
	if !ok {
		return engine.Decision[GuessResult]{}, fmt.Errorf("missing vote result")
	}
	accusedID, majorityCaught := majorityAccusation(voteResult.Votes)
	if !majorityCaught || accusedID != role.FakeID {
		return engine.Decision[GuessResult]{AdvancePhase: true, Result: GuessResult{Needed: false}}, nil
	}
	payload, _ := json.Marshal(proto.PromptIssuedPayload{
		PlayerID: role.FakeID,
		Prompt:   "The room caught you. Guess the real prompt to steal the round.",
	})
	return engine.Decision[GuessResult]{
		Broadcast: []engine.Event{{Type: proto.S2CPromptIssued, Payload: payload}},
	}, nil
}

func (p *guessPrimitive) Handle(ctx *engine.PhaseContext, in engine.Input) (engine.Decision[GuessResult], error) {
	role, ok := engine.GetResult[RoleResult](ctx.State, phaseName("fake_artist_role", p.round, 0))
	if !ok {
		return engine.Decision[GuessResult]{}, fmt.Errorf("missing role result")
	}
	if string(in.PlayerID) != role.FakeID {
		return engine.Decision[GuessResult]{}, fmt.Errorf("only the fake artist can guess")
	}
	var payload proto.SubmitFakeArtistGuessPayload
	if err := json.Unmarshal(in.Value, &payload); err != nil {
		return engine.Decision[GuessResult]{}, err
	}
	guess := strings.TrimSpace(payload.Prompt)
	return engine.Decision[GuessResult]{
		AdvancePhase: true,
		Result: GuessResult{
			Needed:  true,
			Guess:   guess,
			Correct: normalize(guess) == normalize(role.Prompt),
		},
	}, nil
}

func (p *guessPrimitive) Timeout(ctx *engine.PhaseContext) (engine.Decision[GuessResult], error) {
	role, ok := engine.GetResult[RoleResult](ctx.State, phaseName("fake_artist_role", p.round, 0))
	if !ok {
		return engine.Decision[GuessResult]{}, fmt.Errorf("missing role result")
	}
	voteResult, ok := engine.GetResult[VoteResult](ctx.State, phaseName("fake_artist_vote", p.round, 0))
	if !ok {
		return engine.Decision[GuessResult]{}, fmt.Errorf("missing vote result")
	}
	accusedID, majorityCaught := majorityAccusation(voteResult.Votes)
	needed := majorityCaught && accusedID == role.FakeID
	return engine.Decision[GuessResult]{AdvancePhase: true, Result: GuessResult{Needed: needed}}, nil
}

type revealPrimitive struct {
	round    int
	resolved bool
	cached   proto.FakeArtistRevealPayload
}

func newReveal(round int) *revealPrimitive { return &revealPrimitive{round: round} }
func (p *revealPrimitive) Name() string    { return fmt.Sprintf("fakeartist.reveal.r%d", p.round) }

func (p *revealPrimitive) Start(ctx *engine.PhaseContext) (engine.Decision[proto.FakeArtistRevealPayload], error) {
	reveal, deltas, scores, err := resolveReveal(ctx.State, p.round)
	if err != nil {
		return engine.Decision[proto.FakeArtistRevealPayload]{}, err
	}
	p.cached = reveal
	p.resolved = true
	revealJSON, _ := json.Marshal(reveal)
	roundJSON, _ := json.Marshal(proto.RoundResultPayload{
		Round:  p.round,
		Deltas: deltas,
		Scores: scores,
	})
	return engine.Decision[proto.FakeArtistRevealPayload]{
		Broadcast: []engine.Event{
			{Type: proto.S2CFakeArtistReveal, Payload: revealJSON},
			{Type: proto.S2CRoundResult, Payload: roundJSON},
		},
	}, nil
}

func (*revealPrimitive) Handle(*engine.PhaseContext, engine.Input) (engine.Decision[proto.FakeArtistRevealPayload], error) {
	return engine.Decision[proto.FakeArtistRevealPayload]{AdvancePhase: false}, nil
}

func (p *revealPrimitive) Timeout(ctx *engine.PhaseContext) (engine.Decision[proto.FakeArtistRevealPayload], error) {
	if !p.resolved {
		reveal, _, _, err := resolveReveal(ctx.State, p.round)
		if err != nil {
			return engine.Decision[proto.FakeArtistRevealPayload]{}, err
		}
		p.cached = reveal
		p.resolved = true
	}
	return engine.Decision[proto.FakeArtistRevealPayload]{AdvancePhase: true, Result: p.cached}, nil
}

func resolveReveal(state *engine.GameState, round int) (proto.FakeArtistRevealPayload, map[string]int, map[string]int, error) {
	role, ok := engine.GetResult[RoleResult](state, phaseName("fake_artist_role", round, 0))
	if !ok {
		return proto.FakeArtistRevealPayload{}, nil, nil, fmt.Errorf("missing role result")
	}
	voteResult, ok := engine.GetResult[VoteResult](state, phaseName("fake_artist_vote", round, 0))
	if !ok {
		return proto.FakeArtistRevealPayload{}, nil, nil, fmt.Errorf("missing vote result")
	}
	guessResult, ok := engine.GetResult[GuessResult](state, phaseName("fake_artist_guess", round, 0))
	if !ok {
		return proto.FakeArtistRevealPayload{}, nil, nil, fmt.Errorf("missing guess result")
	}

	accusedID, majorityCaught := majorityAccusation(voteResult.Votes)
	fakeGuessedPrompt := guessResult.Needed && guessResult.Correct
	fakeWins := !majorityCaught || accusedID != role.FakeID || fakeGuessedPrompt

	deltas := map[string]int{}
	winners := make([]string, 0, len(role.Order))
	if fakeWins {
		deltas[role.FakeID] = 1500
		state.Scores[engine.PlayerID(role.FakeID)] += 1500
		winners = append(winners, role.FakeID)
	} else {
		for _, playerID := range role.Order {
			if playerID == role.FakeID {
				continue
			}
			deltas[playerID] = 1000
			state.Scores[engine.PlayerID(playerID)] += 1000
			winners = append(winners, playerID)
		}
	}

	scores := map[string]int{}
	for playerID, score := range state.Scores {
		scores[string(playerID)] = score
	}
	return proto.FakeArtistRevealPayload{
		Round:             round,
		FakeID:            role.FakeID,
		FakeName:          role.FakeName,
		Prompt:            role.Prompt,
		AccusedID:         accusedID,
		AccusedName:       role.PlayerNames[accusedID],
		MajorityCaught:    majorityCaught && accusedID == role.FakeID,
		FakeGuess:         guessResult.Guess,
		FakeGuessedPrompt: fakeGuessedPrompt,
		FakeWins:          fakeWins,
		Votes:             voteResult.Votes,
		Winners:           winners,
	}, deltas, scores, nil
}

func buildRoleResult(state *engine.GameState, round int) RoleResult {
	players := sortedActivePlayers(state)
	rng := rand.New(rand.NewSource(int64(len(players)*113 + round*977)))
	order := append([]engine.PlayerID(nil), players...)
	rng.Shuffle(len(order), func(i, j int) {
		order[i], order[j] = order[j], order[i]
	})
	fakeIdx := rng.Intn(len(order))
	fakeID := order[fakeIdx]
	playerNames := map[string]string{}
	for _, playerID := range order {
		playerNames[string(playerID)] = state.Players[playerID].Name
	}
	colorByPlayer := map[string]string{}
	useColors := boolOption(state.Settings.GameOptions, "color_coded_drawers", false)
	for i, playerID := range order {
		color := "#0b0f14"
		if useColors {
			color = fakeArtistColors[i%len(fakeArtistColors)]
		}
		colorByPlayer[string(playerID)] = color
	}
	drawOrder := make([]string, 0, len(order)*2)
	turnIDs := toStringIDs(order)
	drawOrder = append(drawOrder, turnIDs...)
	drawOrder = append(drawOrder, turnIDs...)
	return RoleResult{
		Round:         round,
		Prompt:        fakeArtistPrompts[(round-1)%len(fakeArtistPrompts)],
		FakeID:        string(fakeID),
		FakeName:      state.Players[fakeID].Name,
		Order:         turnIDs,
		DrawOrder:     drawOrder,
		PlayerNames:   playerNames,
		ColorByPlayer: colorByPlayer,
	}
}

func promptForPlayer(role RoleResult, playerID string) string {
	if playerID == role.FakeID {
		return fmt.Sprintf("You are the fake artist. Watch the canvas, blend in, and avoid the vote.\nRound %d", role.Round)
	}
	return fmt.Sprintf("Draw this together: %s", role.Prompt)
}

func sortedActivePlayers(state *engine.GameState) []engine.PlayerID {
	players := state.ActivePlayers()
	slices.Sort(players)
	return players
}

func readConfig(raw json.RawMessage) (int, int, error) {
	if len(raw) == 0 {
		return 1, 0, nil
	}
	var cfg phaseConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return 0, 0, err
	}
	if cfg.Round < 1 {
		cfg.Round = 1
	}
	if cfg.Turn < 0 {
		cfg.Turn = 0
	}
	return cfg.Round, cfg.Turn, nil
}

func phaseName(kind string, round, turn int) string {
	if turn > 0 {
		return fmt.Sprintf("%s_r%d_t%d", kind, round, turn)
	}
	return fmt.Sprintf("%s_r%d", kind, round)
}

func currentDrawer(role RoleResult, turn int) (string, string) {
	if turn < 1 || turn > len(role.DrawOrder) {
		return "", ""
	}
	id := role.DrawOrder[turn-1]
	return id, role.PlayerNames[id]
}

func combinedDataForTurn(state *engine.GameState, round, turn int) string {
	data := `{"strokes":[]}`
	for i := 1; i <= turn; i++ {
		result, ok := engine.GetResult[TurnDrawingResult](state, phaseName("fake_artist_draw", round, i))
		if !ok {
			continue
		}
		data = mergeStrokeData(data, result.Data)
	}
	return data
}

func mergeStrokeData(existing, addition string) string {
	type strokePayload struct {
		Strokes []map[string]any `json:"strokes"`
	}
	var left strokePayload
	var right strokePayload
	if err := json.Unmarshal([]byte(existing), &left); err != nil {
		left = strokePayload{}
	}
	if err := json.Unmarshal([]byte(addition), &right); err != nil {
		right = strokePayload{}
	}
	left.Strokes = append(left.Strokes, right.Strokes...)
	raw, err := json.Marshal(left)
	if err != nil {
		return `{"strokes":[]}`
	}
	return string(raw)
}

func drawPhaseDependencies(round, turnCount int) []string {
	out := make([]string, 0, turnCount)
	for turn := 1; turn <= turnCount; turn++ {
		out = append(out, phaseName("fake_artist_draw", round, turn))
	}
	return out
}

func replayPayload(state *engine.GameState, round int) proto.FakeArtistReplayPayload {
	role, _ := engine.GetResult[RoleResult](state, phaseName("fake_artist_role", round, 0))
	segments := make([]proto.FakeArtistReplaySegment, 0, len(role.DrawOrder))
	for turn := 1; turn <= len(role.DrawOrder); turn++ {
		result, ok := engine.GetResult[TurnDrawingResult](state, phaseName("fake_artist_draw", round, turn))
		if !ok {
			continue
		}
		segments = append(segments, proto.FakeArtistReplaySegment{
			Turn:       result.Turn,
			DrawerID:   result.DrawerID,
			DrawerName: result.DrawerName,
			Color:      result.Color,
			Format:     result.Format,
			Data:       result.Data,
		})
	}
	return proto.FakeArtistReplayPayload{
		Round:            round,
		ReplayCount:      intOption(state.Settings.GameOptions, "replay_count", 1),
		ContinuousReplay: boolOption(state.Settings.GameOptions, "continuous_replay", false),
		Segments:         segments,
	}
}

func majorityAccusation(votes map[string]string) (string, bool) {
	counts := map[string]int{}
	total := 0
	for _, target := range votes {
		if target == "" {
			continue
		}
		counts[target]++
		total++
	}
	bestID := ""
	bestCount := 0
	for target, count := range counts {
		if count > bestCount {
			bestID = target
			bestCount = count
		}
	}
	return bestID, bestCount > total/2
}

func replayDuration(settingsState *engine.GameState, turnCount int) time.Duration {
	replays := intOption(settingsState.Settings.GameOptions, "replay_count", 1)
	if replays < 1 {
		replays = 1
	}
	const replayStepMillis = 700
	stepsPerTurn := 4
	totalMillis := turnCount * stepsPerTurn * replayStepMillis * replays
	if totalMillis < 8000 {
		totalMillis = 8000
	}
	return time.Duration(totalMillis+1000) * time.Millisecond
}

func toStringIDs(in []engine.PlayerID) []string {
	out := make([]string, len(in))
	for i, id := range in {
		out[i] = string(id)
	}
	return out
}

func cloneMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func boolOption(options map[string]any, key string, fallback bool) bool {
	if options == nil {
		return fallback
	}
	value, ok := options[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case bool:
		return typed
	default:
		return fallback
	}
}

func intOption(options map[string]any, key string, fallback int) int {
	if options == nil {
		return fallback
	}
	value, ok := options[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	default:
		return fallback
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

var fakeArtistColors = []string{
	"#ef4444",
	"#f97316",
	"#eab308",
	"#22c55e",
	"#38bdf8",
	"#6366f1",
	"#ec4899",
	"#14b8a6",
}

var fakeArtistPrompts = []string{
	"spider",
	"pizza slice",
	"school bus",
	"roller coaster",
	"dragon",
	"lightning bolt",
	"treasure chest",
	"spaceship",
}
