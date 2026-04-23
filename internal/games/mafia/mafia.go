package mafia

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jj/trivia/internal/engine"
	"github.com/jj/trivia/internal/proto"
)

const (
	regKeyRoleAssign  = "mafia.role_assign"
	regKeyNightAction = "mafia.night_action"
	regKeyNightReveal = "mafia.night_reveal"
	regKeyDayDiscuss  = "mafia.day_discuss"
	regKeyDayNominate = "mafia.day_nominate"
	regKeyDayVote     = "mafia.day_vote"
	regKeyDayRevote   = "mafia.day_revote"
	regKeyDayReveal   = "mafia.day_reveal"

	roleAssignSeconds = 8
	revealSeconds     = 6
	revoteSeconds     = 60

	defaultNightSecs   = 20
	defaultDiscussSecs = 180
	defaultVoteSecs    = 25
)

const (
	roleCitizen   = "citizen"
	roleMafia     = "mafia"
	roleDetective = "detective"
	roleDoctor    = "doctor"
	roleMayor     = "mayor"
)

const (
	tieRuleNoElimination = "no_elimination"
	tieRuleRevote        = "revote"
	tieRuleMayorBreaks   = "mayor_breaks"

	investigationFaction   = "faction"
	investigationExactRole = "exact_role"
)

type phaseConfig struct {
	Round int `json:"round"`
}

type LobbyConfig struct {
	GameID             string            `json:"game_id"`
	PlayerCount        int               `json:"player_count"`
	Roles              []string          `json:"roles"`
	RoleCounts         map[string]int    `json:"role_counts"`
	RevealOnDeath      bool              `json:"reveal_on_death"`
	TieRule            string            `json:"tie_rule"`
	SelfProtect        bool              `json:"self_protect"`
	InvestigationMode  string            `json:"investigation_mode"`
	NominationMinimum  int               `json:"nomination_minimum"`
	DayVoteThreshold   int               `json:"day_vote_threshold"`
	RecommendedSeconds map[string]int    `json:"recommended_seconds,omitempty"`
	GameOptions        map[string]string `json:"game_options,omitempty"`
}

type RoleAssignmentResult struct {
	Order        []string          `json:"order"`
	Names        map[string]string `json:"names"`
	Roles        map[string]string `json:"roles"`
	MafiaIDs     []string          `json:"mafia_ids"`
	DetectiveIDs []string          `json:"detective_ids"`
	DoctorIDs    []string          `json:"doctor_ids"`
	MayorID      string            `json:"mayor_id"`
	Config       LobbyConfig       `json:"config"`
}

type NightActionResult struct {
	Choices          map[string]string `json:"choices"`
	MafiaChoices     map[string]string `json:"mafia_choices"`
	DoctorChoices    map[string]string `json:"doctor_choices"`
	DetectiveChoices map[string]string `json:"detective_choices"`
}

type InvestigationResult struct {
	TargetID   string `json:"target_id"`
	TargetName string `json:"target_name"`
	Result     string `json:"result"`
}

type NightRevealResult struct {
	Payload           proto.MafiaRevealPayload       `json:"payload"`
	ProtectedID       string                         `json:"protected_id"`
	ProtectionWorked  bool                           `json:"protection_worked"`
	DoctorFeedback    map[string]string              `json:"doctor_feedback"`
	DetectiveFindings map[string]InvestigationResult `json:"detective_findings"`
}

type NominationResult struct {
	Nominations map[string]string `json:"nominations"`
}

type DayVoteResult struct {
	Votes map[string]string `json:"votes"`
}

type DayRevoteResult struct {
	Votes map[string]string `json:"votes"`
}

var registerOnce sync.Once

func Register() {
	registerOnce.Do(func() {
		engine.Register(regKeyRoleAssign, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			return engine.Erase(newRoleAssign()), nil
		})
		engine.Register(regKeyNightAction, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, err := readRound(cfg)
			if err != nil {
				return nil, err
			}
			return engine.Erase(newNightAction(round)), nil
		})
		engine.Register(regKeyNightReveal, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, err := readRound(cfg)
			if err != nil {
				return nil, err
			}
			return engine.Erase(newNightReveal(round)), nil
		})
		engine.Register(regKeyDayDiscuss, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, err := readRound(cfg)
			if err != nil {
				return nil, err
			}
			return engine.Erase(newDayDiscuss(round)), nil
		})
		engine.Register(regKeyDayNominate, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, err := readRound(cfg)
			if err != nil {
				return nil, err
			}
			return engine.Erase(newDayNominate(round)), nil
		})
		engine.Register(regKeyDayVote, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, err := readRound(cfg)
			if err != nil {
				return nil, err
			}
			return engine.Erase(newDayVote(round)), nil
		})
		engine.Register(regKeyDayRevote, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, err := readRound(cfg)
			if err != nil {
				return nil, err
			}
			return engine.Erase(newDayRevote(round)), nil
		})
		engine.Register(regKeyDayReveal, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, err := readRound(cfg)
			if err != nil {
				return nil, err
			}
			return engine.Erase(newDayReveal(round)), nil
		})
	})
}

func BuildLobbyConfig(playerCount int, overrides map[string]any) LobbyConfig {
	if playerCount < 5 {
		playerCount = 5
	}
	roleList := buildRoleList(playerCount)
	roleCounts := map[string]int{}
	for _, role := range roleList {
		roleCounts[role]++
	}

	cfg := LobbyConfig{
		GameID:            "mafia",
		PlayerCount:       playerCount,
		Roles:             append([]string(nil), roleList...),
		RoleCounts:        roleCounts,
		NominationMinimum: 2,
		DayVoteThreshold:  (playerCount + 1) / 2,
		RecommendedSeconds: map[string]int{
			"night":      defaultNightSecs,
			"discussion": defaultDiscussSecs,
			"vote":       defaultVoteSecs,
			"revote":     revoteSeconds,
		},
		GameOptions: map[string]string{},
	}

	switch {
	case playerCount <= 6:
		cfg.RevealOnDeath = true
		cfg.TieRule = tieRuleNoElimination
		cfg.SelfProtect = false
		cfg.InvestigationMode = investigationFaction
	case playerCount <= 8:
		cfg.RevealOnDeath = true
		cfg.TieRule = tieRuleRevote
		cfg.SelfProtect = true
		cfg.InvestigationMode = investigationFaction
	case playerCount <= 10:
		cfg.RevealOnDeath = true
		cfg.TieRule = tieRuleMayorBreaks
		cfg.SelfProtect = true
		cfg.InvestigationMode = investigationFaction
	default:
		cfg.RevealOnDeath = false
		cfg.TieRule = tieRuleRevote
		cfg.SelfProtect = true
		cfg.InvestigationMode = investigationExactRole
	}

	cfg.RevealOnDeath = boolOption(overrides, "reveal_on_death", cfg.RevealOnDeath)
	cfg.SelfProtect = boolOption(overrides, "self_protect", cfg.SelfProtect)
	cfg.TieRule = normalizeTieRule(stringOption(overrides, "tie_rule", cfg.TieRule))
	cfg.InvestigationMode = normalizeInvestigationMode(stringOption(overrides, "investigation_mode", cfg.InvestigationMode))
	cfg.GameOptions["reveal_on_death"] = fmt.Sprintf("%t", cfg.RevealOnDeath)
	cfg.GameOptions["self_protect"] = fmt.Sprintf("%t", cfg.SelfProtect)
	cfg.GameOptions["tie_rule"] = cfg.TieRule
	cfg.GameOptions["investigation_mode"] = cfg.InvestigationMode
	return cfg
}

