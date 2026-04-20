// Single-source-of-truth store for the client. Deliberately lightweight —
// plain subscription model wrapped in a React hook. No Redux, no context
// provider; the store is a module-singleton, which fits a single-room
// client and keeps the render path boring.

import { useSyncExternalStore } from "react";
import type {
  DrawingSummary,
  ErrorPayload,
  PhaseChangePayload,
  PlayerInfo,
  RevealAward,
  RevealPayload,
  RoomState,
  RoundResultPayload,
  SettingsState,
  VotingChoicesEntry,
} from "./proto";
import { Client, type WireEvent } from "./ws";

export type ConnState = "idle" | "connecting" | "open" | "closed" | "error";

export type GameState = {
  // Connection / identity
  conn: ConnState;
  closeCode: number | null;
  closeReason: string | null;
  playerId: string | null;

  // Lobby + room
  room: RoomState | null;
  players: PlayerInfo[];
  round: number;
  phase: string;
  deadlineMs: number | null;

  // Drawing phase
  myPrompt: string | null;

  // Fake + voting phases — drawings broadcast at fake-phase start; choices
  // broadcast at voting-phase start. Indexed by drawing_id for O(1) lookup.
  drawings: Record<string, DrawingSummary>;
  votingChoices: Record<string, VotingChoicesEntry>;

  // Reveal phase — one payload per drawing this round
  reveals: RevealPayload[];

  // Reveal step animation — server-driven during leaderboard_rN. eliminated
  // tracks which choice IDs the UI has been told to grey out, finals tracks
  // drawings whose final-step event has landed (so the UI can show per-
  // drawing deltas + awards), and currentDrawing is the latest drawing the
  // animator is focused on. Reset at the start of each new round.
  revealEliminated: Record<string, string[]>;
  revealFinals: Record<string, { deltas: Record<string, number>; awards: RevealAward[] }>;
  revealCurrentDrawing: string | null;

  // Round + final scores
  scores: Record<string, number>;
  lastDeltas: Record<string, number> | null;

  // Party-leader / pause / pencils-down state (mirrored from server).
  leaderId: string | null;
  paused: boolean;
  pencilsDown: boolean;
  pauseRemainingMs: number | null;
  settings: SettingsState;

  // Banners
  lastError: ErrorPayload | null;
  evictingReason: string | null;
};

export const initialState: GameState = {
  conn: "idle",
  closeCode: null,
  closeReason: null,
  playerId: null,
  room: null,
  players: [],
  round: 0,
  phase: "",
  deadlineMs: null,
  myPrompt: null,
  drawings: {},
  votingChoices: {},
  reveals: [],
  revealEliminated: {},
  revealFinals: {},
  revealCurrentDrawing: null,
  scores: {},
  lastDeltas: null,
  leaderId: null,
  paused: false,
  pencilsDown: false,
  pauseRemainingMs: null,
  settings: {
    round_count: 3,
    generated_fake_count: 0,
    drawing_seconds: 60,
    fake_prompt_seconds: 90,
    voting_seconds: 20,
  },
  lastError: null,
  evictingReason: null,
};

let state: GameState = initialState;
const subs = new Set<() => void>();

// Test-only: reset the singleton so unit tests start from a clean slate.
// Not part of the public API — don't call this from screens.
export function __resetStoreForTest(): void {
  state = initialState;
  for (const fn of subs) fn();
}

const snapshotGetter = (): GameState => state;
const subscribe = (fn: () => void): (() => void) => {
  subs.add(fn);
  return () => subs.delete(fn);
};

export function useGameState(): GameState {
  return useSyncExternalStore(subscribe, snapshotGetter, snapshotGetter);
}

// The client is exported as a singleton; screens send inputs through it.
export const client = new Client();

// Wire the client's events into store mutations. Done once at module load.
client.on((ev: WireEvent) => {
  state = reduceState(state, ev);
  for (const fn of subs) fn();
});

