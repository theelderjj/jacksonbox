// Package room contains the Room Manager and Room Actor. The actor owns
// all GameState mutation; no one else touches it. See docs/DESIGN.md §6, §12.
package room

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/jj/trivia/internal/auth"
	"github.com/jj/trivia/internal/engine"
	"github.com/jj/trivia/internal/gateway"
	platformcatalog "github.com/jj/trivia/internal/platform/catalog"
	"github.com/jj/trivia/internal/proto"
	"github.com/jj/trivia/internal/reveal"
)

// RevealStepInterval is retained for compatibility with older timed reveal
// behavior. Leaderboard reveal is now leader-driven one click at a time, but
// tests and pause bookkeeping still reference the symbol.
//
// Exposed as var (not const) so integration tests can shrink it; production
// callers must not mutate it.
var RevealStepInterval = 900 * time.Millisecond

// Note: RevealStepHold moved to internal/reveal.StepHold to break the
// room <-> games/drawful import cycle that surfaced once room integration
// tests started pulling in a game mode. The room actor reads reveal.StepHold
// directly where the inter-drawing gap duration is needed.

// DisconnectGrace: §11. After this, pending inputs get default substitutes.
// Exposed as var (not const) so integration tests can shrink it; production
// callers must not mutate it.
var DisconnectGrace = 60 * time.Second

// DemoEvictionGrace: §7. Demo clients get this long before hard close.
// Exposed as var (not const) so integration tests can shrink it; production
// callers must not mutate it.
var DemoEvictionGrace = 5 * time.Second

// InboxSize: §12.
const InboxSize = 256

// Status of a room — §7.
type Status string

const (
	StatusIdle     Status = "idle"
	StatusInGame   Status = "in_game"
	StatusEvicting Status = "evicting"
	StatusClosed   Status = "closed"
)

type RoomMode string

const (
	RoomModeGamePicker RoomMode = "game_picker"
	RoomModeGameLobby  RoomMode = "game_lobby"
	RoomModeInGame     RoomMode = "in_game"
	RoomModeResults    RoomMode = "results"
)

// Room is the actor's public handle. The goroutine runs Loop(); callers
// post messages via the exported methods which enqueue onto inbox.
// PhaseBuilder produces the full phase list for a game run. Called at game
// start once the player count is known, so round-count-dependent games
// (e.g., Jrawful's configured round count) can inline the right number of rounds.
type PhaseBuilder func(state *engine.GameState) []engine.Phase

type Room struct {
	ID       string
	IsDemo   bool
	log      *slog.Logger
	store    *auth.Store
	builder  PhaseBuilder
	phases   []engine.Phase // populated at onStartGame
	defaults Defaults       // game-specific default generators for disconnects

	inbox    chan actorMsg
	shutdown chan struct{}

	// Owned exclusively by the goroutine below this line.
	state      *engine.GameState
	status     Status
	conns      map[engine.PlayerID]*gateway.Conn
	phaseTimer *time.Timer
	heartbeat  *time.Ticker
	current    engine.AnyPrimitive
	// postGame latches between S2CGameEnd and the first ready toggle for a
	// new game. While true, snapshotRoomState reports phase="game_end" so
	// clients keep the GameEnd screen mounted even through incidental
	// state_update broadcasts (joins/leaves) during the post-game lull.
	postGame bool

	// joinOrder preserves first-connected ordering so leader promotion
	// picks a deterministic "next up" on disconnect. PlayerIDs of every
	// fresh joiner are appended here; reconnects keep their existing slot.
	joinOrder []engine.PlayerID
	// leaderID is the party leader — the only player whose set_pause /
	// set_pencils_down messages are honored. Empty string means "no leader
	// yet" (lobby hasn't been joined), rare in practice.
	leaderID engine.PlayerID

	// Pause state. paused freezes the phaseTimer and stepTimer. Leader only.
	// pauseRemaining is how much phaseTimer had left when we paused (so we
	// can resume with the same budget). stepTimer has its own cached
	// remaining in stepRemaining.
	paused         bool
	pencilsDown    bool
	phaseDeadline  time.Time     // absolute deadline while running; zero when no timer
	pauseRemaining time.Duration // phase-timer remaining at pause time
	stepRemaining  time.Duration // step-timer remaining at pause time (0 if stepTimer inactive)

	// Reveal step animation. stepTimer and stepState are non-nil only while
	// running the leaderboard phase. stepState.cursor indexes the queue of
	// step events the actor will emit one-per-RevealStepInterval.
	stepTimer      *time.Timer
	stepState      *revealStepState
	settings       engine.GameSettings
	selectedGameID string
	rerolled       map[string]bool
	drawn          map[string]bool

	// Readable from outside the actor (manager uses for gating). Atomic-ish
	// via the statusMu guarding this cell and conns count.
	publicMu     sync.RWMutex
	pubStatus    Status
	pubConnCount int
}

// revealStepState is the server-side cursor driving the leaderboard-phase
// reveal animation. The step queue is fully pre-computed at phase start;
// the actor just walks it on each stepTimer fire. Pause freezes the walk
// by remembering how far we got.
type revealStepState struct {
	round int
	queue []reveal.Step
	idx   int
}

// Defaults is the game-specific substitution set called when a player
// passes the disconnect grace period with pending inputs. Keeps the
// engine free of Jrawful-specific logic while still letting the actor
// satisfy §11's "game does not stall" requirement.
//
// BuildRevealSteps is called when a leaderboard_rN phase starts so the
// game mode can supply the per-round step animation queue. Absent hook =
// no animation (client renders reveal statically). The returned slice
// uses reveal.Step (in internal/reveal) specifically so game modes don't
// need to import room — that was the cycle we broke when adding the hook.
type Defaults struct {
	ApplyFor         func(state *engine.GameState, phase string, playerID engine.PlayerID) (engine.Input, bool)
	BuildRevealSteps func(state *engine.GameState, round int) []reveal.Step
	RerollPrompt     func(state *engine.GameState, promptPhase string, playerID engine.PlayerID) (string, []engine.Event, error)
}

// NewRoom constructs a room with the given game configuration.
// The room does NOT start its goroutine; call Run.
func NewRoom(id string, isDemo bool, builder PhaseBuilder, defaults Defaults, store *auth.Store, log *slog.Logger) (*Room, error) {
	if builder == nil {
		return nil, fmt.Errorf("room: PhaseBuilder required")
	}
	return &Room{
		ID:       id,
		IsDemo:   isDemo,
		log:      log.With("room_id", id),
		store:    store,
		builder:  builder,
		defaults: defaults,
		inbox:    make(chan actorMsg, InboxSize),
		shutdown: make(chan struct{}),
		status:   StatusIdle,
		conns:    map[engine.PlayerID]*gateway.Conn{},
		settings: normalizeSettings(engine.GameSettings{}),
		rerolled: map[string]bool{},
		drawn:    map[string]bool{},
	}, nil
}

