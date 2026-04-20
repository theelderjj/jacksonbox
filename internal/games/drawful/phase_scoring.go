package drawful

import (
	"encoding/json"

	"github.com/jj/trivia/internal/engine"
	"github.com/jj/trivia/internal/primitives"
	"github.com/jj/trivia/internal/proto"
)

// Scoring constants — §8. Tunable; kept as consts for compile-time clarity.
const (
	PointsDrawerPerCorrectVote = 1000
	PointsFakerPerFooledVote   = 500
	PointsGuesserForTrue       = 1000
)

// newScoring builds a ScoreTransform that converts the RevealResult into
// per-player deltas and applies them. Implements merge/split credit:
// when a choice has multiple AuthorIDs, the per-vote Faker payout is
// divided by len(AuthorIDs); integer division remainder drops (§8).
func newScoring(round int) engine.AnyPrimitive {
	revealKey := phaseName("reveal", round)

	return engine.Erase(primitives.NewScoreTransform("scoring",
		func(s *engine.GameState) (map[engine.PlayerID]int, []engine.Event, error) {
			reveal, _ := engine.GetResult[RevealResult](s, revealKey)
			deltas := map[engine.PlayerID]int{}

			for _, dr := range reveal.ByDrawing {
				for _, ch := range dr.Choices {
					voters := ch.Voters
					if ch.IsTrue {
						// Drawer gets points per correct vote.
						deltas[dr.AuthorID] += PointsDrawerPerCorrectVote * len(voters)
						// Guessers each get points for picking true.
						for _, v := range voters {
							deltas[v] += PointsGuesserForTrue
						}
						continue
					}
					// Fake choice. Split faker credit across all merged authors.
					n := len(ch.AuthorIDs)
					if n == 0 || len(voters) == 0 {
						continue
					}
					per := (PointsFakerPerFooledVote * len(voters)) / n
					for _, a := range ch.AuthorIDs {
						deltas[a] += per
					}
				}
			}

			// Build round_result event with post-apply totals. ScoreTransform
			// applies deltas after this closure returns, so compute totals
			// manually here for the event payload.
			totals := map[string]int{}
			for id, base := range s.Scores {
				totals[string(id)] = base + deltas[id]
			}
			// Players with no prior score but a non-zero delta need a row too.
			for id, d := range deltas {
				if _, ok := totals[string(id)]; !ok {
					totals[string(id)] = d
				}
			}
			deltasOut := map[string]int{}
			for id, d := range deltas {
				deltasOut[string(id)] = d
			}
			raw, _ := json.Marshal(proto.RoundResultPayload{
				Round:  round,
				Deltas: deltasOut,
				Scores: totals,
			})
			events := []engine.Event{{Type: proto.S2CRoundResult, Payload: raw}}
			return deltas, events, nil
		}))
}
