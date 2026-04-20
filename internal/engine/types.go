// Package engine defines the phase runner and the shared primitive contract.
// All game logic plugs in via primitives. See docs/DESIGN.md §2, §5, §6.
package engine

import (
	"encoding/json"
	"time"
)

// PlayerID is a server-generated UUID. Never trust a client-supplied value.
type PlayerID string

// Event is a server-originated broadcast payload. It's opaque to the engine;
// primitives build it, the room actor ships it via PhaseContext.Broadcast.
type Event struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// Input is a validated client message delivered to a primitive's Handle.
// Shape-level checks happen in the gateway; semantic checks (phase match,
// eligibility) happen in the room actor before Handle is called.
type Input struct {
	PlayerID    PlayerID        // server-assigned
	Phase       string          // target phase name; rejected on mismatch
	Type        string          // message type, e.g. "submit_drawing"
	Value       json.RawMessage // primitive decodes per its own schema
	ClientMsgID string          // idempotency key
	Timestamp   time.Time       // server-assigned on receipt
}

// Decision is what a primitive returns from Start/Handle/Timeout.
// Results are typed via generics; PhaseResult boxes them for PhaseData storage.
type Decision[R any] struct {
	AdvancePhase bool
	Broadcast    []Event
	Result       R
}

// Primitive is the composable unit of phase behavior. See §2.
// Each concrete primitive declares R = its result type so downstream phases
// can read results without `any`-casting.
type Primitive[R any] interface {
	Start(ctx *PhaseContext) (Decision[R], error)
	Handle(ctx *PhaseContext, in Input) (Decision[R], error)
	Timeout(ctx *PhaseContext) (Decision[R], error)
	Name() string
}

// PhaseResult is a sealed marker: every primitive's Result type implements it.
// "Sealed" here means the isPhaseResult() method is unexported and thus can't
// be satisfied from outside this module — the compiler is the gatekeeper.
type PhaseResult interface {
	isPhaseResult()
}

// StoredResult wraps a typed primitive result for PhaseData storage.
// Downstream phases retrieve it through GetResult[T].
type StoredResult[R any] struct {
	Phase     string
	Primitive string
	Value     R
}

func (StoredResult[R]) isPhaseResult() {}

// GetResult retrieves a previously-stored result by phase name.
// Panics at startup (DependsOn validation) ensure this is safe at runtime.
func GetResult[R any](state *GameState, phase string) (R, bool) {
	var zero R
	raw, ok := state.PhaseData[phase]
	if !ok {
		return zero, false
	}
	typed, ok := raw.(StoredResult[R])
	if !ok {
		return zero, false
	}
	return typed.Value, true
}

// PutResult stores a typed result under the current phase name.
// Called by the room actor after a primitive advances.
func PutResult[R any](state *GameState, phase, primitive string, v R) {
	state.PhaseData[phase] = StoredResult[R]{Phase: phase, Primitive: primitive, Value: v}
}

// Phase describes one step in a game. See §5.
type Phase struct {
	Name        string
	Primitive   string          // registry key; must resolve at startup
	Duration    time.Duration   // hard cap; 0 = no timeout (rare)
	MinDuration time.Duration   // optional UX floor
	Config      json.RawMessage // primitive-specific configuration
	DependsOn   []string        // prior phase names whose Results must exist
}

// GameSettings are lobby-configurable knobs that game modes may read when
// building phases or rendering choices. Values are normalized by the room
// actor before a game starts.
type GameSettings struct {
	RoundCount         int
	GeneratedFakeCount int
	DrawingSeconds     int
	FakePromptSeconds  int
	VotingSeconds      int
}

// Player tracks connection/identity. SessionToken is opaque; never sent in events.
type Player struct {
	ID           PlayerID
	SessionToken string
	Name         string
	Connected    bool
	LastSeen     time.Time
	Ready        bool
}

// GameState is the authoritative room state. Exclusively owned by the
// room actor goroutine — no locks, no sharing. See §4, §12.
type GameState struct {
	RoomID    string
	Players   map[PlayerID]*Player
	PhaseIdx  int
	Round     int
	Settings  GameSettings
	PhaseData map[string]PhaseResult
	Scores    map[PlayerID]int
	StartedAt time.Time
}

// ActivePlayers returns players currently connected. Primitives that need to
// know "who must submit" use this.
func (s *GameState) ActivePlayers() []PlayerID {
	out := make([]PlayerID, 0, len(s.Players))
	for id, p := range s.Players {
		if p.Connected {
			out = append(out, id)
		}
	}
	return out
}
