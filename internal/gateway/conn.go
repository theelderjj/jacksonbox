package gateway

import (
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/jj/trivia/internal/engine"
	"github.com/jj/trivia/internal/proto"
)

// writerBuffer matches §12: per-client outbound channel size. Slow clients
// drop (don't block the actor); drops are logged per-player.
const writerBuffer = 64

// Conn is the gateway's handle on a single WebSocket connection. It owns
// the reader and writer goroutines. Ownership of GameState never lives
// here — anything touching game state goes through the room actor.
type Conn struct {
	ID       string             // ephemeral connection ID (not PlayerID)
	ws       *websocket.Conn
	send     chan []byte
	closed   atomic.Bool
	closeMu  sync.Mutex
	log      *slog.Logger
	idemp    Idempotency

	// Last close metadata. Captured by Close for observability/testing.
	lastCloseCode   int
	lastCloseReason string

	// Set once by Dispatcher after a successful join_room.
	PlayerID engine.PlayerID
	RoomID   string
}

func newConn(ws *websocket.Conn, log *slog.Logger) *Conn {
	return &Conn{
		ID:   uuid.NewString(),
		ws:   ws,
		send: make(chan []byte, writerBuffer),
		log:  log,
	}
}

// Send enqueues a pre-encoded envelope. Returns false if the buffer is
// full — caller (the room actor's outbound fan-out) treats this as a
// "slow client, drop and log" signal per §12.
func (c *Conn) Send(frame []byte) bool {
	if c.closed.Load() {
		return false
	}
	select {
	case c.send <- frame:
		return true
	default:
		return false
	}
}

// SendEnvelope is a convenience; it marshals the envelope then Sends.
func (c *Conn) SendEnvelope(typ string, payload any) bool {
	frame, err := proto.EncodeEnvelope(uuid.NewString(), typ, time.Now().UnixMilli(), payload)
	if err != nil {
		c.log.Error("encode envelope", "err", err, "conn_id", c.ID)
		return false
	}
	return c.Send(frame)
}

// SendError is the canonical error reply.
func (c *Conn) SendError(code, msg, forMsgID string) {
	c.SendEnvelope(proto.S2CError, proto.ErrorPayload{Code: code, Message: msg, ForMsgID: forMsgID})
}

// Close shuts the socket with a WS close code. Idempotent. Safe when the
// underlying websocket is nil (used by in-process test conns — see
// NewTestConn), in which case only the send channel is closed and the
// close code is surfaced via LastCloseCode.
func (c *Conn) Close(code int, reason string) {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	if c.closed.Swap(true) {
		return
	}
	c.lastCloseCode = code
	c.lastCloseReason = reason
	if c.ws != nil {
		deadline := time.Now().Add(500 * time.Millisecond)
		_ = c.ws.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(code, reason),
			deadline,
		)
		_ = c.ws.Close()
	}
	close(c.send)
}

// IsClosed reports whether Close has been called.
func (c *Conn) IsClosed() bool { return c.closed.Load() }

// LastCloseCode returns the last close code passed to Close, or zero if
// the connection is still open.
func (c *Conn) LastCloseCode() int {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()
	return c.lastCloseCode
}

// CheckIdempotent returns true if the client msg id was already processed
// within the TTL window. Records the id on miss.
func (c *Conn) CheckIdempotent(id string, now time.Time) bool {
	return c.idemp.Seen(id, now)
}

// NewTestConn creates a Conn for in-process tests — no real websocket, no
// network I/O. Outbox() taps the outbound frame channel so tests can
// assert what the room actor broadcast. Not intended for production use.
func NewTestConn(log *slog.Logger) *Conn {
	return &Conn{
		ID:   uuid.NewString(),
		ws:   nil,
		send: make(chan []byte, writerBuffer),
		log:  log,
	}
}

// Outbox returns the channel of outgoing envelope frames. Test-only.
func (c *Conn) Outbox() <-chan []byte { return c.send }
