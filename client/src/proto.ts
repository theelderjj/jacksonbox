// Mirror of internal/proto/messages.go. Keep this in lock-step with the
// backend — the single-deployer policy (see DESIGN.md §9) makes schema
// drift visible at the first reconnect.

export const PROTOCOL_VERSION = 1;
export const MAX_FRAME_BYTES = 256 * 1024;

export const C2S = {
  JoinRoom: "join_room",
  LeaveRoom: "leave_room",
  Ready: "ready",
  SubmitDrawing: "submit_drawing",
  SubmitFakePrompt: "submit_fake_prompt",
  SubmitVote: "submit_vote",
  SubmitTap: "submit_tap",
  SubmitPriceGuess: "submit_price_guess",
  SubmitSplitSetup: "submit_split_setup",
  SubmitSplitChoice: "submit_split_choice",
  SubmitFakeArtistGuess: "submit_fake_artist_guess",
  SubmitWordList: "submit_word_list",
  Ping: "ping",
  SetPause: "set_pause",
  SetPencilsDown: "set_pencils_down",
  AdvanceReveal: "advance_reveal",
  UpdateSettings: "update_settings",
  RerollPrompt: "reroll_prompt",
  SelectGame: "select_game",
  StartGame: "start_game",
  ReturnToPicker: "return_to_picker",
} as const;

export const S2C = {
  JoinAck: "join_ack",
  StateUpdate: "state_update",
  PhaseChange: "phase_change",
  RoundResult: "round_result",
  GameEnd: "game_end",
  Error: "error",
  RoomEvicting: "room_evicting",
  Pong: "pong",
  PromptIssued: "prompt_issued",
  SubmitTick: "submit_tick",
  Reveal: "reveal",
  RevealStep: "reveal_step",
  Drawings: "drawings",
  VotingChoices: "voting_choices",
  PauseState: "pause_state",
  LeaderChange: "leader_change",
  GameCatalog: "game_catalog",
  GameSelected: "game_selected",
  ReactionResult: "reaction_result",
  PricePrompt: "price_prompt",
  PriceResult: "price_result",
  SplitVotePrompt: "split_vote_prompt",
  SplitReveal: "split_reveal",
  WordPrompt: "word_prompt",
  WordEntry: "word_entry",
  WordResult: "word_result",
  FakeArtistTurn: "fake_artist_turn",
  FakeArtistCanvas: "fake_artist_canvas",
  FakeArtistReplay: "fake_artist_replay",
  FakeArtistReveal: "fake_artist_reveal",
  DrawDuelRound: "draw_duel_round",
  DrawDuelReveal: "draw_duel_reveal",
  MafiaState: "mafia_state",
  MafiaReveal: "mafia_reveal",
} as const;

export type Envelope<P = unknown> = {
  v: number;
  id: string;
  type: string;
  ts: number;
  payload: P;
};

export type PlayerInfo = {
  id: string;
  name: string;
  connected: boolean;
  ready: boolean;
};

export type RoomState = {
  room_id: string;
  status: string;
  room_mode?: string;
  phase?: string;
  round: number;
  players: PlayerInfo[];
  scores: Record<string, number>;
  deadline_ms?: number;
  leader_id?: string;
  paused?: boolean;
  pencils_down?: boolean;
  remaining_ms?: number;
  settings: SettingsState;
  selected_game_id?: string;
  game_catalog?: GameDefinition[];
};

export type SettingsState = {
  game_id?: string;
  round_count: number;
  generated_fake_count: number;
  drawing_seconds: number;
  fake_prompt_seconds: number;
  voting_seconds: number;
  game_options?: Record<string, unknown>;
};

export type GameDefinition = {
  id: string;
  name: string;
  summary: string;
  min_players: number;
  max_players: number;
  estimated_minutes: number;
  tags: string[];
  status: string;
};

export type JoinAckPayload = {
  player_id: string;
  session_token: string;
  room_state: RoomState;
};

export type PhaseChangePayload = {
  phase: string;
  round: number;
  deadline_ms: number;
  extra?: Record<string, unknown>;
};

export type PromptIssuedPayload = {
  player_id: string;
  prompt: string;
};

