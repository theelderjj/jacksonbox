package room

// Integration tests for the room actor. These drive a real Room goroutine
// end-to-end: the actor runs, the gateway Conn is an in-process test
// double (no network), and the Jrawful primitives run unmodified. The
// tests cover the §15 must-haves:
//
//   - Full round end-to-end with deterministic scoring
//   - Reconnect mid-game restores PlayerID
//   - Demo eviction closes with 4000
//   - Phase deadline fires with zero submissions
//   - Panic runbook closes conns with an error broadcast
//
// Each test constructs a fresh Room + auth.Store so they are isolated.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jj/trivia/internal/auth"
	"github.com/jj/trivia/internal/engine"
	"github.com/jj/trivia/internal/games/drawduel"
	"github.com/jj/trivia/internal/games/drawful"
	"github.com/jj/trivia/internal/games/fakeartist"
	"github.com/jj/trivia/internal/games/mafia"
	"github.com/jj/trivia/internal/games/priceisright"
	"github.com/jj/trivia/internal/games/reactionduel"
	"github.com/jj/trivia/internal/games/splitthevote"
	"github.com/jj/trivia/internal/gateway"
	platformcatalog "github.com/jj/trivia/internal/platform/catalog"
	"github.com/jj/trivia/internal/primitives"
	"github.com/jj/trivia/internal/proto"
)

// -----------------------------------------------------------------------------
// Test fixtures / helpers
// -----------------------------------------------------------------------------

// silentLog discards structured log output; noise is only useful when a
// test fails and we want to repro it by flipping this to os.Stderr.
func silentLog() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

// drainCapture drains a conn outbox into an append-only log of envelopes
// for post-hoc assertions. Starts its own goroutine, which exits when
// the conn's send channel closes (i.e., Conn.Close was called).
type drainCapture struct {
	mu     sync.Mutex
	frames []proto.Envelope
	done   chan struct{}
}

func newDrainCapture(c *gateway.Conn) *drainCapture {
	dc := &drainCapture{done: make(chan struct{})}
	go func() {
		defer close(dc.done)
		for raw := range c.Outbox() {
			var env proto.Envelope
			if err := json.Unmarshal(raw, &env); err != nil {
				continue
			}
			dc.mu.Lock()
			dc.frames = append(dc.frames, env)
			dc.mu.Unlock()
		}
	}()
	return dc
}

func (d *drainCapture) envelopes() []proto.Envelope {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]proto.Envelope, len(d.frames))
	copy(out, d.frames)
	return out
}

