import { describe, expect, it } from "vitest";
import { PROTOCOL_VERSION, S2C } from "./proto";
import { parseWireEvent } from "./ws";

// Helper to produce a valid wire frame (string JSON) for a given type.
function frame(type: string, payload: unknown): string {
  return JSON.stringify({
    v: PROTOCOL_VERSION,
    id: "test-" + type,
    type,
    ts: Date.now(),
    payload,
  });
}

describe("parseWireEvent", () => {
  it("drops non-string input", () => {
    // Scenario: browser delivers a Blob or ArrayBuffer (binary frame) — not
    // a protocol case we support, but must not crash.
    // Expected: null, so the Client layer ignores it.
    expect(parseWireEvent(new Uint8Array([1, 2, 3]))).toBeNull();
    expect(parseWireEvent(42)).toBeNull();
    expect(parseWireEvent(null)).toBeNull();
  });

  it("drops malformed JSON", () => {
    // Scenario: corrupted frame arrives.
    // Expected: null, no throw. Matches the `try { JSON.parse } catch`
    // branch in the implementation.
    expect(parseWireEvent("{not json")).toBeNull();
  });

  it("drops pong (handled for liveness, not routed)", () => {
    // Scenario: server answers a ping.
    // Expected: null — the Client only uses pong to keep the socket warm;
    // screens don't react to it.
    expect(parseWireEvent(frame(S2C.Pong, {}))).toBeNull();
  });

  it("drops unknown message types", () => {
    // Scenario: a future server sends a message type this client doesn't
    // know. Clients must be forward-compatible.
    // Expected: null — no crash, ignore silently, no WireEvent emitted.
    expect(parseWireEvent(frame("brand_new_thing", { x: 1 }))).toBeNull();
  });

  it("routes join_ack to WireEvent with payload intact", () => {
    // Scenario: server accepts our JoinRoom.
    // Expected: WireEvent of type 'join_ack' with the full payload; the
    // sessionStorage side effect is NOT the parser's responsibility.
    const payload = {
      player_id: "p1",
      session_token: "tok",
      room_state: {
        room_id: "r1",
        status: "lobby",
        round: 0,
        players: [],
        scores: {},
      },
    };
    const ev = parseWireEvent(frame(S2C.JoinAck, payload));
    expect(ev?.type).toBe("join_ack");
    if (ev?.type === "join_ack") {
      expect(ev.payload.player_id).toBe("p1");
      expect(ev.payload.session_token).toBe("tok");
    }
  });

  it("routes every known S2C type to the matching WireEvent variant", () => {
    // Scenario: exhaustive check across the S2C surface (except Pong,
    // covered above).
    // Expected: each input frame produces a WireEvent whose `type` matches
    // the documented mapping. Guards against drift if new S2C types land
    // in proto.ts without a corresponding ws.ts case.
    const cases: Array<[string, string]> = [
      [S2C.StateUpdate, "state_update"],
      [S2C.PhaseChange, "phase_change"],
      [S2C.PromptIssued, "prompt_issued"],
      [S2C.SubmitTick, "submit_tick"],
      [S2C.Reveal, "reveal"],
      [S2C.Drawings, "drawings"],
      [S2C.VotingChoices, "voting_choices"],
      [S2C.RoundResult, "round_result"],
      [S2C.GameEnd, "game_end"],
      [S2C.RoomEvicting, "room_evicting"],
      [S2C.GameCatalog, "game_catalog"],
      [S2C.GameSelected, "game_selected"],
      [S2C.ReactionResult, "reaction_result"],
      [S2C.PricePrompt, "price_prompt"],
      [S2C.PriceResult, "price_result"],
      [S2C.SplitVotePrompt, "split_vote_prompt"],
      [S2C.SplitReveal, "split_reveal"],
      [S2C.MafiaState, "mafia_state"],
      [S2C.MafiaReveal, "mafia_reveal"],
      [S2C.Error, "error"],
    ];
    for (const [wire, ev] of cases) {
      const parsed = parseWireEvent(frame(wire, {}));
      expect(parsed, `${wire} → ${ev}`).not.toBeNull();
      expect(parsed!.type).toBe(ev);
    }
  });

  it("preserves payload shape through the parser (no transformation)", () => {
    // Scenario: server sends a round_result with deltas + scores.
    // Expected: payload is passed through verbatim — the parser is a
    // router, not a validator or shaper.
    const payload = {
      round: 2,
      deltas: { p1: 1000, p2: -200 },
      scores: { p1: 3000, p2: 500 },
    };
    const ev = parseWireEvent(frame(S2C.RoundResult, payload));
    expect(ev?.type).toBe("round_result");
    if (ev?.type === "round_result") {
      expect(ev.payload).toEqual(payload);
    }
  });
});