// Run drives the actor loop. Blocks until shutdown. Launches in a goroutine.
func (r *Room) Run() {
	r.state = r.newGameState()
	r.heartbeat = time.NewTicker(2 * time.Second)
	defer r.heartbeat.Stop()

	defer r.recoverPanic()

	for {
		select {
		case msg := <-r.inbox:
			r.handle(msg)
		case <-timerChan(r.phaseTimer):
			r.onTimeout()
		case <-timerChan(r.stepTimer):
			r.onRevealStep()
		case <-r.heartbeat.C:
			r.onTick()
		case <-r.shutdown:
			return
		}
	}
}

// timerChan returns the timer's channel or nil (which blocks forever in select).
func timerChan(t *time.Timer) <-chan time.Time {
	if t == nil {
		return nil
	}
	return t.C
}

// recoverPanic implements the runbook from §13.
func (r *Room) recoverPanic() {
	p := recover()
	if p == nil {
		return
	}
	phase := "(no phase)"
	round := 0
	if r.state != nil {
		if idx := r.state.PhaseIdx; idx < len(r.phases) && idx >= 0 {
			phase = r.phases[idx].Name
		}
		round = r.state.Round
	}
	r.log.Error("room actor panic",
		"panic", fmt.Sprintf("%v", p),
		"phase", phase,
		"round", round,
		"stack", string(debug.Stack()))

	r.setStatus(StatusClosed)
	for id, c := range r.conns {
		c.SendEnvelope(proto.S2CError, proto.ErrorPayload{
			Code: proto.ErrRoomCrashed, Message: "room crashed; reconnect into a fresh lobby",
		})
		c.Close(proto.CloseAbnormal, "room crashed")
		delete(r.conns, id)
	}
	close(r.shutdown)
}

// --- External (non-actor) surface ---

// Stop signals the actor to exit. Safe to call multiple times.
func (r *Room) Stop() {
	select {
	case <-r.shutdown:
		return
	default:
		close(r.shutdown)
	}
}

// PublicStatus returns a snapshot for the manager's gating logic. Safe
// from outside the actor.
func (r *Room) PublicStatus() (Status, int) {
	r.publicMu.RLock()
	defer r.publicMu.RUnlock()
	return r.pubStatus, r.pubConnCount
}

// PostJoin enqueues a join request and waits for the reply (blocking on
// the caller side). Non-blocking on the actor.
func (r *Room) PostJoin(ctx context.Context, conn *gateway.Conn, p proto.JoinRoomPayload) joinReply {
	reply := make(chan joinReply, 1)
	select {
	case r.inbox <- actorMsg{kind: kindJoin, conn: conn, joinPayload: p, reply: reply}:
	case <-ctx.Done():
		return joinReply{Err: ctx.Err()}
	default:
		return joinReply{Err: gateway.ErrInboxFull}
	}
	select {
	case res := <-reply:
		return res
	case <-ctx.Done():
		return joinReply{Err: ctx.Err()}
	}
}

// PostInput enqueues a game input. Non-blocking; returns ErrInboxFull on saturation.
func (r *Room) PostInput(conn *gateway.Conn, env proto.Envelope) error {
	select {
	case r.inbox <- actorMsg{kind: kindInput, conn: conn, env: env}:
		return nil
	default:
		return gateway.ErrInboxFull
	}
}

// PostLeave enqueues an explicit leave (connection closed).
func (r *Room) PostLeave(conn *gateway.Conn) {
	select {
	case r.inbox <- actorMsg{kind: kindLeave, conn: conn, playerID: conn.PlayerID}:
	default:
		// Dropping a leave is survivable — heartbeat will eventually prune.
	}
}

// PostEvict triggers the demo-eviction sequence.
func (r *Room) PostEvict(reason string) {
	select {
	case r.inbox <- actorMsg{kind: kindEvict, evictReason: reason}:
	default:
	}
}

// PostStart triggers a transition from lobby into the first real phase.
// Used by tests and by an operator-hook later.
func (r *Room) PostStart() {
	select {
	case r.inbox <- actorMsg{kind: kindStartGame}:
	default:
	}
}

// snapshotForTest is a test-only synchronous state reader. The actor
// services a snapshot message so callers observe a point-in-time copy
// with no race. Returns (_, false) if the actor is unresponsive (e.g.,
// exited after a panic recovery) within 500ms.
func (r *Room) snapshotForTest() (stateSnapshot, bool) {
	reply := make(chan stateSnapshot, 1)
	select {
	case r.inbox <- actorMsg{kind: kindSnapshot, snapshotReply: reply}:
	case <-time.After(500 * time.Millisecond):
		return stateSnapshot{}, false
	}
	select {
	case s := <-reply:
		return s, true
	case <-time.After(500 * time.Millisecond):
		return stateSnapshot{}, false
	}
}

// --- Actor-side handlers ---

func (r *Room) handle(m actorMsg) {
	switch m.kind {
	case kindJoin:
		r.onJoin(m)
	case kindLeave:
		r.onLeave(m)
	case kindInput:
		r.onInput(m)
	case kindEvict:
		r.onEvict(m)
	case kindEvictFinalize:
		r.onEvictFinalize()
	case kindStartGame:
		r.onStartGame()
	case kindSnapshot:
		r.onSnapshot(m)
	}
}

// onSnapshot services a test-only synchronous state read. Actor-safe: runs
// inside the goroutine, replies on the caller's channel. Never called from
// production code paths.
func (r *Room) onSnapshot(m actorMsg) {
	snap := stateSnapshot{
		Status:         r.status,
		RoomMode:       r.roomMode(),
		PhaseIdx:       r.state.PhaseIdx,
		Round:          r.state.Round,
		SelectedGameID: r.selectedGameID,
		PhaseData:      map[string]engine.PhaseResult{},
		Scores:         map[engine.PlayerID]int{},
		Players:        map[engine.PlayerID]engine.Player{},
	}
	if r.state.PhaseIdx >= 0 && r.state.PhaseIdx < len(r.phases) {
		snap.PhaseName = r.phases[r.state.PhaseIdx].Name
	}
	for k, v := range r.state.PhaseData {
		snap.PhaseData[k] = v
	}
	for k, v := range r.state.Scores {
		snap.Scores[k] = v
	}
	for k, v := range r.state.Players {
		snap.Players[k] = *v
	}
	m.snapshotReply <- snap
}

func (r *Room) newGameState() *engine.GameState {
	return &engine.GameState{
		RoomID:    r.ID,
		LeaderID:  r.leaderID,
		Players:   map[engine.PlayerID]*engine.Player{},
		PhaseIdx:  0,
		Round:     0,
		Settings:  r.settings,
		PhaseData: map[string]engine.PhaseResult{},
		Scores:    map[engine.PlayerID]int{},
	}
}

func normalizeSettings(s engine.GameSettings) engine.GameSettings {
	if s.GameID == "" {
		s.GameID = "jrawful"
	}
	if s.GameOptions == nil {
		s.GameOptions = map[string]any{}
	}
	if s.RoundCount < 1 {
		s.RoundCount = 1
	}
	if s.RoundCount > 60 {
		s.RoundCount = 60
	}
	if s.GeneratedFakeCount < 0 {
		s.GeneratedFakeCount = 0
	}
	if s.GeneratedFakeCount > 6 {
		s.GeneratedFakeCount = 6
	}
	if s.DrawingSeconds < 10 {
		s.DrawingSeconds = 60
	}
	if s.DrawingSeconds > 300 {
		s.DrawingSeconds = 300
	}
	if s.FakePromptSeconds < 10 {
		s.FakePromptSeconds = 90
	}
	if s.FakePromptSeconds > 300 {
		s.FakePromptSeconds = 300
	}
	if s.VotingSeconds < 10 {
		s.VotingSeconds = 20
	}
	if s.VotingSeconds > 180 {
		s.VotingSeconds = 180
	}
	return s
}

