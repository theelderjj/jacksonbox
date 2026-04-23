package proto

// Message type constants — single source of truth for gateway whitelist,
// per-phase guards, and client reference.
const (
	// Client -> Server
	C2SJoinRoom              = "join_room"
	C2SLeaveRoom             = "leave_room"
	C2SReady                 = "ready"
	C2SSubmitDraw            = "submit_drawing"
	C2SSubmitFake            = "submit_fake_prompt"
	C2SSubmitVote            = "submit_vote"
	C2SSubmitTap             = "submit_tap"
	C2SSubmitPriceGuess      = "submit_price_guess"
	C2SSubmitSplitSetup      = "submit_split_setup"
	C2SSubmitSplitChoice     = "submit_split_choice"
	C2SSubmitFakeArtistGuess = "submit_fake_artist_guess"
	C2SSubmitWordList        = "submit_word_list"
	C2SPing                  = "ping"
	C2SSetPause              = "set_pause"        // leader only: pause/resume timers
	C2SSetPencilsDown        = "set_pencils_down" // leader only: lock/unlock drawing
	C2SAdvanceReveal         = "advance_reveal"   // leader only: step leaderboard reveal / start next round
	C2SUpdateSettings        = "update_settings"  // leader only: lobby game settings
	C2SRerollPrompt          = "reroll_prompt"    // drawing phase: one reroll per player per round
	C2SSelectGame            = "select_game"      // leader only: choose game from picker
	C2SStartGame             = "start_game"       // leader only: explicitly start once everyone is ready
	C2SReturnToPicker        = "return_to_picker" // leader only: leave results/lobby back to game picker

	// Server -> Client
	S2CJoinAck          = "join_ack"
	S2CStateUpdate      = "state_update"
	S2CPhaseChange      = "phase_change"
	S2CRoundResult      = "round_result"
	S2CGameEnd          = "game_end"
	S2CError            = "error"
	S2CRoomEvicting     = "room_evicting"
	S2CPong             = "pong"
	S2CPromptIssued     = "prompt_issued"
	S2CSubmitTick       = "submit_tick" // "X of Y submitted"
	S2CReveal           = "reveal"
	S2CRevealStep       = "reveal_step"    // one elimination or final-truth tick during reveal
	S2CDrawings         = "drawings"       // drawings visible at fake-phase start
	S2CVotingChoices    = "voting_choices" // merged choices visible at voting start
	S2CPauseState       = "pause_state"    // paused / pencils-down / remaining-ms / leader
	S2CLeaderChange     = "leader_change"  // leader promotion (also mirrored in state_update)
	S2CGameCatalog      = "game_catalog"
	S2CGameSelected     = "game_selected"
	S2CReactionResult   = "reaction_result"
	S2CPricePrompt      = "price_prompt"
	S2CPriceResult      = "price_result"
	S2CSplitVotePrompt  = "split_vote_prompt"
	S2CSplitReveal      = "split_reveal"
	S2CWordPrompt       = "word_prompt"
	S2CWordEntry        = "word_entry"
	S2CWordResult       = "word_result"
	S2CFakeArtistTurn   = "fake_artist_turn"
	S2CFakeArtistCanvas = "fake_artist_canvas"
	S2CFakeArtistReplay = "fake_artist_replay"
	S2CFakeArtistReveal = "fake_artist_reveal"
	S2CDrawDuelRound    = "draw_duel_round"
	S2CDrawDuelReveal   = "draw_duel_reveal"
	S2CMafiaState       = "mafia_state"
	S2CMafiaReveal      = "mafia_reveal"
)

