package mafia

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jj/trivia/internal/engine"
	"github.com/jj/trivia/internal/proto"
)

func TestBuildLobbyConfigScalesRolesAndDefaults(t *testing.T) {
	t.Parallel()

	cfg6 := BuildLobbyConfig(6, nil)
	if got := cfg6.RoleCounts[roleMafia]; got != 1 {
		t.Fatalf("6-player mafia count: want 1, got %d", got)
	}
	if got := cfg6.RoleCounts[roleDetective]; got != 1 {
		t.Fatalf("6-player detective count: want 1, got %d", got)
	}
	if got := cfg6.RoleCounts[roleDoctor]; got != 1 {
		t.Fatalf("6-player doctor count: want 1, got %d", got)
	}
	if cfg6.SelfProtect {
		t.Fatal("6-player config should disable self protect")
	}
	if cfg6.TieRule != tieRuleNoElimination {
		t.Fatalf("6-player tie rule: want %q, got %q", tieRuleNoElimination, cfg6.TieRule)
	}

	cfg10 := BuildLobbyConfig(10, nil)
	if got := cfg10.RoleCounts[roleMafia]; got != 2 {
		t.Fatalf("10-player mafia count: want 2, got %d", got)
	}
	if got := cfg10.RoleCounts[roleMayor]; got != 1 {
		t.Fatalf("10-player mayor count: want 1, got %d", got)
	}
	if cfg10.TieRule != tieRuleMayorBreaks {
		t.Fatalf("10-player tie rule: want %q, got %q", tieRuleMayorBreaks, cfg10.TieRule)
	}

	cfg12 := BuildLobbyConfig(12, nil)
	if got := cfg12.RoleCounts[roleMafia]; got != 3 {
		t.Fatalf("12-player mafia count: want 3, got %d", got)
	}
	if got := cfg12.RoleCounts[roleDetective]; got != 2 {
		t.Fatalf("12-player detective count: want 2, got %d", got)
	}
	if cfg12.InvestigationMode != investigationExactRole {
		t.Fatalf("12-player investigation mode: want %q, got %q", investigationExactRole, cfg12.InvestigationMode)
	}
	if cfg12.RevealOnDeath {
		t.Fatal("12-player config should hide roles on death by default")
	}
}

func TestResolveNightRevealFollowsProtectKillInvestigateOrder(t *testing.T) {
	t.Parallel()

	state := &engine.GameState{
		PhaseData: map[string]engine.PhaseResult{},
		Scores:    map[engine.PlayerID]int{},
	}
	roleResult := RoleAssignmentResult{
		Order: []string{"m1", "d1", "x1", "c1"},
		Names: map[string]string{
			"m1": "Mara",
			"d1": "Dina",
			"x1": "Dex",
			"c1": "Cory",
		},
		Roles: map[string]string{
			"m1": roleMafia,
			"d1": roleDoctor,
			"x1": roleDetective,
			"c1": roleCitizen,
		},
		MafiaIDs:     []string{"m1"},
		DoctorIDs:    []string{"d1"},
		DetectiveIDs: []string{"x1"},
		Config: LobbyConfig{
			RevealOnDeath:     true,
			TieRule:           tieRuleNoElimination,
			SelfProtect:       false,
			InvestigationMode: investigationFaction,
			NominationMinimum: 2,
			DayVoteThreshold:  2,
		},
	}
	engine.PutResult(state, "mafia_role_assign", "test", roleResult)
	engine.PutResult(state, "mafia_night_collect_r1", "test", NightActionResult{
		MafiaChoices:     map[string]string{"m1": "c1"},
		DoctorChoices:    map[string]string{"d1": "c1"},
		DetectiveChoices: map[string]string{"x1": "m1"},
	})

	reveal, winner, err := resolveNightReveal(state, 1)
	if err != nil {
		t.Fatalf("resolveNightReveal error: %v", err)
	}
	if winner != "" {
		t.Fatalf("winner: want none, got %q", winner)
	}
	if len(reveal.Payload.Deaths) != 0 {
		t.Fatalf("deaths: want none, got %+v", reveal.Payload.Deaths)
	}
	if !reveal.ProtectionWorked {
		t.Fatal("expected doctor protection to block the kill")
	}
	finding, ok := reveal.DetectiveFindings["x1"]
	if !ok {
		t.Fatal("expected detective finding")
	}
	if finding.Result != "Mafia" {
		t.Fatalf("detective result: want Mafia, got %q", finding.Result)
	}
}