// waitForType polls until an envelope of the given type appears or the
// timeout elapses. Returns the first matching envelope or fatals the test.
func (d *drainCapture) waitForType(t *testing.T, typ string, timeout time.Duration) proto.Envelope {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, e := range d.envelopes() {
			if e.Type == typ {
				return e
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for envelope type %q", typ)
	return proto.Envelope{}
}

func (d *drainCapture) phaseChanges() []string {
	frames := d.envelopes()
	out := make([]string, 0, len(frames))
	for _, env := range frames {
		if env.Type != proto.S2CPhaseChange {
			continue
		}
		var payload proto.PhaseChangePayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			continue
		}
		out = append(out, payload.Phase)
	}
	return out
}

func waitPhaseSequence(t *testing.T, d *drainCapture, expected []string, timeout time.Duration) []string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var got []string
	for time.Now().Before(deadline) {
		got = d.phaseChanges()
		if slices.Equal(got, expected) {
			return got
		}
		if len(got) >= len(expected) && slices.Equal(got[:len(expected)], expected) {
			return got[:len(expected)]
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for phase sequence\nwant: %+v\ngot:  %+v", expected, got)
	return nil
}

// testPhaseBuilder returns a single-round Jrawful phase list with short
// timers, so "timeout" tests complete in hundreds of ms and "happy path"
// tests don't risk hitting the timer. We don't use drawful.BuildPhases
// here because it enforces min=3 rounds (§17); a 1-round run is enough
// to prove the engine wiring.
func testPhaseBuilder(drawDur, fakeDur, voteDur time.Duration) PhaseBuilder {
	return func(state *engine.GameState) []engine.Phase {
		cfg, _ := json.Marshal(map[string]int{"round": 1})
		return []engine.Phase{
			{Name: "prompt_distribute_r1", Primitive: "drawful.prompt_distribute", Config: cfg},
			{Name: "drawing_submit_r1", Primitive: "drawful.drawing_collect",
				Duration: drawDur, Config: cfg,
				DependsOn: []string{"prompt_distribute_r1"}},
			{Name: "fake_prompt_submit_r1", Primitive: "drawful.fake_collect",
				Duration: fakeDur, Config: cfg,
				DependsOn: []string{"drawing_submit_r1"}},
			{Name: "voting_r1", Primitive: "drawful.vote_collect",
				Duration: voteDur, Config: cfg,
				DependsOn: []string{"drawing_submit_r1", "fake_prompt_submit_r1"}},
			{Name: "reveal_r1", Primitive: "drawful.reveal", Config: cfg,
				DependsOn: []string{"drawing_submit_r1", "voting_r1"}},
			{Name: "scoring_r1", Primitive: "drawful.scoring", Config: cfg,
				DependsOn: []string{"reveal_r1"}},
		}
	}
}

// newTestRoom wires a Room with Jrawful primitives registered (idempotent),
// an isolated auth store, and a silent logger. The caller is responsible
// for stopping the room (t.Cleanup is fine).
func newTestRoom(t *testing.T, id string, isDemo bool, builder PhaseBuilder) (*Room, *auth.Store) {
	t.Helper()
	drawful.Register()
	store := auth.NewStore()
	room, err := NewRoom(id, isDemo, builder, Defaults{}, store, silentLog())
	if err != nil {
		t.Fatalf("NewRoom: %v", err)
	}
	go room.Run()
	t.Cleanup(func() { room.Stop() })
	return room, store
}

// newPlayer creates an in-process conn + drain + joins the room. Fatals
// on join failure.
func newPlayer(t *testing.T, r *Room, name string) (*gateway.Conn, *drainCapture, engine.PlayerID, string) {
	t.Helper()
	conn := gateway.NewTestConn(silentLog())
	dc := newDrainCapture(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	reply := r.PostJoin(ctx, conn, proto.JoinRoomPayload{RoomID: r.ID, Name: name})
	if reply.Err != nil {
		t.Fatalf("join %s: %v", name, reply.Err)
	}
	return conn, dc, reply.PlayerID, reply.Token
}

func playableGameBuilder(state *engine.GameState) []engine.Phase {
	switch state.Settings.GameID {
	case "draw_duel":
		return drawduel.BuildPhases(state)
	case "fake_artist":
		return fakeartist.BuildPhases(state)
	case "mafia":
		return mafia.BuildPhases(state)
	case "price_is_right":
		return priceisright.BuildPhases(state)
	case "reaction_duel":
		return reactionduel.BuildPhases(state)
	case "split_vote":
		return splitthevote.BuildPhases(state)
	case "jrawful", "":
		fallthrough
	default:
		return drawful.BuildPhases(state)
	}
}

func shrunkPlayableGameBuilder(state *engine.GameState) []engine.Phase {
	phases := playableGameBuilder(state)
	out := make([]engine.Phase, len(phases))
	copy(out, phases)
	for idx := range out {
		if out[idx].Duration > 0 || startsWith(out[idx].Name, "leaderboard") {
			out[idx].Duration = 50 * time.Millisecond
		}
	}
	return out
}

func expectedPlayablePhases(gameID string, playerCount int, settings engine.GameSettings) []string {
	state := &engine.GameState{
		Settings: settings,
		Players:  map[engine.PlayerID]*engine.Player{},
	}
	for idx := 0; idx < playerCount; idx++ {
		pid := engine.PlayerID("player-" + uuid.NewString())
		state.Players[pid] = &engine.Player{
			ID:        pid,
			Name:      "Player",
			Connected: true,
		}
	}
	state.Settings.GameID = gameID
	phases := playableGameBuilder(state)
	names := make([]string, 0, len(phases))
	for _, phase := range phases {
		names = append(names, phase.Name)
	}
	return names
}

// postEnv sends an envelope to the room. Fatals on channel saturation.
func postEnv(t *testing.T, r *Room, conn *gateway.Conn, typ string, payload any) {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal %s: %v", typ, err)
	}
	env := proto.Envelope{V: proto.ProtocolVersion, ID: uuid.NewString(), Type: typ, Payload: raw}
	if err := r.PostInput(conn, env); err != nil {
		t.Fatalf("PostInput %s: %v", typ, err)
	}
}

// waitPhase polls the room's state until it matches wantPhase or the
// deadline fires. Returns the snapshot observed at match time.
func waitPhase(t *testing.T, r *Room, wantPhase string, timeout time.Duration) stateSnapshot {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last stateSnapshot
	for time.Now().Before(deadline) {
		snap, ok := r.snapshotForTest()
		if ok {
			last = snap
			if snap.PhaseName == wantPhase {
				return snap
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for phase %q (last observed: %q)", wantPhase, last.PhaseName)
	return stateSnapshot{}
}

func waitRoomMode(t *testing.T, r *Room, wantMode RoomMode, timeout time.Duration) stateSnapshot {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last stateSnapshot
	for time.Now().Before(deadline) {
		snap, ok := r.snapshotForTest()
		if ok {
			last = snap
			if snap.RoomMode == wantMode {
				return snap
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for room mode %q (last observed: %q)", wantMode, last.RoomMode)
	return stateSnapshot{}
}

// getFromSnapshot is GetResult adapted to a stateSnapshot's PhaseData map.
// Kept local to the test package — production code always reads from a
// live *engine.GameState.
func getFromSnapshot[R any](snap stateSnapshot, phase string) (R, bool) {
	var zero R
	raw, ok := snap.PhaseData[phase]
	if !ok {
		return zero, false
	}
	typed, ok := raw.(engine.StoredResult[R])
	if !ok {
		return zero, false
	}
	return typed.Value, true
}

// -----------------------------------------------------------------------------
// Test 1: Full Jrawful round, all TRUE votes, deterministic scoring.
// -----------------------------------------------------------------------------

func TestIntegration_FullJrawfulRound(t *testing.T) {
	t.Log("Scenario: 3 players (alice, bob, carol) join a fresh room, all ready up, submit blank drawings, submit fakes, then vote TRUE on every non-self drawing.")
	t.Log("Expected: the game walks through prompt_distribute → draw → fake → vote → reveal → scoring, emits S2CGameEnd, and every player has exactly 4000 points.")
	t.Log("Math: per drawing, drawer earns 1000 × 2 TRUE voters = 2000; each voter earns 1000 per correct TRUE × 2 non-self drawings = 2000. Total = 4000.")

	builder := testPhaseBuilder(5*time.Second, 5*time.Second, 5*time.Second)
	r, _ := newTestRoom(t, "TEST", false, builder)

	connA, capA, pidA, _ := newPlayer(t, r, "alice")
	connB, capB, pidB, _ := newPlayer(t, r, "bob")
	connC, capC, pidC, _ := newPlayer(t, r, "carol")
	t.Logf("Joined: alice=%s bob=%s carol=%s", pidA, pidB, pidC)

	// All three ready; third ready triggers onStartGame.
	postEnv(t, r, connA, proto.C2SReady, proto.ReadyPayload{Ready: true})
	postEnv(t, r, connB, proto.C2SReady, proto.ReadyPayload{Ready: true})
	postEnv(t, r, connC, proto.C2SReady, proto.ReadyPayload{Ready: true})
	t.Log("All three readied; onStartGame should fire.")

	// Prompt_distribute is an Aggregate, it auto-advances in Start; the
	// first phase we block on is drawing_submit_r1.
	waitPhase(t, r, "drawing_submit_r1", 2*time.Second)
	t.Log("Reached drawing_submit_r1.")

	// Every player submits an empty but valid stroke drawing. The server
	// pulls the true prompt from PromptDistributionResult; the client
	// payload is shape-only here.
	blankDraw := proto.SubmitDrawingPayload{Format: "strokes", Data: `{"strokes":[]}`}
	postEnv(t, r, connA, proto.C2SSubmitDraw, blankDraw)
	postEnv(t, r, connB, proto.C2SSubmitDraw, blankDraw)
	postEnv(t, r, connC, proto.C2SSubmitDraw, blankDraw)
	t.Log("All three submitted drawings; expect short-circuit to fake_prompt_submit_r1.")

	snap := waitPhase(t, r, "fake_prompt_submit_r1", 2*time.Second)
	t.Log("Reached fake_prompt_submit_r1.")

	// Pull the drawing ids; we need them to address fake/vote payloads.
	draws, ok := getFromSnapshot[primitives.CollectAllResult[drawful.Drawing]](snap, "drawing_submit_r1")
	if !ok {
		t.Fatalf("drawings not in PhaseData")
	}
	if len(draws.Items) != 3 {
		t.Fatalf("want 3 drawings, got %d", len(draws.Items))
	}
	byAuthor := map[engine.PlayerID]drawful.Drawing{}
	for pid, d := range draws.Items {
		byAuthor[pid] = d
	}

	// Each player submits a fake per non-self drawing. Texts are unique
	// per (player, drawing) and intentionally not matching any real prompt.
	submitFake := func(conn *gateway.Conn, did drawful.DrawingID, text string) {
		postEnv(t, r, conn, proto.C2SSubmitFake, proto.SubmitFakePromptPayload{
			DrawingID: string(did), Text: text,
		})
	}
	conns := map[engine.PlayerID]*gateway.Conn{pidA: connA, pidB: connB, pidC: connC}
	for voter, conn := range conns {
		for author, d := range byAuthor {
			if author == voter {
				continue
			}
			submitFake(conn, d.ID, "fake:"+string(voter)+":"+string(d.ID))
		}
	}
	t.Log("Each player submitted one fake per non-self drawing (6 total); expect advance to voting_r1.")

	snap = waitPhase(t, r, "voting_r1", 2*time.Second)
	t.Log("Reached voting_r1.")

	// Every player votes TRUE on every non-self drawing.
	for voter, conn := range conns {
		for author, d := range byAuthor {
			if author == voter {
				continue
			}
			postEnv(t, r, conn, proto.C2SSubmitVote, proto.SubmitVotePayload{
				DrawingID: string(d.ID), ChoiceID: "TRUE",
			})
		}
	}
	t.Log("Every player voted TRUE on every non-self drawing (6 votes total); expect reveal + scoring to cascade and emit game_end.")

	// The reveal + scoring phases are Aggregates that auto-advance.
	// After them, the phase index walks off the end and game_end fires.
	end := capA.waitForType(t, proto.S2CGameEnd, 3*time.Second)
	t.Log("game_end received on alice's conn.")

	var endPayload struct {
		Scores map[string]int `json:"scores"`
	}
	if err := json.Unmarshal(end.Payload, &endPayload); err != nil {
		t.Fatalf("decode game_end: %v", err)
	}

	// Expected: drawer earns 2×1000 (two correct TRUE voters per drawing);
	// each voter earns 2×1000 (two TRUE votes cast per player); fakers earn
	// 0 since nobody voted for a fake. Total per player: 4000.
	t.Logf("Final scores: %+v — expected 4000 apiece.", endPayload.Scores)
	for _, pid := range []engine.PlayerID{pidA, pidB, pidC} {
		if got := endPayload.Scores[string(pid)]; got != 4000 {
			t.Errorf("%s: want 4000, got %d", pid, got)
		}
	}

	// Other two conns should have also received game_end (fan-out check).
	_ = capB.waitForType(t, proto.S2CGameEnd, time.Second)
	_ = capC.waitForType(t, proto.S2CGameEnd, time.Second)

	connA.Close(proto.CloseGoingAway, "test done")
	connB.Close(proto.CloseGoingAway, "test done")
	connC.Close(proto.CloseGoingAway, "test done")
}

func TestIntegration_SplitVoteTwoPlayerScores(t *testing.T) {
	t.Log("Scenario: Alice and Bob join, the leader selects Split the Vote, both ready, Alice sets up the round, Bob is the only voter, and Bob hits the one-voter target.")
	t.Log("Expected: Alice cannot vote, Bob alone determines the split, reveal resolves as achieved, and game_end carries Alice=1500 / Bob=500.")

	drawful.Register()
	splitthevote.Register()
	builder := func(state *engine.GameState) []engine.Phase {
		switch state.Settings.GameID {
		case "split_vote":
			return splitthevote.BuildPhases(state)
		default:
			return drawful.BuildPhases(state)
		}
	}
	r, _ := newTestRoom(t, "SPLIT", false, builder)

	connA, capA, pidA, _ := newPlayer(t, r, "alice")
	connB, capB, pidB, _ := newPlayer(t, r, "bob")

	postEnv(t, r, connA, proto.C2SSelectGame, proto.SelectGamePayload{GameID: "split_vote"})
	postEnv(t, r, connA, proto.C2SReady, proto.ReadyPayload{Ready: true})
	postEnv(t, r, connB, proto.C2SReady, proto.ReadyPayload{Ready: true})
	postEnv(t, r, connA, proto.C2SStartGame, proto.StartGamePayload{})

	waitPhase(t, r, "split_setup_r1", 2*time.Second)
	postEnv(t, r, connA, proto.C2SSubmitSplitSetup, proto.SubmitSplitSetupPayload{
		Prompt:  "Which side wins?",
		OptionA: "Left",
		OptionB: "Right",
	})

	waitPhase(t, r, "split_vote_r1", 2*time.Second)
	postEnv(t, r, connB, proto.C2SSubmitSplitChoice, proto.SubmitSplitChoicePayload{ChoiceID: "B"})

	waitPhase(t, r, "split_reveal_r1", 2*time.Second)
	round := capA.waitForType(t, proto.S2CRoundResult, 2*time.Second)

	var roundPayload struct {
		Scores map[string]int `json:"scores"`
	}
	if err := json.Unmarshal(round.Payload, &roundPayload); err != nil {
		t.Fatalf("decode round_result: %v", err)
	}
	if got := roundPayload.Scores[string(pidA)]; got != 1500 {
		t.Fatalf("alice round_result score: want 1500, got %d (all=%+v)", got, roundPayload.Scores)
	}
	if got := roundPayload.Scores[string(pidB)]; got != 500 {
		t.Fatalf("bob round_result score: want 500, got %d (all=%+v)", got, roundPayload.Scores)
	}

	end := capA.waitForType(t, proto.S2CGameEnd, 15*time.Second)

	var payload struct {
		Scores map[string]int `json:"scores"`
	}
	if err := json.Unmarshal(end.Payload, &payload); err != nil {
		t.Fatalf("decode game_end: %v", err)
	}
	if got := payload.Scores[string(pidA)]; got != 1500 {
		t.Fatalf("alice score: want 1500, got %d (all=%+v)", got, payload.Scores)
	}
	if got := payload.Scores[string(pidB)]; got != 500 {
		t.Fatalf("bob score: want 500, got %d (all=%+v)", got, payload.Scores)
	}

	_ = capB.waitForType(t, proto.S2CGameEnd, time.Second)

	connA.Close(proto.CloseGoingAway, "test done")
	connB.Close(proto.CloseGoingAway, "test done")
}

func TestIntegration_PriceIsRightClosestWithoutGoingOver(t *testing.T) {
	t.Log("Scenario: 3 players join, the leader selects Price is Right, everyone readies, then all three submit guesses for the same listing.")
	t.Log("Expected: the closest guess without going over wins the round and receives 1000 points.")

	drawful.Register()
	priceisright.Register()
	builder := func(state *engine.GameState) []engine.Phase {
		switch state.Settings.GameID {
		case "price_is_right":
			return priceisright.BuildPhases(state)
		default:
			return drawful.BuildPhases(state)
		}
	}
	r, _ := newTestRoom(t, "PRICE", false, builder)

	connA, capA, pidA, _ := newPlayer(t, r, "alice")
	connB, _, pidB, _ := newPlayer(t, r, "bob")
	connC, _, pidC, _ := newPlayer(t, r, "cara")

	postEnv(t, r, connA, proto.C2SSelectGame, proto.SelectGamePayload{GameID: "price_is_right"})
	postEnv(t, r, connA, proto.C2SUpdateSettings, proto.UpdateSettingsPayload{
		RoundCount:         1,
		GeneratedFakeCount: 0,
		DrawingSeconds:     60,
		FakePromptSeconds:  90,
		VotingSeconds:      10,
		GameOptions: map[string]any{
			"minimum_price_dollars": 5,
			"threshold_mode":        "fixed",
		},
	})
	postEnv(t, r, connA, proto.C2SReady, proto.ReadyPayload{Ready: true})
	postEnv(t, r, connB, proto.C2SReady, proto.ReadyPayload{Ready: true})
	postEnv(t, r, connC, proto.C2SReady, proto.ReadyPayload{Ready: true})
	postEnv(t, r, connA, proto.C2SStartGame, proto.StartGamePayload{})

	snap := waitPhase(t, r, "price_guess_r1", 3*time.Second)
	prompt, ok := getFromSnapshot[priceisright.PromptResult](snap, "price_prompt_r1")
	if !ok {
		t.Fatal("missing price prompt result")
	}

	postEnv(t, r, connA, proto.C2SSubmitPriceGuess, proto.SubmitPriceGuessPayload{GuessCents: prompt.ActualPriceCents - 100})
	postEnv(t, r, connB, proto.C2SSubmitPriceGuess, proto.SubmitPriceGuessPayload{GuessCents: prompt.ActualPriceCents + 100})
	postEnv(t, r, connC, proto.C2SSubmitPriceGuess, proto.SubmitPriceGuessPayload{GuessCents: prompt.ActualPriceCents / 2})

	end := capA.waitForType(t, proto.S2CGameEnd, 10*time.Second)
	var payload struct {
		Scores map[string]int `json:"scores"`
	}
	if err := json.Unmarshal(end.Payload, &payload); err != nil {
		t.Fatalf("decode game_end: %v", err)
	}
	if got := payload.Scores[string(pidA)]; got != 1000 {
		t.Fatalf("alice score: want 1000, got %d (scores=%+v)", got, payload.Scores)
	}
	if got := payload.Scores[string(pidB)]; got != 0 {
		t.Fatalf("bob score: want 0, got %d (scores=%+v)", got, payload.Scores)
	}
	if got := payload.Scores[string(pidC)]; got != 0 {
		t.Fatalf("cara score: want 0, got %d (scores=%+v)", got, payload.Scores)
	}

	connA.Close(proto.CloseGoingAway, "test done")
	connB.Close(proto.CloseGoingAway, "test done")
	connC.Close(proto.CloseGoingAway, "test done")
}

func TestIntegration_MafiaTownEliminatesTheMafia(t *testing.T) {
	t.Log("Scenario: 6 players join, the leader selects Mafia, everyone readies up, night actions resolve, the room nominates a suspect, and the town votes out the single Mafia member on day 1.")
	t.Log("Expected: game_end fires after the first day reveal, every non-mafia player has 1000 points, and the mafia player has 0.")

	drawful.Register()
	mafia.Register()
	builder := func(state *engine.GameState) []engine.Phase {
		switch state.Settings.GameID {
		case "mafia":
			return mafia.BuildPhases(state)
		default:
			return drawful.BuildPhases(state)
		}
	}
	r, _ := newTestRoom(t, "MAFIA", false, builder)

	names := []string{"alice", "bob", "cara", "drew", "eli", "fran"}
	conns := make(map[string]*gateway.Conn, len(names))
	var leader *gateway.Conn
	var leaderCap *drainCapture
	for idx, name := range names {
		conn, cap, _, _ := newPlayer(t, r, name)
		conns[name] = conn
		if idx == 0 {
			leader = conn
			leaderCap = cap
		}
	}

	postEnv(t, r, leader, proto.C2SSelectGame, proto.SelectGamePayload{GameID: "mafia"})
	postEnv(t, r, leader, proto.C2SUpdateSettings, proto.UpdateSettingsPayload{
		RoundCount:         1,
		GeneratedFakeCount: 0,
		DrawingSeconds:     10,
		FakePromptSeconds:  10,
		VotingSeconds:      10,
	})
	for _, name := range names {
		postEnv(t, r, conns[name], proto.C2SReady, proto.ReadyPayload{Ready: true})
	}
	postEnv(t, r, leader, proto.C2SStartGame, proto.StartGamePayload{})

	snap := waitPhase(t, r, "mafia_night_collect_r1", 12*time.Second)
	roleResult, ok := getFromSnapshot[mafia.RoleAssignmentResult](snap, "mafia_role_assign")
	if !ok {
		t.Fatal("missing mafia role assignment")
	}
	if len(roleResult.MafiaIDs) != 1 {
		t.Fatalf("expected 1 mafia in a 6-player room, got %d", len(roleResult.MafiaIDs))
	}

	mafiaID := roleResult.MafiaIDs[0]
	targetID := ""
	for _, playerID := range roleResult.Order {
		if roleResult.Roles[playerID] == "citizen" {
			targetID = playerID
			break
		}
	}
	if targetID == "" {
		t.Fatal("missing citizen target for the night")
	}

	for _, playerID := range roleResult.Order {
		switch roleResult.Roles[playerID] {
		case "mafia":
			postEnv(t, r, conns[roleResult.Names[playerID]], proto.C2SSubmitVote, proto.SubmitVotePayload{
				DrawingID: "mafia_night",
				ChoiceID:  targetID,
			})
		case "doctor":
			postEnv(t, r, conns[roleResult.Names[playerID]], proto.C2SSubmitVote, proto.SubmitVotePayload{
				DrawingID: "mafia_night",
				ChoiceID:  targetID,
			})
		case "detective":
			postEnv(t, r, conns[roleResult.Names[playerID]], proto.C2SSubmitVote, proto.SubmitVotePayload{
				DrawingID: "mafia_night",
				ChoiceID:  mafiaID,
			})
		}
	}

	waitPhase(t, r, "mafia_day_nominate_r1", 24*time.Second)
	for _, playerID := range roleResult.Order {
		conn := conns[roleResult.Names[playerID]]
		choice := mafiaID
		if playerID == mafiaID {
			choice = targetID
		}
		postEnv(t, r, conn, proto.C2SSubmitVote, proto.SubmitVotePayload{
			DrawingID: "mafia_nominate",
			ChoiceID:  choice,
		})
	}

	waitPhase(t, r, "mafia_day_vote_r1", 24*time.Second)
	for _, playerID := range roleResult.Order {
		conn := conns[roleResult.Names[playerID]]
		postEnv(t, r, conn, proto.C2SSubmitVote, proto.SubmitVotePayload{
			DrawingID: "mafia_day",
			ChoiceID:  mafiaID,
		})
	}

	end := leaderCap.waitForType(t, proto.S2CGameEnd, 12*time.Second)
	var payload struct {
		Scores map[string]int `json:"scores"`
	}
	if err := json.Unmarshal(end.Payload, &payload); err != nil {
		t.Fatalf("decode game_end: %v", err)
	}

	for _, playerID := range roleResult.Order {
		got := payload.Scores[playerID]
		want := 1000
		if playerID == mafiaID {
			want = 0
		}
		if got != want {
			t.Fatalf("player %s (%s): want %d, got %d (scores=%+v)", roleResult.Names[playerID], roleResult.Roles[playerID], want, got, payload.Scores)
		}
	}

	for _, conn := range conns {
		conn.Close(proto.CloseGoingAway, "test done")
	}
}

func TestIntegration_PlayableGamesPhaseSweep10Players(t *testing.T) {
	t.Log("Scenario: every currently playable picker game launches inside a 10-player room with phase timers compressed for integration coverage.")
	t.Log("Expected: each game emits its full phase_change sequence in order, reaches game_end, and does not surface server errors to the leader.")

	drawful.Register()
	drawduel.Register()
	fakeartist.Register()
	mafia.Register()
	priceisright.Register()
	reactionduel.Register()
	splitthevote.Register()

	settings := engine.GameSettings{
		RoundCount:         1,
		GeneratedFakeCount: 0,
		DrawingSeconds:     1,
		FakePromptSeconds:  1,
		VotingSeconds:      1,
		GameOptions: map[string]any{
			"countdown_seconds":      1,
			"minimum_price_dollars":  5,
			"threshold_mode":         "fixed",
			"target_mode":            "strict_split",
			"split_authoring_mode":   "splitter_prompt_and_options",
			"show_target_to_players": false,
		},
	}

	for _, game := range platformcatalog.DefaultCatalog() {
		if game.Status != platformcatalog.GameStatusAvailable {
			continue
		}
		game := game
		t.Run(game.ID, func(t *testing.T) {
			playerCount := min(10, game.MaxPlayers)
			r, _ := newTestRoom(t, "SWEEP-"+game.ID, false, shrunkPlayableGameBuilder)

			conns := make([]*gateway.Conn, 0, playerCount)
			var leaderConn *gateway.Conn
			var leaderCap *drainCapture
			for idx := 0; idx < playerCount; idx++ {
				conn, cap, _, _ := newPlayer(t, r, fmt.Sprintf("%s-%02d", game.ID, idx+1))
				conns = append(conns, conn)
				if idx == 0 {
					leaderConn = conn
					leaderCap = cap
				}
			}
			t.Cleanup(func() {
				for _, conn := range conns {
					conn.Close(proto.CloseGoingAway, "test done")
				}
			})

			postEnv(t, r, leaderConn, proto.C2SSelectGame, proto.SelectGamePayload{GameID: game.ID})
			postEnv(t, r, leaderConn, proto.C2SUpdateSettings, proto.UpdateSettingsPayload{
				RoundCount:         settings.RoundCount,
				GeneratedFakeCount: settings.GeneratedFakeCount,
				DrawingSeconds:     settings.DrawingSeconds,
				FakePromptSeconds:  settings.FakePromptSeconds,
				VotingSeconds:      settings.VotingSeconds,
				GameOptions:        settings.GameOptions,
			})
			for _, conn := range conns {
				postEnv(t, r, conn, proto.C2SReady, proto.ReadyPayload{Ready: true})
			}
			postEnv(t, r, leaderConn, proto.C2SStartGame, proto.StartGamePayload{})

			expected := expectedPlayablePhases(game.ID, playerCount, settings)
			got := waitPhaseSequence(t, leaderCap, expected, 20*time.Second)
			if !slices.Equal(got, expected) {
				t.Fatalf("phase sequence mismatch for %s\nwant: %+v\ngot:  %+v", game.ID, expected, got)
			}

			if got := countByType(leaderCap, proto.S2CError); got != 0 {
				t.Fatalf("%s emitted %d server errors during phase sweep", game.ID, got)
			}

			snap := waitRoomMode(t, r, RoomModeResults, 5*time.Second)
			if snap.RoomMode != RoomModeResults {
				t.Fatalf("%s room mode: want %q, got %q", game.ID, RoomModeResults, snap.RoomMode)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Test 2: Reconnect restores the original PlayerID.
// -----------------------------------------------------------------------------

func TestIntegration_Reconnect(t *testing.T) {
	t.Log("Scenario: alice joins, gets a session token, then drops (simulated transport blip). A fresh conn resumes with the original token.")
	t.Log("Expected: PostJoin returns the same PlayerID and echoes the same Token; a new JoinAck lands on the new conn; Players[pid].Connected flips back to true.")

	r, _ := newTestRoom(t, "TEST", false, testPhaseBuilder(5*time.Second, 5*time.Second, 5*time.Second))

	// Initial join.
	conn1, _, pid1, tok1 := newPlayer(t, r, "alice")
	t.Logf("Initial join: pid=%s token=%s", pid1, tok1)

	// Simulate a transport blip: reader goroutine exits → Disconnect →
	// PostLeave fires in production. We emulate that directly.
	r.PostLeave(conn1)
	conn1.Close(proto.CloseGoingAway, "simulated drop")
	t.Log("Simulated drop: PostLeave + Close issued on conn1. Waiting for actor to flip Connected=false.")

	// Wait for the actor to process the leave (Connected=false).
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		snap, _ := r.snapshotForTest()
		if p, ok := snap.Players[pid1]; ok && !p.Connected {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// New conn, resume with the token. Actor should restore the PlayerID.
	conn2 := gateway.NewTestConn(silentLog())
	dc2 := newDrainCapture(conn2)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	reply := r.PostJoin(ctx, conn2, proto.JoinRoomPayload{
		RoomID:       r.ID,
		Name:         "alice-was-here",
		SessionToken: tok1,
	})
	t.Logf("Reconnect reply: pid=%s token=%s err=%v", reply.PlayerID, reply.Token, reply.Err)
	if reply.Err != nil {
		t.Fatalf("reconnect: %v", reply.Err)
	}
	if reply.PlayerID != pid1 {
		t.Fatalf("reconnect PlayerID mismatch: want %s, got %s", pid1, reply.PlayerID)
	}
	if reply.Token != tok1 {
		t.Fatalf("reconnect token should echo the original; want %s, got %s", tok1, reply.Token)
	}

	// join_ack should have been sent to the new conn.
	ack := dc2.waitForType(t, proto.S2CJoinAck, time.Second)
	var ackPayload proto.JoinAckPayload
	if err := json.Unmarshal(ack.Payload, &ackPayload); err != nil {
		t.Fatalf("decode join_ack: %v", err)
	}
	if ackPayload.PlayerID != string(pid1) {
		t.Fatalf("join_ack player_id: want %s, got %s", pid1, ackPayload.PlayerID)
	}

	// Player should now be marked connected again in state.
	snap, ok := r.snapshotForTest()
	if !ok {
		t.Fatal("snapshot failed")
	}
	p, ok := snap.Players[pid1]
	if !ok || !p.Connected {
		t.Fatalf("player not connected after reconnect: %+v", p)
	}

	conn2.Close(proto.CloseGoingAway, "test done")
}

// -----------------------------------------------------------------------------
// Regression: disconnect re-evaluates the start-game predicate.
// Bug: before the fix, allReadyAndEnough was only rechecked on ready
// toggles, so a 4-player lobby where 3 were ready and the 4th dropped
// could sit idle forever even though the predicate was satisfied.
// -----------------------------------------------------------------------------

func TestIntegration_DisconnectStartsGame(t *testing.T) {
	t.Log("Scenario: 4 players join; 3 ready up; the 4th (never-readied) drops before pressing ready.")
	t.Log("Expected: on disconnect the server re-evaluates allReadyAndEnough — connected=3, readies=3 — and advances out of StatusIdle without any further ready toggle.")

	builder := testPhaseBuilder(5*time.Second, 5*time.Second, 5*time.Second)
	r, _ := newTestRoom(t, "TEST", false, builder)

	connA, _, _, _ := newPlayer(t, r, "alice")
	connB, _, _, _ := newPlayer(t, r, "bob")
	connC, _, _, _ := newPlayer(t, r, "carol")
	connD, _, pidD, _ := newPlayer(t, r, "dave") // dave never readies
	t.Logf("Four players joined; dave=%s will drop without readying.", pidD)

	postEnv(t, r, connA, proto.C2SReady, proto.ReadyPayload{Ready: true})
	postEnv(t, r, connB, proto.C2SReady, proto.ReadyPayload{Ready: true})
	postEnv(t, r, connC, proto.C2SReady, proto.ReadyPayload{Ready: true})

	// Confirm we're still in lobby: 3 ready + 1 not-ready connected player
	// means allReadyAndEnough() returns false (readies != connected). Give
	// the actor a beat to process the three ready toggles.
	time.Sleep(50 * time.Millisecond)
	snap, ok := r.snapshotForTest()
	if !ok {
		t.Fatal("snapshot failed")
	}
	if snap.Status != StatusIdle {
		t.Fatalf("pre-disconnect status: want %s, got %s", StatusIdle, snap.Status)
	}
	t.Log("Confirmed: room is still StatusIdle because dave hasn't readied.")

	// Dave drops. Under the old code, the room sits here forever. Under
	// the fix, onLeave calls allReadyAndEnough which now returns true
	// (connected=3, readies=3) and the game starts.
	r.PostLeave(connD)
	connD.Close(proto.CloseGoingAway, "simulated drop")
	t.Log("Dave disconnected. Room should now advance to in-game on its own.")

	// Wait for the first real phase (drawing_submit_r1). prompt_distribute
	// is an Aggregate that auto-advances, so it's not a reliable wait target.
	waitPhase(t, r, "drawing_submit_r1", 2*time.Second)
	t.Log("Reached drawing_submit_r1 — the regression is fixed.")

	connA.Close(proto.CloseGoingAway, "test done")
	connB.Close(proto.CloseGoingAway, "test done")
	connC.Close(proto.CloseGoingAway, "test done")
}

// -----------------------------------------------------------------------------
// Test 3: Demo eviction closes conns with code 4000.
// -----------------------------------------------------------------------------

func TestIntegration_DemoEviction(t *testing.T) {
	t.Log("Scenario: a demo room with one player gets evicted (reason=main_started). Grace is shrunk to 50ms for fast feedback.")
	t.Log("Expected: clients receive S2CRoomEvicting before the grace elapses, conns close with CloseDemoEvicted (4000), and the room returns to StatusIdle.")

	// Shrink the grace window so the test doesn't sit for 5s. Restore
	// after — tests are serial in a package unless t.Parallel is set.
	origGrace := DemoEvictionGrace
	DemoEvictionGrace = 50 * time.Millisecond
	t.Cleanup(func() { DemoEvictionGrace = origGrace })
	t.Logf("Shrunk DemoEvictionGrace: %s -> %s", origGrace, DemoEvictionGrace)

	r, _ := newTestRoom(t, "DEMO", true, testPhaseBuilder(5*time.Second, 5*time.Second, 5*time.Second))

	conn, dc, _, _ := newPlayer(t, r, "demo-player")

	r.PostEvict("main_started")
	t.Log("PostEvict(main_started) sent; waiting for S2CRoomEvicting broadcast.")

	// Clients should see a room_evicting notice within the grace window.
	_ = dc.waitForType(t, proto.S2CRoomEvicting, time.Second)
	t.Log("room_evicting received; waiting for socket close after grace.")

	// After the grace elapses, the actor runs onEvictFinalize which closes
	// sockets with CloseDemoEvicted=4000. Wait for IsClosed.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if conn.IsClosed() {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !conn.IsClosed() {
		t.Fatal("demo conn should be closed after eviction grace")
	}
	if got := conn.LastCloseCode(); got != proto.CloseDemoEvicted {
		t.Fatalf("close code: want %d, got %d", proto.CloseDemoEvicted, got)
	}
	t.Logf("Conn closed with code %d (CloseDemoEvicted).", conn.LastCloseCode())

	// Room should be idle again for future demo clients.
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		status, _ := r.PublicStatus()
		if status == StatusIdle {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if status, _ := r.PublicStatus(); status != StatusIdle {
		t.Fatalf("room should be idle after eviction, got %s", status)
	}
}

// -----------------------------------------------------------------------------
// Test 4: Phase deadline fires with zero submissions → game completes with
// zero scores, no hangs. Exercises the §5 timer drain and CollectAll.Timeout.
// -----------------------------------------------------------------------------

func TestIntegration_ZeroSubmissionDeadline(t *testing.T) {
	t.Log("Scenario: 3 players ready up but never submit anything. All three collection phases (draw/fake/vote) have 80ms timers.")
	t.Log("Expected: every phase times out cleanly via CollectAll.Timeout, reveal+scoring still run over empty state, S2CGameEnd fires with all zeros. No hangs, no deadlocks.")

	// All three time-bounded phases get 80ms each — well under the test
	// deadline but long enough that scheduler jitter won't flake the race.
	short := 80 * time.Millisecond
	r, _ := newTestRoom(t, "TEST", false, testPhaseBuilder(short, short, short))

	connA, capA, pidA, _ := newPlayer(t, r, "alice")
	connB, _, pidB, _ := newPlayer(t, r, "bob")
	connC, _, pidC, _ := newPlayer(t, r, "carol")

	postEnv(t, r, connA, proto.C2SReady, proto.ReadyPayload{Ready: true})
	postEnv(t, r, connB, proto.C2SReady, proto.ReadyPayload{Ready: true})
	postEnv(t, r, connC, proto.C2SReady, proto.ReadyPayload{Ready: true})
	t.Log("Readied up; no further input. Waiting for the cascade of timeouts to emit game_end (budget ≈3 × 80ms + overhead).")

	// With zero inputs, the drawing phase expires on its timer; fake and
	// vote also time out (empty drawings collection → nothing to collect);
	// reveal + scoring aggregates run over empty state. Game_end fires.
	end := capA.waitForType(t, proto.S2CGameEnd, 2*time.Second)
	t.Log("game_end received after timeout cascade.")
	var payload struct {
		Scores map[string]int `json:"scores"`
	}
	if err := json.Unmarshal(end.Payload, &payload); err != nil {
		t.Fatalf("decode game_end: %v", err)
	}
	// All zero — nobody drew, nobody voted. The important assertion is
	// that it ended at all.
	t.Logf("Final scores: %+v — expected all zero.", payload.Scores)
	for _, pid := range []engine.PlayerID{pidA, pidB, pidC} {
		if payload.Scores[string(pid)] != 0 {
			t.Errorf("%s: want 0, got %d", pid, payload.Scores[string(pid)])
		}
	}

	connA.Close(proto.CloseGoingAway, "test done")
	connB.Close(proto.CloseGoingAway, "test done")
	connC.Close(proto.CloseGoingAway, "test done")
}

// -----------------------------------------------------------------------------
// Test 5: Panic runbook. A primitive that panics in Start() triggers
// recoverPanic, which broadcasts ErrRoomCrashed and closes all sockets
// with CloseAbnormal (1011). See §13.
// -----------------------------------------------------------------------------

var panicPrimRegistered sync.Once

// panicPrim panics on Start so we can exercise the room's recover path.
type panicPrim struct{}

func (panicPrim) Name() string { return "test.panic" }
func (panicPrim) Start(*engine.PhaseContext) (engine.AnyDecision, error) {
	panic("intentional test panic")
}
func (panicPrim) Handle(*engine.PhaseContext, engine.Input) (engine.AnyDecision, error) {
	return engine.AnyDecision{}, nil
}
func (panicPrim) Timeout(*engine.PhaseContext) (engine.AnyDecision, error) {
	return engine.AnyDecision{}, nil
}

func registerPanicPrim() {
	panicPrimRegistered.Do(func() {
		engine.Register("test.panic", func(json.RawMessage) (engine.AnyPrimitive, error) {
			return panicPrim{}, nil
		})
	})
}

func TestIntegration_PanicRunbook(t *testing.T) {
	t.Log("Scenario: the second phase's primitive panics inside Start(). 3 players are connected when this happens.")
	t.Log("Expected: recoverPanic catches the panic, broadcasts S2CError{code=room_crashed} to every conn, closes all sockets with CloseAbnormal (1011), and transitions the room to StatusClosed.")

	registerPanicPrim()
	// Phase list: one Jrawful prompt phase, then the panicking primitive.
	// Using a real Jrawful phase first ensures the phase index parser and
	// status transitions look "real" before the panic fires.
	builder := func(state *engine.GameState) []engine.Phase {
		return []engine.Phase{
			{Name: "prompt_distribute_r1", Primitive: "drawful.prompt_distribute"},
			{Name: "boom_r1", Primitive: "test.panic",
				DependsOn: []string{"prompt_distribute_r1"}},
		}
	}
	r, _ := newTestRoom(t, "TEST", false, builder)

	connA, capA, _, _ := newPlayer(t, r, "alice")
	connB, capB, _, _ := newPlayer(t, r, "bob")
	connC, capC, _, _ := newPlayer(t, r, "carol")

	postEnv(t, r, connA, proto.C2SReady, proto.ReadyPayload{Ready: true})
	postEnv(t, r, connB, proto.C2SReady, proto.ReadyPayload{Ready: true})
	postEnv(t, r, connC, proto.C2SReady, proto.ReadyPayload{Ready: true})
	t.Log("Readied up; game starts, prompt_distribute runs cleanly, then boom_r1 panics in Start(). Waiting for S2CError.")

	// Every conn should receive S2CError with ErrRoomCrashed, then close.
	errEnv := capA.waitForType(t, proto.S2CError, time.Second)
	var errPayload proto.ErrorPayload
	if err := json.Unmarshal(errEnv.Payload, &errPayload); err != nil {
		t.Fatalf("decode error payload: %v", err)
	}
	if errPayload.Code != proto.ErrRoomCrashed {
		t.Fatalf("expected %s, got %s", proto.ErrRoomCrashed, errPayload.Code)
	}
	t.Logf("alice received S2CError code=%s. Checking fan-out to bob and carol.", errPayload.Code)
	// Fan-out — the other two conns should see the same error.
	_ = capB.waitForType(t, proto.S2CError, time.Second)
	_ = capC.waitForType(t, proto.S2CError, time.Second)
	t.Log("All three conns received the error broadcast. Waiting for sockets to close with CloseAbnormal (1011).")

	// All three conns close with CloseAbnormal.
	for name, c := range map[string]*gateway.Conn{"A": connA, "B": connB, "C": connC} {
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			if c.IsClosed() {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		if !c.IsClosed() {
			t.Fatalf("conn %s: still open after panic", name)
		}
		if got := c.LastCloseCode(); got != proto.CloseAbnormal {
			t.Errorf("conn %s: close code want %d, got %d", name, proto.CloseAbnormal, got)
		}
	}

	// Room status should be closed (actor goroutine has exited).
	status, _ := r.PublicStatus()
	if status != StatusClosed {
		t.Fatalf("room status want %s, got %s", StatusClosed, status)
	}
	t.Log("Room transitioned to StatusClosed — actor goroutine has exited cleanly.")
}

// -----------------------------------------------------------------------------
// Leader / pause / pencils-down integration tests
// -----------------------------------------------------------------------------
//
// These pin the contract between the client and the actor around the three
// party-leader controls shipped alongside the reveal animation:
//
//   1. First-joiner becomes leader; S2CLeaderChange fires exactly once per
//      promotion. On disconnect, the next earliest-joined connected player
//      is promoted deterministically.
//   2. set_pause freezes the phase clock: the phase does NOT advance on its
//      original deadline. Resuming arms a fresh timer with the captured
//      remaining budget.
//   3. set_pencils_down rejects incoming C2SSubmitDraw with ErrNotAllowed
//      while still accepting submissions once lifted.
//   4. Non-leaders calling set_pause get ErrNotAllowed.
//
// Each test uses a short phase duration so pause-capture math can be asserted
// with real time. Test constants keep the timing stable on CI.

// TestIntegration_LeaderPromotionOnDisconnect pins §leader promotion: first
// fresh joiner is leader; when they disconnect, the next joinOrder entry is
// promoted and S2CLeaderChange fires with the new id.
func TestIntegration_LeaderPromotionOnDisconnect(t *testing.T) {
	t.Log("Scenario: alice joins first (leader), bob + carol follow. alice disconnects.")
	t.Log("Expected: initial S2CLeaderChange names alice; after alice drops, S2CLeaderChange fires again with bob's PlayerID (earliest-joined survivor).")

	r, _ := newTestRoom(t, "TEST", false, testPhaseBuilder(5*time.Second, 5*time.Second, 5*time.Second))

	connA, capA, pidA, _ := newPlayer(t, r, "alice")
	_, _, pidB, _ := newPlayer(t, r, "bob")
	_, capC, _, _ := newPlayer(t, r, "carol")

	// Initial leader_change names alice (first fresh join seeds leaderID).
	firstLeader := capA.waitForType(t, proto.S2CLeaderChange, time.Second)
	var firstPayload proto.LeaderChangePayload
	if err := json.Unmarshal(firstLeader.Payload, &firstPayload); err != nil {
		t.Fatalf("decode leader_change: %v", err)
	}
	if firstPayload.LeaderID != string(pidA) {
		t.Fatalf("initial leader: want %s, got %s", pidA, firstPayload.LeaderID)
	}
	t.Logf("Initial leader is alice (%s) as expected.", pidA)

	// alice disconnects; promotion walks joinOrder → bob.
	r.PostLeave(connA)
	connA.Close(proto.CloseGoingAway, "alice drop")

	// carol should see the promotion broadcast (fan-out goes to every
	// connected player, including the new leader's peers).
	promo := capC.waitForType(t, proto.S2CLeaderChange, time.Second)
	var promoPayload proto.LeaderChangePayload
	if err := json.Unmarshal(promo.Payload, &promoPayload); err != nil {
		t.Fatalf("decode promotion leader_change: %v", err)
	}
	if promoPayload.LeaderID != string(pidB) {
		t.Fatalf("promoted leader: want %s (bob), got %s", pidB, promoPayload.LeaderID)
	}
	t.Logf("After alice dropped, bob (%s) was promoted to leader.", pidB)
}

// TestIntegration_PauseFreezesPhaseTimer verifies that a pause issued by the
// leader halts the phase-timer countdown. Without pause, a 100ms drawing phase
// would time out well before 300ms; with pause, it must still be live.
func TestIntegration_PauseFreezesPhaseTimer(t *testing.T) {
	t.Log("Scenario: short 100ms drawing phase. Leader alice sends set_pause{paused:true} ~20ms in. Test sleeps past the original deadline.")
	t.Log("Expected: phase is still drawing_submit_r1 after the original deadline would have elapsed, because the phaseTimer was frozen on pause.")

	// 100ms draw phase — long enough to pause before it times out, short
	// enough that the test fails fast if pause doesn't actually freeze.
	builder := testPhaseBuilder(100*time.Millisecond, 5*time.Second, 5*time.Second)
	r, _ := newTestRoom(t, "TEST", false, builder)

	connA, _, _, _ := newPlayer(t, r, "alice") // leader
	connB, _, _, _ := newPlayer(t, r, "bob")
	connC, _, _, _ := newPlayer(t, r, "carol")

	postEnv(t, r, connA, proto.C2SReady, proto.ReadyPayload{Ready: true})
	postEnv(t, r, connB, proto.C2SReady, proto.ReadyPayload{Ready: true})
	postEnv(t, r, connC, proto.C2SReady, proto.ReadyPayload{Ready: true})
	waitPhase(t, r, "drawing_submit_r1", 2*time.Second)

	// Pause ~20ms in — well before the 100ms phase deadline.
	time.Sleep(20 * time.Millisecond)
	postEnv(t, r, connA, proto.C2SSetPause, proto.SetPausePayload{Paused: true})

	// Give the actor a beat to process the pause message.
	time.Sleep(20 * time.Millisecond)

	// Sleep past what would have been the original deadline (+150ms buffer).
	time.Sleep(200 * time.Millisecond)

	snap, ok := r.snapshotForTest()
	if !ok {
		t.Fatal("snapshot failed")
	}
	if snap.PhaseName != "drawing_submit_r1" {
		t.Fatalf("phase advanced despite pause: want drawing_submit_r1, got %s", snap.PhaseName)
	}
	t.Log("Phase did not advance past its original 100ms deadline — pause is freezing the phaseTimer as designed.")

	// Resume to let the test clean up without blocking on a stale timer.
	postEnv(t, r, connA, proto.C2SSetPause, proto.SetPausePayload{Paused: false})
	connA.Close(proto.CloseGoingAway, "test done")
	connB.Close(proto.CloseGoingAway, "test done")
	connC.Close(proto.CloseGoingAway, "test done")
}

// TestIntegration_PencilsDownRejectsSubmit ensures the server-side trust
// boundary rejects C2SSubmitDraw while pencils are down, with ErrNotAllowed.
// Defense-in-depth: even if the client's canvas disable flag is bypassed, the
// submit still fails at the actor.
func TestIntegration_PencilsDownRejectsSubmit(t *testing.T) {
	t.Log("Scenario: drawing phase is live. Leader alice flips pencils-down on. Bob attempts to submit a drawing.")
	t.Log("Expected: bob receives S2CError{code=ErrNotAllowed}; no drawing is stored. Flipping pencils-down off allows subsequent submissions through.")

	builder := testPhaseBuilder(5*time.Second, 5*time.Second, 5*time.Second)
	r, _ := newTestRoom(t, "TEST", false, builder)

	connA, _, _, _ := newPlayer(t, r, "alice")
	connB, capB, _, _ := newPlayer(t, r, "bob")
	connC, _, _, _ := newPlayer(t, r, "carol")

	postEnv(t, r, connA, proto.C2SReady, proto.ReadyPayload{Ready: true})
	postEnv(t, r, connB, proto.C2SReady, proto.ReadyPayload{Ready: true})
	postEnv(t, r, connC, proto.C2SReady, proto.ReadyPayload{Ready: true})
	waitPhase(t, r, "drawing_submit_r1", 2*time.Second)

	// Leader flips pencils-down on.
	postEnv(t, r, connA, proto.C2SSetPencilsDown, proto.SetPencilsDownPayload{Disabled: true})
	// Let the actor absorb the pencils-down toggle before bob submits.
	time.Sleep(20 * time.Millisecond)

	// Bob tries to submit → rejected.
	postEnv(t, r, connB, proto.C2SSubmitDraw, proto.SubmitDrawingPayload{
		Format: "strokes", Data: `{"strokes":[]}`,
	})
	rej := capB.waitForType(t, proto.S2CError, time.Second)
	var rejPayload proto.ErrorPayload
	if err := json.Unmarshal(rej.Payload, &rejPayload); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if rejPayload.Code != proto.ErrNotAllowed {
		t.Fatalf("reject code: want %s, got %s (msg=%q)", proto.ErrNotAllowed, rejPayload.Code, rejPayload.Message)
	}
	t.Logf("Bob's submit rejected with %s (%q) — server-side pencils-down enforcement confirmed.", rejPayload.Code, rejPayload.Message)

	// Leader lifts pencils-down; bob's next submit must go through. We don't
	// have a direct "drawing stored" signal, but we can assert no new S2CError
	// arrives within a brief window after a valid submit.
	postEnv(t, r, connA, proto.C2SSetPencilsDown, proto.SetPencilsDownPayload{Disabled: false})
	time.Sleep(20 * time.Millisecond)

	errCountBefore := countByType(capB, proto.S2CError)
	postEnv(t, r, connB, proto.C2SSubmitDraw, proto.SubmitDrawingPayload{
		Format: "strokes", Data: `{"strokes":[]}`,
	})
	time.Sleep(30 * time.Millisecond)
	errCountAfter := countByType(capB, proto.S2CError)
	if errCountAfter != errCountBefore {
		t.Fatalf("unexpected new S2CError after pencils lifted (before=%d after=%d)", errCountBefore, errCountAfter)
	}
	t.Log("After pencils-down lifted, bob's resubmit produced no new S2CError — submissions accepted as expected.")

	connA.Close(proto.CloseGoingAway, "test done")
	connB.Close(proto.CloseGoingAway, "test done")
	connC.Close(proto.CloseGoingAway, "test done")
}

// TestIntegration_NonLeaderPauseRejected pins the leader-only guard on
// set_pause. A non-leader calling pause gets ErrNotAllowed; no state change.
func TestIntegration_NonLeaderPauseRejected(t *testing.T) {
	t.Log("Scenario: alice (leader), bob, carol. bob attempts set_pause.")
	t.Log("Expected: bob gets S2CError{code=ErrNotAllowed}; room.paused stays false.")

	r, _ := newTestRoom(t, "TEST", false, testPhaseBuilder(5*time.Second, 5*time.Second, 5*time.Second))

	connA, _, _, _ := newPlayer(t, r, "alice")
	connB, capB, _, _ := newPlayer(t, r, "bob")
	connC, _, _, _ := newPlayer(t, r, "carol")

	postEnv(t, r, connB, proto.C2SSetPause, proto.SetPausePayload{Paused: true})
	rej := capB.waitForType(t, proto.S2CError, time.Second)
	var rejPayload proto.ErrorPayload
	if err := json.Unmarshal(rej.Payload, &rejPayload); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if rejPayload.Code != proto.ErrNotAllowed {
		t.Fatalf("reject code: want %s, got %s", proto.ErrNotAllowed, rejPayload.Code)
	}
	t.Log("Non-leader pause rejected with ErrNotAllowed.")

	connA.Close(proto.CloseGoingAway, "test done")
	connB.Close(proto.CloseGoingAway, "test done")
	connC.Close(proto.CloseGoingAway, "test done")
}

// countByType is a small drainCapture helper — returns how many envelopes of
// the given type we've seen so far.
func countByType(d *drainCapture, typ string) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := 0
	for _, e := range d.frames {
		if e.Type == typ {
			n++
		}
	}
	return n
}

// TestIntegration_FreshJoinResetsAbandonedGame pins the abandonment-recovery
// path in onJoin. Prior behavior: if a game reached StatusInGame and every
// player then disconnected (tab closed, Playwright killed, network drop
// without a reconnect attempt), the room stayed in StatusInGame forever, and
// the next fresh join was rejected with "game in progress; cannot join".
// That caused the Playwright suite to hang whenever a previous run crashed
// mid-game against the same dev server (reuseExistingServer: true).
//
// Expected: on a fresh join into a StatusInGame room whose socket set is
// empty, the room wipes its per-game state, returns to StatusIdle, and
// admits the joiner. Reconnect path (session_token) is unaffected and still
// takes precedence — this only triggers for truly fresh joins.
func TestIntegration_FreshJoinResetsAbandonedGame(t *testing.T) {
	t.Log("Scenario: 3 players start a game (status=InGame), then all three disconnect without the game reaching game_end. A fourth, fresh joiner arrives some time later.")
	t.Log("Expected: instead of ErrNotAllowed, the room resets to StatusIdle, wipes Players/Scores/joinOrder/leader/paused/pencils, and admits the joiner as the new leader.")

	builder := testPhaseBuilder(5*time.Second, 5*time.Second, 5*time.Second)
	r, _ := newTestRoom(t, "TEST", false, builder)

	// Bring up an in-progress game.
	connA, _, pidA, _ := newPlayer(t, r, "alice")
	connB, _, _, _ := newPlayer(t, r, "bob")
	connC, _, _, _ := newPlayer(t, r, "carol")
	postEnv(t, r, connA, proto.C2SReady, proto.ReadyPayload{Ready: true})
	postEnv(t, r, connB, proto.C2SReady, proto.ReadyPayload{Ready: true})
	postEnv(t, r, connC, proto.C2SReady, proto.ReadyPayload{Ready: true})
	waitPhase(t, r, "drawing_submit_r1", 2*time.Second)
	t.Logf("Game started with alice (leader=%s) / bob / carol.", pidA)

	// All three abandon mid-game. Mirrors "Playwright killed after draw phase
	// began" or "every tab crashed". No postGame latch — we never reached
	// the terminal game_end broadcast.
	r.PostLeave(connA)
	r.PostLeave(connB)
	r.PostLeave(connC)
	connA.Close(proto.CloseGoingAway, "abandon")
	connB.Close(proto.CloseGoingAway, "abandon")
	connC.Close(proto.CloseGoingAway, "abandon")

	// Give the actor a beat to process the three leaves. We expect the room
	// to still be flagged StatusInGame at this point — the old code had no
	// path to auto-reset.
	time.Sleep(50 * time.Millisecond)
	snap, ok := r.snapshotForTest()
	if !ok {
		t.Fatal("snapshot failed after leaves")
	}
	if snap.Status != StatusInGame {
		t.Fatalf("pre-rejoin status: want %s (stale abandoned), got %s", StatusInGame, snap.Status)
	}
	t.Logf("Confirmed: room is stuck in %s with zero conns. Pre-fix this rejected any fresh join.", snap.Status)

	// Fresh joiner arrives. Under the new recovery path this should succeed.
	conn, _, pidD, _ := newPlayer(t, r, "dave")
	t.Logf("dave joined as %s; verifying the room was fully wiped.", pidD)

	snap, ok = r.snapshotForTest()
	if !ok {
		t.Fatal("snapshot failed after dave joined")
	}
	if snap.Status != StatusIdle {
		t.Fatalf("post-rejoin status: want %s, got %s", StatusIdle, snap.Status)
	}
	if snap.Round != 0 {
		t.Errorf("round should reset: want 0, got %d", snap.Round)
	}
	if snap.PhaseName != "" {
		t.Errorf("phase should be cleared: want empty, got %q", snap.PhaseName)
	}
	if len(snap.Players) != 1 {
		t.Errorf("players map should contain only dave: got %d entries (%+v)", len(snap.Players), snap.Players)
	}
	if _, ok := snap.Players[pidD]; !ok {
		t.Errorf("dave (%s) missing from players map", pidD)
	}
	for _, s := range snap.Scores {
		if s != 0 {
			t.Errorf("scores should be wiped, got %+v", snap.Scores)
			break
		}
	}

	conn.Close(proto.CloseGoingAway, "test done")
}

// TestIntegration_FreshJoinWithLiveParticipantsStillRejected is the
// negative twin of the previous test: as long as at least one socket is
// still attached, StatusInGame is authoritative and late joiners must be
// rejected. Otherwise a mid-game spectator could hijack the room and we'd
// drop the live game on the floor.
func TestIntegration_FreshJoinWithLiveParticipantsStillRejected(t *testing.T) {
	t.Log("Scenario: 3 players start a game; two disconnect but alice stays connected. A fresh joiner (dave) arrives.")
	t.Log("Expected: dave's join is rejected with the existing 'game in progress; cannot join' error — the abandonment reset only fires when the socket set is truly empty.")

	builder := testPhaseBuilder(5*time.Second, 5*time.Second, 5*time.Second)
	r, _ := newTestRoom(t, "TEST", false, builder)

	connA, _, _, _ := newPlayer(t, r, "alice")
	connB, _, _, _ := newPlayer(t, r, "bob")
	connC, _, _, _ := newPlayer(t, r, "carol")
	postEnv(t, r, connA, proto.C2SReady, proto.ReadyPayload{Ready: true})
	postEnv(t, r, connB, proto.C2SReady, proto.ReadyPayload{Ready: true})
	postEnv(t, r, connC, proto.C2SReady, proto.ReadyPayload{Ready: true})
	waitPhase(t, r, "drawing_submit_r1", 2*time.Second)

	// Drop bob + carol. alice stays — someone is still holding the game open.
	r.PostLeave(connB)
	r.PostLeave(connC)
	connB.Close(proto.CloseGoingAway, "abandon")
	connC.Close(proto.CloseGoingAway, "abandon")
	time.Sleep(30 * time.Millisecond)

	// Dave tries to join. Should be rejected.
	conn := gateway.NewTestConn(silentLog())
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	reply := r.PostJoin(ctx, conn, proto.JoinRoomPayload{RoomID: r.ID, Name: "dave"})
	if reply.Err == nil {
		t.Fatalf("dave join should have been rejected (alice is still in-game), got %+v", reply)
	}
	t.Logf("As expected, dave's fresh join was rejected: %v", reply.Err)

	connA.Close(proto.CloseGoingAway, "test done")
	conn.Close(proto.CloseGoingAway, "test done")
}
