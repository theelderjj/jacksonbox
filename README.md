# jacksonbox

Jrawful-style realtime drawing-and-bluffing game. Go backend (actor model, single-writer goroutine), Vite/React/TS client. Invite-only, single room, ≤20 players. Full architecture in [`docs/DESIGN.md`](docs/DESIGN.md).

## Demo

Add a short gameplay GIF or a 30-60 second clip here before sharing publicly.

Technical highlights for reviewers:
- Actor-model room engine keeps all game state mutations single-threaded.
- Typed wire protocol is mirrored between Go and TypeScript to catch drift early.
- Integration tests exercise real room actors and gateway connections.
- Playwright E2E runs full round-trip gameplay over real WebSockets.
- Deterministic scoring and reducer tests lock in game behavior.

## Current limitations

- Single room only (invite-only, up to 20 players).
- In-memory state only; restarting server resets all rooms/sessions.
- No spectator mode yet.
- Metrics/dashboarding not shipped.
- PNG drawing format and submit progress events are still backlog items.

## Quick start

### Backend
```bash
go test ./cmd/... ./internal/...
go run ./cmd/server         # :8080
```

### Client
```bash
cd client
npm install                 # first time only
npm run dev                 # Vite on :5173 with /ws proxy to :8080
```

Open two browser tabs on the dev server, join the same room with different names, hit ready, play. No Docker and no persistence — restart the server and the world resets.

## Testing

### Run everything
```bash
make verify
```

### Backend package examples
```bash
go test ./internal/games/drawful
go test ./internal/primitives
go test ./internal/room
```

### Just one test (with verbose output + race detector)
```bash
go test -race -v -run TestIntegration_FullJrawfulRound ./internal/room
```

`npm run typecheck` exercises the strict TS compiler against the whole source tree (catches most lock-step-drift bugs with the backend protocol).

### Client — unit tests (Vitest + jsdom)

```bash
cd client
npm install                     # first time only — installs Vitest + Playwright + Testing Library
npm test                        # single-shot; CI mode
npm run test:watch              # watch mode during development
npm run test:ui                 # Vitest UI in browser
```

Unit suites live next to the source under `src/**/*.test.ts`. They do not spin up a WebSocket — the pieces that matter (pure `reduceState` reducer, pure `parseWireEvent` router, `shuffleStable` util) are extracted and tested in isolation.

| File | What it covers |
|---|---|
| `src/util/shuffle.test.ts` | `shuffleStable` determinism, permutation invariant, no input mutation, varied-seed spread, empty/singleton edges. |
| `src/store.test.ts` | Every `WireEvent` case of `reduceState`: open/closed, join_ack, phase_change reset boundary, prompt_issued self-filter, drawings/voting_choices indexing, reveal appending, round_result, game_end, error, room_evicting, submit_tick no-op, purity. |
| `src/ws.test.ts` | `parseWireEvent` routing: non-string input, malformed JSON, pong drop, unknown type drop, all 11 known S2C types → correct WireEvent variant, payload pass-through. |

### Client — end-to-end (Playwright)

```bash
cd client
npx playwright install          # first time only — downloads Chromium
npm run e2e                     # headless
npm run e2e:ui                  # Playwright UI (step through interactively)
```

The E2E config (`playwright.config.ts`) brings up the Go backend (`go run ./cmd/server`) **and** the Vite dev server before any test, then tears them down after. No mocks — real WebSocket, real frame encoding.

| File | What it proves |
|---|---|
| `e2e/full-round.spec.ts` | Mirror of `TestIntegration_FullJrawfulRound`: three browser contexts join MAIN, ready up, draw one stroke each, submit fakes, vote, and reach `Final scores`. Catches drift between the TS client and the Go server that unit tests can't. |

## What's tested where

Unit tests are colocated with the package under test.

| Package | File | What it covers |
|---|---|---|
| `internal/engine` | `startup_test.go` | Phase DAG validation: forward deps, missing deps, duplicate names. |
| `internal/primitives` | `collectall_test.go` | CollectAll: happy path, timeout with partial, duplicates rejected, invalid rejected, short-circuit on all-submitted, empty-eligible advance, eligibility filtering. |
| `internal/primitives` | `aggregate_test.go` | Aggregate + ScoreTransform: happy path, error propagation, inputs rejected, delta application, no-negative clamp. |
| `internal/proto` | `envelope_test.go` | Envelope decode: happy, missing fields, size cap, submit_drawing validation, name cleaner rules. |
| `internal/gateway` | `idempotency_test.go` | Ring buffer: unique IDs pass, duplicates rejected, TTL expiry, ring eviction. |
| `internal/games/drawful` | `reveal_test.go` | Collision merge groups fakes by normalized text. |
| `internal/games/drawful` | `scoring_test.go` | Canonical scoring: drawer+guesser for TRUE, faker fooled, merged split credit, remainder drops on integer divide, zero-vote round, drawer+faker double role. |

### Integration tests — `internal/room/integration_test.go`

These drive the full actor + phase runner end-to-end using in-process gateway connections (`gateway.NewTestConn`). No HTTP, no real sockets. State is observed via the test-only `snapshotForTest()` path which posts a `kindSnapshot` message into the actor inbox and reads a read-only copy out.

| Test | What it proves |
|---|---|
| `TestIntegration_FullJrawfulRound` | Full round happy path: 3 players × all-TRUE votes yields deterministic 4000/4000/4000 final score. |
| `TestIntegration_Reconnect` | Rejoining with a stashed `session_token` returns the same `PlayerID` and the JoinAck carries the same ID. 60s disconnect grace honored. |
| `TestIntegration_ZeroSubmissionDeadline` | With nobody submitting, every phase times out cleanly and the game emits `S2CGameEnd` with zero scores — no deadlocks. |
| `TestIntegration_PanicRunbook` | A primitive that panics triggers the actor's `defer recoverPanic` path: every conn receives `S2CError{code: room_crashed}`, closes with `1011` (CloseAbnormal), room enters `StatusClosed`. |

Helpers in the same file: `testPhaseBuilder(drawDur, fakeDur, voteDur)` returns a 1-round Jrawful phase list with short timers — v1's real `drawful.BuildPhases` enforces 3 rounds minimum which is too slow for test feedback.

## Knobs tests flip

`DisconnectGrace` in `internal/room` is a `var`, not a `const`, specifically so tests can shrink it. Do not mutate it in production code; call sites assume production-grade timing.

## Not yet shipped

- Prometheus metrics + dashboards
- Property tests for the scoring function
- `S2CSubmitTick` progress events per submission
- PNG drawing format (schema exists, validator stubbed)
- Spectator mode
- Persistence / crash recovery

See `docs/DESIGN.md` for the full v1.1 backlog.