func (r *Room) roomMode() RoomMode {
	switch {
	case r.status == StatusInGame:
		return RoomModeInGame
	case r.postGame:
		return RoomModeResults
	case r.selectedGameID == "":
		return RoomModeGamePicker
	default:
		return RoomModeGameLobby
	}
}

func protoCatalogGames() []proto.GameDefinition {
	source := platformcatalog.DefaultCatalog()
	out := make([]proto.GameDefinition, 0, len(source))
	for _, game := range source {
		tags := make([]string, len(game.Tags))
		copy(tags, game.Tags)
		out = append(out, proto.GameDefinition{
			ID:               game.ID,
			Name:             game.Name,
			Summary:          game.Summary,
			MinPlayers:       game.MinPlayers,
			MaxPlayers:       game.MaxPlayers,
			EstimatedMinutes: game.EstimatedMinutes,
			Tags:             tags,
			Status:           string(game.Status),
		})
	}
	return out
}

func (r *Room) setStatus(s Status) {
	r.status = s
	r.publicMu.Lock()
	r.pubStatus = s
	r.publicMu.Unlock()
}

func (r *Room) updateConnCount() {
	n := 0
	for _, p := range r.state.Players {
		if p.Connected {
			n++
		}
	}
	r.publicMu.Lock()
	r.pubConnCount = n
	r.publicMu.Unlock()
}

func (r *Room) onJoin(m actorMsg) {
	now := time.Now()

	// Reconnect path
	if m.joinPayload.SessionToken != "" {
		if pid, name, ok := r.store.Resume(m.joinPayload.SessionToken, now); ok {
			p, exists := r.state.Players[pid]
			if !exists {
				p = &engine.Player{ID: pid, SessionToken: m.joinPayload.SessionToken, Name: name}
				r.state.Players[pid] = p
			}
			p.Connected = true
			p.LastSeen = now
			m.conn.PlayerID = pid
			m.conn.RoomID = r.ID
			r.conns[pid] = m.conn
			r.updateConnCount()
			m.reply <- joinReply{PlayerID: pid, Token: m.joinPayload.SessionToken}
			r.sendJoinAck(m.conn, pid, m.joinPayload.SessionToken)
			r.broadcastState()
			r.log.Info("reconnect", "player_id", pid, "outcome", "restored")
			return
		}
		r.log.Info("reconnect", "outcome", "expired_or_invalid")
		// fall through to a fresh join
	}

	// Fresh join
	if r.status == StatusClosed || r.status == StatusEvicting {
		m.reply <- joinReply{Err: errors.New("room not accepting joins")}
		return
	}

	// Abandonment recovery. If postGame is latched OR the game is still
	// flagged in-progress but every socket is gone, the previous session's
	// participants have all disappeared. A fresh joiner is effectively
	// starting a new session — collapse the latch, stop any lingering phase
	// or step timer, and wipe per-game state so this player lands in Lobby
	// rather than on a stale GameEnd with someone else's scoreboard (or
	// worse, getting rejected for a game nobody is playing any more).
	//
	// Without this, back-to-back test runs — or any real "last viewer
	// closed the tab before clicking Ready for another" / "Playwright
	// killed mid-game leaving the room stuck in StatusInGame" case —
	// stall joiners on the terminal screen (or on the Lobby with an
	// ErrNotAllowed banner) forever. Stale disconnected Player records
	// from the prior game are also cleared here so the Lobby doesn't
	// render ghost names.
	//
	// Ordering note: the reconnect path above (session_token != "" &&
	// Resume succeeded) short-circuits before we get here, so players
	// who reconnect within the token TTL still land on their prior
	// state — this reset only fires for genuinely fresh joins.
	abandoned := (r.postGame || r.status == StatusInGame) && len(r.conns) == 0
	if abandoned {
		r.postGame = false
		r.phases = nil
		r.current = nil
		r.stopTimer()
		r.stopStepTimer()
		r.stepState = nil
		r.phaseDeadline = time.Time{}
		r.pauseRemaining = 0
		r.stepRemaining = 0
		r.setStatus(StatusIdle)
		r.state.PhaseIdx = 0
		r.state.Round = 0
		r.state.Settings = r.settings
		r.state.PhaseData = map[string]engine.PhaseResult{}
		r.state.Scores = map[engine.PlayerID]int{}
		r.state.Players = map[engine.PlayerID]*engine.Player{}
		r.joinOrder = nil
		r.leaderID = ""
		r.state.LeaderID = ""
		r.selectedGameID = ""
		r.paused = false
		r.pencilsDown = false
		r.rerolled = map[string]bool{}
		r.drawn = map[string]bool{}
		r.log.Info("room reset", "reason", "all_disconnected_stale_game")
	}

	// After potential reset the only status that still blocks fresh joins
	// is an active game with live participants.
	if r.status == StatusInGame {
		m.reply <- joinReply{Err: errors.New("game in progress; cannot join")}
		return
	}

	name, err := proto.CleanName(m.joinPayload.Name)
	if err != nil {
		m.reply <- joinReply{Err: err}
		return
	}

	pid, tok := r.store.Issue(name, now)
	p := &engine.Player{
		ID: pid, SessionToken: tok, Name: name,
		Connected: true, LastSeen: now,
	}
	r.state.Players[pid] = p
	m.conn.PlayerID = pid
	m.conn.RoomID = r.ID
	r.conns[pid] = m.conn
	// Append to joinOrder (only for fresh joins — reconnects don't re-queue).
	r.joinOrder = append(r.joinOrder, pid)
	// First-ever fresh join seeds the leader. Leader election is
	// deterministic: first connected player in joinOrder. Never changes
	// unless the leader disconnects, in which case onLeave promotes the
	// next connected entry.
	leaderChanged := false
	if r.leaderID == "" {
		r.leaderID = pid
		r.state.LeaderID = pid
		leaderChanged = true
	}
	r.updateConnCount()

	m.reply <- joinReply{PlayerID: pid, Token: tok}
	r.sendJoinAck(m.conn, pid, tok)
	if leaderChanged {
		r.broadcastLeader()
	}
	r.broadcastState()
	r.log.Info("join", "player_id", pid, "name", name, "leader", r.leaderID)
}

func (r *Room) sendJoinAck(conn *gateway.Conn, pid engine.PlayerID, token string) {
	conn.SendEnvelope(proto.S2CJoinAck, proto.JoinAckPayload{
		PlayerID:     string(pid),
		SessionToken: token,
		RoomState:    r.snapshotRoomState(),
	})
}

