package gateway

import (
	"sync"
	"time"
)

// ringSize matches §10. Deliberately small and bounded — invite-only scale,
// we're guarding against accidental resend storms not adversarial floods.
const ringSize = 128

// ringTTL: entries older than this are ignored (treated as fresh IDs again).
// This prevents a long-lived ring from suppressing legitimate reuse of a
// recycled short ID.
const ringTTL = 60 * time.Second

// Idempotency is a bounded per-player seen-ID cache. Thread-safe because
// multiple conn goroutines may race during a reconnect window.
type Idempotency struct {
	mu  sync.Mutex
	ids [ringSize]string
	ts  [ringSize]time.Time
	pos int
}

// Seen returns true if id was already recorded within the TTL. Otherwise
// records it and returns false. now is injectable for tests.
func (i *Idempotency) Seen(id string, now time.Time) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	for k := 0; k < ringSize; k++ {
		if i.ids[k] == "" {
			continue
		}
		if now.Sub(i.ts[k]) > ringTTL {
			continue
		}
		if i.ids[k] == id {
			return true
		}
	}
	i.ids[i.pos] = id
	i.ts[i.pos] = now
	i.pos = (i.pos + 1) % ringSize
	return false
}
