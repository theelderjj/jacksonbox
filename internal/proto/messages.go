package proto

// Message type constants — single source of truth for gateway whitelist,
// per-phase guards, and client reference.
const (
	// Client -> Server
	C2SJoinRoom       = "join_room"
	C2SLeaveRoom      = "leave_room"
	C2SReady          = "ready"
	C2SSubmitDraw     = "submit_drawing"
	C2SSubmitFake     = "submit_fake_prompt"
	C2SSubmitVote     = "submit_vote"
	C2SPing           = "ping"
	C2SSetPause       = "set_pause"        // leader only: pause/resume timers
	C2SSetPencilsDown = "set_pencils_down" // leader only: lock/unlock drawing
	C2SAdvanceReveal  = "advance_reveal"   // leader only: step leaderboard reveal / start next round
	C2SUpdateSettings = "update_settings"  // leader only: lobby game settings
	C2SRerollPrompt   = "reroll_prompt"    // drawing phase: one reroll per player per round

	// Server -> Client
	S2CJoinAck       = "join_ack"
	S2CStateUpdate   = "state_update"
	S2CPhaseChange   = "phase_change"
	S2CRoundResult   = "round_result"
	S2CGameEnd       = "game_end"
	S2CError         = "error"
	S2CRoomEvicting  = "room_evicting"
	S2CPong          = "pong"
	S2CPromptIssued  = "prompt_issued"
	S2CSubmitTick    = "submit_tick" // "X of Y submitted"
	S2CReveal        = "reveal"
	S2CRevealStep    = "reveal_step"    // one elimination or final-truth tick during reveal
	S2CDrawings      = "drawings"       // drawings visible at fake-phase start
	S2CVotingChoices = "voting_choices" // merged choices visible at voting start
	S2CPauseState    = "pause_state"    // paused / pencils-down / remaining-ms / leader
	S2CLeaderChange  = "leader_change"  // leader promotion (also mirrored in state_update)
)

// KnownC2S is the gateway whitelist. Anything else -> error "unknown_type".
var KnownC2S = map[string]struct{}{
	C2SJoinRoom:       {},
	C2SLeaveRoom:      {},
	C2SReady:          {},
	C2SSubmitDraw:     {},
	C2SSubmitFake:     {},
	C2SSubmitVote:     {},
	C2SPing:           {},
	C2SSetPause:       {},
	C2SSetPencilsDown: {},
	C2SAdvanceReveal:  {},
	C2SUpdateSettings: {},
	C2SRerollPrompt:   {},
}

// --- C->S payloads ---

type JoinRoomPayload struct {
	RoomID       string `json:"room_id"`
	Name         string `json:"name"`
	SessionToken string `json:"session_token,omitempty"`
}

type ReadyPayload struct {
	Ready bool `json:"ready"`
}

type SubmitDrawingPayload struct {
	Data   string `json:"data"`   // vector JSON or base64 PNG
	Format string `json:"format"` // "strokes" | "png"
}

type SubmitFakePromptPayload struct {
	DrawingID string `json:"drawing_id"`
	Text      string `json:"text"`
}

type SubmitVotePayload struct {
	DrawingID string `json:"drawing_id"`
	ChoiceID  string `json:"choice_id"` // FakePromptID or "TRUE"
}

// SetPausePayload is sent by the party leader to freeze or resume the current
// phase timer. Pausing also freezes the reveal step timer. Pause does NOT
// imply pencils-down — that's a separate toggle so the leader can, e.g.,
// allow more drawing time while the clock is stopped.
type SetPausePayload struct {
	Paused bool `json:"paused"`
}

// SetPencilsDownPayload is sent by the party leader to lock or unlock the
// drawing canvas across all players. Enforced both client-side (canvas short-
// circuits pointer events) and server-side (submit_drawing rejected when true).
type SetPencilsDownPayload struct {
	Disabled bool `json:"disabled"`
}

// AdvanceRevealPayload is intentionally empty. The leader uses it during
// leaderboard_rN to advance one reveal step at a time; once the queue is
// exhausted, the same action advances into the next round.
type AdvanceRevealPayload struct{}

type UpdateSettingsPayload struct {
	RoundCount         int `json:"round_count"`
	GeneratedFakeCount int `json:"generated_fake_count"`
	DrawingSeconds     int `json:"drawing_seconds"`
	FakePromptSeconds  int `json:"fake_prompt_seconds"`
	VotingSeconds      int `json:"voting_seconds"`
}

type RerollPromptPayload struct{}

// --- S->C payloads ---

type JoinAckPayload struct {
	PlayerID     string    `json:"player_id"`
	SessionToken string    `json:"session_token"`
	RoomState    RoomState `json:"room_state"`
}

type RoomState struct {
	RoomID      string         `json:"room_id"`
	Status      string         `json:"status"`
	Phase       string         `json:"phase,omitempty"`
	Round       int            `json:"round"`
	Players     []PlayerInfo   `json:"players"`
	Scores      map[string]int `json:"scores"`
	DeadlineMs  int64          `json:"deadline_ms,omitempty"`
	LeaderID    string         `json:"leader_id,omitempty"`
	Paused      bool           `json:"paused,omitempty"`
	PencilsDown bool           `json:"pencils_down,omitempty"`
	RemainingMs int64          `json:"remaining_ms,omitempty"` // only meaningful while paused
	Settings    SettingsState  `json:"settings"`
}

type SettingsState struct {
	RoundCount         int `json:"round_count"`
	GeneratedFakeCount int `json:"generated_fake_count"`
	DrawingSeconds     int `json:"drawing_seconds"`
	FakePromptSeconds  int `json:"fake_prompt_seconds"`
	VotingSeconds      int `json:"voting_seconds"`
}

type PlayerInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Connected bool   `json:"connected"`
	Ready     bool   `json:"ready"`
}

type PhaseChangePayload struct {
	Phase      string         `json:"phase"`
	Round      int            `json:"round"`
	DeadlineMs int64          `json:"deadline_ms"`
	Extra      map[string]any `json:"extra,omitempty"`
}

type ErrorPayload struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	ForMsgID string `json:"for_msg_id,omitempty"`
}

type RoomEvictingPayload struct {
	Reason  string `json:"reason"`
	GraceMs int64  `json:"grace_ms"`
}

type PromptIssuedPayload struct {
	PlayerID string `json:"player_id"`
	Prompt   string `json:"prompt"` // player's own prompt only
}

type SubmitTickPayload struct {
	Phase     string `json:"phase"`
	Submitted int    `json:"submitted"`
	Total     int    `json:"total"`
}

// DrawingSummary is a client-facing view of one drawing for the fake-prompt
// and voting screens. `Data` carries the same strokes/png payload the author
// submitted; the server performs no re-rendering.
type DrawingSummary struct {
	DrawingID string `json:"drawing_id"`
	AuthorID  string `json:"author_id"`
	Format    string `json:"format"`
	Data      string `json:"data"`
}

type DrawingsPayload struct {
	Round    int              `json:"round"`
	Drawings []DrawingSummary `json:"drawings"`
}

// VotingChoice is a shrunken RevealChoice used before reveal: it intentionally
// omits voter lists (which are not yet populated) and AuthorIDs (which would
// leak fake authorship before scoring).
type VotingChoice struct {
	ChoiceID string `json:"choice_id"`
	Text     string `json:"text"`
	IsTrue   bool   `json:"is_true"`
}

type VotingChoicesEntry struct {
	DrawingID string         `json:"drawing_id"`
	Choices   []VotingChoice `json:"choices"`
}

type VotingChoicesPayload struct {
	Round   int                  `json:"round"`
	Entries []VotingChoicesEntry `json:"entries"`
}

type RevealPayload struct {
	DrawingID  string         `json:"drawing_id"`
	AuthorID   string         `json:"author_id"`
	TruePrompt string         `json:"true_prompt"`
	Choices    []RevealChoice `json:"choices"`
}

type RevealChoice struct {
	ChoiceID  string   `json:"choice_id"`
	Text      string   `json:"text"`
	AuthorIDs []string `json:"author_ids,omitempty"` // multiple on merge
	Voters    []string `json:"voters"`
	IsTrue    bool     `json:"is_true"`
}

type RoundResultPayload struct {
	Round  int            `json:"round"`
	Deltas map[string]int `json:"deltas"`
	Scores map[string]int `json:"scores"`
}

// RevealStepPayload drives the per-drawing elimination animation. Steps are
// emitted server-side on a fixed cadence; pausing freezes the cadence.
// Non-final steps carry EliminatedChoiceID (a fake to grey out). The final
// step of a drawing carries IsFinal=true along with the per-drawing point
// deltas (duplicated from scoring, but scoped to this drawing only, so the UI
// can show "Alice fooled Bob → +500" inline).
type RevealStepPayload struct {
	Round              int            `json:"round"`
	DrawingID          string         `json:"drawing_id"`
	Step               int            `json:"step"`                           // 1-indexed
	EliminatedChoiceID string         `json:"eliminated_choice_id,omitempty"` // empty on final step
	IsFinal            bool           `json:"is_final,omitempty"`
	Deltas             map[string]int `json:"deltas,omitempty"` // player_id → points from this drawing
	Awards             []RevealAward  `json:"awards,omitempty"` // human-readable breakdown
}

// RevealAward is one line in the per-drawing scoring breakdown. Reason is a
// stable tag ("drawer_truth", "faker_fool", "guesser_truth") so the client
// can localize if it ever wants to.
type RevealAward struct {
	PlayerID string `json:"player_id"`
	Points   int    `json:"points"`
	Reason   string `json:"reason"`
}

// PauseStatePayload carries the current pause/pencils state. Sent on any
// change (including leader promotion that inherits state) and on initial
// join_ack via RoomState.
type PauseStatePayload struct {
	Paused      bool   `json:"paused"`
	PencilsDown bool   `json:"pencils_down"`
	RemainingMs int64  `json:"remaining_ms,omitempty"` // only meaningful while paused
	LeaderID    string `json:"leader_id"`
	DeadlineMs  int64  `json:"deadline_ms,omitempty"` // fresh deadline after unpause
}

// LeaderChangePayload announces a new party leader. Usually fires when the
// current leader disconnects and the next-joined connected player is
// promoted. The client uses this to show "<name> is now the host" and to
// re-render the leader-only pause controls.
type LeaderChangePayload struct {
	LeaderID string `json:"leader_id"`
}

// Error codes — single source of truth. See §9.
const (
	ErrUnknownType = "unknown_type"
	ErrWrongPhase  = "wrong_phase"
	ErrBadPayload  = "bad_payload"
	ErrIneligible  = "ineligible"
	ErrDuplicate   = "duplicate_msg"
	ErrRoomFull    = "room_full"
	ErrRoomClosed  = "room_closed"
	ErrNotAllowed  = "not_allowed"
	ErrServer      = "server_error"
	ErrRoomCrashed = "room_crashed"
)

// WS close codes.
const (
	CloseFrameTooBig = 1009
	CloseGoingAway   = 1001
	CloseAbnormal    = 1011
	CloseRestart     = 1012
	CloseDemoEvicted = 4000
	CloseVersionBump = 4001
)
