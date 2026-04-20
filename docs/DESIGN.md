# Realtime Trivia Game Engine — DESIGN

This is the in-repo companion to the v4 handoff spec (see `docs/trivia_game_design_v4.txt` if attached, or the original upload). It summarizes the shipped v1 architecture, flags deltas from the spec, and lists the failure-mode runbook.

## Scope

Single-room real-time multiplayer trivia engine. Portfolio scope: invite-only, ≤20 players, single operator, one VM/container, no persistence. Jrawful-style game mode is the only game shipped in v1.

## Top-level shape

```
cmd/server            HTTP + WebSocket boot
internal/
  engine              Phase runner + primitive contract (generics via Erase[R])
  primitives          CollectAll, FirstValid, Aggregate, TurnBased, ScoreTransform
  games/drawful       Jrawful phases + prompts + scoring
  proto               Wire types; S2C/C2S constants; close codes; error codes
  gateway             WS connection, frame validation, idempotency ring buffer
  auth                Session token issue + reconnect validation
  obs                 slog JSON wiring
  room                Actor-model Room (single-writer goroutine) + manager
client/               Vite + React 18 + TS strict (single-page)
docs/DESIGN.md        This doc
CLAUDE.md             Orientation for future Claude sessions
```

## Actor model

`internal/room/Room` owns `GameState` and runs a single goroutine that drains a tagged-union `actorMsg` inbox. All mutations happen there. External callers post messages; there are no locks on `GameState` and no cross-goroutine reads of its interior fields.

Observability + test-only `kindSnapshot` message reads a read-only copy out of the actor. `snapshotForTest()` times out at 500ms on both send and reply so a crashed actor doesn't hang the suite.

Panic recovery runs in a `defer`: actor broadcasts `S2CError{code: room_crashed}`, closes all conns with code `1011` (CloseAbnormal), and sets status to `Closed`. Covered by `TestIntegration_PanicRunbook`.

## Phase contract

```go
type Primitive[R any] interface {
    Start(*PhaseContext) (Decision[R], error)
    Handle(*PhaseContext, Input) (Decision[R], error)
    Timeout(*PhaseContext) (Decision[R], error)
    Name() string
}

type Decision[R any] struct {
    AdvancePhase bool
    Broadcast    []Event
    Result       R
}
```

`Erase[R]` lifts typed primitives into `AnyPrimitive`; `GetResult[R]` recovers them downstream. The sealed-interface trick (`PhaseResult` with unexported `isPhaseResult()`) makes `StoredResult[R]` the only thing storable in `PhaseData`.

## Jrawful Phase Sequence (Per Round)

1. `prompt_distribute` — Aggregate; emits one `S2CPromptIssued` per player, stores `PromptDistributionResult`.
2. `drawing_submit` — CollectAll[Drawing]; 60s.
3. `fake_prompt_submit` — bespoke batch collector; 90s. **On Start**: broadcasts `S2CDrawings` so clients can render the drawings.
4. `voting` — bespoke collector; 20s. **On Start**: broadcasts `S2CVotingChoices` (merged fakes + TRUE, no authorship or voters).
5. `reveal` — Aggregate; one `S2CReveal` per drawing with full authorship + voter tally.
6. `scoring` — ScoreTransform; emits `S2CRoundResult`.

Round count is `ceil(players/2)`, min 3. At the final round, `startPhase` detects the end and broadcasts `S2CGameEnd`, resets the room to Idle, and clears ready flags.

## Wire protocol

`proto.Envelope{v,id,type,ts,payload}`. `v=1`. Frame cap 256KB. Drawing strokes capped at ≤5000 total points; coordinates normalized 0..1000 so server validation is display-size-agnostic.

### Close codes (extra to RFC 6455)

- `4000` demo evicted — client stops reconnecting, clears session token
- `4001` version bump — client stops reconnecting, clears session token
- `1011` abnormal — room crashed; client treats as reconnectable

### Reconnect

Clients persist `session_token` in `sessionStorage`. On WS open they re-send `join_room` with the token; the `auth` package validates and returns the same `PlayerID`. 60s grace on disconnect before server reaps. Tested in `TestIntegration_Reconnect`.

## Delta from v4 spec

- **Added `S2CDrawings` + `S2CVotingChoices` broadcasts.** The spec implicitly assumed a presenter-style UI where the TV showed drawings; in a web-only client, the phone needs them. These events fire at the Start of the fake and vote phases respectively. Voting choices exclude voter lists and fake authorship to avoid spoiling the round.
- **`DemoEvictionGrace` and `DisconnectGrace` are `var`, not `const`.** Tests override them to keep runtimes under a second. Production callers must not mutate.
- **`testPhaseBuilder(draw, fake, vote)` helper.** Integration tests use a 1-round phase list instead of Jrawful's 3-round minimum. Production `BuildPhases` path is unchanged.

## Failure-mode runbook

| Symptom | Cause | Recovery |
|---|---|---|
| Client disconnected, can't rejoin | Session token expired (TTL or explicit `4000`/`4001`) | Client clears `sessionStorage`, rejoin fresh. |
| Room stuck in phase | Primitive returned no AdvancePhase decision + no timeout | Deadline timer fires, Timeout advances; if timer is zero-duration (Aggregate) and primitive returned no advance, that's a bug — check logs for `primitive start error`. |
| `S2CError{code: room_crashed}` to all clients | Panic in primitive Start/Handle/Timeout | Actor's deferred `recoverPanic` fired. All conns closed `1011`. Server log carries stack. Restart room by rejoining; state is gone on purpose. |
| Frames > 256KB | Oversize drawing/fake | Gateway closes conn with `1009` (CloseFrameTooBig) before delivery. |
| Player idle with no heartbeat | Ping/pong stopped | Conn reader's read-deadline fires; actor receives `kindLeave`; 60s disconnect grace starts. |
| Demo room evicted mid-game | Main room fills past 2 players, demo gets notice | Demo room broadcasts `S2CRoomEvicting`, closes conns with `4000` after grace. |

## Tests

Unit tests are co-located with their packages. Integration tests live in `internal/room/integration_test.go` and drive the actor end-to-end using in-process `NewTestConn` gateway conns:

- `TestIntegration_FullJrawfulRound` — happy path, deterministic scoring (3 players × TRUE votes → 4000 each)
- `TestIntegration_Reconnect` — rejoin with stashed token returns same PlayerID
- `TestIntegration_DemoEviction` — eviction grace + close code 4000
- `TestIntegration_ZeroSubmissionDeadline` — no inputs, timeouts advance, zero scores
- `TestIntegration_PanicRunbook` — primitive panic → S2CError + 1011 close fan-out

## v1.1 candidates (explicitly out of scope for v1)

- Prometheus metrics + dashboards
- Property tests for the scoring function
- `S2CSubmitTick` progress events on each submission
- PNG drawing format (schema is ready, validator is stubbed)
- Spectator mode
- Persistence + crash recovery (currently state is gone on crash by design)