func BuildPhases(state *engine.GameState) []engine.Phase {
	rounds := state.Settings.RoundCount
	if rounds <= 0 {
		rounds = 10
	}
	nightSeconds := state.Settings.DrawingSeconds
	if nightSeconds <= 0 {
		nightSeconds = defaultNightSecs
	}
	discussSeconds := state.Settings.FakePromptSeconds
	if discussSeconds <= 0 {
		discussSeconds = defaultDiscussSecs
	}
	voteSeconds := state.Settings.VotingSeconds
	if voteSeconds <= 0 {
		voteSeconds = defaultVoteSecs
	}

	phases := make([]engine.Phase, 0, rounds*7+1)
	phases = append(phases, engine.Phase{
		Name:      "mafia_role_assign",
		Primitive: regKeyRoleAssign,
		Duration:  roleAssignSeconds * time.Second,
	})
	for round := 1; round <= rounds; round++ {
		cfg, _ := json.Marshal(phaseConfig{Round: round})
		nightDepends := []string{"mafia_role_assign"}
		if round > 1 {
			nightDepends = append(nightDepends, fmt.Sprintf("mafia_day_reveal_r%d", round-1))
		}
		phases = append(phases,
			engine.Phase{
				Name:      fmt.Sprintf("mafia_night_collect_r%d", round),
				Primitive: regKeyNightAction,
				Duration:  time.Duration(nightSeconds) * time.Second,
				Config:    cfg,
				DependsOn: nightDepends,
			},
			engine.Phase{
				Name:      fmt.Sprintf("mafia_night_reveal_r%d", round),
				Primitive: regKeyNightReveal,
				Duration:  revealSeconds * time.Second,
				Config:    cfg,
				DependsOn: []string{"mafia_role_assign", fmt.Sprintf("mafia_night_collect_r%d", round)},
			},
			engine.Phase{
				Name:      fmt.Sprintf("mafia_day_discuss_r%d", round),
				Primitive: regKeyDayDiscuss,
				Duration:  time.Duration(discussSeconds) * time.Second,
				Config:    cfg,
				DependsOn: []string{"mafia_role_assign", fmt.Sprintf("mafia_night_reveal_r%d", round)},
			},
			engine.Phase{
				Name:      fmt.Sprintf("mafia_day_nominate_r%d", round),
				Primitive: regKeyDayNominate,
				Duration:  time.Duration(voteSeconds) * time.Second,
				Config:    cfg,
				DependsOn: []string{"mafia_role_assign", fmt.Sprintf("mafia_night_reveal_r%d", round)},
			},
			engine.Phase{
				Name:      fmt.Sprintf("mafia_day_vote_r%d", round),
				Primitive: regKeyDayVote,
				Duration:  time.Duration(voteSeconds) * time.Second,
				Config:    cfg,
				DependsOn: []string{"mafia_role_assign", fmt.Sprintf("mafia_day_nominate_r%d", round)},
			},
			engine.Phase{
				Name:      fmt.Sprintf("mafia_day_revote_r%d", round),
				Primitive: regKeyDayRevote,
				Duration:  revoteSeconds * time.Second,
				Config:    cfg,
				DependsOn: []string{"mafia_role_assign", fmt.Sprintf("mafia_day_vote_r%d", round)},
			},
			engine.Phase{
				Name:      fmt.Sprintf("mafia_day_reveal_r%d", round),
				Primitive: regKeyDayReveal,
				Duration:  revealSeconds * time.Second,
				Config:    cfg,
				DependsOn: []string{"mafia_role_assign", fmt.Sprintf("mafia_day_vote_r%d", round), fmt.Sprintf("mafia_day_revote_r%d", round)},
			},
		)
	}
	return phases
}

type roleAssignPrimitive struct {
	resolved bool
	cached   RoleAssignmentResult
}

func newRoleAssign() *roleAssignPrimitive { return &roleAssignPrimitive{} }
func (p *roleAssignPrimitive) Name() string {
	return "mafia.role_assign"
}

func (p *roleAssignPrimitive) Start(ctx *engine.PhaseContext) (engine.Decision[RoleAssignmentResult], error) {
	if !p.resolved {
		p.cached = assignRoles(ctx.State)
		p.resolved = true
	}
	return engine.Decision[RoleAssignmentResult]{
		Broadcast: roleAssignEvents(p.cached),
	}, nil
}

func (*roleAssignPrimitive) Handle(*engine.PhaseContext, engine.Input) (engine.Decision[RoleAssignmentResult], error) {
	return engine.Decision[RoleAssignmentResult]{AdvancePhase: false}, nil
}

func (p *roleAssignPrimitive) Timeout(*engine.PhaseContext) (engine.Decision[RoleAssignmentResult], error) {
	return engine.Decision[RoleAssignmentResult]{
		AdvancePhase: true,
		Result:       p.cached,
	}, nil
}

type nightActionPrimitive struct {
	round   int
	choices map[string]string
}

func newNightAction(round int) *nightActionPrimitive {
	return &nightActionPrimitive{round: round, choices: map[string]string{}}
}

func (p *nightActionPrimitive) Name() string { return fmt.Sprintf("mafia.night_action.r%d", p.round) }

func (p *nightActionPrimitive) Start(ctx *engine.PhaseContext) (engine.Decision[NightActionResult], error) {
	roleResult, ok := getRoleResult(ctx.State)
	if !ok {
		return engine.Decision[NightActionResult]{}, fmt.Errorf("missing role assignment")
	}
	alive := aliveBeforeNight(ctx.State, p.round)
	return engine.Decision[NightActionResult]{
		Broadcast: nightCollectEvents(roleResult, p.round, alive, p.choices),
	}, nil
}

func (p *nightActionPrimitive) Handle(ctx *engine.PhaseContext, in engine.Input) (engine.Decision[NightActionResult], error) {
	roleResult, ok := getRoleResult(ctx.State)
	if !ok {
		return engine.Decision[NightActionResult]{}, fmt.Errorf("missing role assignment")
	}
	playerID := string(in.PlayerID)
	alive := aliveBeforeNight(ctx.State, p.round)
	if !alive[playerID] {
		return engine.Decision[NightActionResult]{}, fmt.Errorf("only living players can act")
	}

	var payload proto.SubmitVotePayload
	if err := json.Unmarshal(in.Value, &payload); err != nil {
		return engine.Decision[NightActionResult]{}, err
	}
	targets := targetIDsForNight(roleResult, alive, playerID)
	if !slices.Contains(targets, payload.ChoiceID) {
		return engine.Decision[NightActionResult]{}, fmt.Errorf("invalid night target")
	}
	p.choices[playerID] = payload.ChoiceID

	update, _ := json.Marshal(buildNightStatePayload(roleResult, p.round, alive, p.choices, playerID))
	decision := engine.Decision[NightActionResult]{
		Broadcast: []engine.Event{{Type: proto.S2CMafiaState, Payload: update}},
	}
	if len(p.choices) >= livingNightActorCount(roleResult, alive) {
		decision.AdvancePhase = true
		decision.Result = buildNightActionResult(roleResult, alive, p.choices)
	}
	return decision, nil
}

func (p *nightActionPrimitive) Timeout(ctx *engine.PhaseContext) (engine.Decision[NightActionResult], error) {
	roleResult, ok := getRoleResult(ctx.State)
	if !ok {
		return engine.Decision[NightActionResult]{}, fmt.Errorf("missing role assignment")
	}
	alive := aliveBeforeNight(ctx.State, p.round)
	return engine.Decision[NightActionResult]{
		AdvancePhase: true,
		Result:       buildNightActionResult(roleResult, alive, p.choices),
	}, nil
}

type nightRevealPrimitive struct {
	round    int
	resolved bool
	cached   NightRevealResult
}

