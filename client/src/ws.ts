// WebSocket client with reconnect + session-token persistence.
//
// Trade-offs:
//   - sessionStorage (not localStorage) so tokens die with the tab; §11's
//     TTL enforces server-side correctness regardless.
//   - Exponential backoff with jitter, capped. Manual triggers (user hits
//     "retry") reset the backoff to zero.
//   - The close codes carry protocol meaning; permanent closes bypass
//     reconnect and surface to UI as permanent.

import {
  C2S,
  PROTOCOL_VERSION,
  type DrawingsPayload,
  type Envelope,
  type ErrorPayload,
  type JoinAckPayload,
  type LeaderChangePayload,
  type PauseStatePayload,
  type PhaseChangePayload,
  type PromptIssuedPayload,
  type RevealPayload,
  type RevealStepPayload,
  type RoomEvictingPayload,
  type RoomState,
  type RoundResultPayload,
  type VotingChoicesPayload,
  S2C,
} from "./proto";

export type WireEvent =
  | { type: "open" }
  | { type: "closed"; code: number; reason: string; permanent: boolean }
  | { type: "join_ack"; payload: JoinAckPayload }
  | { type: "state_update"; payload: RoomState }
  | { type: "phase_change"; payload: PhaseChangePayload }
  | { type: "prompt_issued"; payload: PromptIssuedPayload }
  | { type: "submit_tick"; payload: { phase: string; submitted: number; total: number } }
  | { type: "reveal"; payload: RevealPayload }
  | { type: "reveal_step"; payload: RevealStepPayload }
  | { type: "drawings"; payload: DrawingsPayload }
  | { type: "voting_choices"; payload: VotingChoicesPayload }
  | { type: "round_result"; payload: RoundResultPayload }
  | { type: "game_end"; payload: { scores: Record<string, number> } }
  | { type: "room_evicting"; payload: RoomEvictingPayload }
  | { type: "pause_state"; payload: PauseStatePayload }
  | { type: "leader_change"; payload: LeaderChangePayload }
  | { type: "error"; payload: ErrorPayload };

type Listener = (ev: WireEvent) => void;

// Codes that mean "do not reconnect; user must re-initiate explicitly."
const PERMANENT_CLOSE = new Set([4000 /* room reset */, 4001 /* version bump */]);

const TOKEN_KEY = "jacksonbox.session_token";
const NAME_KEY = "jacksonbox.player_name";
const ROOM_KEY = "jacksonbox.room_id";

const BACKOFF_MIN_MS = 250;
const BACKOFF_MAX_MS = 15_000;

export class Client {
  private ws: WebSocket | null = null;
  private listeners = new Set<Listener>();
  private attempts = 0;
  private wantOpen = false;
  private lastRoom: string | null = null;
  private lastName: string | null = null;
  private reconnectTimer: number | null = null;

  constructor(private readonly url: string = defaultWsURL()) {}

  on(l: Listener): () => void {
    this.listeners.add(l);
    return () => this.listeners.delete(l);
  }

  /** Open the socket and auto-join the given room. */
  connect(roomId: string, name: string): void {
    this.lastRoom = roomId;
    this.lastName = name;
    sessionStorage.setItem(ROOM_KEY, roomId);
    sessionStorage.setItem(NAME_KEY, name);
    this.wantOpen = true;
    this.open();
  }

  /** Force-close. Suppresses reconnect until connect() is called again. */
  disconnect(): void {
    this.wantOpen = false;
    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.ws?.close(1000, "client closed");
    this.ws = null;
  }

  /** Send a client->server envelope. Returns the envelope id. */
  send<P>(type: string, payload: P): string | null {
    const ws = this.ws;
    if (!ws || ws.readyState !== WebSocket.OPEN) return null;
    const id = crypto.randomUUID();
    const env: Envelope<P> = {
      v: PROTOCOL_VERSION,
      id,
      type,
      ts: Date.now(),
      payload,
    };
    ws.send(JSON.stringify(env));
    return id;
  }

  private open(): void {
    const ws = new WebSocket(this.url);
    this.ws = ws;
    ws.onopen = () => {
      this.attempts = 0;
      this.emit({ type: "open" });
      this.sendJoin();
    };
    ws.onmessage = (ev) => this.onMessage(ev.data);
    ws.onclose = (ev) => this.onClose(ev.code, ev.reason);
    ws.onerror = () => {
      // onclose will follow; emit nothing here to avoid double-handling.
    };
  }