func (r *Room) snapshotRoomState() proto.RoomState {
	players := make([]proto.PlayerInfo, 0, len(r.state.Players))
	for _, p := range r.state.Players {
		players = append(players, proto.PlayerInfo{
			ID: string(p.ID), Name: p.Name, Connected: p.Connected, Ready: p.Ready,
		})
	}
	scores := map[string]int{}
	for id, s := range r.state.Scores {
		scores[string(id)] = s
	}
	phase := ""
	var deadlineMs int64
	switch {
	case r.postGame:
		// Keep clients on GameEnd for any broadcastState during the
		// post-game lull (pre "ready for another").
		phase = "game_end"
	case r.state.PhaseIdx >= 0 && r.state.PhaseIdx < len(r.phases):
		phase = r.phases[r.state.PhaseIdx].Name
	}
	// Deadline: authoritative value while running; while paused we
	// intentionally report zero and rely on RemainingMs so clients can
	// render "paused — 23s left" without a constantly-advancing wall clock.
	if !r.paused && !r.phaseDeadline.IsZero() {
		deadlineMs = r.phaseDeadline.UnixMilli()
	}
	var remainingMs int64
	if r.paused {
		remainingMs = r.pauseRemaining.Milliseconds()
	}
	return proto.RoomState{
		RoomID:      r.ID,
		Status:      string(r.status),
		RoomMode:    string(r.roomMode()),
		Phase:       phase,
		Round:       r.state.Round,
		Players:     players,
		Scores:      scores,
		DeadlineMs:  deadlineMs,
		LeaderID:    string(r.leaderID),
		Paused:      r.paused,
		PencilsDown: r.pencilsDown,
		RemainingMs: remainingMs,
		Settings: proto.SettingsState{
			GameID:             r.settings.GameID,
			RoundCount:         r.settings.RoundCount,
			GeneratedFakeCount: r.settings.GeneratedFakeCount,
			DrawingSeconds:     r.settings.DrawingSeconds,
			FakePromptSeconds:  r.settings.FakePromptSeconds,
			VotingSeconds:      r.settings.VotingSeconds,
			GameOptions:        r.settings.GameOptions,
		},
		SelectedGameID: r.selectedGameID,
		GameCatalog:    protoCatalogGames(),
	}
}

func (r *Room) onLeave(m actorMsg) {
	pid := m.playerID
	p, ok := r.state.Players[pid]
	if !ok {
		return
	}
	p.Connected = false
	p.LastSeen = time.Now()
	delete(r.conns, pid)
	r.updateConnCount()
	r.log.Info("disconnect", "player_id", pid)

	// Leader promotion. If the departing player was the leader, pick the
	// next connected entry in joinOrder (i.e., earliest-joined survivor).
	// If no one is connected, leaderID stays put — the eventual reconnect
	// or fresh join will either restore or replace it. The explicit empty
	// case keeps postGame teardown's leader-reset path uncontested.
	if pid == r.leaderID {
		next := r.pickLeader()
		if next != r.leaderID {
			r.leaderID = next
			r.state.LeaderID = next
			if next != "" {
				// If we were paused by the departing leader, preserve that
				// state — the new leader inherits control. No action needed
				// here; the pause/pencils fields are not cleared.
				r.broadcastLeader()
			}
		}
	}

	r.broadcastState()
	// Default substitution happens in onTick after grace elapses.

	// Re-evaluate the start condition: if the last un-ready player just
	// dropped, the remaining connected players may now satisfy
	if r.status == StatusIdle && r.selectedGameID == "" && r.allReadyAndEnough() {
		r.selectedGameID = "jrawful"
		r.onStartGame()
	}
}

// pickLeader returns the earliest-joined still-connected player's ID, or ""
// if no one is connected. Used for leader promotion.
func (r *Room) pickLeader() engine.PlayerID {
	for _, id := range r.joinOrder {
		if p, ok := r.state.Players[id]; ok && p.Connected {
			return id
		}
	}
	return ""
}

func (r *Room) onInput(m actorMsg) {
	// §5 stale-phase guard: reject messages targeted at a prior phase.
	currentPhase := ""
	if r.state.PhaseIdx < len(r.phases) {
		currentPhase = r.phases[r.state.PhaseIdx].Name
	}

	// Leader controls: set_pause / set_pencils_down. Honored in any status
	// (even lobby) so the leader can pre-set pencils-down before the first
	// drawing phase begins. Non-leader gets ErrNotAllowed inside the
	// handler, so we don't filter here.
	if m.env.Type == proto.C2SSetPause {
		r.handleSetPause(m)
		return
	}
	if m.env.Type == proto.C2SSetPencilsDown {
		r.handleSetPencilsDown(m)
		return
	}
	if m.env.Type == proto.C2SAdvanceReveal {
		r.handleAdvanceReveal(m)
		return
	}
	if m.env.Type == proto.C2SUpdateSettings {
		r.handleUpdateSettings(m)
		return
	}
	if m.env.Type == proto.C2SRerollPrompt {
		r.handleRerollPrompt(m)
		return
	}
	if m.env.Type == proto.C2SSelectGame {
		r.handleSelectGame(m)
		return
	}
	if m.env.Type == proto.C2SStartGame {
		r.handleStartGame(m)
		return
	}
	if m.env.Type == proto.C2SReturnToPicker {
		r.handleReturnToPicker(m)
		return
	}

	// The ready message is lobby-only; we handle it here rather than via a primitive.
	if m.env.Type == proto.C2SReady {
		if r.status != StatusIdle {
			m.conn.SendError(proto.ErrWrongPhase, "ready only allowed in lobby", m.env.ID)
			return
		}
		var p proto.ReadyPayload
		if err := json.Unmarshal(m.env.Payload, &p); err != nil {
			m.conn.SendError(proto.ErrBadPayload, err.Error(), m.env.ID)
			return
		}
		if player, ok := r.state.Players[m.conn.PlayerID]; ok {
			// First ready after game_end collapses the post-game latch and
			// wipes per-game fields so the room is a clean lobby. Done here
			// (not at startPhase game-end) so the GameEnd screen can render
			// final scores until someone explicitly moves on.
			if r.postGame {
				r.postGame = false
				r.phases = nil
				r.state.PhaseIdx = 0
				r.state.Round = 0
				r.state.Settings = r.settings
				r.state.PhaseData = map[string]engine.PhaseResult{}
				r.state.Scores = map[engine.PlayerID]int{}
				r.rerolled = map[string]bool{}
				r.drawn = map[string]bool{}
			}
			player.Ready = p.Ready
			r.broadcastState()
			if r.selectedGameID == "" && r.allReadyAndEnough() {
				r.selectedGameID = "jrawful"
				r.onStartGame()
			}
		}
		return
	}

	if r.status != StatusInGame || r.current == nil {
		m.conn.SendError(proto.ErrWrongPhase, "game not in progress", m.env.ID)
		return
	}

	// While paused the leader has frozen all clocks; game inputs are
	// rejected to prevent last-second submits from leaking past a pause
	// boundary. Leader controls (set_pause / set_pencils_down) already
	// returned above, so they're unaffected.
	if r.paused {
		m.conn.SendError(proto.ErrNotAllowed, "game is paused", m.env.ID)
		return
	}

	// Pencils-down: reject drawing submissions server-side as defense in
	// depth. The client-side canvas also short-circuits pointer events,
	// but the server is the trust boundary.
	if r.pencilsDown && m.env.Type == proto.C2SSubmitDraw {
		m.conn.SendError(proto.ErrNotAllowed, "drawing is locked by the party leader", m.env.ID)
		return
	}

	// Phase-kind guard: message type must match the phase family. This is
	// the "phase-guard in validate()" from §5's race resolution.
	if !phaseAcceptsType(currentPhase, m.env.Type) {
		m.conn.SendError(proto.ErrWrongPhase, "type "+m.env.Type+" not allowed in phase "+currentPhase, m.env.ID)
		return
	}

	in := engine.Input{
		PlayerID:    m.conn.PlayerID,
		Phase:       currentPhase,
		Type:        m.env.Type,
		Value:       m.env.Payload,
		ClientMsgID: m.env.ID,
		Timestamp:   time.Now(),
	}

	decision, err := r.current.Handle(r.phaseCtx(), in)
	if err != nil {
		m.conn.SendError(proto.ErrNotAllowed, err.Error(), m.env.ID)
		return
	}
	if m.env.Type == proto.C2SSubmitDraw {
		r.drawn[roundPlayerKey(r.state.Round, m.conn.PlayerID)] = true
	}
	r.apply(decision)
}

