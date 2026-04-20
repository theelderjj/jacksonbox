// Package gateway is the WS boundary. Everything below it assumes shape-
// level correctness; see docs/DESIGN.md §3 trust boundary.
package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/jj/trivia/internal/proto"
)

// Dispatcher is implemented by the room manager. Gateway knows nothing
// about game state; it only parses, validates, and routes.
type Dispatcher interface {
	// Join attempts to attach a connection to a room. Responds on conn with
	// join_ack or error. Blocking (fast) call; must not touch game state
	// directly — under the hood this sends to the room actor and waits.
	Join(ctx context.Context, conn *Conn, payload proto.JoinRoomPayload, msgID string) error

	// Route forwards a validated non-join message to the room actor.
	// Non-blocking: returns after enqueueing.
	Route(conn *Conn, env proto.Envelope) error

	// Disconnect is called when the reader terminates. The manager marks
	// the player disconnected and may arm the grace-period default logic.
	Disconnect(conn *Conn)
}

// Config tunes timeouts. Defaults are fine for local dev.
type Config struct {
	ReadDeadline  time.Duration // pong-based liveness; default 90s
	WriteDeadline time.Duration // per-frame; default 10s
	PingInterval  time.Duration // server-initiated ping; default 30s
}

// DefaultConfig returns production-sane defaults.
func DefaultConfig() Config {
	return Config{
		ReadDeadline:  90 * time.Second,
		WriteDeadline: 10 * time.Second,
		PingInterval:  30 * time.Second,
	}
}

// Gateway wires WS upgrade + read/write loops.
type Gateway struct {
	dispatcher Dispatcher
	upgrader   websocket.Upgrader
	log        *slog.Logger
	cfg        Config
}

func New(d Dispatcher, log *slog.Logger, cfg Config) *Gateway {
	return &Gateway{
		dispatcher: d,
		log:        log,
		cfg:        cfg,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin:     func(*http.Request) bool { return true }, // invite-only deployment
		},
	}
}

// Handle is the HTTP handler. Mount at /ws.
func (g *Gateway) Handle(w http.ResponseWriter, r *http.Request) {
	ws, err := g.upgrader.Upgrade(w, r, nil)
	if err != nil {
		g.log.Error("ws upgrade failed", "err", err)
		return
	}
	ws.SetReadLimit(int64(proto.MaxFrameBytes))

	conn := newConn(ws, g.log)
	g.log.Info("ws connected", "conn_id", conn.ID, "remote", r.RemoteAddr)

	go g.writeLoop(conn)
	g.readLoop(r.Context(), conn)
}

func (g *Gateway) readLoop(ctx context.Context, conn *Conn) {
	defer g.dispatcher.Disconnect(conn)
	defer conn.Close(websocket.CloseNormalClosure, "")

	_ = conn.ws.SetReadDeadline(time.Now().Add(g.cfg.ReadDeadline))
	conn.ws.SetPongHandler(func(string) error {
		return conn.ws.SetReadDeadline(time.Now().Add(g.cfg.ReadDeadline))
	})

	for {
		_, raw, err := conn.ws.ReadMessage()
		if err != nil {
			if !websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				g.log.Info("ws read ended", "conn_id", conn.ID, "err", err)
			}
			return
		}
		if ctx.Err() != nil {
			return
		}
		g.dispatch(ctx, conn, raw)
	}
}

func (g *Gateway) dispatch(ctx context.Context, conn *Conn, raw []byte) {
	env, err := proto.DecodeEnvelope(raw)
	if err != nil {
		conn.SendError(proto.ErrBadPayload, err.Error(), "")
		return
	}
	if env.V != proto.ProtocolVersion {
		conn.SendError(proto.ErrBadPayload, "protocol version mismatch", env.ID)
		conn.Close(proto.CloseVersionBump, "version mismatch")
		return
	}
	if _, ok := proto.KnownC2S[env.Type]; !ok {
		conn.SendError(proto.ErrUnknownType, "unknown type: "+env.Type, env.ID)
		return
	}
	if err := proto.ValidatePayload(env.Type, env.Payload); err != nil {
		conn.SendError(proto.ErrBadPayload, err.Error(), env.ID)
		return
	}
	// Idempotency: only for state-mutating messages. ping is exempt.
	if env.Type != proto.C2SPing {
		if conn.CheckIdempotent(env.ID, time.Now()) {
			// Duplicate is not an error from the client's view; silently drop.
			return
		}
	}

	// ping is handled here, not by the room.
	if env.Type == proto.C2SPing {
		conn.SendEnvelope(proto.S2CPong, struct{}{})
		return
	}

	// join_room gets a special entry point because the dispatcher must
	// assign the player to a room before subsequent messages can route.
	if env.Type == proto.C2SJoinRoom {
		var p proto.JoinRoomPayload
		if err := json.Unmarshal(env.Payload, &p); err != nil {
			conn.SendError(proto.ErrBadPayload, err.Error(), env.ID)
			return
		}
		if err := g.dispatcher.Join(ctx, conn, p, env.ID); err != nil {
			conn.SendError(proto.ErrNotAllowed, err.Error(), env.ID)
		}
		return
	}

	if conn.PlayerID == "" {
		conn.SendError(proto.ErrNotAllowed, "must join_room first", env.ID)
		return
	}
	if err := g.dispatcher.Route(conn, env); err != nil {
		// Transient errors (inbox full) surface as server_error; routing
		// decisions about phase / eligibility return their own codes from
		// inside the actor.
		if errors.Is(err, ErrInboxFull) {
			conn.SendError(proto.ErrServer, "room busy, retry", env.ID)
		} else {
			conn.SendError(proto.ErrServer, err.Error(), env.ID)
		}
	}
}

func (g *Gateway) writeLoop(conn *Conn) {
	pingTicker := time.NewTicker(g.cfg.PingInterval)
	defer pingTicker.Stop()

	for {
		select {
		case frame, ok := <-conn.send:
			if !ok {
				return
			}
			_ = conn.ws.SetWriteDeadline(time.Now().Add(g.cfg.WriteDeadline))
			if err := conn.ws.WriteMessage(websocket.TextMessage, frame); err != nil {
				g.log.Info("ws write failed", "conn_id", conn.ID, "err", err)
				conn.Close(proto.CloseAbnormal, "write error")
				return
			}
		case <-pingTicker.C:
			_ = conn.ws.SetWriteDeadline(time.Now().Add(g.cfg.WriteDeadline))
			if err := conn.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ErrInboxFull is surfaced when the room actor's channel is saturated.
var ErrInboxFull = errors.New("gateway: inbox full")
