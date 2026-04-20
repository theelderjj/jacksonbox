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
  Ping: "ping",
  SetPause: "set_pause",
  SetPencilsDown: "set_pencils_down",
  AdvanceReveal: "advance_reveal",
  UpdateSettings: "update_settings",
  RerollPrompt: "reroll_prompt",
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
};

export type SettingsState = {
  round_count: number;
  generated_fake_count: number;
  drawing_seconds: number;
  fake_prompt_seconds: number;
  voting_seconds: number;
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

export type SetPausePayload = {
  paused: boolean;
};

export type SetPencilsDownPayload = {
  disabled: boolean;
};

export type AdvanceRevealPayload = Record<string, never>;

export type UpdateSettingsPayload = {
  round_count: number;
  generated_fake_count: number;
  drawing_seconds: number;
  fake_prompt_seconds: number;
  voting_seconds: number;
};

export type RerollPromptPayload = Record<string, never>;

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