func newNightReveal(round int) *nightRevealPrimitive { return &nightRevealPrimitive{round: round} }
func (p *nightRevealPrimitive) Name() string         { return fmt.Sprintf("mafia.night_reveal.r%d", p.round) }

func (p *nightRevealPrimitive) Start(ctx *engine.PhaseContext) (engine.Decision[NightRevealResult], error) {
	if !p.resolved {
		reveal, winner, err := resolveNightReveal(ctx.State, p.round)
		if err != nil {
			return engine.Decision[NightRevealResult]{}, err
		}
		p.cached = reveal
		p.resolved = true
		if winner != "" {
			awardWinners(ctx.State, winner, reveal.Payload.RoleMap)
		}
	}
	raw, _ := json.Marshal(p.cached.Payload)
	return engine.Decision[NightRevealResult]{
		Broadcast: []engine.Event{{Type: proto.S2CMafiaReveal, Payload: raw}},
	}, nil
}

func (*nightRevealPrimitive) Handle(*engine.PhaseContext, engine.Input) (engine.Decision[NightRevealResult], error) {
	return engine.Decision[NightRevealResult]{AdvancePhase: false}, nil
}

func (p *nightRevealPrimitive) Timeout(ctx *engine.PhaseContext) (engine.Decision[NightRevealResult], error) {
	if p.cached.Payload.Winner != "" {
		ctx.State.EndAfterAdvance = true
	}
	return engine.Decision[NightRevealResult]{
		AdvancePhase: true,
		Result:       p.cached,
	}, nil
}

type dayDiscussPrimitive struct {
	round int
}

func newDayDiscuss(round int) *dayDiscussPrimitive { return &dayDiscussPrimitive{round: round} }
func (p *dayDiscussPrimitive) Name() string        { return fmt.Sprintf("mafia.day_discuss.r%d", p.round) }

func (p *dayDiscussPrimitive) Start(ctx *engine.PhaseContext) (engine.Decision[struct{}], error) {
	roleResult, ok := getRoleResult(ctx.State)
	if !ok {
		return engine.Decision[struct{}]{}, fmt.Errorf("missing role assignment")
	}
	alive := aliveBeforeDay(ctx.State, p.round)
	nightReveal, _ := getNightRevealResult(ctx.State, p.round)
	return engine.Decision[struct{}]{
		Broadcast: dayDiscussEvents(roleResult, p.round, alive, nightReveal),
	}, nil
}

func (*dayDiscussPrimitive) Handle(*engine.PhaseContext, engine.Input) (engine.Decision[struct{}], error) {
	return engine.Decision[struct{}]{AdvancePhase: false}, nil
}

func (*dayDiscussPrimitive) Timeout(*engine.PhaseContext) (engine.Decision[struct{}], error) {
	return engine.Decision[struct{}]{AdvancePhase: true}, nil
}

type dayNominatePrimitive struct {
	round       int
	nominations map[string]string
}

func newDayNominate(round int) *dayNominatePrimitive {
	return &dayNominatePrimitive{round: round, nominations: map[string]string{}}
}

func (p *dayNominatePrimitive) Name() string { return fmt.Sprintf("mafia.day_nominate.r%d", p.round) }

func (p *dayNominatePrimitive) Start(ctx *engine.PhaseContext) (engine.Decision[NominationResult], error) {
	roleResult, ok := getRoleResult(ctx.State)
	if !ok {
		return engine.Decision[NominationResult]{}, fmt.Errorf("missing role assignment")
	}
	alive := aliveBeforeDay(ctx.State, p.round)
	return engine.Decision[NominationResult]{
		Broadcast: dayNominateEvents(roleResult, p.round, alive, p.nominations),
	}, nil
}

func (p *dayNominatePrimitive) Handle(ctx *engine.PhaseContext, in engine.Input) (engine.Decision[NominationResult], error) {
	roleResult, ok := getRoleResult(ctx.State)
	if !ok {
		return engine.Decision[NominationResult]{}, fmt.Errorf("missing role assignment")
	}
	playerID := string(in.PlayerID)
	alive := aliveBeforeDay(ctx.State, p.round)
	if !alive[playerID] {
		return engine.Decision[NominationResult]{}, fmt.Errorf("only living players can nominate")
	}
	var payload proto.SubmitVotePayload
	if err := json.Unmarshal(in.Value, &payload); err != nil {
		return engine.Decision[NominationResult]{}, err
	}
	targets := targetIDsForNomination(alive, playerID)
	if !slices.Contains(targets, payload.ChoiceID) {
		return engine.Decision[NominationResult]{}, fmt.Errorf("invalid nomination target")
	}
	p.nominations[playerID] = payload.ChoiceID

	update, _ := json.Marshal(buildNominationStatePayload(roleResult, p.round, alive, p.nominations, playerID))
	decision := engine.Decision[NominationResult]{
		Broadcast: []engine.Event{{Type: proto.S2CMafiaState, Payload: update}},
	}
	if len(p.nominations) >= livingCount(alive) {
		decision.AdvancePhase = true
		decision.Result = NominationResult{Nominations: cloneStringMap(p.nominations)}
	}
	return decision, nil
}

func (p *dayNominatePrimitive) Timeout(_ *engine.PhaseContext) (engine.Decision[NominationResult], error) {
	return engine.Decision[NominationResult]{
		AdvancePhase: true,
		Result:       NominationResult{Nominations: cloneStringMap(p.nominations)},
	}, nil
}

type dayVotePrimitive struct {
	round int
	votes map[string]string
}

func newDayVote(round int) *dayVotePrimitive {
	return &dayVotePrimitive{round: round, votes: map[string]string{}}
}

func (p *dayVotePrimitive) Name() string { return fmt.Sprintf("mafia.day_vote.r%d", p.round) }

func (p *dayVotePrimitive) Start(ctx *engine.PhaseContext) (engine.Decision[DayVoteResult], error) {
	roleResult, ok := getRoleResult(ctx.State)
	if !ok {
		return engine.Decision[DayVoteResult]{}, fmt.Errorf("missing role assignment")
	}
	alive := aliveBeforeDay(ctx.State, p.round)
	candidates := nominatedCandidates(ctx.State, p.round, roleResult)
	decision := engine.Decision[DayVoteResult]{
		Broadcast: dayVoteEvents(roleResult, p.round, alive, candidates, p.votes),
	}
	if len(candidates) == 0 {
		decision.AdvancePhase = true
		decision.Result = DayVoteResult{Votes: map[string]string{}}
	}
	return decision, nil
}

func (p *dayVotePrimitive) Handle(ctx *engine.PhaseContext, in engine.Input) (engine.Decision[DayVoteResult], error) {
	roleResult, ok := getRoleResult(ctx.State)
	if !ok {
		return engine.Decision[DayVoteResult]{}, fmt.Errorf("missing role assignment")
	}
	playerID := string(in.PlayerID)
	alive := aliveBeforeDay(ctx.State, p.round)
	if !alive[playerID] {
		return engine.Decision[DayVoteResult]{}, fmt.Errorf("only living players can vote")
	}
	candidates := nominatedCandidates(ctx.State, p.round, roleResult)
	if len(candidates) == 0 {
		return engine.Decision[DayVoteResult]{}, fmt.Errorf("no nominated candidates")
	}

	var payload proto.SubmitVotePayload
	if err := json.Unmarshal(in.Value, &payload); err != nil {
		return engine.Decision[DayVoteResult]{}, err
	}
	if !slices.Contains(candidates, payload.ChoiceID) {
		return engine.Decision[DayVoteResult]{}, fmt.Errorf("invalid vote target")
	}
	p.votes[playerID] = payload.ChoiceID

	update, _ := json.Marshal(buildDayVoteStatePayload(roleResult, p.round, alive, candidates, p.votes, playerID))
	decision := engine.Decision[DayVoteResult]{
		Broadcast: []engine.Event{{Type: proto.S2CMafiaState, Payload: update}},
	}
	if len(p.votes) >= livingCount(alive) {
		decision.AdvancePhase = true
		decision.Result = DayVoteResult{Votes: cloneStringMap(p.votes)}
	}
	return decision, nil
}