export type RevealChoice = {
  choice_id: string;
  text: string;
  author_ids?: string[];
  voters: string[];
  is_true: boolean;
};

export type RevealPayload = {
  drawing_id: string;
  author_id: string;
  true_prompt: string;
  choices: RevealChoice[];
};

export type RoundResultPayload = {
  round: number;
  deltas: Record<string, number>;
  scores: Record<string, number>;
};

export type ErrorPayload = {
  code: string;
  message: string;
  for_msg_id?: string;
};

export type RoomEvictingPayload = {
  reason: string;
  grace_ms: number;
};

export type SubmitDrawingPayload = {
  data: string;
  format: "strokes" | "png";
};

export type SubmitFakePromptPayload = {
  drawing_id: string;
  text: string;
};

export type SubmitVotePayload = {
  drawing_id: string;
  choice_id: string;
};

export type SubmitTapPayload = Record<string, never>;
export type SubmitPriceGuessPayload = {
  guess_cents: number;
};
export type SubmitSplitSetupPayload = {
  prompt: string;
  option_a: string;
  option_b: string;
};
export type SubmitSplitChoicePayload = {
  choice_id: "A" | "B";
};
export type SubmitFakeArtistGuessPayload = {
  prompt: string;
};
export type SubmitWordListPayload = {
  words: string[];
};

export type SetPausePayload = {
  paused: boolean;
};

export type SetPencilsDownPayload = {
  disabled: boolean;
};

export type AdvanceRevealPayload = Record<string, never>;

export type UpdateSettingsPayload = {
  game_id?: string;
  round_count: number;
  generated_fake_count: number;
  drawing_seconds: number;
  fake_prompt_seconds: number;
  voting_seconds: number;
  game_options?: Record<string, unknown>;
};

export type RerollPromptPayload = Record<string, never>;
export type SelectGamePayload = { game_id: string };
export type StartGamePayload = Record<string, never>;
export type ReturnToPickerPayload = Record<string, never>;

export type RevealAward = {
  player_id: string;
  points: number;
  reason: string;
};

export type RevealStepPayload = {
  round: number;
  drawing_id: string;
  step: number;
  eliminated_choice_id?: string;
  is_final?: boolean;
  deltas?: Record<string, number>;
  awards?: RevealAward[];
};

export type PauseStatePayload = {
  paused: boolean;
  pencils_down: boolean;
  remaining_ms?: number;
  leader_id: string;
  deadline_ms?: number;
};

export type LeaderChangePayload = {
  leader_id: string;
};

export type GameCatalogPayload = {
  games: GameDefinition[];
};

export type GameSelectedPayload = {
  game_id: string;
};

export type ReactionPlayerResult = {
  player_id: string;
  player_name: string;
  false_start: boolean;
  reaction_ms?: number;
  rank?: number;
  points: number;
};

export type ReactionResultPayload = {
  round: number;
  results: ReactionPlayerResult[];
};

export type PricePromptPayload = {
  round: number;
  product_id: string;
  product_name: string;
  image_url: string;
  threshold_cents: number;
  threshold_mode: string;
  threshold_base_cents: number;
};

export type PriceResultPayload = {
  round: number;
  product_id: string;
  product_name: string;
  image_url: string;
  actual_price_cents: number;
  threshold_cents: number;
  threshold_mode: string;
  guesses: Record<string, number>;
  winner_ids: string[];
};

export type SplitVotePromptPayload = {
  round: number;
  splitter_id: string;
  splitter_name: string;
  prompt: string;
  option_a: string;
  option_b: string;
  show_target: boolean;
};

export type SplitRevealPayload = {
  round: number;
  splitter_id: string;
  splitter_name: string;
  prompt: string;
  option_a: string;
  option_b: string;
  count_a: number;
  count_b: number;
  target_a: number;
  target_b: number;
  target_mode: string;
  show_target: boolean;
  achieved: boolean;
  player_choices: Record<string, string>;
};

export type WordPromptPayload = {
  letters: WordLetterWindow[];
  letter_seconds: number;
  base_points_per_letter: number;
  growth_percent: number;
};

export type WordLetterWindow = {
  letter: string;
  index: number;
};