func TestResolveNightRevealSelfProtectFailsWhenDisabled(t *testing.T) {
	t.Parallel()

	state := &engine.GameState{
		PhaseData: map[string]engine.PhaseResult{},
		Scores:    map[engine.PlayerID]int{},
	}
	roleResult := RoleAssignmentResult{
		Order: []string{"m1", "d1", "c1"},
		Names: map[string]string{
			"m1": "Mara",
			"d1": "Dina",
			"c1": "Cory",
		},
		Roles: map[string]string{
			"m1": roleMafia,
			"d1": roleDoctor,
			"c1": roleCitizen,
		},
		MafiaIDs:  []string{"m1"},
		DoctorIDs: []string{"d1"},
		Config: LobbyConfig{
			RevealOnDeath:     true,
			TieRule:           tieRuleNoElimination,
			SelfProtect:       false,
			InvestigationMode: investigationFaction,
			NominationMinimum: 2,
			DayVoteThreshold:  2,
		},
	}
	engine.PutResult(state, "mafia_role_assign", "test", roleResult)
	engine.PutResult(state, "mafia_night_collect_r1", "test", NightActionResult{
		MafiaChoices:  map[string]string{"m1": "d1"},
		DoctorChoices: map[string]string{"d1": "d1"},
	})

	reveal, _, err := resolveNightReveal(state, 1)
	if err != nil {
		t.Fatalf("resolveNightReveal error: %v", err)
	}
	if len(reveal.Payload.Deaths) != 1 || reveal.Payload.Deaths[0] != "d1" {
		t.Fatalf("deaths: want doctor to die, got %+v", reveal.Payload.Deaths)
	}
	if reveal.ProtectionWorked {
		t.Fatal("self-protect should fail silently when disabled")
	}
}

func TestResolveDayRevealUsesNominationThresholdAndMayorWeight(t *testing.T) {
	t.Parallel()

	state := &engine.GameState{
		PhaseData: map[string]engine.PhaseResult{},
		Scores:    map[engine.PlayerID]int{},
	}
	roleResult := RoleAssignmentResult{
		Order: []string{"m1", "y1", "c1", "c2", "c3"},
		Names: map[string]string{
			"m1": "Mara",
			"y1": "Milo",
			"c1": "Cory",
			"c2": "Casey",
			"c3": "Cleo",
		},
		Roles: map[string]string{
			"m1": roleMafia,
			"y1": roleMayor,
			"c1": roleCitizen,
			"c2": roleCitizen,
			"c3": roleCitizen,
		},
		MafiaIDs: []string{"m1"},
		MayorID:  "y1",
		Config: LobbyConfig{
			RevealOnDeath:     true,
			TieRule:           tieRuleNoElimination,
			SelfProtect:       false,
			InvestigationMode: investigationFaction,
			NominationMinimum: 2,
			DayVoteThreshold:  3,
		},
	}
	engine.PutResult(state, "mafia_role_assign", "test", roleResult)
	engine.PutResult(state, "mafia_night_reveal_r1", "test", NightRevealResult{
		Payload: protoPayloadAlive(1, roleResult.Order),
	})
	engine.PutResult(state, "mafia_day_nominate_r1", "test", NominationResult{
		Nominations: map[string]string{
			"y1": "m1",
			"c1": "m1",
		},
	})
	engine.PutResult(state, "mafia_day_vote_r1", "test", DayVoteResult{
		Votes: map[string]string{
			"y1": "m1",
		},
	})
	engine.PutResult(state, "mafia_day_revote_r1", "test", DayRevoteResult{Votes: map[string]string{}})

	reveal, winner, err := resolveDayReveal(state, 1)
	if err != nil {
		t.Fatalf("resolveDayReveal error: %v", err)
	}
	if reveal.EliminatedID != "m1" {
		t.Fatalf("eliminated id: want m1, got %q", reveal.EliminatedID)
	}
	if winner != "town" {
		t.Fatalf("winner: want town, got %q", winner)
	}
}

