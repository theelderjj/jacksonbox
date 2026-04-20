package engine

import (
	"time"
)

// PhaseContext is handed to every primitive call. Valid only inside the
// owning room actor's goroutine — do not stash and use later.
type PhaseContext struct {
	State     *GameState
	Phase     Phase
	Deadline  time.Time
	Now       func() time.Time // injectable for tests; defaults to time.Now
	Broadcast func(Event)      // routes through the actor's outbound channel
}

// TimeLeft reports remaining budget for the current phase. Negative if past.
func (c *PhaseContext) TimeLeft() time.Duration {
	if c.Phase.Duration == 0 {
		return 1<<62 // effectively infinite
	}
	return c.Deadline.Sub(c.Now())
}
