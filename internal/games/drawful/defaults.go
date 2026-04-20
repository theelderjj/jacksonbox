package drawful

import (
	"encoding/json"
	"math/rand"
	"strings"
	"time"

	"github.com/jj/trivia/internal/engine"
	"github.com/jj/trivia/internal/primitives"
	"github.com/jj/trivia/internal/proto"
)

// DisconnectDefaults returns the per-phase default-substitution function
// used by the room actor after §11's grace period lapses for a disconnected
// player. Returns (input, true) when a substitution is appropriate.
//
// Spec defaults:
//   - drawing       → blank canvas (empty strokes)
//   - fake prompt   → "..." (merged into a single choice at reveal)
//   - vote          → uniform random among eligible non-self, non-self-fake choices
//
// Called from the actor goroutine; safe to read state directly.
func DisconnectDefaults(rng *rand.Rand) func(state *engine.GameState, phase string, pid engine.PlayerID) (engine.Input, bool) {
	return func(state *engine.GameState, phase string, pid engine.PlayerID) (engine.Input, bool) {
		switch {
		case strings.HasPrefix(phase, "drawing_submit"):
			payload, _ := json.Marshal(proto.SubmitDrawingPayload{
				Format: "strokes",
				Data:   `{"strokes":[]}`,
			})
			return engine.Input{
				PlayerID:  pid,
				Phase:     phase,
				Type:      proto.C2SSubmitDraw,
				Value:     payload,
				Timestamp: time.Now(),
			}, true

		case strings.HasPrefix(phase, "fake_prompt_submit"):
			// We need a target drawing — pick the first one this player owes.
			round := roundFromPhase(phase)
			drawings, ok := engine.GetResult[primitives.CollectAllResult[Drawing]](state, phaseName("drawing_submit", round))
			if !ok {
				return engine.Input{}, false
			}
			for _, d := range drawings.Items {
				if d.AuthorID == pid {
					continue
				}
				payload, _ := json.Marshal(proto.SubmitFakePromptPayload{
					DrawingID: string(d.ID),
					Text:      defaultFakeText,
				})
				return engine.Input{
					PlayerID:  pid,
					Phase:     phase,
					Type:      proto.C2SSubmitFake,
					Value:     payload,
					Timestamp: time.Now(),
				}, true
			}
			return engine.Input{}, false

		case strings.HasPrefix(phase, "voting"):
			round := roundFromPhase(phase)
			drawings, ok := engine.GetResult[primitives.CollectAllResult[Drawing]](state, phaseName("drawing_submit", round))
			if !ok {
				return engine.Input{}, false
			}
			fakes, _ := engine.GetResult[FakeCollectionResult](state, phaseName("fake_prompt_submit", round))
			for _, d := range drawings.Items {
				if d.AuthorID == pid {
					continue
				}
				// Build eligible choice pool: TRUE plus fakes not authored by pid.
				choices := []string{"TRUE"}
				for _, f := range fakes.ByDrawing[d.ID] {
					if f.AuthorID == pid {
						continue
					}
					choices = append(choices, string(f.ID))
				}
				pick := choices[rng.Intn(len(choices))]
				payload, _ := json.Marshal(proto.SubmitVotePayload{
					DrawingID: string(d.ID),
					ChoiceID:  pick,
				})
				return engine.Input{
					PlayerID:  pid,
					Phase:     phase,
					Type:      proto.C2SSubmitVote,
					Value:     payload,
					Timestamp: time.Now(),
				}, true
			}
			return engine.Input{}, false
		}
		return engine.Input{}, false
	}
}

// roundFromPhase parses "..._r3" -> 3.
func roundFromPhase(phase string) int {
	i := strings.LastIndex(phase, "_r")
	if i < 0 {
		return 1
	}
	n := 0
	for _, c := range phase[i+2:] {
		if c < '0' || c > '9' {
			return 1
		}
		n = n*10 + int(c-'0')
	}
	if n == 0 {
		return 1
	}
	return n
}