func TestRevoteDecisionUsesConfiguredTieRule(t *testing.T) {
	t.Parallel()

	state := &engine.GameState{
		PhaseData: map[string]engine.PhaseResult{},
		Scores:    map[engine.PlayerID]int{},
	}
	roleResult := RoleAssignmentResult{
		Order: []string{"m1", "y1", "c1", "c2", "c3", "c4"},
		Names: map[string]string{
			"m1": "Mara",
			"y1": "Milo",
			"c1": "Cory",
			"c2": "Casey",
			"c3": "Cleo",
			"c4": "Cade",
		},
		Roles: map[string]string{
			"m1": roleMafia,
			"y1": roleMayor,
			"c1": roleCitizen,
			"c2": roleCitizen,
			"c3": roleCitizen,
			"c4": roleCitizen,
		},
		MafiaIDs: []string{"m1"},
		MayorID:  "y1",
		Config: LobbyConfig{
			RevealOnDeath:     true,
			TieRule:           tieRuleMayorBreaks,
			SelfProtect:       false,
			InvestigationMode: investigationFaction,
			NominationMinimum: 2,
			DayVoteThreshold:  3,
		},
	}
	engine.PutResult(state, "mafia_role_assign", "test", roleResult)
	engine.PutResult(state, "mafia_night_reveal_r1", "test", NightRevealResult{
		Payload: protoPayloadAlive(1, roleResult.Order),
	})
	engine.PutResult(state, "mafia_day_nominate_r1", "test", NominationResult{
		Nominations: map[string]string{
			"y1": "m1",
			"c1": "m1",
			"c2": "c3",
			"c4": "c3",
		},
	})
	engine.PutResult(state, "mafia_day_vote_r1", "test", DayVoteResult{
		Votes: map[string]string{
			"y1": "m1",
			"c2": "c3",
			"c3": "c3",
			"c4": "c3",
		},
	})

	alive := aliveMapFor(roleResult.Order)
	info, err := revoteDecision(state, 1, roleResult, alive)
	if err != nil {
		t.Fatalf("revoteDecision error: %v", err)
	}
	if !info.MayorOnly {
		t.Fatal("expected mayor-only tie break")
	}
	if len(info.CandidateIDs) != 2 {
		t.Fatalf("expected two tied candidates, got %+v", info.CandidateIDs)
	}
}