func (r *Room) phaseCtx() *engine.PhaseContext {
	phase := r.phases[r.state.PhaseIdx]
	return &engine.PhaseContext{
		State:    r.state,
		Phase:    phase,
		Deadline: time.Now().Add(phase.Duration),
		Now:      time.Now,
		Broadcast: func(e engine.Event) {
			r.broadcastEvent(e)
		},
	}
}

func (r *Room) onTimeout() {
	if r.status != StatusInGame || r.current == nil {
		r.stopTimer()
		return
	}
	decision, err := r.current.Timeout(r.phaseCtx())
	if err != nil {
		r.log.Error("primitive timeout error", "err", err)
	}
	r.apply(decision)
}

func (r *Room) onTick() {
	now := time.Now()
	// Disconnect default substitution — §11. Skipped while paused: defaults
	// are a "keep the game moving" fallback, and pausing intentionally stops
	// the clock for everyone (including absent players).
	if !r.paused {
		for pid, p := range r.state.Players {
			if !p.Connected && now.Sub(p.LastSeen) > DisconnectGrace && r.defaults.ApplyFor != nil && r.status == StatusInGame {
				phase := r.phases[r.state.PhaseIdx].Name
				if fake, ok := r.defaults.ApplyFor(r.state, phase, pid); ok {
					// Feed the default through Handle to preserve invariants.
					if d, err := r.current.Handle(r.phaseCtx(), fake); err == nil {
						r.apply(d)
					}
				}
			}
		}
	}
	r.log.Debug("tick", "inbox_depth", len(r.inbox), "conn_count", len(r.conns), "paused", r.paused)
}

func (r *Room) onEvict(m actorMsg) {
	if !r.IsDemo {
		r.log.Warn("evict requested on non-demo room")
		return
	}
	r.setStatus(StatusEvicting)
	r.broadcastType(proto.S2CRoomEvicting, proto.RoomEvictingPayload{
		Reason:  m.evictReason,
		GraceMs: DemoEvictionGrace.Milliseconds(),
	})
	// Finalize via the actor so conns map mutation stays single-writer.
	time.AfterFunc(DemoEvictionGrace, func() {
		select {
		case r.inbox <- actorMsg{kind: kindEvictFinalize}:
		default:
			// Inbox saturated — rare; room will reset on next heartbeat.
		}
	})
}

// onEvictFinalize runs inside the actor. Closes every socket with 4000,
// resets room state to a clean lobby. See §7.
func (r *Room) onEvictFinalize() {
	for pid, c := range r.conns {
		c.Close(proto.CloseDemoEvicted, "demo evicted")
		delete(r.conns, pid)
	}
	r.state = r.newGameState()
	r.stopTimer()
	r.current = nil
	r.setStatus(StatusIdle)
	r.updateConnCount()
	r.log.Info("demo room reset")
}

// --- Game lifecycle ---

func (r *Room) allReadyAndEnough() bool {
	connected := 0
	readies := 0
	for _, p := range r.state.Players {
		if p.Connected {
			connected++
			if p.Ready {
				readies++
			}
		}
	}
	minPlayers := 3
	gameID := r.selectedGameID
	if gameID == "" {
		gameID = "jrawful"
	}
	if game, ok := platformcatalog.FindGame(gameID); ok && game.MinPlayers > 0 {
		minPlayers = game.MinPlayers
	}
	return connected >= minPlayers && readies == connected
}

func (r *Room) onStartGame() {
	if r.status == StatusInGame {
		return
	}
	if r.selectedGameID == "" {
		return
	}
	r.settings = normalizeSettings(r.settings)
	r.settings.GameID = r.selectedGameID
	r.state.Settings = r.settings
	r.rerolled = map[string]bool{}
	r.drawn = map[string]bool{}
	// Build the phase sequence now that player count is known. Validated
	// each start so a buggy builder surfaces at game-start rather than later.
	phases := r.builder(r.state)
	if err := engine.ValidatePhaseSequence(phases); err != nil {
		r.log.Error("phase sequence invalid", "err", err)
		panic(err) // caught by recoverPanic → clean room close
	}
	r.phases = phases
	r.setStatus(StatusInGame)
	r.state.StartedAt = time.Now()
	r.state.Round = 1
	r.state.PhaseIdx = 0
	r.state.PhaseData = map[string]engine.PhaseResult{}
	r.log.Info("game start", "players", len(r.state.Players), "phase_count", len(phases))
	r.startPhase()
}