// Pure reducer: (prev, event) → next. Exported so unit tests can exercise
// every case without spinning up a WebSocket. Do not reach into module-level
// state from inside this function — it would defeat testability.
export function reduceState(prev: GameState, ev: WireEvent): GameState {
  switch (ev.type) {
    case "open":
      return { ...prev, conn: "open", closeCode: null, closeReason: null };

    case "closed":
      return {
        ...prev,
        conn: ev.permanent ? "closed" : "connecting",
        closeCode: ev.code,
        closeReason: ev.reason,
      };

    case "join_ack":
      return {
        ...prev,
        playerId: ev.payload.player_id,
        room: ev.payload.room_state,
        players: ev.payload.room_state.players,
        round: ev.payload.room_state.round,
        phase: ev.payload.room_state.phase ?? "",
        scores: ev.payload.room_state.scores ?? {},
        leaderId: ev.payload.room_state.leader_id ?? null,
        paused: ev.payload.room_state.paused ?? false,
        pencilsDown: ev.payload.room_state.pencils_down ?? false,
        pauseRemainingMs: ev.payload.room_state.remaining_ms ?? null,
        settings: ev.payload.room_state.settings ?? prev.settings,
        lastError: null,
      };

    case "state_update":
      return {
        ...prev,
        room: ev.payload,
        players: ev.payload.players,
        round: ev.payload.round,
        phase: ev.payload.phase ?? "",
        scores: ev.payload.scores ?? {},
        leaderId: ev.payload.leader_id ?? null,
        paused: ev.payload.paused ?? false,
        pencilsDown: ev.payload.pencils_down ?? false,
        pauseRemainingMs: ev.payload.remaining_ms ?? null,
        settings: ev.payload.settings ?? prev.settings,
      };

    case "phase_change": {
      const p = ev.payload as PhaseChangePayload;
      // Reset per-round buffers at the start of `prompt_distribute` — that's
      // the canonical "new round" boundary. The reset MUST fire BEFORE
      // prompt_issued events arrive; pegging it to `drawing_submit` (the
      // previous version) nulled myPrompt after it had already been set,
      // which left the Drawing screen stuck on "…waiting for your prompt".
      const resetRound = startsWith(p.phase, "prompt_distribute")
        ? {
            reveals: [] as RevealPayload[],
            drawings: {} as Record<string, DrawingSummary>,
            votingChoices: {} as Record<string, VotingChoicesEntry>,
            myPrompt: null as string | null,
            lastDeltas: null as Record<string, number> | null,
            revealEliminated: {} as Record<string, string[]>,
            revealFinals: {} as Record<
              string,
              { deltas: Record<string, number>; awards: RevealAward[] }
            >,
            revealCurrentDrawing: null as string | null,
          }
        : {};
      const leaderboardStart = startsWith(p.phase, "leaderboard")
        ? {
            revealCurrentDrawing:
              prev.reveals.find((r) => !prev.revealFinals[r.drawing_id])?.drawing_id ??
              prev.reveals[0]?.drawing_id ??
              null,
          }
        : {};
      return {
        ...prev,
        round: p.round,
        phase: p.phase,
        deadlineMs: p.deadline_ms || null,
        lastError: null,
        ...resetRound,
        ...leaderboardStart,
      };
    }

    case "prompt_issued":
      // prompt_issued is fan-out; each client filters on their own player_id.
      if (ev.payload.player_id === prev.playerId) {
        return { ...prev, myPrompt: ev.payload.prompt };
      }
      return prev;

    case "submit_tick":
      // v1.1: surface as a progress indicator. For v1 we drop silently.
      return prev;

    case "drawings": {
      const next: Record<string, DrawingSummary> = {};
      for (const d of ev.payload.drawings) next[d.drawing_id] = d;
      return { ...prev, drawings: next };
    }

    case "voting_choices": {
      const next: Record<string, VotingChoicesEntry> = {};
      for (const e of ev.payload.entries) next[e.drawing_id] = e;
      return { ...prev, votingChoices: next };
    }

    case "reveal":
      return { ...prev, reveals: [...prev.reveals, ev.payload] };

    case "reveal_step": {
      const step = ev.payload;
      // Focus the UI on this drawing (so the canvas + choice list swaps
      // into view whenever a new drawing's animation starts).
      const currentDrawing = step.drawing_id || prev.revealCurrentDrawing;
      if (step.eliminated_choice_id) {
        const prior = prev.revealEliminated[step.drawing_id] ?? [];
        if (prior.includes(step.eliminated_choice_id)) {
          // Idempotent: duplicate broadcast shouldn't double-grey.
          return { ...prev, revealCurrentDrawing: currentDrawing };
        }
        return {
          ...prev,
          revealCurrentDrawing: currentDrawing,
          revealEliminated: {
            ...prev.revealEliminated,
            [step.drawing_id]: [...prior, step.eliminated_choice_id],
          },
        };
      }
      if (step.is_final) {
        return {
          ...prev,
          revealCurrentDrawing: currentDrawing,
          revealFinals: {
            ...prev.revealFinals,
            [step.drawing_id]: {
              deltas: step.deltas ?? {},
              awards: step.awards ?? [],
            },
          },
        };
      }
      return { ...prev, revealCurrentDrawing: currentDrawing };
    }

    case "round_result": {
      const rr = ev.payload as RoundResultPayload;
      return { ...prev, lastDeltas: rr.deltas, scores: rr.scores };
    }

    case "game_end":
      return { ...prev, scores: ev.payload.scores, phase: "game_end" };

    case "room_evicting":
      return { ...prev, evictingReason: ev.payload.reason };

    case "pause_state":
      return {
        ...prev,
        paused: ev.payload.paused,
        pencilsDown: ev.payload.pencils_down,
        pauseRemainingMs: ev.payload.remaining_ms ?? null,
        leaderId: ev.payload.leader_id,
        deadlineMs: ev.payload.deadline_ms || prev.deadlineMs,
      };

    case "leader_change":
      return { ...prev, leaderId: ev.payload.leader_id };

    case "error":
      return { ...prev, lastError: ev.payload };
  }
}

function startsWith(s: string, prefix: string): boolean {
  return s.length >= prefix.length && s.slice(0, prefix.length) === prefix;
}

// Helper: the current player's info, or null if we don't know our id yet.
export function selfPlayer(g: GameState): PlayerInfo | null {
  if (!g.playerId) return null;
  return g.players.find((p) => p.id === g.playerId) ?? null;
}

// Helper: all other players, useful for lobby + voting UI.
export function otherPlayers(g: GameState): PlayerInfo[] {
  return g.players.filter((p) => p.id !== g.playerId);
}