func (p *dayVotePrimitive) Timeout(_ *engine.PhaseContext) (engine.Decision[DayVoteResult], error) {
	return engine.Decision[DayVoteResult]{
		AdvancePhase: true,
		Result:       DayVoteResult{Votes: cloneStringMap(p.votes)},
	}, nil
}

type dayRevotePrimitive struct {
	round int
	votes map[string]string
}

func newDayRevote(round int) *dayRevotePrimitive {
	return &dayRevotePrimitive{round: round, votes: map[string]string{}}
}

func (p *dayRevotePrimitive) Name() string { return fmt.Sprintf("mafia.day_revote.r%d", p.round) }

func (p *dayRevotePrimitive) Start(ctx *engine.PhaseContext) (engine.Decision[DayRevoteResult], error) {
	roleResult, ok := getRoleResult(ctx.State)
	if !ok {
		return engine.Decision[DayRevoteResult]{}, fmt.Errorf("missing role assignment")
	}
	alive := aliveBeforeDay(ctx.State, p.round)
	decisionInfo, err := revoteDecision(ctx.State, p.round, roleResult, alive)
	if err != nil {
		return engine.Decision[DayRevoteResult]{}, err
	}
	decision := engine.Decision[DayRevoteResult]{
		Broadcast: dayRevoteEvents(roleResult, p.round, alive, decisionInfo, p.votes),
	}
	if len(decisionInfo.CandidateIDs) == 0 || (!decisionInfo.MayorOnly && len(decisionInfo.CandidateIDs) < 2) {
		decision.AdvancePhase = true
		decision.Result = DayRevoteResult{Votes: map[string]string{}}
	}
	return decision, nil
}

func (p *dayRevotePrimitive) Handle(ctx *engine.PhaseContext, in engine.Input) (engine.Decision[DayRevoteResult], error) {
	roleResult, ok := getRoleResult(ctx.State)
	if !ok {
		return engine.Decision[DayRevoteResult]{}, fmt.Errorf("missing role assignment")
	}
	alive := aliveBeforeDay(ctx.State, p.round)
	playerID := string(in.PlayerID)
	if !alive[playerID] {
		return engine.Decision[DayRevoteResult]{}, fmt.Errorf("only living players can vote")
	}
	decisionInfo, err := revoteDecision(ctx.State, p.round, roleResult, alive)
	if err != nil {
		return engine.Decision[DayRevoteResult]{}, err
	}
	if decisionInfo.MayorOnly && playerID != roleResult.MayorID {
		return engine.Decision[DayRevoteResult]{}, fmt.Errorf("only the mayor can break this tie")
	}
	var payload proto.SubmitVotePayload
	if err := json.Unmarshal(in.Value, &payload); err != nil {
		return engine.Decision[DayRevoteResult]{}, err
	}
	if !slices.Contains(decisionInfo.CandidateIDs, payload.ChoiceID) {
		return engine.Decision[DayRevoteResult]{}, fmt.Errorf("invalid revote target")
	}
	p.votes[playerID] = payload.ChoiceID

	update, _ := json.Marshal(buildDayRevoteStatePayload(roleResult, p.round, alive, decisionInfo, p.votes, playerID))
	decision := engine.Decision[DayRevoteResult]{
		Broadcast: []engine.Event{{Type: proto.S2CMafiaState, Payload: update}},
	}
	if decisionInfo.MayorOnly || len(p.votes) >= livingCount(alive) {
		decision.AdvancePhase = true
		decision.Result = DayRevoteResult{Votes: cloneStringMap(p.votes)}
	}
	return decision, nil
}

func (p *dayRevotePrimitive) Timeout(_ *engine.PhaseContext) (engine.Decision[DayRevoteResult], error) {
	return engine.Decision[DayRevoteResult]{
		AdvancePhase: true,
		Result:       DayRevoteResult{Votes: cloneStringMap(p.votes)},
	}, nil
}

type dayRevealPrimitive struct {
	round    int
	resolved bool
	cached   proto.MafiaRevealPayload
}

func newDayReveal(round int) *dayRevealPrimitive { return &dayRevealPrimitive{round: round} }
func (p *dayRevealPrimitive) Name() string       { return fmt.Sprintf("mafia.day_reveal.r%d", p.round) }

func (p *dayRevealPrimitive) Start(ctx *engine.PhaseContext) (engine.Decision[proto.MafiaRevealPayload], error) {
	if !p.resolved {
		reveal, winner, err := resolveDayReveal(ctx.State, p.round)
		if err != nil {
			return engine.Decision[proto.MafiaRevealPayload]{}, err
		}
		p.cached = reveal
		p.resolved = true
		if winner != "" {
			awardWinners(ctx.State, winner, reveal.RoleMap)
		}
	}
	raw, _ := json.Marshal(p.cached)
	return engine.Decision[proto.MafiaRevealPayload]{
		Broadcast: []engine.Event{{Type: proto.S2CMafiaReveal, Payload: raw}},
	}, nil
}

func (*dayRevealPrimitive) Handle(*engine.PhaseContext, engine.Input) (engine.Decision[proto.MafiaRevealPayload], error) {
	return engine.Decision[proto.MafiaRevealPayload]{AdvancePhase: false}, nil
}

func (p *dayRevealPrimitive) Timeout(ctx *engine.PhaseContext) (engine.Decision[proto.MafiaRevealPayload], error) {
	if p.cached.Winner != "" {
		ctx.State.EndAfterAdvance = true
	}
	return engine.Decision[proto.MafiaRevealPayload]{
		AdvancePhase: true,
		Result:       p.cached,
	}, nil
}

func assignRoles(state *engine.GameState) RoleAssignmentResult {
	players := orderedPlayers(state)
	config := BuildLobbyConfig(len(players), state.Settings.GameOptions)
	shuffled := append([]string(nil), players...)
	rng := rand.New(rand.NewSource(int64(seedFor(state, players))))
	rng.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	names := map[string]string{}
	roles := map[string]string{}
	for _, playerID := range players {
		names[playerID] = state.Players[engine.PlayerID(playerID)].Name
		roles[playerID] = roleCitizen
	}

	mafiaIDs := []string{}
	detectiveIDs := []string{}
	doctorIDs := []string{}
	mayorID := ""
	for idx, role := range config.Roles {
		if idx >= len(shuffled) {
			break
		}
		playerID := shuffled[idx]
		roles[playerID] = role
		switch role {
		case roleMafia:
			mafiaIDs = append(mafiaIDs, playerID)
		case roleDetective:
			detectiveIDs = append(detectiveIDs, playerID)
		case roleDoctor:
			doctorIDs = append(doctorIDs, playerID)
		case roleMayor:
			mayorID = playerID
		}
	}

	slices.Sort(mafiaIDs)
	slices.Sort(detectiveIDs)
	slices.Sort(doctorIDs)
	return RoleAssignmentResult{
		Order:        players,
		Names:        names,
		Roles:        roles,
		MafiaIDs:     mafiaIDs,
		DetectiveIDs: detectiveIDs,
		DoctorIDs:    doctorIDs,
		MayorID:      mayorID,
		Config:       config,
	}
}