func (r *Room) startPhase() {
	if r.state.PhaseIdx >= len(r.phases) {
		// Game over. Two earlier bugs lived here:
		//
		//   1) `newGameState()` wiped the Players map entirely, so any
		//      subsequent C2SReady from the "Ready for another" button on
		//      GameEnd.tsx silently failed because the server no longer
		//      recognized the player.
		//   2) Broadcasting state right after game_end overwrote the client's
		//      phase="game_end" with phase="" on the very next wire frame,
		//      causing clients to flash GameEnd for ~0 frames before routing
		//      back to Lobby.
		//
		// Fix: flip an explicit postGame flag. Players map stays intact,
		// final scores stay intact, phase list stays intact. snapshotRoomState
		// reports phase="game_end" while postGame is true, so any incidental
		// broadcastState from onJoin/onLeave/onReady during this window still
		// keeps clients on the GameEnd screen.
		//
		// The flag clears when a player readies for a new game — onReady
		// performs the per-game reset there.
		r.setStatus(StatusIdle)
		r.broadcastType(proto.S2CGameEnd, struct {
			Scores map[string]int `json:"scores"`
		}{Scores: r.scoresJSON()})
		r.postGame = true
		for _, p := range r.state.Players {
			p.Ready = false
		}
		r.current = nil
		r.stopTimer()
		return
	}
	phase := r.phases[r.state.PhaseIdx]
	if round := roundFromPhaseName(phase.Name); round > 0 {
		r.state.Round = round
	}
	prim, err := engine.Make(phase)
	if err != nil {
		r.log.Error("unknown primitive", "phase", phase.Name, "err", err)
		panic(err) // startup invariant failure; recoverPanic handles it
	}
	r.current = prim

	deadline := time.Time{}
	// Always stop any lingering phaseTimer + stepTimer first — entering a
	// new phase is the natural boundary for reveal animation reset.
	r.stopTimer()
	r.stopStepTimer()
	r.stepState = nil
	r.phaseDeadline = time.Time{}
	r.pauseRemaining = 0
	r.stepRemaining = 0
	if phase.Duration > 0 {
		deadline = time.Now().Add(phase.Duration)
		r.phaseDeadline = deadline
		r.phaseTimer = time.NewTimer(phase.Duration)
	}

	r.broadcastType(proto.S2CPhaseChange, proto.PhaseChangePayload{
		Phase:      phase.Name,
		Round:      r.state.Round,
		DeadlineMs: timeMs(deadline),
	})
	r.log.Info("phase start", "phase", phase.Name, "round", r.state.Round)

	// leaderboard_rN needs the side-channel reveal step animator. Kicked off
	// here (not inside the primitive) because the actor owns the stepTimer
	// and must be able to pause it in lock-step with phaseTimer.
	if isLeaderboardPhase(phase.Name) {
		r.initRevealSteps()
	}

	d, err := r.current.Start(r.phaseCtx())
	if err != nil {
		r.log.Error("primitive start error", "err", err)
	}
	r.apply(d)
}

func (r *Room) apply(d engine.AnyDecision) {
	for _, ev := range d.Broadcast {
		r.broadcastEvent(ev)
	}
	if !d.AdvancePhase {
		return
	}
	// Store result under current phase name.
	if d.Result != nil {
		r.state.PhaseData[r.phases[r.state.PhaseIdx].Name] = d.Result
	}
	r.stopTimer()
	from := r.phases[r.state.PhaseIdx].Name
	if r.state.EndAfterAdvance {
		r.state.EndAfterAdvance = false
		r.state.PhaseIdx = len(r.phases)
	} else {
		r.state.PhaseIdx++
	}
	to := "end"
	if r.state.PhaseIdx < len(r.phases) {
		to = r.phases[r.state.PhaseIdx].Name
	}
	r.log.Info("phase advance", "from", from, "to", to, "round", r.state.Round)
	r.startPhase()
}

// stopTimer stops and drains the phase timer per §5. Non-blocking drain
// so we don't deadlock if the channel is already empty.
func (r *Room) stopTimer() {
	if r.phaseTimer == nil {
		return
	}
	if !r.phaseTimer.Stop() {
		select {
		case <-r.phaseTimer.C:
		default:
		}
	}
	r.phaseTimer = nil
}

// --- Broadcast helpers ---

func (r *Room) broadcastType(typ string, payload any) {
	frame, err := proto.EncodeEnvelope(uuid.NewString(), typ, time.Now().UnixMilli(), payload)
	if err != nil {
		r.log.Error("encode broadcast", "err", err, "type", typ)
		return
	}
	for pid, c := range r.conns {
		if !c.Send(frame) {
			r.log.Warn("writer drop", "player_id", pid, "type", typ)
		}
	}
}

func (r *Room) broadcastEvent(e engine.Event) {
	r.broadcastType(e.Type, e.Payload)
}

func (r *Room) broadcastState() {
	r.broadcastType(proto.S2CStateUpdate, r.snapshotRoomState())
}

func (r *Room) scoresJSON() map[string]int {
	out := map[string]int{}
	for id, s := range r.state.Scores {
		out[string(id)] = s
	}
	return out
}