// KnownC2S is the gateway whitelist. Anything else -> error "unknown_type".
var KnownC2S = map[string]struct{}{
	C2SJoinRoom:              {},
	C2SLeaveRoom:             {},
	C2SReady:                 {},
	C2SSubmitDraw:            {},
	C2SSubmitFake:            {},
	C2SSubmitVote:            {},
	C2SSubmitTap:             {},
	C2SSubmitPriceGuess:      {},
	C2SSubmitSplitSetup:      {},
	C2SSubmitSplitChoice:     {},
	C2SSubmitFakeArtistGuess: {},
	C2SSubmitWordList:        {},
	C2SPing:                  {},
	C2SSetPause:              {},
	C2SSetPencilsDown:        {},
	C2SAdvanceReveal:         {},
	C2SUpdateSettings:        {},
	C2SRerollPrompt:          {},
	C2SSelectGame:            {},
	C2SStartGame:             {},
	C2SReturnToPicker:        {},
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

type SubmitTapPayload struct{}

type SubmitPriceGuessPayload struct {
	GuessCents int `json:"guess_cents"`
}

type SubmitSplitSetupPayload struct {
	Prompt  string `json:"prompt"`
	OptionA string `json:"option_a"`
	OptionB string `json:"option_b"`
}

type SubmitSplitChoicePayload struct {
	ChoiceID string `json:"choice_id"`
}

type SubmitFakeArtistGuessPayload struct {
	Prompt string `json:"prompt"`
}

type SubmitWordListPayload struct {
	Words []string `json:"words"`
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
	GameID             string         `json:"game_id,omitempty"`
	RoundCount         int            `json:"round_count"`
	GeneratedFakeCount int            `json:"generated_fake_count"`
	DrawingSeconds     int            `json:"drawing_seconds"`
	FakePromptSeconds  int            `json:"fake_prompt_seconds"`
	VotingSeconds      int            `json:"voting_seconds"`
	GameOptions        map[string]any `json:"game_options,omitempty"`
}

type RerollPromptPayload struct{}

type SelectGamePayload struct {
	GameID string `json:"game_id"`
}

type StartGamePayload struct{}

type ReturnToPickerPayload struct{}

// --- S->C payloads ---

type JoinAckPayload struct {
	PlayerID     string    `json:"player_id"`
	SessionToken string    `json:"session_token"`
	RoomState    RoomState `json:"room_state"`
}

type RoomState struct {
	RoomID         string           `json:"room_id"`
	Status         string           `json:"status"`
	RoomMode       string           `json:"room_mode,omitempty"`
	Phase          string           `json:"phase,omitempty"`
	Round          int              `json:"round"`
	Players        []PlayerInfo     `json:"players"`
	Scores         map[string]int   `json:"scores"`
	DeadlineMs     int64            `json:"deadline_ms,omitempty"`
	LeaderID       string           `json:"leader_id,omitempty"`
	Paused         bool             `json:"paused,omitempty"`
	PencilsDown    bool             `json:"pencils_down,omitempty"`
	RemainingMs    int64            `json:"remaining_ms,omitempty"` // only meaningful while paused
	Settings       SettingsState    `json:"settings"`
	SelectedGameID string           `json:"selected_game_id,omitempty"`
	GameCatalog    []GameDefinition `json:"game_catalog,omitempty"`
}

type SettingsState struct {
	GameID             string         `json:"game_id,omitempty"`
	RoundCount         int            `json:"round_count"`
	GeneratedFakeCount int            `json:"generated_fake_count"`
	DrawingSeconds     int            `json:"drawing_seconds"`
	FakePromptSeconds  int            `json:"fake_prompt_seconds"`
	VotingSeconds      int            `json:"voting_seconds"`
	GameOptions        map[string]any `json:"game_options,omitempty"`
}

type GameDefinition struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Summary          string   `json:"summary"`
	MinPlayers       int      `json:"min_players"`
	MaxPlayers       int      `json:"max_players"`
	EstimatedMinutes int      `json:"estimated_minutes"`
	Tags             []string `json:"tags"`
	Status           string   `json:"status"`
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

type GameCatalogPayload struct {
	Games []GameDefinition `json:"games"`
}

type GameSelectedPayload struct {
	GameID string `json:"game_id"`
}

type ReactionPlayerResult struct {
	PlayerID   string `json:"player_id"`
	PlayerName string `json:"player_name"`
	FalseStart bool   `json:"false_start"`
	ReactionMs int    `json:"reaction_ms,omitempty"`
	Rank       int    `json:"rank,omitempty"`
	Points     int    `json:"points"`
}

type ReactionResultPayload struct {
	Round   int                    `json:"round"`
	Results []ReactionPlayerResult `json:"results"`
}

type PricePromptPayload struct {
	Round              int    `json:"round"`
	ProductID          string `json:"product_id"`
	ProductName        string `json:"product_name"`
	ImageURL           string `json:"image_url"`
	ThresholdCents     int    `json:"threshold_cents"`
	ThresholdMode      string `json:"threshold_mode"`
	ThresholdBaseCents int    `json:"threshold_base_cents"`
}

type PriceResultPayload struct {
	Round            int            `json:"round"`
	ProductID        string         `json:"product_id"`
	ProductName      string         `json:"product_name"`
	ImageURL         string         `json:"image_url"`
	ActualPriceCents int            `json:"actual_price_cents"`
	ThresholdCents   int            `json:"threshold_cents"`
	ThresholdMode    string         `json:"threshold_mode"`
	Guesses          map[string]int `json:"guesses"`
	WinnerIDs        []string       `json:"winner_ids"`
}

type SplitVotePromptPayload struct {
	Round        int    `json:"round"`
	SplitterID   string `json:"splitter_id"`
	SplitterName string `json:"splitter_name"`
	Prompt       string `json:"prompt"`
	OptionA      string `json:"option_a"`
	OptionB      string `json:"option_b"`
	ShowTarget   bool   `json:"show_target"`
}

type SplitRevealPayload struct {
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

type WordLetterWindow struct {
	Letter string `json:"letter"`
	Index  int    `json:"index"`
}

type WordPromptPayload struct {
	Letters             []WordLetterWindow `json:"letters"`
	LetterSeconds       int                `json:"letter_seconds"`
	BasePointsPerLetter int                `json:"base_points_per_letter"`
	GrowthPercent       int                `json:"growth_percent"`
}

type WordEntryPayload struct {
	PlayerID   string `json:"player_id"`
	PlayerName string `json:"player_name"`
	Word       string `json:"word"`
	Letter     string `json:"letter"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	Points     int    `json:"points,omitempty"`
}

type WordPlayerResult struct {
	PlayerID    string             `json:"player_id"`
	PlayerName  string             `json:"player_name"`
	Entries     []WordEntryPayload `json:"entries"`
	Score       int                `json:"score"`
	MadeUpCount int                `json:"made_up_count"`
}

type WordResultPayload struct {
	Letters             []WordLetterWindow `json:"letters"`
	LetterSeconds       int                `json:"letter_seconds"`
	BasePointsPerLetter int                `json:"base_points_per_letter"`
	GrowthPercent       int                `json:"growth_percent"`
	Results             []WordPlayerResult `json:"results"`
	AcceptedWords       []string           `json:"accepted_words"`
}

type FakeArtistTurnPayload struct {
	Round            int    `json:"round"`
	Turn             int    `json:"turn"`
	DrawerID         string `json:"drawer_id"`
	DrawerName       string `json:"drawer_name"`
	Color            string `json:"color"`
	Phase            string `json:"phase"`
	CountdownSeconds int    `json:"countdown_seconds,omitempty"`
	Format           string `json:"format,omitempty"`
	Data             string `json:"data,omitempty"`
}

type FakeArtistCanvasPayload struct {
	Round      int    `json:"round"`
	Turn       int    `json:"turn"`
	DrawerID   string `json:"drawer_id"`
	DrawerName string `json:"drawer_name"`
	Color      string `json:"color"`
	Format     string `json:"format"`
	Data       string `json:"data"`
}

type FakeArtistReplaySegment struct {
	Turn       int    `json:"turn"`
	DrawerID   string `json:"drawer_id"`
	DrawerName string `json:"drawer_name"`
	Color      string `json:"color"`
	Format     string `json:"format"`
	Data       string `json:"data"`
}

type FakeArtistReplayPayload struct {
	Round            int                       `json:"round"`
	ReplayCount      int                       `json:"replay_count"`
	ContinuousReplay bool                      `json:"continuous_replay"`
	Segments         []FakeArtistReplaySegment `json:"segments"`
}

type FakeArtistRevealPayload struct {
	Round             int               `json:"round"`
	FakeID            string            `json:"fake_id"`
	FakeName          string            `json:"fake_name"`
	Prompt            string            `json:"prompt"`
	AccusedID         string            `json:"accused_id,omitempty"`
	AccusedName       string            `json:"accused_name,omitempty"`
	MajorityCaught    bool              `json:"majority_caught"`
	FakeGuess         string            `json:"fake_guess,omitempty"`
	FakeGuessedPrompt bool              `json:"fake_guessed_prompt"`
	FakeWins          bool              `json:"fake_wins"`
	Votes             map[string]string `json:"votes"`
	Winners           []string          `json:"winners"`
}

type DrawDuelRoundPayload struct {
	Round       int      `json:"round"`
	Prompt      string   `json:"prompt"`
	ArtistAID   string   `json:"artist_a_id"`
	ArtistAName string   `json:"artist_a_name"`
	ArtistBID   string   `json:"artist_b_id"`
	ArtistBName string   `json:"artist_b_name"`
	JudgeIDs    []string `json:"judge_ids"`
}

type DrawDuelRevealPayload struct {
	Round            int               `json:"round"`
	Prompt           string            `json:"prompt"`
	ArtistAID        string            `json:"artist_a_id"`
	ArtistAName      string            `json:"artist_a_name"`
	ArtistBID        string            `json:"artist_b_id"`
	ArtistBName      string            `json:"artist_b_name"`
	Drawings         []DrawingSummary  `json:"drawings"`
	VotesByJudge     map[string]string `json:"votes_by_judge"`
	VoteCountByDraw  map[string]int    `json:"vote_count_by_drawing"`
	WinnerDrawingID  string            `json:"winner_drawing_id,omitempty"`
	WinnerArtistID   string            `json:"winner_artist_id,omitempty"`
	WinnerArtistName string            `json:"winner_artist_name,omitempty"`
	Tied             bool              `json:"tied"`
}

type MafiaPlayerState struct {
	PlayerID string `json:"player_id"`
	Name     string `json:"name"`
	Alive    bool   `json:"alive"`
}

type MafiaStatePayload struct {
	PlayerID     string             `json:"player_id"`
	Round        int                `json:"round"`
	Phase        string             `json:"phase"`
	YourRole     string             `json:"your_role"`
	TeamIDs      []string           `json:"team_ids"`
	AlivePlayers []MafiaPlayerState `json:"alive_players"`
	CanAct       bool               `json:"can_act"`
	TargetIDs    []string           `json:"target_ids"`
	VoteMode     string             `json:"vote_mode,omitempty"`
	PublicVotes  map[string]string  `json:"public_votes,omitempty"`
	CurrentVoter string             `json:"current_voter_id,omitempty"`
	Note         string             `json:"note,omitempty"`
	LockedIn     bool               `json:"locked_in,omitempty"`
}

type MafiaRevealPayload struct {
	Round          int               `json:"round"`
	Phase          string            `json:"phase"`
	Deaths         []string          `json:"deaths,omitempty"`
	EliminatedID   string            `json:"eliminated_id,omitempty"`
	EliminatedName string            `json:"eliminated_name,omitempty"`
	EliminatedRole string            `json:"eliminated_role,omitempty"`
	VoteMode       string            `json:"vote_mode,omitempty"`
	Votes          map[string]string `json:"votes,omitempty"`
	AliveIDs       []string          `json:"alive_ids"`
	Winner         string            `json:"winner,omitempty"`
	RoleMap        map[string]string `json:"role_map,omitempty"`
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