func resolveNightReveal(state *engine.GameState, round int) (NightRevealResult, string, error) {
	roleResult, ok := getRoleResult(state)
	if !ok {
		return NightRevealResult{}, "", fmt.Errorf("missing role assignment")
	}
	alive := aliveBeforeNight(state, round)
	actions, ok := engine.GetResult[NightActionResult](state, fmt.Sprintf("mafia_night_collect_r%d", round))
	if !ok {
		return NightRevealResult{}, "", fmt.Errorf("missing night actions")
	}

	protectedID := chooseProtectedTarget(actions.DoctorChoices, roleResult, alive)
	killTarget, _ := chooseKillTarget(actions.MafiaChoices, roleResult, alive)
	deaths := []string{}
	protectionWorked := false
	if killTarget != "" {
		if killTarget == protectedID {
			protectionWorked = true
		} else {
			deaths = append(deaths, killTarget)
		}
	}

	findings := map[string]InvestigationResult{}
	for detectiveID, targetID := range actions.DetectiveChoices {
		if targetID == "" {
			continue
		}
		findings[detectiveID] = InvestigationResult{
			TargetID:   targetID,
			TargetName: roleResult.Names[targetID],
			Result:     investigationResult(roleResult.Roles[targetID], roleResult.Config.InvestigationMode),
		}
	}

	doctorFeedback := map[string]string{}
	for doctorID := range actions.DoctorChoices {
		if protectedID == "" {
			doctorFeedback[doctorID] = "Your protection had no effect."
			continue
		}
		if protectionWorked {
			doctorFeedback[doctorID] = fmt.Sprintf("Your protection worked on %s.", roleResult.Names[protectedID])
			continue
		}
		doctorFeedback[doctorID] = fmt.Sprintf("You protected %s, but the attack landed elsewhere.", roleResult.Names[protectedID])
	}

	nextAlive := cloneAlive(alive)
	winner := ""
	for _, death := range deaths {
		nextAlive[death] = false
		if winner == "" {
			winner = winningTeam(nextAlive, roleResult.Roles)
		}
	}
	reveal := proto.MafiaRevealPayload{
		Round:    round,
		Phase:    "night_reveal",
		Deaths:   deaths,
		AliveIDs: aliveIDs(roleResult.Order, nextAlive),
	}
	if winner != "" {
		reveal.Winner = winner
		reveal.RoleMap = cloneStringMap(roleResult.Roles)
	}
	return NightRevealResult{
		Payload:           reveal,
		ProtectedID:       protectedID,
		ProtectionWorked:  protectionWorked,
		DoctorFeedback:    doctorFeedback,
		DetectiveFindings: findings,
	}, winner, nil
}

func resolveDayReveal(state *engine.GameState, round int) (proto.MafiaRevealPayload, string, error) {
	roleResult, ok := getRoleResult(state)
	if !ok {
		return proto.MafiaRevealPayload{}, "", fmt.Errorf("missing role assignment")
	}
	alive := aliveBeforeDay(state, round)
	nominationResult, ok := engine.GetResult[NominationResult](state, fmt.Sprintf("mafia_day_nominate_r%d", round))
	if !ok {
		return proto.MafiaRevealPayload{}, "", fmt.Errorf("missing nomination result")
	}
	dayVoteResult, ok := engine.GetResult[DayVoteResult](state, fmt.Sprintf("mafia_day_vote_r%d", round))
	if !ok {
		return proto.MafiaRevealPayload{}, "", fmt.Errorf("missing day vote result")
	}
	revoteResult, _ := engine.GetResult[DayRevoteResult](state, fmt.Sprintf("mafia_day_revote_r%d", round))

	candidates := nominatedCandidatesFromResult(roleResult, nominationResult)
	primary := resolveVoteRound(roleResult, alive, candidates, dayVoteResult.Votes)
	revoteInfo, err := revoteDecision(state, round, roleResult, alive)
	if err != nil {
		return proto.MafiaRevealPayload{}, "", err
	}

	eliminatedID := ""
	switch {
	case primary.WinnerID != "":
		eliminatedID = primary.WinnerID
	case len(primary.TiedIDs) > 1 && roleResult.Config.TieRule == tieRuleRevote:
		secondary := resolveVoteRound(roleResult, alive, primary.TiedIDs, revoteResult.Votes)
		eliminatedID = secondary.WinnerID
	case len(primary.TiedIDs) > 1 && roleResult.Config.TieRule == tieRuleMayorBreaks && revoteInfo.MayorOnly:
		eliminatedID = revoteResult.Votes[roleResult.MayorID]
	}

	nextAlive := cloneAlive(alive)
	reveal := proto.MafiaRevealPayload{
		Round:    round,
		Phase:    "day_reveal",
		Votes:    cloneStringMap(dayVoteResult.Votes),
		AliveIDs: nil,
	}
	if eliminatedID != "" {
		nextAlive[eliminatedID] = false
		reveal.EliminatedID = eliminatedID
		reveal.EliminatedName = roleResult.Names[eliminatedID]
		if roleResult.Config.RevealOnDeath {
			reveal.EliminatedRole = roleResult.Roles[eliminatedID]
		}
	}
	reveal.AliveIDs = aliveIDs(roleResult.Order, nextAlive)

	winner := ""
	if eliminatedID != "" {
		winner = winningTeam(nextAlive, roleResult.Roles)
	}
	if winner != "" {
		reveal.Winner = winner
		reveal.RoleMap = cloneStringMap(roleResult.Roles)
	}
	return reveal, winner, nil
}

func roleAssignEvents(roleResult RoleAssignmentResult) []engine.Event {
	out := make([]engine.Event, 0, len(roleResult.Order))
	for _, playerID := range roleResult.Order {
		payload := proto.MafiaStatePayload{
			PlayerID:     playerID,
			Round:        0,
			Phase:        "role_assign",
			YourRole:     roleResult.Roles[playerID],
			TeamIDs:      teamIDsFor(roleResult, playerID),
			AlivePlayers: alivePlayersPayload(roleResult, aliveMapFor(roleResult.Order)),
			CanAct:       false,
			TargetIDs:    []string{},
			Note:         roleIntro(roleResult, playerID),
		}
		raw, _ := json.Marshal(payload)
		out = append(out, engine.Event{Type: proto.S2CMafiaState, Payload: raw})
	}
	return out
}

func nightCollectEvents(roleResult RoleAssignmentResult, round int, alive map[string]bool, selections map[string]string) []engine.Event {
	out := make([]engine.Event, 0, len(roleResult.Order))
	for _, playerID := range roleResult.Order {
		payload := buildNightStatePayload(roleResult, round, alive, selections, playerID)
		raw, _ := json.Marshal(payload)
		out = append(out, engine.Event{Type: proto.S2CMafiaState, Payload: raw})
	}
	return out
}

func dayDiscussEvents(roleResult RoleAssignmentResult, round int, alive map[string]bool, nightReveal NightRevealResult) []engine.Event {
	out := make([]engine.Event, 0, len(roleResult.Order))
	for _, playerID := range roleResult.Order {
		note := "Talk through the night and prepare your nominations."
		if finding, ok := nightReveal.DetectiveFindings[playerID]; ok {
			note = fmt.Sprintf("Discuss with the room. Investigation: %s is %s.", finding.TargetName, finding.Result)
		}
		if feedback, ok := nightReveal.DoctorFeedback[playerID]; ok {
			note = feedback
		}
		payload := proto.MafiaStatePayload{
			PlayerID:     playerID,
			Round:        round,
			Phase:        "day_discuss",
			YourRole:     roleResult.Roles[playerID],
			TeamIDs:      teamIDsFor(roleResult, playerID),
			AlivePlayers: alivePlayersPayload(roleResult, alive),
			CanAct:       false,
			TargetIDs:    []string{},
			Note:         note,
		}
		raw, _ := json.Marshal(payload)
		out = append(out, engine.Event{Type: proto.S2CMafiaState, Payload: raw})
	}
	return out
}

