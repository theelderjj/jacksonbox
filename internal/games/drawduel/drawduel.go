package drawduel

import (
	"encoding/json"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/jj/trivia/internal/engine"
	"github.com/jj/trivia/internal/games/drawful"
	"github.com/jj/trivia/internal/proto"
)

const (
	regKeyPrompt = "drawduel.prompt"
	regKeyDraw   = "drawduel.draw"
	regKeyVote   = "drawduel.vote"
	regKeyReveal = "drawduel.reveal"

	defaultDrawSeconds   = 45
	defaultVoteSeconds   = 20
	defaultRevealSeconds = 5
)

type phaseConfig struct {
	Round int `json:"round"`
}

type SetupResult struct {
	Round       int      `json:"round"`
	Prompt      string   `json:"prompt"`
	ArtistAID   string   `json:"artist_a_id"`
	ArtistAName string   `json:"artist_a_name"`
	ArtistBID   string   `json:"artist_b_id"`
	ArtistBName string   `json:"artist_b_name"`
	JudgeIDs    []string `json:"judge_ids"`
}

type DrawingResult struct {
	ByPlayer map[string]proto.DrawingSummary `json:"by_player"`
}

type VoteResult struct {
	ByJudge map[string]string `json:"by_judge"`
}

var registerOnce sync.Once