func TestNightActionHandleEnforcesRoleTargets(t *testing.T) {
	t.Parallel()

	roleResult := testRoleResult(LobbyConfig{
		RevealOnDeath:     true,
		TieRule:           tieRuleNoElimination,
		SelfProtect:       false,
		InvestigationMode: investigationFaction,
		NominationMinimum: 2,
		DayVoteThreshold:  3,
	})
	state := testMafiaState(roleResult)
	ctx := &engine.PhaseContext{State: state}

	tests := []struct {
		name     string
		playerID string
		targetID string
		wantErr  string
	}{
		{
			name:     "citizen has no night action",
			playerID: "c1",
			targetID: "m1",
			wantErr:  "invalid night target",
		},
		{
			name:     "mafia cannot target mafia teammate",
			playerID: "m1",
			targetID: "m2",
			wantErr:  "invalid night target",
		},
		{
			name:     "detective cannot investigate self",
			playerID: "x1",
			targetID: "x1",
			wantErr:  "invalid night target",
		},
		{
			name:     "doctor cannot self protect when disabled",
			playerID: "d1",
			targetID: "d1",
			wantErr:  "invalid night target",
		},
		{
			name:     "mafia can target living town",
			playerID: "m1",
			targetID: "c1",
		},
		{
			name:     "detective can target other living player",
			playerID: "x1",
			targetID: "m1",
		},
		{
			name:     "doctor can protect other living player",
			playerID: "d1",
			targetID: "c2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			primitive := newNightAction(1)
			_, err := primitive.Handle(ctx, voteInput(tt.playerID, tt.targetID))
			assertErrContains(t, err, tt.wantErr)
		})
	}
}

func TestNightActionHandleAllowsDoctorSelfProtectWhenEnabled(t *testing.T) {
	t.Parallel()

	roleResult := testRoleResult(LobbyConfig{
		RevealOnDeath:     true,
		TieRule:           tieRuleRevote,
		SelfProtect:       true,
		InvestigationMode: investigationFaction,
		NominationMinimum: 2,
		DayVoteThreshold:  3,
	})
	state := testMafiaState(roleResult)
	ctx := &engine.PhaseContext{State: state}
	primitive := newNightAction(1)

	decision, err := primitive.Handle(ctx, voteInput("d1", "d1"))
	if err != nil {
		t.Fatalf("doctor self-protect with setting enabled returned error: %v", err)
	}
	if decision.AdvancePhase {
		t.Fatal("one night action should not advance while mafia and detective are still pending")
	}
}

func TestMafiaKillRequiresConsensusAndRejectsTies(t *testing.T) {
	t.Parallel()

	roleResult := testRoleResult(LobbyConfig{
		RevealOnDeath:     true,
		TieRule:           tieRuleRevote,
		SelfProtect:       true,
		InvestigationMode: investigationFaction,
		NominationMinimum: 2,
		DayVoteThreshold:  3,
	})
	alive := aliveMapFor(roleResult.Order)

	target, ok := chooseKillTarget(map[string]string{"m1": "c1", "m2": "c2"}, roleResult, alive)
	if ok || target != "" {
		t.Fatalf("split mafia vote should not kill, got target=%q ok=%t", target, ok)
	}

	target, ok = chooseKillTarget(map[string]string{"m1": "c1", "m2": "c1"}, roleResult, alive)
	if !ok || target != "c1" {
		t.Fatalf("mafia consensus should kill c1, got target=%q ok=%t", target, ok)
	}
}

func TestNominationHandleRejectsSelfDeadAndStoresValidNomination(t *testing.T) {
	t.Parallel()

	roleResult := testRoleResult(LobbyConfig{
		RevealOnDeath:     true,
		TieRule:           tieRuleNoElimination,
		SelfProtect:       false,
		InvestigationMode: investigationFaction,
		NominationMinimum: 2,
		DayVoteThreshold:  3,
	})
	state := testMafiaState(roleResult)
	engine.PutResult(state, "mafia_night_reveal_r1", "test", NightRevealResult{
		Payload: proto.MafiaRevealPayload{
			Round:    1,
			Phase:    "night_reveal",
			Deaths:   []string{"c2"},
			AliveIDs: []string{"m1", "m2", "x1", "d1", "y1", "c1"},
		},
	})
	ctx := &engine.PhaseContext{State: state}

	primitive := newDayNominate(1)
	_, err := primitive.Handle(ctx, voteInput("c1", "c1"))
	assertErrContains(t, err, "invalid nomination target")

	_, err = primitive.Handle(ctx, voteInput("c2", "m1"))
	assertErrContains(t, err, "only living players can nominate")

	decision, err := primitive.Handle(ctx, voteInput("c1", "m1"))
	if err != nil {
		t.Fatalf("valid nomination returned error: %v", err)
	}
	if got := decision.Result.Nominations["c1"]; got != "" {
		t.Fatalf("partial nomination should not be final result yet, got %q", got)
	}
	timeout, err := primitive.Timeout(ctx)
	if err != nil {
		t.Fatalf("nomination timeout returned error: %v", err)
	}
	if got := timeout.Result.Nominations["c1"]; got != "m1" {
		t.Fatalf("stored nomination: want m1, got %q", got)
	}
}