func dayNominateEvents(roleResult RoleAssignmentResult, round int, alive map[string]bool, selections map[string]string) []engine.Event {
	out := make([]engine.Event, 0, len(roleResult.Order))
	for _, playerID := range roleResult.Order {
		targets := targetIDsForNomination(alive, playerID)
		payload := proto.MafiaStatePayload{
			PlayerID:     playerID,
			Round:        round,
			Phase:        "day_nominate",
			YourRole:     roleResult.Roles[playerID],
			TeamIDs:      teamIDsFor(roleResult, playerID),
			AlivePlayers: alivePlayersPayload(roleResult, alive),
			CanAct:       alive[playerID] && len(targets) > 0,
			TargetIDs:    targets,
			LockedIn:     selections != nil && selections[playerID] != "",
			Note:         "Nominate one suspect. Anyone with at least 2 nominations reaches the vote.",
		}
		raw, _ := json.Marshal(payload)
		out = append(out, engine.Event{Type: proto.S2CMafiaState, Payload: raw})
	}
	return out
}

func dayVoteEvents(roleResult RoleAssignmentResult, round int, alive map[string]bool, candidates []string, selections map[string]string) []engine.Event {
	out := make([]engine.Event, 0, len(roleResult.Order))
	for _, playerID := range roleResult.Order {
		note := "Vote to eliminate one nominated player."
		if len(candidates) == 0 {
			note = "Nobody received enough nominations. No elimination will happen today."
		} else if roleResult.Roles[playerID] == roleMayor {
			note = "Vote to eliminate one nominated player. Your vote counts as 3."
		}
		payload := proto.MafiaStatePayload{
			PlayerID:     playerID,
			Round:        round,
			Phase:        "day_vote",
			YourRole:     roleResult.Roles[playerID],
			TeamIDs:      teamIDsFor(roleResult, playerID),
			AlivePlayers: alivePlayersPayload(roleResult, alive),
			CanAct:       alive[playerID] && len(candidates) > 0,
			TargetIDs:    append([]string(nil), candidates...),
			LockedIn:     selections != nil && selections[playerID] != "",
			Note:         note,
		}
		raw, _ := json.Marshal(payload)
		out = append(out, engine.Event{Type: proto.S2CMafiaState, Payload: raw})
	}
	return out
}

func dayRevoteEvents(roleResult RoleAssignmentResult, round int, alive map[string]bool, decision revoteInfo, selections map[string]string) []engine.Event {
	out := make([]engine.Event, 0, len(roleResult.Order))
	for _, playerID := range roleResult.Order {
		canAct := false
		targets := []string{}
		note := decision.Note
		switch {
		case len(decision.CandidateIDs) == 0:
			note = "No revote is required."
		case decision.MayorOnly:
			canAct = alive[playerID] && playerID == roleResult.MayorID
			targets = append([]string(nil), decision.CandidateIDs...)
			if playerID != roleResult.MayorID {
				note = "The mayor is breaking the tie."
			}
		default:
			canAct = alive[playerID]
			targets = append([]string(nil), decision.CandidateIDs...)
		}
		payload := proto.MafiaStatePayload{
			PlayerID:     playerID,
			Round:        round,
			Phase:        "day_revote",
			YourRole:     roleResult.Roles[playerID],
			TeamIDs:      teamIDsFor(roleResult, playerID),
			AlivePlayers: alivePlayersPayload(roleResult, alive),
			CanAct:       canAct,
			TargetIDs:    targets,
			LockedIn:     selections != nil && selections[playerID] != "",
			Note:         note,
		}
		raw, _ := json.Marshal(payload)
		out = append(out, engine.Event{Type: proto.S2CMafiaState, Payload: raw})
	}
	return out
}

func buildNightStatePayload(roleResult RoleAssignmentResult, round int, alive map[string]bool, selections map[string]string, playerID string) proto.MafiaStatePayload {
	targets := targetIDsForNight(roleResult, alive, playerID)
	note := "Sleep through the night and wait for dawn."
	if len(targets) > 0 {
		note = nightPrompt(roleResult.Roles[playerID], roleResult.Config)
	}
	return proto.MafiaStatePayload{
		PlayerID:     playerID,
		Round:        round,
		Phase:        "night_collect",
		YourRole:     roleResult.Roles[playerID],
		TeamIDs:      teamIDsFor(roleResult, playerID),
		AlivePlayers: alivePlayersPayload(roleResult, alive),
		CanAct:       alive[playerID] && len(targets) > 0,
		TargetIDs:    targets,
		LockedIn:     selections != nil && selections[playerID] != "",
		Note:         note,
	}
}

func buildNominationStatePayload(roleResult RoleAssignmentResult, round int, alive map[string]bool, selections map[string]string, playerID string) proto.MafiaStatePayload {
	targets := targetIDsForNomination(alive, playerID)
	return proto.MafiaStatePayload{
		PlayerID:     playerID,
		Round:        round,
		Phase:        "day_nominate",
		YourRole:     roleResult.Roles[playerID],
		TeamIDs:      teamIDsFor(roleResult, playerID),
		AlivePlayers: alivePlayersPayload(roleResult, alive),
		CanAct:       alive[playerID] && len(targets) > 0,
		TargetIDs:    targets,
		LockedIn:     selections != nil && selections[playerID] != "",
		Note:         "Nominate one suspect. Anyone with at least 2 nominations reaches the vote.",
	}
}

func buildDayVoteStatePayload(roleResult RoleAssignmentResult, round int, alive map[string]bool, candidates []string, selections map[string]string, playerID string) proto.MafiaStatePayload {
	note := "Vote to eliminate one nominated player."
	if roleResult.Roles[playerID] == roleMayor {
		note = "Vote to eliminate one nominated player. Your vote counts as 3."
	}
	if len(candidates) == 0 {
		note = "Nobody received enough nominations. No elimination will happen today."
	}
	return proto.MafiaStatePayload{
		PlayerID:     playerID,
		Round:        round,
		Phase:        "day_vote",
		YourRole:     roleResult.Roles[playerID],
		TeamIDs:      teamIDsFor(roleResult, playerID),
		AlivePlayers: alivePlayersPayload(roleResult, alive),
		CanAct:       alive[playerID] && len(candidates) > 0,
		TargetIDs:    append([]string(nil), candidates...),
		LockedIn:     selections != nil && selections[playerID] != "",
		Note:         note,
	}
}

func buildDayRevoteStatePayload(roleResult RoleAssignmentResult, round int, alive map[string]bool, decision revoteInfo, selections map[string]string, playerID string) proto.MafiaStatePayload {
	canAct := false
	targets := []string{}
	note := decision.Note
	switch {
	case len(decision.CandidateIDs) == 0:
		note = "No revote is required."
	case decision.MayorOnly:
		canAct = alive[playerID] && playerID == roleResult.MayorID
		targets = append([]string(nil), decision.CandidateIDs...)
		if playerID != roleResult.MayorID {
			note = "The mayor is breaking the tie."
		}
	default:
		canAct = alive[playerID]
		targets = append([]string(nil), decision.CandidateIDs...)
	}
	return proto.MafiaStatePayload{
		PlayerID:     playerID,
		Round:        round,
		Phase:        "day_revote",
		YourRole:     roleResult.Roles[playerID],
		TeamIDs:      teamIDsFor(roleResult, playerID),
		AlivePlayers: alivePlayersPayload(roleResult, alive),
		CanAct:       canAct,
		TargetIDs:    targets,
		LockedIn:     selections != nil && selections[playerID] != "",
		Note:         note,
	}
}