func Register() {
	registerOnce.Do(func() {
		engine.Register(regKeyPrompt, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, err := readRound(cfg)
			if err != nil {
				return nil, err
			}
			return engine.Erase(newPrompt(round)), nil
		})
		engine.Register(regKeyDraw, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, err := readRound(cfg)
			if err != nil {
				return nil, err
			}
			return engine.Erase(newDraw(round)), nil
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
	drawSeconds := state.Settings.DrawingSeconds
	if drawSeconds <= 0 {
		drawSeconds = defaultDrawSeconds
	}
	voteSeconds := state.Settings.VotingSeconds
	if voteSeconds <= 0 {
		voteSeconds = defaultVoteSeconds
	}
	phases := make([]engine.Phase, 0, rounds*4)
	for round := 1; round <= rounds; round++ {
		cfg, _ := json.Marshal(phaseConfig{Round: round})
		phases = append(phases,
			engine.Phase{
				Name:      fmt.Sprintf("draw_duel_prompt_r%d", round),
				Primitive: regKeyPrompt,
				Config:    cfg,
			},
			engine.Phase{
				Name:      fmt.Sprintf("draw_duel_draw_r%d", round),
				Primitive: regKeyDraw,
				Duration:  time.Duration(drawSeconds) * time.Second,
				Config:    cfg,
				DependsOn: []string{fmt.Sprintf("draw_duel_prompt_r%d", round)},
			},
			engine.Phase{
				Name:      fmt.Sprintf("draw_duel_vote_r%d", round),
				Primitive: regKeyVote,
				Duration:  time.Duration(voteSeconds) * time.Second,
				Config:    cfg,
				DependsOn: []string{fmt.Sprintf("draw_duel_prompt_r%d", round), fmt.Sprintf("draw_duel_draw_r%d", round)},
			},
			engine.Phase{
				Name:      fmt.Sprintf("draw_duel_reveal_r%d", round),
				Primitive: regKeyReveal,
				Duration:  defaultRevealSeconds * time.Second,
				Config:    cfg,
				DependsOn: []string{fmt.Sprintf("draw_duel_prompt_r%d", round), fmt.Sprintf("draw_duel_draw_r%d", round), fmt.Sprintf("draw_duel_vote_r%d", round)},
			},
		)
	}
	return phases
}

type promptPrimitive struct {
	round int
}

func newPrompt(round int) *promptPrimitive { return &promptPrimitive{round: round} }
func (p *promptPrimitive) Name() string    { return fmt.Sprintf("drawduel.prompt.r%d", p.round) }

func (p *promptPrimitive) Start(ctx *engine.PhaseContext) (engine.Decision[SetupResult], error) {
	setup := buildSetup(ctx.State, p.round)
	broadcast := []engine.Event{}
	for _, playerID := range activePlayers(ctx.State) {
		payload, _ := json.Marshal(proto.PromptIssuedPayload{
			PlayerID: playerID,
			Prompt:   setup.Prompt,
		})
		broadcast = append(broadcast, engine.Event{Type: proto.S2CPromptIssued, Payload: payload})
	}
	roundPayload, _ := json.Marshal(toRoundPayload(setup))
	broadcast = append(broadcast, engine.Event{Type: proto.S2CDrawDuelRound, Payload: roundPayload})
	return engine.Decision[SetupResult]{
		AdvancePhase: true,
		Broadcast:    broadcast,
		Result:       setup,
	}, nil
}

func (*promptPrimitive) Handle(*engine.PhaseContext, engine.Input) (engine.Decision[SetupResult], error) {
	return engine.Decision[SetupResult]{AdvancePhase: false}, nil
}

func (p *promptPrimitive) Timeout(ctx *engine.PhaseContext) (engine.Decision[SetupResult], error) {
	return p.Start(ctx)
}

type drawPrimitive struct {
	round    int
	drawings map[string]proto.DrawingSummary
}

func newDraw(round int) *drawPrimitive {
	return &drawPrimitive{round: round, drawings: map[string]proto.DrawingSummary{}}
}

func (p *drawPrimitive) Name() string { return fmt.Sprintf("drawduel.draw.r%d", p.round) }

func (*drawPrimitive) Start(*engine.PhaseContext) (engine.Decision[DrawingResult], error) {
	return engine.Decision[DrawingResult]{AdvancePhase: false}, nil
}

func (p *drawPrimitive) Handle(ctx *engine.PhaseContext, in engine.Input) (engine.Decision[DrawingResult], error) {
	setup, ok := engine.GetResult[SetupResult](ctx.State, fmt.Sprintf("draw_duel_prompt_r%d", p.round))
	if !ok {
		return engine.Decision[DrawingResult]{}, fmt.Errorf("missing setup result")
	}
	playerID := string(in.PlayerID)
	if playerID != setup.ArtistAID && playerID != setup.ArtistBID {
		return engine.Decision[DrawingResult]{}, fmt.Errorf("only the duel artists can draw this round")
	}
	var payload proto.SubmitDrawingPayload
	if err := json.Unmarshal(in.Value, &payload); err != nil {
		return engine.Decision[DrawingResult]{}, err
	}
	if err := drawful.ValidateDrawing(payload.Format, payload.Data); err != nil {
		return engine.Decision[DrawingResult]{}, err
	}
	p.drawings[playerID] = proto.DrawingSummary{
		DrawingID: playerID,
		AuthorID:  playerID,
		Format:    payload.Format,
		Data:      payload.Data,
	}
	if len(p.drawings) >= 2 {
		return engine.Decision[DrawingResult]{AdvancePhase: true, Result: DrawingResult{ByPlayer: cloneDrawings(p.drawings)}}, nil
	}
	return engine.Decision[DrawingResult]{AdvancePhase: false}, nil
}

func (p *drawPrimitive) Timeout(ctx *engine.PhaseContext) (engine.Decision[DrawingResult], error) {
	setup, ok := engine.GetResult[SetupResult](ctx.State, fmt.Sprintf("draw_duel_prompt_r%d", p.round))
	if !ok {
		return engine.Decision[DrawingResult]{}, fmt.Errorf("missing setup result")
	}
	if _, ok := p.drawings[setup.ArtistAID]; !ok {
		p.drawings[setup.ArtistAID] = blankDrawing(setup.ArtistAID)
	}
	if _, ok := p.drawings[setup.ArtistBID]; !ok {
		p.drawings[setup.ArtistBID] = blankDrawing(setup.ArtistBID)
	}
	return engine.Decision[DrawingResult]{AdvancePhase: true, Result: DrawingResult{ByPlayer: cloneDrawings(p.drawings)}}, nil
}

type votePrimitive struct {
	round int
	votes map[string]string
}

func newVote(round int) *votePrimitive { return &votePrimitive{round: round, votes: map[string]string{}} }
func (p *votePrimitive) Name() string  { return fmt.Sprintf("drawduel.vote.r%d", p.round) }

func (p *votePrimitive) Start(ctx *engine.PhaseContext) (engine.Decision[VoteResult], error) {
	setup, ok := engine.GetResult[SetupResult](ctx.State, fmt.Sprintf("draw_duel_prompt_r%d", p.round))
	if !ok {
		return engine.Decision[VoteResult]{}, fmt.Errorf("missing setup result")
	}
	drawings, ok := engine.GetResult[DrawingResult](ctx.State, fmt.Sprintf("draw_duel_draw_r%d", p.round))
	if !ok {
		return engine.Decision[VoteResult]{}, fmt.Errorf("missing drawing result")
	}
	roundPayload, _ := json.Marshal(toRoundPayload(setup))
	drawingsPayload, _ := json.Marshal(proto.DrawingsPayload{
		Round:    p.round,
		Drawings: orderedDrawings(drawings.ByPlayer, setup),
	})
	return engine.Decision[VoteResult]{
		Broadcast: []engine.Event{
			{Type: proto.S2CDrawDuelRound, Payload: roundPayload},
			{Type: proto.S2CDrawings, Payload: drawingsPayload},
		},
	}, nil
}

func (p *votePrimitive) Handle(ctx *engine.PhaseContext, in engine.Input) (engine.Decision[VoteResult], error) {
	setup, ok := engine.GetResult[SetupResult](ctx.State, fmt.Sprintf("draw_duel_prompt_r%d", p.round))
	if !ok {
		return engine.Decision[VoteResult]{}, fmt.Errorf("missing setup result")
	}
	playerID := string(in.PlayerID)
	if playerID == setup.ArtistAID || playerID == setup.ArtistBID {
		return engine.Decision[VoteResult]{}, fmt.Errorf("artists do not judge their own duel")
	}
	var payload proto.SubmitVotePayload
	if err := json.Unmarshal(in.Value, &payload); err != nil {
		return engine.Decision[VoteResult]{}, err
	}
	if payload.ChoiceID != setup.ArtistAID && payload.ChoiceID != setup.ArtistBID {
		return engine.Decision[VoteResult]{}, fmt.Errorf("vote must target one of the duel drawings")
	}
	p.votes[playerID] = payload.ChoiceID
	if len(p.votes) >= len(setup.JudgeIDs) {
		return engine.Decision[VoteResult]{AdvancePhase: true, Result: VoteResult{ByJudge: cloneVotes(p.votes)}}, nil
	}
	return engine.Decision[VoteResult]{AdvancePhase: false}, nil
}

func (p *votePrimitive) Timeout(*engine.PhaseContext) (engine.Decision[VoteResult], error) {
	return engine.Decision[VoteResult]{AdvancePhase: true, Result: VoteResult{ByJudge: cloneVotes(p.votes)}}, nil
}

type revealPrimitive struct {
	round    int
	resolved bool
	cached   proto.DrawDuelRevealPayload
}

func newReveal(round int) *revealPrimitive { return &revealPrimitive{round: round} }
func (p *revealPrimitive) Name() string    { return fmt.Sprintf("drawduel.reveal.r%d", p.round) }

func (p *revealPrimitive) Start(ctx *engine.PhaseContext) (engine.Decision[proto.DrawDuelRevealPayload], error) {
	reveal, deltas, scores, err := resolveReveal(ctx.State, p.round)
	if err != nil {
		return engine.Decision[proto.DrawDuelRevealPayload]{}, err
	}
	p.cached = reveal
	p.resolved = true
	revealJSON, _ := json.Marshal(reveal)
	roundJSON, _ := json.Marshal(proto.RoundResultPayload{
		Round:  p.round,
		Deltas: deltas,
		Scores: scores,
	})
	return engine.Decision[proto.DrawDuelRevealPayload]{
		Broadcast: []engine.Event{
			{Type: proto.S2CDrawDuelReveal, Payload: revealJSON},
			{Type: proto.S2CRoundResult, Payload: roundJSON},
		},
	}, nil
}

func (*revealPrimitive) Handle(*engine.PhaseContext, engine.Input) (engine.Decision[proto.DrawDuelRevealPayload], error) {
	return engine.Decision[proto.DrawDuelRevealPayload]{AdvancePhase: false}, nil
}

func (p *revealPrimitive) Timeout(ctx *engine.PhaseContext) (engine.Decision[proto.DrawDuelRevealPayload], error) {
	if !p.resolved {
		reveal, _, _, err := resolveReveal(ctx.State, p.round)
		if err != nil {
			return engine.Decision[proto.DrawDuelRevealPayload]{}, err
		}
		p.cached = reveal
		p.resolved = true
	}
	return engine.Decision[proto.DrawDuelRevealPayload]{AdvancePhase: true, Result: p.cached}, nil
}

func resolveReveal(state *engine.GameState, round int) (proto.DrawDuelRevealPayload, map[string]int, map[string]int, error) {
	setup, ok := engine.GetResult[SetupResult](state, fmt.Sprintf("draw_duel_prompt_r%d", round))
	if !ok {
		return proto.DrawDuelRevealPayload{}, nil, nil, fmt.Errorf("missing setup result")
	}
	drawings, ok := engine.GetResult[DrawingResult](state, fmt.Sprintf("draw_duel_draw_r%d", round))
	if !ok {
		return proto.DrawDuelRevealPayload{}, nil, nil, fmt.Errorf("missing drawing result")
	}
	votes, ok := engine.GetResult[VoteResult](state, fmt.Sprintf("draw_duel_vote_r%d", round))
	if !ok {
		return proto.DrawDuelRevealPayload{}, nil, nil, fmt.Errorf("missing vote result")
	}
	counts := map[string]int{
		setup.ArtistAID: 0,
		setup.ArtistBID: 0,
	}
	for _, drawingID := range votes.ByJudge {
		counts[drawingID]++
	}
	deltas := map[string]int{}
	reveal := proto.DrawDuelRevealPayload{
		Round:           round,
		Prompt:          setup.Prompt,
		ArtistAID:       setup.ArtistAID,
		ArtistAName:     setup.ArtistAName,
		ArtistBID:       setup.ArtistBID,
		ArtistBName:     setup.ArtistBName,
		Drawings:        orderedDrawings(drawings.ByPlayer, setup),
		VotesByJudge:    cloneVotes(votes.ByJudge),
		VoteCountByDraw: counts,
		Tied:            counts[setup.ArtistAID] == counts[setup.ArtistBID],
	}
	if reveal.Tied {
		deltas[setup.ArtistAID] = 500
		deltas[setup.ArtistBID] = 500
		state.Scores[engine.PlayerID(setup.ArtistAID)] += 500
		state.Scores[engine.PlayerID(setup.ArtistBID)] += 500
	} else {
		winnerID := setup.ArtistAID
		winnerName := setup.ArtistAName
		if counts[setup.ArtistBID] > counts[setup.ArtistAID] {
			winnerID = setup.ArtistBID
			winnerName = setup.ArtistBName
		}
		reveal.WinnerDrawingID = winnerID
		reveal.WinnerArtistID = winnerID
		reveal.WinnerArtistName = winnerName
		deltas[winnerID] += 1000
		state.Scores[engine.PlayerID(winnerID)] += 1000
		for judgeID, picked := range votes.ByJudge {
			if picked == winnerID {
				deltas[judgeID] += 500
				state.Scores[engine.PlayerID(judgeID)] += 500
			}
		}
	}
	scores := map[string]int{}
	for playerID, score := range state.Scores {
		scores[string(playerID)] = score
	}
	return reveal, deltas, scores, nil
}

func buildSetup(state *engine.GameState, round int) SetupResult {
	players := activePlayers(state)
	artistA := players[((round - 1) * 2) % len(players)]
	artistB := players[(((round - 1) * 2) + 1) % len(players)]
	prompt := prompts[(round-1)%len(prompts)]
	out := SetupResult{
		Round:       round,
		Prompt:      prompt,
		ArtistAID:   artistA,
		ArtistAName: state.Players[engine.PlayerID(artistA)].Name,
		ArtistBID:   artistB,
		ArtistBName: state.Players[engine.PlayerID(artistB)].Name,
		JudgeIDs:    []string{},
	}
	for _, playerID := range players {
		if playerID == artistA || playerID == artistB {
			continue
		}
		out.JudgeIDs = append(out.JudgeIDs, playerID)
	}
	return out
}

func orderedDrawings(in map[string]proto.DrawingSummary, setup SetupResult) []proto.DrawingSummary {
	return []proto.DrawingSummary{in[setup.ArtistAID], in[setup.ArtistBID]}
}

func toRoundPayload(setup SetupResult) proto.DrawDuelRoundPayload {
	return proto.DrawDuelRoundPayload{
		Round:       setup.Round,
		Prompt:      setup.Prompt,
		ArtistAID:   setup.ArtistAID,
		ArtistAName: setup.ArtistAName,
		ArtistBID:   setup.ArtistBID,
		ArtistBName: setup.ArtistBName,
		JudgeIDs:    append([]string(nil), setup.JudgeIDs...),
	}
}

func activePlayers(state *engine.GameState) []string {
	type namedPlayer struct {
		id   string
		name string
	}
	out := make([]namedPlayer, 0, len(state.Players))
	for _, playerID := range state.ActivePlayers() {
		player := state.Players[playerID]
		out = append(out, namedPlayer{id: string(playerID), name: player.Name})
	}
	slices.SortFunc(out, func(a, b namedPlayer) int {
		if a.name < b.name {
			return -1
		}
		if a.name > b.name {
			return 1
		}
		if a.id < b.id {
			return -1
		}
		if a.id > b.id {
			return 1
		}
		return 0
	})
	ids := make([]string, 0, len(out))
	for _, player := range out {
		ids = append(ids, player.id)
	}
	return ids
}

func blankDrawing(playerID string) proto.DrawingSummary {
	return proto.DrawingSummary{
		DrawingID: playerID,
		AuthorID:  playerID,
		Format:    "strokes",
		Data:      `{"strokes":[]}`,
	}
}

func cloneDrawings(in map[string]proto.DrawingSummary) map[string]proto.DrawingSummary {
	out := make(map[string]proto.DrawingSummary, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneVotes(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
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

var prompts = []string{
	"Draw the most dramatic way to eat a sandwich.",
	"Draw a superhero having a terrible day.",
	"Draw the weirdest pet you can imagine.",
	"Draw a school picture gone completely wrong.",
	"Draw someone trying to act cool and failing.",
	"Draw the ultimate road-trip disaster.",
	"Draw a cartoon character stuck at the DMV.",
	"Draw a villain whose evil plan is deeply silly.",
}