export type WordEntryPayload = {
  player_id: string;
  player_name: string;
  word: string;
  letter: string;
  status: "pending" | "valid" | "invalid" | "duplicate";
  message: string;
  points?: number;
};

export type WordPlayerResult = {
  player_id: string;
  player_name: string;
  entries: WordEntryPayload[];
  score: number;
  made_up_count: number;
};

export type WordResultPayload = {
  letters: WordLetterWindow[];
  letter_seconds: number;
  base_points_per_letter: number;
  growth_percent: number;
  results: WordPlayerResult[];
  accepted_words: string[];
};

export type FakeArtistTurnPayload = {
  round: number;
  turn: number;
  drawer_id: string;
  drawer_name: string;
  color: string;
  phase: "countdown" | "draw";
  countdown_seconds?: number;
  format?: "strokes" | "png";
  data?: string;
};

export type FakeArtistCanvasPayload = {
  round: number;
  turn: number;
  drawer_id: string;
  drawer_name: string;
  color: string;
  format: "strokes" | "png";
  data: string;
};

export type FakeArtistReplaySegment = {
  turn: number;
  drawer_id: string;
  drawer_name: string;
  color: string;
  format: "strokes" | "png";
  data: string;
};

export type FakeArtistReplayPayload = {
  round: number;
  replay_count: number;
  continuous_replay: boolean;
  segments: FakeArtistReplaySegment[];
};

export type FakeArtistRevealPayload = {
  round: number;
  fake_id: string;
  fake_name: string;
  prompt: string;
  accused_id?: string;
  accused_name?: string;
  majority_caught: boolean;
  fake_guess?: string;
  fake_guessed_prompt: boolean;
  fake_wins: boolean;
  votes: Record<string, string>;
  winners: string[];
};

export type DrawDuelRoundPayload = {
  round: number;
  prompt: string;
  artist_a_id: string;
  artist_a_name: string;
  artist_b_id: string;
  artist_b_name: string;
  judge_ids: string[];
};

export type DrawDuelRevealPayload = {
  round: number;
  prompt: string;
  artist_a_id: string;
  artist_a_name: string;
  artist_b_id: string;
  artist_b_name: string;
  drawings: DrawingSummary[];
  votes_by_judge: Record<string, string>;
  vote_count_by_drawing: Record<string, number>;
  winner_drawing_id?: string;
  winner_artist_id?: string;
  winner_artist_name?: string;
  tied: boolean;
};

export type MafiaPlayerState = {
  player_id: string;
  name: string;
  alive: boolean;
};

export type MafiaStatePayload = {
  player_id: string;
  round: number;
  phase: string;
  your_role: string;
  team_ids: string[];
  alive_players: MafiaPlayerState[];
  can_act: boolean;
  target_ids: string[];
  vote_mode?: string;
  public_votes?: Record<string, string>;
  current_voter_id?: string;
  note?: string;
  locked_in?: boolean;
};

export type MafiaRevealPayload = {
  round: number;
  phase: string;
  deaths?: string[];
  eliminated_id?: string;
  eliminated_name?: string;
  eliminated_role?: string;
  vote_mode?: string;
  votes?: Record<string, string>;
  alive_ids: string[];
  winner?: string;
  role_map?: Record<string, string>;
};

// Stroke data model used by the drawing canvas. Server validates: coords
// must be 0..1000, total points ≤ 5000.
export type Stroke = {
  points: [number, number][];
  color: string;
  width: number;
};

export type StrokesData = {
  strokes: Stroke[];
};

// Pre-reveal drawing summary broadcast at the start of the fake-prompt phase.
export type DrawingSummary = {
  drawing_id: string;
  author_id: string;
  format: "strokes" | "png";
  data: string;
};

export type DrawingsPayload = {
  round: number;
  drawings: DrawingSummary[];
};

// Pre-reveal choice list broadcast at the start of the voting phase. Omits
// voter lists + fake authorship on purpose.
export type VotingChoice = {
  choice_id: string;
  text: string;
  is_true: boolean;
};

export type VotingChoicesEntry = {
  drawing_id: string;
  choices: VotingChoice[];
};

export type VotingChoicesPayload = {
  round: number;
  entries: VotingChoicesEntry[];
};