func buildNightActionResult(roleResult RoleAssignmentResult, alive map[string]bool, selections map[string]string) NightActionResult {
	result := NightActionResult{
		Choices:          cloneStringMap(selections),
		MafiaChoices:     map[string]string{},
		DoctorChoices:    map[string]string{},
		DetectiveChoices: map[string]string{},
	}
	for playerID, targetID := range selections {
		switch roleResult.Roles[playerID] {
		case roleMafia:
			if alive[playerID] {
				result.MafiaChoices[playerID] = targetID
			}
		case roleDoctor:
			if alive[playerID] {
				result.DoctorChoices[playerID] = targetID
			}
		case roleDetective:
			if alive[playerID] {
				result.DetectiveChoices[playerID] = targetID
			}
		}
	}
	return result
}

func getRoleResult(state *engine.GameState) (RoleAssignmentResult, bool) {
	return engine.GetResult[RoleAssignmentResult](state, "mafia_role_assign")
}

func getNightRevealResult(state *engine.GameState, round int) (NightRevealResult, bool) {
	return engine.GetResult[NightRevealResult](state, fmt.Sprintf("mafia_night_reveal_r%d", round))
}

func aliveBeforeNight(state *engine.GameState, round int) map[string]bool {
	roleResult, _ := getRoleResult(state)
	alive := aliveMapFor(roleResult.Order)
	for current := 1; current < round; current++ {
		if reveal, ok := engine.GetResult[NightRevealResult](state, fmt.Sprintf("mafia_night_reveal_r%d", current)); ok {
			for _, death := range reveal.Payload.Deaths {
				alive[death] = false
			}
		}
		if reveal, ok := engine.GetResult[proto.MafiaRevealPayload](state, fmt.Sprintf("mafia_day_reveal_r%d", current)); ok && reveal.EliminatedID != "" {
			alive[reveal.EliminatedID] = false
		}
	}
	return alive
}

func aliveBeforeDay(state *engine.GameState, round int) map[string]bool {
	alive := aliveBeforeNight(state, round)
	if reveal, ok := getNightRevealResult(state, round); ok {
		for _, death := range reveal.Payload.Deaths {
			alive[death] = false
		}
	}
	return alive
}

func targetIDsForNight(roleResult RoleAssignmentResult, alive map[string]bool, playerID string) []string {
	if !alive[playerID] {
		return nil
	}
	role := roleResult.Roles[playerID]
	targets := []string{}
	for _, candidate := range roleResult.Order {
		if !alive[candidate] {
			continue
		}
		switch role {
		case roleMafia:
			if roleResult.Roles[candidate] != roleMafia {
				targets = append(targets, candidate)
			}
		case roleDoctor:
			if candidate == playerID && !roleResult.Config.SelfProtect {
				continue
			}
			targets = append(targets, candidate)
		case roleDetective:
			if candidate != playerID {
				targets = append(targets, candidate)
			}
		}
	}
	return targets
}

func targetIDsForNomination(alive map[string]bool, playerID string) []string {
	targets := []string{}
	for candidate, isAlive := range alive {
		if !isAlive || candidate == playerID {
			continue
		}
		targets = append(targets, candidate)
	}
	slices.Sort(targets)
	return targets
}

func livingNightActorCount(roleResult RoleAssignmentResult, alive map[string]bool) int {
	count := 0
	for _, playerID := range roleResult.Order {
		if !alive[playerID] {
			continue
		}
		switch roleResult.Roles[playerID] {
		case roleMafia, roleDoctor, roleDetective:
			count++
		}
	}
	return count
}

func livingCount(alive map[string]bool) int {
	count := 0
	for _, isAlive := range alive {
		if isAlive {
			count++
		}
	}
	return count
}

func chooseProtectedTarget(doctorChoices map[string]string, roleResult RoleAssignmentResult, alive map[string]bool) string {
	for _, doctorID := range roleResult.Order {
		if roleResult.Roles[doctorID] != roleDoctor {
			continue
		}
		targetID := doctorChoices[doctorID]
		if targetID == "" || !alive[targetID] {
			continue
		}
		if targetID == doctorID && !roleResult.Config.SelfProtect {
			continue
		}
		return targetID
	}
	return ""
}

func chooseKillTarget(mafiaChoices map[string]string, roleResult RoleAssignmentResult, alive map[string]bool) (string, bool) {
	livingMafia := 0
	counts := map[string]int{}
	for _, mafiaID := range roleResult.MafiaIDs {
		if !alive[mafiaID] {
			continue
		}
		livingMafia++
		targetID := mafiaChoices[mafiaID]
		if targetID == "" || !alive[targetID] || roleResult.Roles[targetID] == roleMafia {
			continue
		}
		counts[targetID]++
	}
	bestID := ""
	bestCount := 0
	tied := false
	for targetID, count := range counts {
		if count > bestCount {
			bestID = targetID
			bestCount = count
			tied = false
			continue
		}
		if count == bestCount && targetID != bestID {
			tied = true
		}
	}
	if livingMafia == 0 || bestID == "" || tied || bestCount <= livingMafia/2 {
		return "", false
	}
	return bestID, true
}

type voteRoundResult struct {
	Tallies   map[string]int
	Threshold int
	WinnerID  string
	TiedIDs   []string
}

func resolveVoteRound(roleResult RoleAssignmentResult, alive map[string]bool, candidates []string, votes map[string]string) voteRoundResult {
	result := voteRoundResult{
		Tallies:   map[string]int{},
		Threshold: roleResult.Config.DayVoteThreshold,
	}
	if len(candidates) == 0 {
		return result
	}
	candidateSet := map[string]struct{}{}
	for _, candidateID := range candidates {
		candidateSet[candidateID] = struct{}{}
	}
	for voterID, targetID := range votes {
		if !alive[voterID] {
			continue
		}
		if _, ok := candidateSet[targetID]; !ok {
			continue
		}
		weight := 1
		if roleResult.Roles[voterID] == roleMayor {
			weight = 3
		}
		result.Tallies[targetID] += weight
	}
	bestCount := 0
	for _, count := range result.Tallies {
		if count > bestCount {
			bestCount = count
		}
	}
	if bestCount == 0 {
		return result
	}
	tied := []string{}
	for _, candidateID := range candidates {
		if result.Tallies[candidateID] == bestCount {
			tied = append(tied, candidateID)
		}
	}
	if len(tied) == 1 && bestCount >= result.Threshold {
		result.WinnerID = tied[0]
		return result
	}
	if len(tied) > 1 {
		result.TiedIDs = tied
	}
	return result
}

type revoteInfo struct {
	CandidateIDs []string
	MayorOnly    bool
	Note         string
}

func revoteDecision(state *engine.GameState, round int, roleResult RoleAssignmentResult, alive map[string]bool) (revoteInfo, error) {
	nominationResult, ok := engine.GetResult[NominationResult](state, fmt.Sprintf("mafia_day_nominate_r%d", round))
	if !ok {
		return revoteInfo{}, fmt.Errorf("missing nomination result")
	}
	dayVoteResult, ok := engine.GetResult[DayVoteResult](state, fmt.Sprintf("mafia_day_vote_r%d", round))
	if !ok {
		return revoteInfo{}, fmt.Errorf("missing day vote result")
	}
	candidates := nominatedCandidatesFromResult(roleResult, nominationResult)
	primary := resolveVoteRound(roleResult, alive, candidates, dayVoteResult.Votes)
	if len(primary.TiedIDs) < 2 {
		return revoteInfo{}, nil
	}
	switch roleResult.Config.TieRule {
	case tieRuleRevote:
		return revoteInfo{
			CandidateIDs: primary.TiedIDs,
			Note:         "Tie vote. Revote between the tied suspects.",
		}, nil
	case tieRuleMayorBreaks:
		if roleResult.MayorID == "" || !alive[roleResult.MayorID] {
			return revoteInfo{}, nil
		}
		return revoteInfo{
			CandidateIDs: primary.TiedIDs,
			MayorOnly:    true,
			Note:         "Tie vote. The mayor will decide who gets eliminated.",
		}, nil
	default:
		return revoteInfo{}, nil
	}
}

