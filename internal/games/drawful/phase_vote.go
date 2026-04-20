package drawful

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jj/trivia/internal/engine"
	"github.com/jj/trivia/internal/primitives"
	"github.com/jj/trivia/internal/proto"
)

// voteCollectPrimitive mirrors fakeCollect: batch per player, one vote per
// non-authored drawing. Enforces §8 invariants (no self-vote, no self-fake-vote).
type voteCollectPrimitive struct {
	round       int
	drawingsKey string
	fakesKey    string
	by          map[engine.PlayerID]map[DrawingID]Vote
}

func newVoteCollect(round int) engine.AnyPrimitive {
	return &erasedVoteCollect{inner: &voteCollectPrimitive{
		round:       round,
		drawingsKey: phaseName("drawing_submit", round),
		fakesKey:    phaseName("fake_prompt_submit", round),
		by:          map[engine.PlayerID]map[DrawingID]Vote{},
	}}
}

type erasedVoteCollect struct{ inner *voteCollectPrimitive }

func (e *erasedVoteCollect) Name() string { return "voting" }

func (e *erasedVoteCollect) Start(ctx *engine.PhaseContext) (engine.AnyDecision, error) {
	dec := e.maybeAdvance(ctx)
	// Surface the pre-reveal choice list (true + merged fakes, without voter
	// tallies or fake authorship) so clients can render the ballot.
	dec.Broadcast = append(dec.Broadcast, e.buildVotingChoicesEvent(ctx)...)
	return dec, nil
}

// buildVotingChoicesEvent is the pre-reveal counterpart to phase_reveal's
// aggregate: same collision merge, but stripped of voter lists and author
// IDs (which would leak fakery).
func (e *erasedVoteCollect) buildVotingChoicesEvent(ctx *engine.PhaseContext) []engine.Event {
	p := e.inner
	drawings, ok := engine.GetResult[primitives.CollectAllResult[Drawing]](ctx.State, p.drawingsKey)
	if !ok {
		return nil
	}
	fakes, ok := engine.GetResult[FakeCollectionResult](ctx.State, p.fakesKey)
	if !ok {
		return nil
	}
	entries := make([]proto.VotingChoicesEntry, 0, len(drawings.Items))
	for _, d := range drawings.Items {
		buckets := map[string][]FakePrompt{}
		trueNorm := strings.ToLower(strings.TrimSpace(d.Prompt))
		for _, f := range fakes.ByDrawing[d.ID] {
			key := strings.ToLower(strings.TrimSpace(f.Text))
			if key == trueNorm {
				continue
			}
			buckets[key] = append(buckets[key], f)
		}
		choices := make([]proto.VotingChoice, 0, len(buckets)+1)
		for _, group := range buckets {
			choices = append(choices, proto.VotingChoice{
				ChoiceID: string(group[0].ID),
				Text:     group[0].Text,
				IsTrue:   false,
			})
		}
		choices = append(choices, proto.VotingChoice{
			ChoiceID: "TRUE",
			Text:     d.Prompt,
			IsTrue:   true,
		})
		entries = append(entries, proto.VotingChoicesEntry{
			DrawingID: string(d.ID),
			Choices:   choices,
		})
	}
	raw, err := json.Marshal(proto.VotingChoicesPayload{Round: p.round, Entries: entries})
	if err != nil {
		return nil
	}
	return []engine.Event{{Type: proto.S2CVotingChoices, Payload: raw}}
}

func (e *erasedVoteCollect) Handle(ctx *engine.PhaseContext, in engine.Input) (engine.AnyDecision, error) {
	p := e.inner
	drawings, ok := engine.GetResult[primitives.CollectAllResult[Drawing]](ctx.State, p.drawingsKey)
	if !ok {
		return engine.AnyDecision{}, fmt.Errorf("drawings missing")
	}
	fakes, ok := engine.GetResult[FakeCollectionResult](ctx.State, p.fakesKey)
	if !ok {
		return engine.AnyDecision{}, fmt.Errorf("fakes missing")
	}

	var payload proto.SubmitVotePayload
	if err := json.Unmarshal(in.Value, &payload); err != nil {
		return engine.AnyDecision{}, fmt.Errorf("parse vote: %w", err)
	}

	var target Drawing
	ok2 := false
	for _, d := range drawings.Items {
		if string(d.ID) == payload.DrawingID {
			target = d
			ok2 = true
			break
		}
	}
	if !ok2 {
		return engine.AnyDecision{}, fmt.Errorf("unknown drawing_id")
	}
	if target.AuthorID == in.PlayerID {
		return engine.AnyDecision{}, fmt.Errorf("cannot vote on your own drawing")
	}

	// ChoiceID is either "TRUE" or a FakePromptID present in the drawing's fake list.
	if payload.ChoiceID != "TRUE" {
		found := false
		for _, f := range fakes.ByDrawing[target.ID] {
			if string(f.ID) == payload.ChoiceID {
				// §8: cannot vote for your own fake prompt.
				if f.AuthorID == in.PlayerID {
					return engine.AnyDecision{}, fmt.Errorf("cannot vote for your own fake")
				}
				found = true
				break
			}
		}
		if !found {
			return engine.AnyDecision{}, fmt.Errorf("choice_id not available for this drawing")
		}
	}

	if p.by[in.PlayerID] == nil {
		p.by[in.PlayerID] = map[DrawingID]Vote{}
	}
	if _, dup := p.by[in.PlayerID][target.ID]; dup {
		return engine.AnyDecision{}, fmt.Errorf("already voted on this drawing")
	}
	p.by[in.PlayerID][target.ID] = Vote{
		VoterID:   in.PlayerID,
		DrawingID: target.ID,
		ChoiceID:  payload.ChoiceID,
	}
	return e.maybeAdvance(ctx), nil
}

func (e *erasedVoteCollect) Timeout(ctx *engine.PhaseContext) (engine.AnyDecision, error) {
	return e.finalize(ctx), nil
}

func (e *erasedVoteCollect) maybeAdvance(ctx *engine.PhaseContext) engine.AnyDecision {
	p := e.inner
	drawings, ok := engine.GetResult[primitives.CollectAllResult[Drawing]](ctx.State, p.drawingsKey)
	if !ok {
		return engine.AnyDecision{}
	}
	required, have := 0, 0
	for pid, pl := range ctx.State.Players {
		if !pl.Connected {
			continue
		}
		for _, d := range drawings.Items {
			if d.AuthorID == pid {
				continue
			}
			required++
			if p.by[pid] != nil {
				if _, ok := p.by[pid][d.ID]; ok {
					have++
				}
			}
		}
	}
	if have >= required && required > 0 {
		return e.finalize(ctx)
	}
	return engine.AnyDecision{}
}

func (e *erasedVoteCollect) finalize(ctx *engine.PhaseContext) engine.AnyDecision {
	p := e.inner
	drawings, _ := engine.GetResult[primitives.CollectAllResult[Drawing]](ctx.State, p.drawingsKey)
	byDrawing := map[DrawingID][]Vote{}
	for _, m := range p.by {
		for did, v := range m {
			byDrawing[did] = append(byDrawing[did], v)
		}
	}
	for _, d := range drawings.Items {
		if _, ok := byDrawing[d.ID]; !ok {
			byDrawing[d.ID] = nil
		}
	}
	return engine.AnyDecision{
		AdvancePhase: true,
		Result: engine.StoredResult[VoteCollectionResult]{
			Phase:     ctx.Phase.Name,
			Primitive: e.Name(),
			Value:     VoteCollectionResult{ByDrawing: byDrawing},
		},
	}
}
