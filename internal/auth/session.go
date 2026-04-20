// Package auth provides the lightweight session-token scheme from §11.
// No refresh tokens, no device binding — "trust the invite ring."
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/jj/trivia/internal/engine"
)

// TokenTTL matches §11.
const TokenTTL = 30 * time.Minute

// Store maps SessionToken -> PlayerID with a last-activity clock. The
// room manager owns one of these per process. Locking is coarse (one
// mutex) — the store is touched on connect / reconnect / heartbeat, not
// in the hot path.
type Store struct {
	mu      sync.Mutex
	byToken map[string]*entry
}

type entry struct {
	PlayerID engine.PlayerID
	Name     string
	LastSeen time.Time
}

func NewStore() *Store {
	return &Store{byToken: map[string]*entry{}}
}

// Issue mints a new player identity: a fresh PlayerID + session token.
// Returns (playerID, token).
func (s *Store) Issue(name string, now time.Time) (engine.PlayerID, string) {
	id := engine.PlayerID(uuid.NewString())
	tok := newToken()
	s.mu.Lock()
	s.byToken[tok] = &entry{PlayerID: id, Name: name, LastSeen: now}
	s.mu.Unlock()
	return id, tok
}

// Resume resolves a token if non-expired. Refreshes LastSeen on hit.
func (s *Store) Resume(token string, now time.Time) (engine.PlayerID, string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.byToken[token]
	if !ok {
		return "", "", false
	}
	if now.Sub(e.LastSeen) > TokenTTL {
		delete(s.byToken, token)
		return "", "", false
	}
	e.LastSeen = now
	return e.PlayerID, e.Name, true
}

// Touch bumps LastSeen for active players so they don't expire mid-game.
func (s *Store) Touch(token string, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.byToken[token]; ok {
		e.LastSeen = now
	}
}

// Forget removes a token (e.g., on explicit leave).
func (s *Store) Forget(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.byToken, token)
}

// Sweep evicts expired tokens. Called periodically by the manager.
func (s *Store) Sweep(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for tok, e := range s.byToken {
		if now.Sub(e.LastSeen) > TokenTTL {
			delete(s.byToken, tok)
			n++
		}
	}
	return n
}

func newToken() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("auth: crypto/rand failure: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}