  private sendJoin(): void {
    if (!this.lastRoom || !this.lastName) return;
    const token = sessionStorage.getItem(TOKEN_KEY) ?? "";
    this.send(C2S.JoinRoom, {
      room_id: this.lastRoom,
      name: this.lastName,
      session_token: token,
    });
  }

  private onMessage(raw: unknown): void {
    const ev = parseWireEvent(raw);
    if (!ev) return;
    // Side effect: persist session token on join_ack so a reconnect in
    // the same tab can skip the name prompt. Kept out of parseWireEvent
    // so that function stays pure + unit-testable.
    if (ev.type === "join_ack") {
      sessionStorage.setItem(TOKEN_KEY, ev.payload.session_token);
    }
    this.emit(ev);
  }

  private onClose(code: number, reason: string): void {
    const permanent = PERMANENT_CLOSE.has(code) || !this.wantOpen;
    this.ws = null;
    this.emit({ type: "closed", code, reason, permanent });
    if (permanent) {
      if (code === 4000 || code === 4001) {
        // Invalidate token on permanent failures — forces fresh identity.
        sessionStorage.removeItem(TOKEN_KEY);
      }
      return;
    }
    this.attempts++;
    const delay = backoffDelay(this.attempts);
    this.reconnectTimer = window.setTimeout(() => this.open(), delay);
  }

  private emit(ev: WireEvent): void {
    for (const l of this.listeners) l(ev);
  }
}

function defaultWsURL(): string {
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${window.location.host}/ws`;
}

// Pure: raw wire frame → WireEvent (or null to drop). Exported for tests.
// Unknown/malformed frames + pong return null. No side effects — the Client
// layers sessionStorage + subscriber fan-out on top.
export function parseWireEvent(raw: unknown): WireEvent | null {
  if (typeof raw !== "string") return null;
  let env: Envelope;
  try {
    env = JSON.parse(raw) as Envelope;
  } catch {
    return null;
  }
  switch (env.type) {
    case S2C.JoinAck:
      return { type: "join_ack", payload: env.payload as JoinAckPayload };
    case S2C.StateUpdate:
      return { type: "state_update", payload: env.payload as RoomState };
    case S2C.PhaseChange:
      return { type: "phase_change", payload: env.payload as PhaseChangePayload };
    case S2C.PromptIssued:
      return { type: "prompt_issued", payload: env.payload as PromptIssuedPayload };
    case S2C.SubmitTick:
      return {
        type: "submit_tick",
        payload: env.payload as { phase: string; submitted: number; total: number },
      };
    case S2C.Reveal:
      return { type: "reveal", payload: env.payload as RevealPayload };
    case S2C.RevealStep:
      return { type: "reveal_step", payload: env.payload as RevealStepPayload };
    case S2C.Drawings:
      return { type: "drawings", payload: env.payload as DrawingsPayload };
    case S2C.VotingChoices:
      return { type: "voting_choices", payload: env.payload as VotingChoicesPayload };
    case S2C.RoundResult:
      return { type: "round_result", payload: env.payload as RoundResultPayload };
    case S2C.GameEnd:
      return {
        type: "game_end",
        payload: env.payload as { scores: Record<string, number> },
      };
    case S2C.RoomEvicting:
      return { type: "room_evicting", payload: env.payload as RoomEvictingPayload };
    case S2C.PauseState:
      return { type: "pause_state", payload: env.payload as PauseStatePayload };
    case S2C.LeaderChange:
      return { type: "leader_change", payload: env.payload as LeaderChangePayload };
    case S2C.Error:
      return { type: "error", payload: env.payload as ErrorPayload };
    case S2C.Pong:
      return null;
    default:
      return null;
  }
}

// Exponential backoff with ±20% jitter. Capped at BACKOFF_MAX_MS.
function backoffDelay(attempts: number): number {
  const base = Math.min(BACKOFF_MAX_MS, BACKOFF_MIN_MS * 2 ** (attempts - 1));
  const jitter = base * 0.2 * (Math.random() * 2 - 1);
  return Math.max(BACKOFF_MIN_MS, Math.floor(base + jitter));
}