func TestDayVoteHandleRequiresNominatedCandidate(t *testing.T) {
	t.Parallel()

	roleResult := testRoleResult(LobbyConfig{
		RevealOnDeath:     true,
		TieRule:           tieRuleNoElimination,
		SelfProtect:       false,
		InvestigationMode: investigationFaction,
		NominationMinimum: 2,
		DayVoteThreshold:  3,
	})
	state := testMafiaState(roleResult)
	engine.PutResult(state, "mafia_night_reveal_r1", "test", NightRevealResult{Payload: protoPayloadAlive(1, roleResult.Order)})
	engine.PutResult(state, "mafia_day_nominate_r1", "test", NominationResult{
		Nominations: map[string]string{
			"c1": "m1",
			"d1": "m1",
			"x1": "c2",
		},
	})
	ctx := &engine.PhaseContext{State: state}
	primitive := newDayVote(1)

	_, err := primitive.Handle(ctx, voteInput("c1", "c2"))
	assertErrContains(t, err, "invalid vote target")

	decision, err := primitive.Handle(ctx, voteInput("c1", "m1"))
	if err != nil {
		t.Fatalf("valid day vote returned error: %v", err)
	}
	if decision.AdvancePhase {
		t.Fatal("one day vote should not advance while other living voters are pending")
	}
	timeout, err := primitive.Timeout(ctx)
	if err != nil {
		t.Fatalf("day vote timeout returned error: %v", err)
	}
	if got := timeout.Result.Votes["c1"]; got != "m1" {
		t.Fatalf("stored vote: want m1, got %q", got)
	}
}

func TestMayorOnlyRevoteRejectsNonMayorAndAcceptsMayor(t *testing.T) {
	t.Parallel()

	roleResult := testRoleResult(LobbyConfig{
		RevealOnDeath:     true,
		TieRule:           tieRuleMayorBreaks,
		SelfProtect:       true,
		InvestigationMode: investigationFaction,
		NominationMinimum: 2,
		DayVoteThreshold:  3,
	})
	state := testMafiaState(roleResult)
	engine.PutResult(state, "mafia_night_reveal_r1", "test", NightRevealResult{Payload: protoPayloadAlive(1, roleResult.Order)})
	engine.PutResult(state, "mafia_day_nominate_r1", "test", NominationResult{
		Nominations: map[string]string{
			"y1": "m1",
			"c1": "m1",
			"m1": "c2",
			"c2": "c2",
		},
	})
	engine.PutResult(state, "mafia_day_vote_r1", "test", DayVoteResult{
		Votes: map[string]string{
			"c1": "m1",
			"m1": "c2",
			"c2": "c2",
			"d1": "m1",
		},
	})
	ctx := &engine.PhaseContext{State: state}
	primitive := newDayRevote(1)

	_, err := primitive.Handle(ctx, voteInput("c1", "m1"))
	assertErrContains(t, err, "only the mayor can break this tie")

	decision, err := primitive.Handle(ctx, voteInput("y1", "m1"))
	if err != nil {
		t.Fatalf("mayor tie break returned error: %v", err)
	}
	if !decision.AdvancePhase {
		t.Fatal("mayor-only revote should advance immediately after mayor vote")
	}
	if got := decision.Result.Votes["y1"]; got != "m1" {
		t.Fatalf("mayor revote: want m1, got %q", got)
	}
}

