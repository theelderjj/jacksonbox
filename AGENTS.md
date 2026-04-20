# AGENTS.md

Orientation for future Codex sessions in this repo.

## What this is

Jrawful-style realtime drawing-and-bluffing game. Go backend (actor model) + Vite/React/TS client. Single room, ≤20 players, invite-only. Full design in `docs/DESIGN.md`.

## Build + run

```bash
# Backend
go test ./...
go run ./cmd/server     # default :8080

# Client
cd client
npm install             # first time only
npm run dev             # Vite dev server with /ws proxy to :8080
```

No Docker, no CI yet. Server is stateless-in-memory — restart resets everything.

## Where logic lives

- **Actor / state machine:** `internal/room/room.go`. Single goroutine owns `GameState`. All mutations here.
- **Phase primitives:** `internal/primitives/*.go` — generic (`CollectAll[R]`, `Aggregate[R]`, etc.).
- **Jrawful game mode:** `internal/games/drawful/`. Registers primitives, builds phase sequence, defines scoring.
- **Wire protocol:** `internal/proto/messages.go` (server) and `client/src/proto.ts` (client). Keep them in lock-step.
- **WebSocket gateway:** `internal/gateway/`. Frame validation, idempotency ring buffer, close-code plumbing.

## Testing conventions

- Unit tests colocated with package.
- Integration tests in `internal/room/integration_test.go` drive real actor + fake conns (`gateway.NewTestConn`). Read outbound frames via `conn.Outbox()`. Read actor state via `room.snapshotForTest()`.
- `DemoEvictionGrace` / `DisconnectGrace` are `var` so tests can shrink them. **Don't** mutate in production code.

## Editing the protocol

Any new `S2C*` or `C2S*` constant needs four coordinated edits:

1. `internal/proto/messages.go` — constant + payload type.
2. `internal/proto/validate.go` — add to `KnownC2S` if it's client→server.
3. `client/src/proto.ts` — mirror constant + type.
4. `client/src/ws.ts` — route the message into a `WireEvent` variant.
5. (often) `client/src/store.ts` — reduce it.

## Gotchas

- Unused imports break the TS build (`noUnusedLocals`). `import type` an unused type and TS will complain.
- Client stores tokens in `sessionStorage` (dies with tab). `localStorage` would outlive the token's server-side TTL.
- Canvas normalized coords are 0..1000. Don't leak canvas pixel size into server-side validation.
- Voting never exposes `is_true` to the user — even though the payload carries it. Keep that in mind when touching `Voting.tsx`.

## Not yet shipped (v1.1)

Metrics, property tests, progress ticks, PNG drawing format, spectator mode. See `docs/DESIGN.md` for the full list.