func nominatedCandidates(state *engine.GameState, round int, roleResult RoleAssignmentResult) []string {
	result, ok := engine.GetResult[NominationResult](state, fmt.Sprintf("mafia_day_nominate_r%d", round))
	if !ok {
		return nil
	}
	return nominatedCandidatesFromResult(roleResult, result)
}

func nominatedCandidatesFromResult(roleResult RoleAssignmentResult, result NominationResult) []string {
	counts := map[string]int{}
	for _, targetID := range result.Nominations {
		if targetID != "" {
			counts[targetID]++
		}
	}
	candidates := []string{}
	for _, playerID := range roleResult.Order {
		if counts[playerID] >= roleResult.Config.NominationMinimum {
			candidates = append(candidates, playerID)
		}
	}
	return candidates
}

func winningTeam(alive map[string]bool, roles map[string]string) string {
	mafiaAlive := 0
	townAlive := 0
	for playerID, isAlive := range alive {
		if !isAlive {
			continue
		}
		if roles[playerID] == roleMafia {
			mafiaAlive++
		} else {
			townAlive++
		}
	}
	switch {
	case mafiaAlive == 0:
		return "town"
	case mafiaAlive >= townAlive:
		return "mafia"
	default:
		return ""
	}
}

func awardWinners(state *engine.GameState, winner string, roleMap map[string]string) {
	if winner == "" {
		return
	}
	for playerID, role := range roleMap {
		if (winner == "mafia" && role == roleMafia) || (winner == "town" && role != roleMafia) {
			state.Scores[engine.PlayerID(playerID)] += 1000
		}
	}
}

func teamIDsFor(roleResult RoleAssignmentResult, playerID string) []string {
	if roleResult.Roles[playerID] != roleMafia {
		return nil
	}
	out := append([]string(nil), roleResult.MafiaIDs...)
	slices.Sort(out)
	return out
}

func alivePlayersPayload(roleResult RoleAssignmentResult, alive map[string]bool) []proto.MafiaPlayerState {
	out := make([]proto.MafiaPlayerState, 0, len(roleResult.Order))
	for _, playerID := range roleResult.Order {
		out = append(out, proto.MafiaPlayerState{
			PlayerID: playerID,
			Name:     roleResult.Names[playerID],
			Alive:    alive[playerID],
		})
	}
	return out
}

func aliveIDs(order []string, alive map[string]bool) []string {
	out := []string{}
	for _, playerID := range order {
		if alive[playerID] {
			out = append(out, playerID)
		}
	}
	return out
}

func roleIntro(roleResult RoleAssignmentResult, playerID string) string {
	switch roleResult.Roles[playerID] {
	case roleMafia:
		names := make([]string, 0, len(roleResult.MafiaIDs))
		for _, teammateID := range roleResult.MafiaIDs {
			if teammateID == playerID {
				continue
			}
			names = append(names, roleResult.Names[teammateID])
		}
		if len(names) == 0 {
			return "You are Mafia. Choose the kill target at night and survive the vote during the day."
		}
		return fmt.Sprintf("You are Mafia. Your teammate(s): %s.", strings.Join(names, ", "))
	case roleDetective:
		return "You are the Detective. Investigate one living player each night."
	case roleDoctor:
		if roleResult.Config.SelfProtect {
			return "You are the Doctor. Protect one living player each night, including yourself."
		}
		return "You are the Doctor. Protect one living player each night, but not yourself."
	case roleMayor:
		return "You are the Mayor. Your daytime vote counts as three."
	default:
		return "You are a Citizen. Talk, nominate, and vote out the Mafia."
	}
}

func nightPrompt(role string, config LobbyConfig) string {
	switch role {
	case roleMafia:
		return "Choose one living town player to eliminate."
	case roleDoctor:
		if config.SelfProtect {
			return "Choose one living player to protect. You may protect yourself."
		}
		return "Choose one living player to protect. You may not protect yourself."
	case roleDetective:
		if config.InvestigationMode == investigationExactRole {
			return "Choose one living player to investigate. You will learn their exact role."
		}
		return "Choose one living player to investigate. You will learn whether they are Mafia or Town."
	default:
		return "Sleep through the night."
	}
}

func investigationResult(role, mode string) string {
	if normalizeInvestigationMode(mode) == investigationExactRole {
		return displayRole(role)
	}
	if role == roleMafia {
		return "Mafia"
	}
	return "Town"
}

func displayRole(role string) string {
	switch role {
	case roleMafia:
		return "Mafia"
	case roleDetective:
		return "Detective"
	case roleDoctor:
		return "Doctor"
	case roleMayor:
		return "Mayor"
	default:
		return "Citizen"
	}
}

func buildRoleList(playerCount int) []string {
	switch {
	case playerCount <= 6:
		return fillCitizens(playerCount, []string{roleMafia, roleDetective, roleDoctor})
	case playerCount <= 8:
		return fillCitizens(playerCount, []string{roleMafia, roleMafia, roleDetective, roleDoctor, roleMayor})
	case playerCount <= 10:
		return fillCitizens(playerCount, []string{roleMafia, roleMafia, roleDetective, roleDoctor, roleMayor})
	default:
		return fillCitizens(playerCount, []string{roleMafia, roleMafia, roleMafia, roleDetective, roleDetective, roleDoctor, roleMayor})
	}
}

func fillCitizens(playerCount int, assigned []string) []string {
	out := append([]string(nil), assigned...)
	for len(out) < playerCount {
		out = append(out, roleCitizen)
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneAlive(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func aliveMapFor(order []string) map[string]bool {
	out := map[string]bool{}
	for _, playerID := range order {
		out[playerID] = true
	}
	return out
}

func orderedPlayers(state *engine.GameState) []string {
	type namedPlayer struct {
		id   string
		name string
	}
	players := make([]namedPlayer, 0, len(state.ActivePlayers()))
	for _, playerID := range state.ActivePlayers() {
		player := state.Players[playerID]
		players = append(players, namedPlayer{id: string(playerID), name: player.Name})
	}
	slices.SortFunc(players, func(a, b namedPlayer) int {
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
	out := make([]string, 0, len(players))
	for _, player := range players {
		out = append(out, player.id)
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

func seedFor(state *engine.GameState, order []string) int {
	seed := len(order) * 97
	for _, ch := range state.RoomID {
		seed += int(ch)
	}
	return seed
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

func stringOption(options map[string]any, key, fallback string) string {
	if options == nil {
		return fallback
	}
	value, ok := options[key]
	if !ok {
		return fallback
	}
	typed, ok := value.(string)
	if !ok {
		return fallback
	}
	if strings.TrimSpace(typed) == "" {
		return fallback
	}
	return strings.TrimSpace(typed)
}

func normalizeTieRule(raw string) string {
	switch raw {
	case tieRuleNoElimination, tieRuleRevote, tieRuleMayorBreaks:
		return raw
	default:
		return tieRuleNoElimination
	}
}

func normalizeInvestigationMode(raw string) string {
	switch raw {
	case investigationFaction, investigationExactRole:
		return raw
	default:
		return investigationFaction
	}
}
