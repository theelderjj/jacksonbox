package room

import (
	"github.com/jj/trivia/internal/engine"
	"github.com/jj/trivia/internal/gateway"
	"github.com/jj/trivia/internal/proto"
)

// actorMsg is a tagged union for the room actor inbox. Keeps the main
// select small (4-5 cases) while supporting the full command set.
type actorMsg struct {
	kind msgKind

	// common
	conn     *gateway.Conn
	playerID engine.PlayerID

	// kindJoin
	joinPayload proto.JoinRoomPayload
	reply       chan joinReply

	// kindInput
	env proto.Envelope

	// kindEvict — demo eviction broadcast
	evictReason string

	// kindSnapshot — test-only actor-safe state reader
	snapshotReply chan stateSnapshot
}

// stateSnapshot is a shallow copy of GameState for read-only test assertions.
// The fields are copies, but map values alias the originals — tests must not
// mutate them. Produced exclusively by the actor to avoid data races.
type stateSnapshot struct {
	Status         Status
	RoomMode       RoomMode
	PhaseName      string
	PhaseIdx       int
	Round          int
	SelectedGameID string
	PhaseData      map[string]engine.PhaseResult
	Scores         map[engine.PlayerID]int
	Players        map[engine.PlayerID]engine.Player
}

type msgKind int

const (
	kindJoin msgKind = iota
	kindLeave
	kindInput
	kindTick
	kindEvict
	kindEvictFinalize
	kindStartGame
	kindSnapshot
)

type joinReply struct {
	PlayerID engine.PlayerID
	Token    string
	Err      error
}