func TestBuildLobbyConfigSupportsDayVoteModes(t *testing.T) {
	t.Parallel()

	cfg := BuildLobbyConfig(7, map[string]any{
		"day_vote_mode": dayVoteModeLivePublic,
	})
	if cfg.DayVoteMode != dayVoteModeLivePublic {
		t.Fatalf("day vote mode: want %q, got %q", dayVoteModeLivePublic, cfg.DayVoteMode)
	}
	if got := cfg.GameOptions["day_vote_mode"]; got != dayVoteModeLivePublic {
		t.Fatalf("game option day_vote_mode: want %q, got %q", dayVoteModeLivePublic, got)
	}
}

func TestDayVoteLivePublicBroadcastsVisibleVotes(t *testing.T) {
	t.Parallel()

	roleResult := testRoleResult(LobbyConfig{
		RevealOnDeath:     true,
		TieRule:           tieRuleNoElimination,
		SelfProtect:       false,
		InvestigationMode: investigationFaction,
		DayVoteMode:       dayVoteModeLivePublic,
		NominationMinimum: 2,
		DayVoteThreshold:  3,
	})
	state := testMafiaState(roleResult)
	engine.PutResult(state, "mafia_night_reveal_r1", "test", NightRevealResult{Payload: protoPayloadAlive(1, roleResult.Order)})
	engine.PutResult(state, "mafia_day_nominate_r1", "test", NominationResult{
		Nominations: map[string]string{
			"c1": "m1",
			"d1": "m1",
			"x1": "c2",
			"y1": "c2",
		},
	})
	ctx := &engine.PhaseContext{State: state}
	primitive := newDayVote(1)

	decision, err := primitive.Handle(ctx, voteInput("c1", "m1"))
	if err != nil {
		t.Fatalf("live public day vote returned error: %v", err)
	}
	if len(decision.Broadcast) == 0 {
		t.Fatal("expected mafia state broadcasts after a live public vote")
	}

	foundVisibleVote := false
	for _, event := range decision.Broadcast {
		if event.Type != proto.S2CMafiaState {
			continue
		}
		var payload proto.MafiaStatePayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			t.Fatalf("unmarshal mafia state: %v", err)
		}
		if payload.VoteMode != dayVoteModeLivePublic {
			t.Fatalf("vote mode: want %q, got %q", dayVoteModeLivePublic, payload.VoteMode)
		}
		if payload.PublicVotes["c1"] == "m1" {
			foundVisibleVote = true
		}
	}
	if !foundVisibleVote {
		t.Fatal("expected public vote table to include c1 -> m1")
	}
}

func TestDayVoteSequentialRequiresCurrentVoter(t *testing.T) {
	t.Parallel()

	roleResult := testRoleResult(LobbyConfig{
		RevealOnDeath:     true,
		TieRule:           tieRuleNoElimination,
		SelfProtect:       false,
		InvestigationMode: investigationFaction,
		DayVoteMode:       dayVoteModeSequential,
		NominationMinimum: 2,
		DayVoteThreshold:  3,
	})
	state := testMafiaState(roleResult)
	engine.PutResult(state, "mafia_night_reveal_r1", "test", NightRevealResult{Payload: protoPayloadAlive(1, roleResult.Order)})
	engine.PutResult(state, "mafia_day_nominate_r1", "test", NominationResult{
		Nominations: map[string]string{
			"c1": "m1",
			"d1": "m1",
			"x1": "c2",
			"y1": "c2",
		},
	})
	ctx := &engine.PhaseContext{State: state}
	primitive := newDayVote(1)

	_, err := primitive.Handle(ctx, voteInput("c1", "m1"))
	assertErrContains(t, err, "it is not your turn to vote")

	decision, err := primitive.Handle(ctx, voteInput("m1", "m1"))
	if err != nil {
		t.Fatalf("sequential vote for current voter returned error: %v", err)
	}
	if len(decision.Broadcast) == 0 {
		t.Fatal("expected state broadcast after sequential vote")
	}
}

