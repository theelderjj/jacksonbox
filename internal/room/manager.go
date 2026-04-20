package room

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/jj/trivia/internal/auth"
	"github.com/jj/trivia/internal/gateway"
	"github.com/jj/trivia/internal/proto"
)

// Manager owns the MAIN room and implements gateway.Dispatcher.
type Manager struct {
	mu    sync.RWMutex
	rooms map[string]*Room
	log   *slog.Logger
	store *auth.Store
}

// Config ties the Jrawful phases + defaults into the manager.
type Config struct {
	Log      *slog.Logger
	Store    *auth.Store
	Builder  PhaseBuilder
	Defaults Defaults
}

func NewManager(cfg Config) (*Manager, error) {
	m := &Manager{
		rooms: map[string]*Room{},
		log:   cfg.Log,
		store: cfg.Store,
	}
	for _, id := range []string{"MAIN"} {
		r, err := NewRoom(id, false, cfg.Builder, cfg.Defaults, cfg.Store, cfg.Log)
		if err != nil {
			return nil, fmt.Errorf("build %s: %w", id, err)
		}
		m.rooms[id] = r
		go r.Run()
	}
	return m, nil
}

// Rooms returns the static rooms. Test-only.
func (m *Manager) Rooms() map[string]*Room {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]*Room, len(m.rooms))
	for k, v := range m.rooms {
		out[k] = v
	}
	return out
}

// Shutdown stops all rooms. Closes all client sockets with CloseGoingAway.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.rooms {
		r.Stop()
	}
}

// --- gateway.Dispatcher implementation ---

func (m *Manager) Join(ctx context.Context, conn *gateway.Conn, p proto.JoinRoomPayload, msgID string) error {
	m.mu.RLock()
	room, ok := m.rooms[p.RoomID]
	m.mu.RUnlock()
	if !ok {
		return errors.New("unknown room_id")
	}

	reply := room.PostJoin(ctx, conn, p)
	if reply.Err != nil {
		return reply.Err
	}
	return nil
}

func (m *Manager) Route(conn *gateway.Conn, env proto.Envelope) error {
	m.mu.RLock()
	room, ok := m.rooms[conn.RoomID]
	m.mu.RUnlock()
	if !ok {
		return errors.New("conn not attached to a room")
	}
	return room.PostInput(conn, env)
}

func (m *Manager) Disconnect(conn *gateway.Conn) {
	if conn.RoomID == "" || conn.PlayerID == "" {
		return
	}
	m.mu.RLock()
	room, ok := m.rooms[conn.RoomID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	room.PostLeave(conn)
}
