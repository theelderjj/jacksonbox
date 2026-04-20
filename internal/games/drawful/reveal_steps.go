package drawful

import (
	"sort"

	"github.com/jj/trivia/internal/engine"
	"github.com/jj/trivia/internal/proto"
	"github.com/jj/trivia/internal/reveal"
)

// BuildRevealSteps constructs the per-round reveal animation queue for the
// room actor to emit during leaderboard_rN. One "eliminate" step per fake
// choice (greying it out client-side), then one "final" step per drawing
// carrying the per-drawing point deltas and awards. The leader advances the
// queue manually one click at a time, so we intentionally omit timing-only
// gap steps between drawings.
//
// Rationale for doing this here rather than inside the reveal primitive:
// the animation is timing-driven (needs RevealStepInterval pacing) and
// pause-aware, both of which live in the room actor. The primitive stays
// a pure Aggregate — it emits the authoritative RevealResult once and is
// done. This function is a read-only view over the stored result.
//
// Point math mirrors phase_scoring.go — keep it in lock-step if the
// scoring constants ever change. Keeping it separated (instead of reusing
// scoring's per-player totals) lets us attribute points per drawing, which
// scoring's rollup no longer distinguishes.
func BuildRevealSteps(state *engine.GameState, round int) []reveal.Step {
	revealKey := phaseName("reveal", round)
	// Named `rr` (not `reveal`) to avoid shadowing the imported `reveal`
	// package — we need both the stored result and the Step/StepHold
	// symbols in this function body.
	rr, ok := engine.GetResult[RevealResult](state, revealKey)
	if !ok {
		return nil
	}

	// Deterministic drawing order: sort by drawing ID so the animation
	// replays identically if tests ever snapshot the queue. Ranging over
	// the map directly would depend on Go's random map iteration.
	ids := make([]DrawingID, 0, len(rr.ByDrawing))
	for id := range rr.ByDrawing {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	steps := make([]reveal.Step, 0, len(ids)*4)
	for _, id := range ids {
		dr := rr.ByDrawing[id]

		// Split choices into fakes and the single true choice. We always
		// eliminate fakes first so the "truth reveal" is the terminal beat.
		fakes := make([]Choice, 0, len(dr.Choices))
		for _, ch := range dr.Choices {
			if !ch.IsTrue {
				fakes = append(fakes, ch)
			}
		}
		// Stable ordering for fakes: by ChoiceID. Deterministic for tests.
		sort.Slice(fakes, func(i, j int) bool { return fakes[i].ChoiceID < fakes[j].ChoiceID })

		// Per-drawing deltas + awards. Mirrors phase_scoring math but scoped
		// to this drawing only. Integer-division remainder behavior matches.
		deltas := map[string]int{}
		awards := make([]proto.RevealAward, 0, 4)
		for _, ch := range dr.Choices {
			voters := ch.Voters
			if ch.IsTrue {
				if len(voters) > 0 {
					pts := PointsDrawerPerCorrectVote * len(voters)
					deltas[string(dr.AuthorID)] += pts
					awards = append(awards, proto.RevealAward{
						PlayerID: string(dr.AuthorID),
						Points:   pts,
						Reason:   "drawer_truth",
					})
				}
				for _, v := range voters {
					deltas[string(v)] += PointsGuesserForTrue
					awards = append(awards, proto.RevealAward{
						PlayerID: string(v),
						Points:   PointsGuesserForTrue,
						Reason:   "guesser_truth",
					})
				}
				continue
			}
			// Fake choice. Split faker credit across merged authors.
			n := len(ch.AuthorIDs)
			if n == 0 || len(voters) == 0 {
				continue
			}
			per := (PointsFakerPerFooledVote * len(voters)) / n
			for _, a := range ch.AuthorIDs {
				deltas[string(a)] += per
				awards = append(awards, proto.RevealAward{
					PlayerID: string(a),
					Points:   per,
					Reason:   "faker_fool",
				})
			}
		}

		// Emit one eliminate step per fake, then the final step.
		for _, f := range fakes {
			steps = append(steps, reveal.Step{
				Kind:      "eliminate",
				DrawingID: string(id),
				ChoiceID:  f.ChoiceID,
			})
		}
		steps = append(steps, reveal.Step{
			Kind:      "final",
			DrawingID: string(id),
			Deltas:    deltas,
			Awards:    awards,
		})
	}
	return steps
}