func TestDayVoteTimeoutMarksMissingVotersAsAbstain(t *testing.T) {
	t.Parallel()

	roleResult := testRoleResult(LobbyConfig{
		RevealOnDeath:     true,
		TieRule:           tieRuleNoElimination,
		SelfProtect:       false,
		InvestigationMode: investigationFaction,
		DayVoteMode:       dayVoteModeRevealEnd,
		NominationMinimum: 2,
		DayVoteThreshold:  3,
	})
	state := testMafiaState(roleResult)
	engine.PutResult(state, "mafia_night_reveal_r1", "test", NightRevealResult{Payload: protoPayloadAlive(1, roleResult.Order)})
	engine.PutResult(state, "mafia_day_nominate_r1", "test", NominationResult{
		Nominations: map[string]string{
			"c1": "m1",
			"d1": "m1",
			"x1": "c2",
			"y1": "c2",
		},
	})
	ctx := &engine.PhaseContext{State: state}
	primitive := newDayVote(1)

	if _, err := primitive.Handle(ctx, voteInput("c1", "m1")); err != nil {
		t.Fatalf("partial vote returned error: %v", err)
	}

	timeout, err := primitive.Timeout(ctx)
	if err != nil {
		t.Fatalf("day vote timeout returned error: %v", err)
	}
	if got := timeout.Result.Votes["c1"]; got != "m1" {
		t.Fatalf("stored vote: want m1, got %q", got)
	}
	if got := timeout.Result.Votes["m1"]; got != voteChoiceAbstain {
		t.Fatalf("missing voter should abstain, got %q", got)
	}
	if got := timeout.Result.Votes["y1"]; got != voteChoiceAbstain {
		t.Fatalf("missing mayor vote should abstain, got %q", got)
	}
}

func protoPayloadAlive(round int, aliveIDs []string) proto.MafiaRevealPayload {
	return proto.MafiaRevealPayload{
		Round:    round,
		Phase:    "night_reveal",
		AliveIDs: append([]string(nil), aliveIDs...),
	}
}

func testMafiaState(roleResult RoleAssignmentResult) *engine.GameState {
	state := &engine.GameState{
		PhaseData: map[string]engine.PhaseResult{},
		Scores:    map[engine.PlayerID]int{},
	}
	engine.PutResult(state, "mafia_role_assign", "test", roleResult)
	return state
}

func testRoleResult(config LobbyConfig) RoleAssignmentResult {
	return RoleAssignmentResult{
		Order: []string{"m1", "m2", "x1", "d1", "y1", "c1", "c2"},
		Names: map[string]string{
			"m1": "Mara",
			"m2": "Mo",
			"x1": "Dex",
			"d1": "Dina",
			"y1": "Milo",
			"c1": "Cory",
			"c2": "Casey",
		},
		Roles: map[string]string{
			"m1": roleMafia,
			"m2": roleMafia,
			"x1": roleDetective,
			"d1": roleDoctor,
			"y1": roleMayor,
			"c1": roleCitizen,
			"c2": roleCitizen,
		},
		MafiaIDs:     []string{"m1", "m2"},
		DetectiveIDs: []string{"x1"},
		DoctorIDs:    []string{"d1"},
		MayorID:      "y1",
		Config:       config,
	}
}

func voteInput(playerID, targetID string) engine.Input {
	raw, _ := json.Marshal(proto.SubmitVotePayload{
		DrawingID: "mafia_test",
		ChoiceID:  targetID,
	})
	return engine.Input{
		PlayerID: engine.PlayerID(playerID),
		Type:     proto.C2SSubmitVote,
		Value:    raw,
	}
}

func assertErrContains(t *testing.T, err error, want string) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected error containing %q, got %q", want, err.Error())
	}
}