func timeMs(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

// phaseAcceptsType is the per-phase message allow-list. Kept here rather
// than pushed into the game package because the allowed set is fully
// derivable from the phase name — and having it here keeps the room's
// §5 guard obvious at review time.
func phaseAcceptsType(phaseName, msgType string) bool {
	switch msgType {
	case proto.C2SSubmitDraw:
		return startsWith(phaseName, "drawing_submit") ||
			startsWith(phaseName, "draw_duel_draw") ||
			startsWith(phaseName, "fake_artist_draw")
	case proto.C2SSubmitFake:
		return startsWith(phaseName, "fake_prompt_submit")
	case proto.C2SSubmitVote:
		return startsWith(phaseName, "voting") ||
			startsWith(phaseName, "draw_duel_vote") ||
			startsWith(phaseName, "fake_artist_vote") ||
			startsWith(phaseName, "mafia_night_collect") ||
			startsWith(phaseName, "mafia_day_nominate") ||
			startsWith(phaseName, "mafia_day_vote") ||
			startsWith(phaseName, "mafia_day_revote")
	case proto.C2SSubmitTap:
		return startsWith(phaseName, "reaction_countdown") ||
			startsWith(phaseName, "reaction_wait") ||
			startsWith(phaseName, "reaction_tap")
	case proto.C2SSubmitPriceGuess:
		return startsWith(phaseName, "price_guess")
	case proto.C2SSubmitSplitSetup:
		return startsWith(phaseName, "split_setup")
	case proto.C2SSubmitSplitChoice:
		return startsWith(phaseName, "split_vote")
	case proto.C2SSubmitFakeArtistGuess:
		return startsWith(phaseName, "fake_artist_guess")
	case proto.C2SReady:
		return false // ready is lobby-only; handled before we get here
	}
	return false
}

func isLeaderboardPhase(name string) bool { return startsWith(name, "leaderboard") }

// stopStepTimer mirrors stopTimer for the reveal step timer. Nil-safe.
func (r *Room) stopStepTimer() {
	if r.stepTimer == nil {
		return
	}
	if !r.stepTimer.Stop() {
		select {
		case <-r.stepTimer.C:
		default:
		}
	}
	r.stepTimer = nil
}

// --- Leader / pause / pencils handlers ---

// handleSetPause processes a leader's pause/resume request. No-ops when the
// request agrees with current state, rejects non-leaders. Freezes the phase
// clock + reveal step clock on pause; restarts both on resume.
func (r *Room) handleSetPause(m actorMsg) {
	if m.conn.PlayerID != r.leaderID {
		m.conn.SendError(proto.ErrNotAllowed, "only the party leader can pause", m.env.ID)
		return
	}
	var p proto.SetPausePayload
	if err := json.Unmarshal(m.env.Payload, &p); err != nil {
		m.conn.SendError(proto.ErrBadPayload, err.Error(), m.env.ID)
		return
	}
	if p.Paused == r.paused {
		// Idempotent no-op; still broadcast so clients reconcile.
		r.broadcastPauseState()
		return
	}
	if p.Paused {
		// Capture remaining time on each live timer, then stop them.
		now := time.Now()
		if !r.phaseDeadline.IsZero() {
			remaining := r.phaseDeadline.Sub(now)
			if remaining < 0 {
				remaining = 0
			}
			r.pauseRemaining = remaining
		}
		if r.stepTimer != nil && r.stepState != nil {
			// Conservative: treat the full step interval as remaining. This
			// overshoots by the slice of the current step already elapsed,
			// but (a) the overshoot is bounded by RevealStepInterval and
			// (b) it avoids a separate wall-clock we'd have to maintain.
			// If finer-grained pausing matters, add stepDeadline like
			// phaseDeadline.
			r.stepRemaining = r.currentStepInterval()
		}
		r.stopTimer()
		r.stopStepTimer()
		r.paused = true
	} else {
		// Restore timers using the captured remaining budgets. If something
		// was zero (e.g., no stepTimer was running) we don't spin one up.
		r.paused = false
		if r.pauseRemaining > 0 {
			r.phaseTimer = time.NewTimer(r.pauseRemaining)
			r.phaseDeadline = time.Now().Add(r.pauseRemaining)
			r.pauseRemaining = 0
		} else {
			r.phaseDeadline = time.Time{}
		}
		if r.stepRemaining > 0 && r.stepState != nil && r.stepState.idx < len(r.stepState.queue) {
			r.stepTimer = time.NewTimer(r.stepRemaining)
			r.stepRemaining = 0
		}
	}
	r.broadcastPauseState()
	r.broadcastState()
}

// handleSetPencilsDown toggles the draw-lock for all players. Leader only.
// Independent of pause — e.g., leader can unpause with pencils still down
// to freeze players while re-explaining the prompt.
func (r *Room) handleSetPencilsDown(m actorMsg) {
	if m.conn.PlayerID != r.leaderID {
		m.conn.SendError(proto.ErrNotAllowed, "only the party leader can toggle pencils-down", m.env.ID)
		return
	}
	var p proto.SetPencilsDownPayload
	if err := json.Unmarshal(m.env.Payload, &p); err != nil {
		m.conn.SendError(proto.ErrBadPayload, err.Error(), m.env.ID)
		return
	}
	r.pencilsDown = p.Disabled
	r.broadcastPauseState()
	r.broadcastState()
}

// handleAdvanceReveal processes the leader's "next" click during
// leaderboard_rN. While reveal steps remain, each click emits exactly one
// step for the currently-focused drawing. Once all steps have been emitted
// (or if no step queue exists), the same action advances into the next phase,
// which starts the next round or ends the game.
func (r *Room) handleAdvanceReveal(m actorMsg) {
	if m.conn.PlayerID != r.leaderID {
		m.conn.SendError(proto.ErrNotAllowed, "only the party leader can advance reveal", m.env.ID)
		return
	}
	if r.status != StatusInGame || r.current == nil {
		m.conn.SendError(proto.ErrWrongPhase, "game not in progress", m.env.ID)
		return
	}
	currentPhase := ""
	if r.state.PhaseIdx < len(r.phases) {
		currentPhase = r.phases[r.state.PhaseIdx].Name
	}
	if !isLeaderboardPhase(currentPhase) {
		m.conn.SendError(proto.ErrWrongPhase, "advance_reveal only allowed during leaderboard", m.env.ID)
		return
	}
	if r.paused {
		m.conn.SendError(proto.ErrNotAllowed, "game is paused", m.env.ID)
		return
	}
	if r.stepState != nil && r.stepState.idx < len(r.stepState.queue) {
		r.onRevealStep()
		return
	}
	decision, err := r.current.Timeout(r.phaseCtx())
	if err != nil {
		m.conn.SendError(proto.ErrServer, err.Error(), m.env.ID)
		return
	}
	r.apply(decision)
}

func (r *Room) handleUpdateSettings(m actorMsg) {
	if m.conn.PlayerID != r.leaderID {
		m.conn.SendError(proto.ErrNotAllowed, "only the party leader can change settings", m.env.ID)
		return
	}
	if r.status != StatusIdle || r.postGame {
		m.conn.SendError(proto.ErrWrongPhase, "settings can only change in the lobby", m.env.ID)
		return
	}
	var p proto.UpdateSettingsPayload
	if err := json.Unmarshal(m.env.Payload, &p); err != nil {
		m.conn.SendError(proto.ErrBadPayload, err.Error(), m.env.ID)
		return
	}
	next := normalizeSettings(engine.GameSettings{
		GameID:             r.selectedGameID,
		RoundCount:         p.RoundCount,
		GeneratedFakeCount: p.GeneratedFakeCount,
		DrawingSeconds:     p.DrawingSeconds,
		FakePromptSeconds:  p.FakePromptSeconds,
		VotingSeconds:      p.VotingSeconds,
		GameOptions:        p.GameOptions,
	})
	r.settings = next
	r.state.Settings = next
	for _, player := range r.state.Players {
		player.Ready = false
	}
	r.broadcastState()
}

func (r *Room) handleSelectGame(m actorMsg) {
	if m.conn.PlayerID != r.leaderID {
		m.conn.SendError(proto.ErrNotAllowed, "only the party leader can choose a game", m.env.ID)
		return
	}
	if r.status == StatusInGame {
		m.conn.SendError(proto.ErrWrongPhase, "cannot change games during an active round", m.env.ID)
		return
	}
	var p proto.SelectGamePayload
	if err := json.Unmarshal(m.env.Payload, &p); err != nil {
		m.conn.SendError(proto.ErrBadPayload, err.Error(), m.env.ID)
		return
	}
	game, ok := platformcatalog.FindGame(p.GameID)
	if !ok {
		m.conn.SendError(proto.ErrBadPayload, "unknown game", m.env.ID)
		return
	}
	if game.Status != platformcatalog.GameStatusAvailable {
		m.conn.SendError(proto.ErrNotAllowed, "that game is coming soon", m.env.ID)
		return
	}
	r.selectedGameID = game.ID
	r.settings = normalizeSettings(engine.GameSettings{GameID: game.ID})
	r.state.Settings = r.settings
	for _, player := range r.state.Players {
		player.Ready = false
	}
	r.broadcastType(proto.S2CGameSelected, proto.GameSelectedPayload{GameID: game.ID})
	r.broadcastState()
}

func (r *Room) handleStartGame(m actorMsg) {
	if m.conn.PlayerID != r.leaderID {
		m.conn.SendError(proto.ErrNotAllowed, "only the party leader can start the game", m.env.ID)
		return
	}
	if r.status != StatusIdle || r.postGame {
		m.conn.SendError(proto.ErrWrongPhase, "game can only start from the game lobby", m.env.ID)
		return
	}
	if r.selectedGameID == "" {
		m.conn.SendError(proto.ErrNotAllowed, "choose a game first", m.env.ID)
		return
	}
	if err := platformcatalog.CanSelectGame(r.selectedGameID, len(r.state.ActivePlayers())); err != nil {
		m.conn.SendError(proto.ErrNotAllowed, err.Error(), m.env.ID)
		return
	}
	if !r.allReadyAndEnough() {
		m.conn.SendError(proto.ErrNotAllowed, "everyone must be ready before starting", m.env.ID)
		return
	}
	r.onStartGame()
}

func (r *Room) handleReturnToPicker(m actorMsg) {
	if m.conn.PlayerID != r.leaderID {
		m.conn.SendError(proto.ErrNotAllowed, "only the party leader can return to the picker", m.env.ID)
		return
	}
	if r.status == StatusInGame {
		m.conn.SendError(proto.ErrWrongPhase, "cannot leave for the picker during an active game", m.env.ID)
		return
	}
	r.postGame = false
	r.phases = nil
	r.current = nil
	r.stopTimer()
	r.stopStepTimer()
	r.stepState = nil
	r.phaseDeadline = time.Time{}
	r.pauseRemaining = 0
	r.stepRemaining = 0
	r.state.PhaseIdx = 0
	r.state.Round = 0
	r.state.PhaseData = map[string]engine.PhaseResult{}
	r.state.Scores = map[engine.PlayerID]int{}
	r.selectedGameID = ""
	r.settings = normalizeSettings(engine.GameSettings{})
	r.state.Settings = r.settings
	r.rerolled = map[string]bool{}
	r.drawn = map[string]bool{}
	for _, player := range r.state.Players {
		player.Ready = false
	}
	r.broadcastState()
}

func (r *Room) handleRerollPrompt(m actorMsg) {
	if r.status != StatusInGame || r.current == nil {
		m.conn.SendError(proto.ErrWrongPhase, "game not in progress", m.env.ID)
		return
	}
	if r.paused {
		m.conn.SendError(proto.ErrNotAllowed, "game is paused", m.env.ID)
		return
	}
	currentPhase := ""
	if r.state.PhaseIdx < len(r.phases) {
		currentPhase = r.phases[r.state.PhaseIdx].Name
	}
	if !startsWith(currentPhase, "drawing_submit") {
		m.conn.SendError(proto.ErrWrongPhase, "reroll only allowed while drawing", m.env.ID)
		return
	}
	key := roundPlayerKey(r.state.Round, m.conn.PlayerID)
	if r.rerolled[key] {
		m.conn.SendError(proto.ErrNotAllowed, "prompt already rerolled this round", m.env.ID)
		return
	}
	if r.drawn[key] {
		m.conn.SendError(proto.ErrNotAllowed, "cannot reroll after submitting a drawing", m.env.ID)
		return
	}
	if r.defaults.RerollPrompt == nil {
		m.conn.SendError(proto.ErrServer, "prompt reroll unavailable", m.env.ID)
		return
	}
	promptPhase := fmt.Sprintf("prompt_distribute_r%d", r.state.Round)
	_, events, err := r.defaults.RerollPrompt(r.state, promptPhase, m.conn.PlayerID)
	if err != nil {
		m.conn.SendError(proto.ErrNotAllowed, err.Error(), m.env.ID)
		return
	}
	r.rerolled[key] = true
	for _, ev := range events {
		r.broadcastEvent(ev)
	}
}

func roundPlayerKey(round int, pid engine.PlayerID) string {
	return fmt.Sprintf("%d:%s", round, pid)
}

func (r *Room) broadcastPauseState() {
	deadlineMs := int64(0)
	if !r.paused && !r.phaseDeadline.IsZero() {
		deadlineMs = r.phaseDeadline.UnixMilli()
	}
	remaining := int64(0)
	if r.paused {
		remaining = r.pauseRemaining.Milliseconds()
	}
	r.broadcastType(proto.S2CPauseState, proto.PauseStatePayload{
		Paused:      r.paused,
		PencilsDown: r.pencilsDown,
		RemainingMs: remaining,
		LeaderID:    string(r.leaderID),
		DeadlineMs:  deadlineMs,
	})
}

func (r *Room) broadcastLeader() {
	r.broadcastType(proto.S2CLeaderChange, proto.LeaderChangePayload{
		LeaderID: string(r.leaderID),
	})
}

// --- Reveal step animation ---

// initRevealSteps builds the per-drawing step queue at leaderboard_rN start
// and kicks off the first timer tick. Delegates queue construction to the
// game mode via Defaults.BuildRevealSteps so room.go stays game-agnostic.
// Silent no-op if no hook is wired — in that case the client must render
// the full reveal without animation (still correct, just snappy).
func (r *Room) initRevealSteps() {
	round := r.state.Round
	if r.defaults.BuildRevealSteps == nil {
		r.log.Debug("leaderboard: no reveal step builder wired", "round", round)
		return
	}
	queue := r.defaults.BuildRevealSteps(r.state, round)
	if len(queue) == 0 {
		r.log.Warn("leaderboard: reveal step builder returned empty queue", "round", round)
		return
	}
	r.stepState = &revealStepState{round: round, queue: queue, idx: 0}
}

// onRevealStep fires each time stepTimer elapses. Walks the queue one
// event at a time. Broadcasts + re-arms the timer until the queue is
// exhausted. Pause-aware: if paused races with a timer fire, we simply
// re-stop on the next broadcastPauseState.
func (r *Room) onRevealStep() {
	r.stepTimer = nil
	if r.stepState == nil {
		return
	}
	st := r.stepState
	if st.idx >= len(st.queue) {
		return
	}
	step := st.queue[st.idx]
	st.idx++
	switch step.Kind {
	case "eliminate":
		r.broadcastType(proto.S2CRevealStep, proto.RevealStepPayload{
			Round:              st.round,
			DrawingID:          step.DrawingID,
			Step:               st.idx,
			EliminatedChoiceID: step.ChoiceID,
		})
	case "final":
		r.broadcastType(proto.S2CRevealStep, proto.RevealStepPayload{
			Round:     st.round,
			DrawingID: step.DrawingID,
			Step:      st.idx,
			IsFinal:   true,
			Deltas:    step.Deltas,
			Awards:    step.Awards,
		})
	case "gap":
		// Pure pacing — nothing to broadcast. Fall through to rearm.
	}
}

// currentStepInterval returns the next-step delay to capture when pausing.
// Mirrors the duration we would have armed for the current step. The actor
// doesn't track a stepDeadline, so we approximate with a full interval;
// see handleSetPause for rationale.
func (r *Room) currentStepInterval() time.Duration {
	if r.stepState == nil || r.stepState.idx >= len(r.stepState.queue) {
		return RevealStepInterval
	}
	// Peek the NEXT step, since by the time we're paused the last-fired
	// step's timer has already been consumed and the state.idx points at
	// the upcoming one.
	if d := r.stepState.queue[r.stepState.idx].Duration; d > 0 {
		return d
	}
	return RevealStepInterval
}

// startsWith is a local helper to avoid importing "strings" just for this.
func startsWith(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}

// roundFromPhaseName parses "_rN" suffix; returns 0 on miss.
func roundFromPhaseName(name string) int {
	i := len(name) - 1
	if i < 2 || name[i-1] != '_' && !(name[i-1] >= '0' && name[i-1] <= '9') {
		// fall through to scan
	}
	// Scan backward for digits.
	end := len(name)
	start := end
	for start > 0 && name[start-1] >= '0' && name[start-1] <= '9' {
		start--
	}
	if start == end {
		return 0
	}
	if start < 2 || name[start-1] != 'r' || name[start-2] != '_' {
		return 0
	}
	n := 0
	for _, c := range name[start:] {
		n = n*10 + int(c-'0')
	}
	return n
}
