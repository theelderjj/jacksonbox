import { describe, expect, it } from "vitest";
import type {
  DrawingsPayload,
  ErrorPayload,
  PhaseChangePayload,
  GameDefinition,
  RevealPayload,
  RoomEvictingPayload,
  RoomState,
  RoundResultPayload,
  VotingChoicesPayload,
} from "./proto";
import { initialState, reduceState, type GameState } from "./store";
import type { WireEvent } from "./ws";

// Tiny test helpers — keeps fixture bloat out of the tests themselves.
function baseRoom(overrides: Partial<RoomState> = {}): RoomState {
  return {
    room_id: "r1",
    status: "in_game",
    room_mode: "game_picker",
    phase: "lobby",
    round: 0,
    players: [{ id: "p1", name: "Alice", connected: true, ready: false }],
    scores: {},
    settings: {
      game_id: "jrawful",
      round_count: 3,
      generated_fake_count: 0,
      drawing_seconds: 60,
      fake_prompt_seconds: 90,
      voting_seconds: 20,
    },
    selected_game_id: "jrawful",
    game_catalog: [],
    ...overrides,
  };
}

function withPlayerId(id: string): GameState {
  return { ...initialState, playerId: id };
}

describe("reduceState", () => {
  it("open: sets conn=open and clears stale close info", () => {
    // Scenario: a closed socket reconnects cleanly.
    // Expected: conn flips to 'open', prior closeCode/closeReason cleared —
    // the UI should not show a stale banner after reconnect.
    const prev: GameState = {
      ...initialState,
      conn: "connecting",
      closeCode: 1006,
      closeReason: "abnormal",
    };
    const next = reduceState(prev, { type: "open" });
    expect(next.conn).toBe("open");
    expect(next.closeCode).toBeNull();
    expect(next.closeReason).toBeNull();
  });

  it("closed permanent → conn=closed, non-permanent → conn=connecting", () => {
    // Scenario: two closes — one with permanent=true (e.g. 4000 demo
    // evicted), one with permanent=false (abnormal drop, will retry).
    // Expected: the permanent close lands on terminal state 'closed'; the
    // retryable close parks the UI in 'connecting' so backoff can run.
    const permClose = reduceState(initialState, {
      type: "closed",
      code: 4000,
      reason: "demo",
      permanent: true,
    });
    expect(permClose.conn).toBe("closed");
    expect(permClose.closeCode).toBe(4000);

    const retry = reduceState(initialState, {
      type: "closed",
      code: 1006,
      reason: "abnormal",
      permanent: false,
    });
    expect(retry.conn).toBe("connecting");
  });

  it("join_ack: populates identity + seeds room snapshot", () => {
    // Scenario: server accepts our JoinRoom and returns player_id + a
    // snapshot of the room (players, round, phase, scores).
    // Expected: the store mirrors all of that. Scores defaulting to {}
    // when unset is load-bearing for the leaderboard screen.
    const ev: WireEvent = {
      type: "join_ack",
      payload: {
        player_id: "p1",
        session_token: "tok-abc",
        room_state: baseRoom({
          round: 2,
          phase: "voting_r2",
          scores: { p1: 500 },
        }),
      },
    };
    const next = reduceState(initialState, ev);
    expect(next.playerId).toBe("p1");
    expect(next.round).toBe(2);
    expect(next.phase).toBe("voting_r2");
    expect(next.roomMode).toBe("game_picker");
    expect(next.scores).toEqual({ p1: 500 });
    expect(next.players).toHaveLength(1);
  });

  it("join_ack clears stale join errors after a successful retry", () => {
    const prev: GameState = {
      ...initialState,
      lastError: { code: "not_allowed", message: "game in progress; cannot join" },
    };
    const next = reduceState(prev, {
      type: "join_ack",
      payload: {
        player_id: "p1",
        session_token: "tok-abc",
        room_state: baseRoom(),
      },
    });
    expect(next.lastError).toBeNull();
  });

  it("phase_change into prompt_distribute resets per-round buffers", () => {
    // Scenario: previous round left drawings, votingChoices, reveals, and
    // myPrompt populated. Now we enter prompt_distribute_r3.
    // Expected: all per-round caches cleared. The reset MUST fire on
    // prompt_distribute (not drawing_submit) so the prompt_issued events
    // that immediately follow land on a clean myPrompt slate instead of
    // being overwritten by a late reset.
    const prev: GameState = {
      ...initialState,
      playerId: "p1",
      drawings: { d1: { drawing_id: "d1", author_id: "p2", format: "strokes", data: "{}" } },
      votingChoices: { d1: { drawing_id: "d1", choices: [] } },
      reveals: [{ drawing_id: "d1", author_id: "p2", true_prompt: "cat", choices: [] }],
      myPrompt: "dog",
      lastDeltas: { p1: 100 },
    };
    const payload: PhaseChangePayload = {
      phase: "prompt_distribute_r3",
      round: 3,
      deadline_ms: 0,
    };
    const next = reduceState(prev, { type: "phase_change", payload });
    expect(next.phase).toBe("prompt_distribute_r3");
    expect(next.round).toBe(3);
    expect(next.drawings).toEqual({});
    expect(next.votingChoices).toEqual({});
    expect(next.reveals).toEqual([]);
    expect(next.myPrompt).toBeNull();
    expect(next.lastDeltas).toBeNull();
  });

  it("phase_change into drawing_submit preserves myPrompt", () => {
    // Regression: previously the reducer reset myPrompt on drawing_submit,
    // which nulled out the prompt that prompt_distribute had just issued
    // (because phase_change for drawing_submit is broadcast AFTER the
    // prompt_issued fan-out). The result: Drawing screen stuck on
    // "…waiting for your prompt" with Submit disabled. Pin that the reset
    // no longer happens here.
    const prev: GameState = {
      ...initialState,
      myPrompt: "dog",
      drawings: { d1: { drawing_id: "d1", author_id: "p2", format: "strokes", data: "{}" } },
    };
    const next = reduceState(prev, {
      type: "phase_change",
      payload: { phase: "drawing_submit_r1", round: 1, deadline_ms: 60000 },
    });
    expect(next.phase).toBe("drawing_submit_r1");
    expect(next.myPrompt).toBe("dog");
    expect(next.drawings).toEqual(prev.drawings);
  });

  it("phase_change into non-round-boundary phase preserves per-round buffers", () => {
    // Scenario: moving from fake_prompt_submit_r1 into voting_r1. We still
    // need the drawings + myPrompt visible; only the prompt_distribute
    // boundary is the reset point.
    const prev: GameState = {
      ...initialState,
      myPrompt: "dog",
      drawings: { d1: { drawing_id: "d1", author_id: "p2", format: "strokes", data: "{}" } },
    };
    const next = reduceState(prev, {
      type: "phase_change",
      payload: { phase: "voting_r1", round: 1, deadline_ms: 500 },
    });
    expect(next.myPrompt).toBe("dog");
    expect(next.drawings).toEqual(prev.drawings);
  });

  it("prompt_issued: only stores prompt when player_id matches self", () => {
    // Scenario: prompt_issued is fan-out — every client gets every prompt.
    // Expected: only the prompt addressed to our own playerId lands in
    // myPrompt. Other players' prompts drop silently.
    const mine = reduceState(withPlayerId("p1"), {
      type: "prompt_issued",
      payload: { player_id: "p1", prompt: "dog" },
    });
    expect(mine.myPrompt).toBe("dog");

    const theirs = reduceState(withPlayerId("p1"), {
      type: "prompt_issued",
      payload: { player_id: "p2", prompt: "cat" },
    });
    expect(theirs.myPrompt).toBeNull();
  });

  it("drawings: indexes by drawing_id", () => {
    // Scenario: server broadcasts 3 drawings at fake-phase start.
    // Expected: store holds them as {drawing_id → DrawingSummary} for O(1)
    // lookup from the FakePrompt + Voting screens.
    const payload: DrawingsPayload = {
      round: 1,
      drawings: [
        { drawing_id: "d1", author_id: "A", format: "strokes", data: "{}" },
        { drawing_id: "d2", author_id: "B", format: "strokes", data: "{}" },
        { drawing_id: "d3", author_id: "C", format: "strokes", data: "{}" },
      ],
    };
    const next = reduceState(initialState, { type: "drawings", payload });
    expect(Object.keys(next.drawings)).toHaveLength(3);
    expect(next.drawings.d2?.author_id).toBe("B");
  });

  it("voting_choices: indexes by drawing_id", () => {
    // Scenario: server broadcasts ballot for d1 (two choices).
    // Expected: store holds entry under d1 with both choices in order.
    const payload: VotingChoicesPayload = {
      round: 1,
      entries: [
        {
          drawing_id: "d1",
          choices: [
            { choice_id: "TRUE", text: "dog", is_true: true },
            { choice_id: "f1", text: "cat", is_true: false },
          ],
        },
      ],
    };
    const next = reduceState(initialState, { type: "voting_choices", payload });
    expect(next.votingChoices.d1?.choices).toHaveLength(2);
  });

  it("reveal: appends (does not replace) so multi-drawing rounds accumulate", () => {
    // Scenario: round has 3 drawings; reveal event fires per drawing.
    // Expected: each arrival appends to the reveals array — otherwise the
    // Reveal screen would only show the last drawing.
    const r1: RevealPayload = {
      drawing_id: "d1",
      author_id: "A",
      true_prompt: "dog",
      choices: [],
    };
    const r2: RevealPayload = {
      drawing_id: "d2",
      author_id: "B",
      true_prompt: "cat",
      choices: [],
    };
    let s = reduceState(initialState, { type: "reveal", payload: r1 });
    s = reduceState(s, { type: "reveal", payload: r2 });
    expect(s.reveals).toHaveLength(2);
    expect(s.reveals[1]?.drawing_id).toBe("d2");
  });

  it("round_result: captures deltas + updates scores", () => {
    // Scenario: end-of-round broadcast arrives with per-player deltas and
    // the new cumulative scores.
    // Expected: both fields land — the leaderboard uses scores, the
    // round-end screen uses deltas for the "+1000!" flourish.
    const payload: RoundResultPayload = {
      round: 1,
      deltas: { p1: 1000, p2: 500 },
      scores: { p1: 1000, p2: 500 },
    };
    const next = reduceState(initialState, { type: "round_result", payload });
    expect(next.lastDeltas).toEqual({ p1: 1000, p2: 500 });
    expect(next.scores).toEqual({ p1: 1000, p2: 500 });
  });

  it("game_end: latches final scores + flips phase to 'game_end'", () => {
    // Scenario: final round concludes.
    // Expected: phase='game_end' so App.tsx routes to GameEnd, and scores
    // hold the final totals for the leaderboard.
    const next = reduceState(initialState, {
      type: "game_end",
      payload: { scores: { p1: 4000, p2: 2500 } },
    });
    expect(next.phase).toBe("game_end");
    expect(next.roomMode).toBe("results");
    expect(next.scores).toEqual({ p1: 4000, p2: 2500 });
  });

  it("game_catalog stores available and planned games for the picker", () => {
    const games: GameDefinition[] = [
      {
        id: "jrawful",
        name: "Jrawful",
        summary: "Draw, bluff, vote.",
        min_players: 3,
        max_players: 20,
        estimated_minutes: 20,
        tags: ["drawing"],
        status: "available",
      },
      {
        id: "fake_artist",
        name: "Fake Artist",
        summary: "Coming soon.",
        min_players: 4,
        max_players: 12,
        estimated_minutes: 10,
        tags: ["drawing"],
        status: "planned",
      },
    ];
    const next = reduceState(initialState, { type: "game_catalog", payload: { games } });
    expect(next.gameCatalog).toEqual(games);
  });

  it("game_selected moves the room into game_lobby and stores the selection", () => {
    const next = reduceState(initialState, {
      type: "game_selected",
      payload: { game_id: "jrawful" },
    });
    expect(next.selectedGameId).toBe("jrawful");
    expect(next.roomMode).toBe("game_lobby");
  });

  it("reaction_result stores the latest duel standings", () => {
    const next = reduceState(initialState, {
      type: "reaction_result",
      payload: {
        round: 1,
        results: [
          { player_id: "p1", player_name: "Alice", false_start: false, reaction_ms: 12, rank: 1, points: 1000 },
        ],
      },
    });
    expect(next.reactionResults).toHaveLength(1);
    expect(next.reactionResults[0]?.player_name).toBe("Alice");
  });

  it("price prompt and result events store the round listing and reveal", () => {
    const withPrompt = reduceState(initialState, {
      type: "price_prompt",
      payload: {
        round: 1,
        product_id: "vacuum",
        product_name: "Cordless Stick Vacuum",
        image_url: "data:image/svg+xml;base64,AAA",
        threshold_cents: 500,
        threshold_mode: "fixed",
        threshold_base_cents: 500,
      },
    });
    expect(withPrompt.pricePrompt?.product_name).toBe("Cordless Stick Vacuum");

    const withResult = reduceState(withPrompt, {
      type: "price_result",
      payload: {
        round: 1,
        product_id: "vacuum",
        product_name: "Cordless Stick Vacuum",
        image_url: "data:image/svg+xml;base64,AAA",
        actual_price_cents: 14999,
        threshold_cents: 500,
        threshold_mode: "fixed",
        guesses: { p1: 14900 },
        winner_ids: ["p1"],
      },
    });
    expect(withResult.priceResult?.actual_price_cents).toBe(14999);
    expect(withResult.priceResult?.winner_ids).toEqual(["p1"]);
  });

  it("split vote prompt and reveal events store their payloads", () => {
    const withPrompt = reduceState(initialState, {
      type: "split_vote_prompt",
      payload: {
        round: 1,
        splitter_id: "p1",
        splitter_name: "Alice",
        prompt: "Who wins?",
        option_a: "A",
        option_b: "B",
        show_target: false,
      },
    });
    expect(withPrompt.splitVotePrompt?.splitter_name).toBe("Alice");

    const withReveal = reduceState(withPrompt, {
      type: "split_reveal",
      payload: {
        round: 1,
        splitter_id: "p1",
        splitter_name: "Alice",
        prompt: "Who wins?",
        option_a: "A",
        option_b: "B",
        count_a: 2,
        count_b: 1,
        target_a: 2,
        target_b: 1,
        target_mode: "variable",
        show_target: true,
        achieved: true,
        player_choices: { p1: "A" },
      },
    });
    expect(withReveal.splitReveal?.achieved).toBe(true);
  });

  it("mafia private state only lands for the current player and reveal is global", () => {
    const mine = reduceState({ ...initialState, playerId: "p1" }, {
      type: "mafia_state",
      payload: {
        player_id: "p1",
        round: 1,
        phase: "night_collect",
        your_role: "detective",
        team_ids: [],
        alive_players: [{ player_id: "p1", name: "Alice", alive: true }],
        can_act: true,
        target_ids: ["p2"],
        note: "Investigate one player.",
      },
    });
    expect(mine.mafiaState?.your_role).toBe("detective");

    const theirs = reduceState({ ...initialState, playerId: "p1" }, {
      type: "mafia_state",
      payload: {
        player_id: "p2",
        round: 1,
        phase: "night_collect",
        your_role: "mafia",
        team_ids: ["p2"],
        alive_players: [{ player_id: "p2", name: "Bob", alive: true }],
        can_act: true,
        target_ids: ["p1"],
      },
    });
    expect(theirs.mafiaState).toBeNull();

    const revealed = reduceState(mine, {
      type: "mafia_reveal",
      payload: {
        round: 1,
        phase: "day_reveal",
        eliminated_id: "p2",
        eliminated_name: "Bob",
        eliminated_role: "mafia",
        votes: { p1: "p2" },
        alive_ids: ["p1"],
        winner: "town",
        role_map: { p1: "detective", p2: "mafia" },
      },
    });
    expect(revealed.mafiaReveal?.winner).toBe("town");
  });

  it("room_evicting: surfaces reason for the eviction banner", () => {
    // Scenario: server is about to evict (demo cap / shutdown).
    // Expected: the reason string is cached so the banner can render it.
    const payload: RoomEvictingPayload = { reason: "demo_cap", grace_ms: 5000 };
    const next = reduceState(initialState, { type: "room_evicting", payload });
    expect(next.evictingReason).toBe("demo_cap");
  });

  it("error: caches lastError for the error banner", () => {
    // Scenario: a C2S validation failure comes back as an error frame.
    // Expected: lastError holds the payload so the banner can show it.
    const payload: ErrorPayload = { code: "BAD_INPUT", message: "too long" };
    const next = reduceState(initialState, { type: "error", payload });
    expect(next.lastError).toEqual(payload);
  });

  it("phase_change clears stale error banners", () => {
    // Scenario: an error arrives late in one phase, then the room moves on.
    // Expected: the global banner does not leak into drawing or later rounds.
    const prev: GameState = {
      ...initialState,
      lastError: { code: "not_allowed", message: "old phase" },
    };
    const next = reduceState(prev, {
      type: "phase_change",
      payload: { phase: "drawing_submit_r2", round: 2, deadline_ms: 60000 },
    });
    expect(next.lastError).toBeNull();
  });

  it("submit_tick is a no-op (v1 does not surface progress)", () => {
    // Scenario: server sends a submit_tick progress frame mid-phase.
    // Expected: state is returned unchanged (===) — §tick is deferred to v1.1.
    const prev: GameState = { ...initialState, playerId: "p1" };
    const next = reduceState(prev, {
      type: "submit_tick",
      payload: { phase: "drawing_submit_r1", submitted: 2, total: 3 },
    });
    expect(next).toBe(prev);
  });

  it("reveal_step eliminate: appends choice id under the drawing bucket", () => {
    // Scenario: server drives leaderboard_rN animation — fires one eliminate
    // per fake choice before the final step. The UI filters choices through
    // revealEliminated[drawing_id] to grey them out progressively.
    // Expected: first eliminate creates the bucket; second appends; duplicate
    // eliminate is idempotent (same ID doesn't double-add).
    const prev: GameState = withPlayerId("p1");
    const s1 = reduceState(prev, {
      type: "reveal_step",
      payload: { round: 1, drawing_id: "d1", step: 1, eliminated_choice_id: "f1" },
    });
    expect(s1.revealEliminated.d1).toEqual(["f1"]);
    expect(s1.revealCurrentDrawing).toBe("d1");

    const s2 = reduceState(s1, {
      type: "reveal_step",
      payload: { round: 1, drawing_id: "d1", step: 2, eliminated_choice_id: "f2" },
    });
    expect(s2.revealEliminated.d1).toEqual(["f1", "f2"]);

    const dup = reduceState(s2, {
      type: "reveal_step",
      payload: { round: 1, drawing_id: "d1", step: 3, eliminated_choice_id: "f1" },
    });
    // Idempotent: no-op on duplicate elimination.
    expect(dup.revealEliminated.d1).toEqual(["f1", "f2"]);
  });

  it("reveal_step final: stores deltas + awards under the drawing bucket", () => {
    // Scenario: after all eliminations land, the server fires one `final`
    // step per drawing. The LeaderboardRound screen reads revealFinals to
    // (a) reveal TRUE, (b) render the per-drawing awards block.
    const prev: GameState = withPlayerId("p1");
    const next = reduceState(prev, {
      type: "reveal_step",
      payload: {
        round: 1,
        drawing_id: "d1",
        step: 5,
        is_final: true,
        deltas: { p1: 500, p2: 1000 },
        awards: [
          { player_id: "p1", points: 500, reason: "faker_fool" },
          { player_id: "p2", points: 1000, reason: "guesser_truth" },
        ],
      },
    });
    expect(next.revealFinals.d1?.deltas).toEqual({ p1: 500, p2: 1000 });
    expect(next.revealFinals.d1?.awards).toHaveLength(2);
    expect(next.revealCurrentDrawing).toBe("d1");
  });

  it("pause_state: mirrors leader-driven pause + pencils-down + remaining", () => {
    // Scenario: leader pauses with 23s left on the drawing timer; Countdown
    // switches to "⏸ 23s" frozen. Pencils-down stays false in this step.
    const paused = reduceState(initialState, {
      type: "pause_state",
      payload: {
        paused: true,
        pencils_down: false,
        remaining_ms: 23_000,
        leader_id: "p1",
        deadline_ms: 0,
      },
    });
    expect(paused.paused).toBe(true);
    expect(paused.pencilsDown).toBe(false);
    expect(paused.pauseRemainingMs).toBe(23_000);
    expect(paused.leaderId).toBe("p1");

    // Scenario: leader flips pencils-down on while still paused.
    const lockedAndPaused = reduceState(paused, {
      type: "pause_state",
      payload: {
        paused: true,
        pencils_down: true,
        remaining_ms: 23_000,
        leader_id: "p1",
        deadline_ms: 0,
      },
    });
    expect(lockedAndPaused.pencilsDown).toBe(true);

    // Scenario: leader resumes — server sends a fresh deadline, remaining=0.
    const resumed = reduceState(lockedAndPaused, {
      type: "pause_state",
      payload: {
        paused: false,
        pencils_down: true,
        remaining_ms: 0,
        leader_id: "p1",
        deadline_ms: 1_700_000_000_000,
      },
    });
    expect(resumed.paused).toBe(false);
    expect(resumed.pauseRemainingMs).toBe(0);
    expect(resumed.deadlineMs).toBe(1_700_000_000_000);
  });

  it("leader_change: updates leaderId after original leader disconnects", () => {
    // Scenario: party leader disconnects; server promotes the next
    // earliest-joined player and fan-outs S2CLeaderChange.
    // Expected: leaderId tracks the new leader so LeaderControls shows the
    // right name + hides/shows pause buttons for the right player.
    const prev: GameState = { ...initialState, leaderId: "p1" };
    const next = reduceState(prev, {
      type: "leader_change",
      payload: { leader_id: "p2" },
    });
    expect(next.leaderId).toBe("p2");
  });

  it("phase_change into prompt_distribute resets reveal animation buffers", () => {
    // Scenario: leaderboard_r1 ends, prompt_distribute_r2 begins. Per-round
    // reveal animation caches (eliminated, finals, currentDrawing) must
    // clear so r2's animation starts fresh, not layered on r1's stale ids.
    const prev: GameState = {
      ...initialState,
      revealEliminated: { d1: ["f1", "f2"] },
      revealFinals: {
        d1: { deltas: { p1: 500 }, awards: [{ player_id: "p1", points: 500, reason: "faker_fool" }] },
      },
      revealCurrentDrawing: "d1",
    };
    const next = reduceState(prev, {
      type: "phase_change",
      payload: { phase: "prompt_distribute_r2", round: 2, deadline_ms: 0 },
    });
    expect(next.revealEliminated).toEqual({});
    expect(next.revealFinals).toEqual({});
    expect(next.revealCurrentDrawing).toBeNull();
  });

  it("phase_change into leaderboard seeds revealCurrentDrawing from the first pending reveal", () => {
    const prev: GameState = {
      ...initialState,
      reveals: [
        { drawing_id: "d1", author_id: "p2", true_prompt: "cat", choices: [] },
        { drawing_id: "d2", author_id: "p3", true_prompt: "dog", choices: [] },
      ],
      revealFinals: {
        d1: { deltas: { p2: 1000 }, awards: [{ player_id: "p2", points: 1000, reason: "drawer_truth" }] },
      },
      revealCurrentDrawing: null,
    };
    const next = reduceState(prev, {
      type: "phase_change",
      payload: { phase: "leaderboard_r1", round: 1, deadline_ms: 0 },
    });
    expect(next.revealCurrentDrawing).toBe("d2");
  });

  it("reducer is pure: does not mutate the input state object", () => {
    // Scenario: run a handful of events against the same prev object and
    // compare it to a snapshot taken beforehand.
    // Expected: prev unchanged — React + useSyncExternalStore rely on
    // reference equality to skip renders.
    const prev: GameState = { ...initialState, playerId: "p1" };
    const snapshot = JSON.stringify(prev);
    reduceState(prev, { type: "open" });
    reduceState(prev, { type: "game_end", payload: { scores: { p1: 1 } } });
    reduceState(prev, {
      type: "reveal",
      payload: { drawing_id: "d1", author_id: "A", true_prompt: "x", choices: [] },
    });
    expect(JSON.stringify(prev)).toBe(snapshot);
  });
});
