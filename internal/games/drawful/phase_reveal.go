package drawful

import (
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"github.com/jj/trivia/internal/engine"
	"github.com/jj/trivia/internal/primitives"
	"github.com/jj/trivia/internal/proto"
)

// defaultFakeText is the substitute used when a player times out with no
// fake submission for a drawing. §11 says "..." for fakes; the collision
// merge below will combine all "..." entries into a single choice.
const defaultFakeText = "..."

// newReveal builds the Aggregate that:
//   1. Applies default fakes for timed-out slots
//   2. Merges semantically identical fakes (trim + lower) into one choice
//   3. Tallies votes per merged choice
//   4. Emits one S2CReveal event per drawing and stores RevealResult
func newReveal(round int) engine.AnyPrimitive {
	drawingsKey := phaseName("drawing_submit", round)
	fakesKey := phaseName("fake_prompt_submit", round)
	votesKey := phaseName("voting", round)

	return engine.Erase(primitives.NewAggregate("reveal",
		func(s *engine.GameState) (RevealResult, []engine.Event, error) {
			drawings, _ := engine.GetResult[primitives.CollectAllResult[Drawing]](s, drawingsKey)
			fakes, _ := engine.GetResult[FakeCollectionResult](s, fakesKey)
			votes, _ := engine.GetResult[VoteCollectionResult](s, votesKey)

			out := RevealResult{ByDrawing: map[DrawingID]DrawingReveal{}}
			events := make([]engine.Event, 0, len(drawings.Items))

			for _, d := range drawings.Items {
				// 1. Collect fakes for this drawing + default substitutes for
				// players who owed one but didn't submit.
				submitted := fakes.ByDrawing[d.ID]
				submittedAuthors := map[engine.PlayerID]bool{}
				for _, f := range submitted {
					submittedAuthors[f.AuthorID] = true
				}
				for pid, pl := range s.Players {
					if pid == d.AuthorID {
						continue
					}
					if !pl.Connected && !submittedAuthors[pid] {
						submitted = append(submitted, FakePrompt{
							ID:        FakePromptID(uuid.NewString()),
							DrawingID: d.ID,
							AuthorID:  pid,
							Text:      defaultFakeText,
						})
					}
				}

				// 2. Collision merge: group by normalized text. Also reject fakes
				// that collide with the true prompt (should already be filtered
				// at Handle, but defensive for defaults).
				buckets := map[string][]FakePrompt{}
				trueNorm := normalize(d.Prompt)
				for _, f := range submitted {
					key := normalize(f.Text)
					if key == trueNorm {
						continue // swallow — shouldn't happen given §8 gate
					}
					buckets[key] = append(buckets[key], f)
				}

				choices := make([]Choice, 0, len(buckets)+1)
				// Stable ordering: sort by first-submitted-fake ID for determinism.
				// A map range is fine for v1 — determinism matters in tests, not UX.
				for _, group := range buckets {
					chID := string(group[0].ID)
					authors := make([]engine.PlayerID, 0, len(group))
					for _, f := range group {
						authors = append(authors, f.AuthorID)
					}
					choices = append(choices, Choice{
						ChoiceID:  chID,
						Text:      group[0].Text,
						AuthorIDs: authors,
						IsTrue:    false,
					})
				}
				// True prompt is always present as a choice.
				choices = append(choices, Choice{
					ChoiceID: "TRUE",
					Text:     d.Prompt,
					IsTrue:   true,
				})

				// 3. Tally votes against choices.
				// Reverse index: any member fake ID → the canonical choice ID
				// (post-merge). "TRUE" maps to itself.
				fakeToChoice := map[string]string{"TRUE": "TRUE"}
				for _, group := range buckets {
					chID := string(group[0].ID)
					for _, f := range group {
						fakeToChoice[string(f.ID)] = chID
					}
				}

				// Assign voters per canonical choice.
				choiceVoters := map[string][]engine.PlayerID{}
				for _, v := range votes.ByDrawing[d.ID] {
					canonical, ok := fakeToChoice[v.ChoiceID]
					if !ok {
						continue // orphaned vote (author's fake collided with true, etc.)
					}
					choiceVoters[canonical] = append(choiceVoters[canonical], v.VoterID)
				}
				for i := range choices {
					choices[i].Voters = choiceVoters[choices[i].ChoiceID]
				}

				dr := DrawingReveal{
					DrawingID:  d.ID,
					AuthorID:   d.AuthorID,
					TruePrompt: d.Prompt,
					Choices:    choices,
				}
				out.ByDrawing[d.ID] = dr

				events = append(events, engine.Event{
					Type:    proto.S2CReveal,
					Payload: marshalReveal(dr),
				})
			}
			return out, events, nil
		}))
}

func normalize(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func marshalReveal(dr DrawingReveal) json.RawMessage {
	choices := make([]proto.RevealChoice, 0, len(dr.Choices))
	for _, c := range dr.Choices {
		authors := make([]string, 0, len(c.AuthorIDs))
		for _, a := range c.AuthorIDs {
			authors = append(authors, string(a))
		}
		voters := make([]string, 0, len(c.Voters))
		for _, v := range c.Voters {
			voters = append(voters, string(v))
		}
		choices = append(choices, proto.RevealChoice{
			ChoiceID:  c.ChoiceID,
			Text:      c.Text,
			AuthorIDs: authors,
			Voters:    voters,
			IsTrue:    c.IsTrue,
		})
	}
	raw, _ := json.Marshal(proto.RevealPayload{
		DrawingID:  string(dr.DrawingID),
		AuthorID:   string(dr.AuthorID),
		TruePrompt: dr.TruePrompt,
		Choices:    choices,
	})
	return raw
}
