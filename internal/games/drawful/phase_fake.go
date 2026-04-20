package drawful

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/jj/trivia/internal/engine"
	"github.com/jj/trivia/internal/primitives"
	"github.com/jj/trivia/internal/proto"
)

// fakeCollectPrimitive is a bespoke batch collector. Each non-authoring
// player must submit exactly one fake per drawing that isn't theirs. Phase
// advances when every (player, drawing) pair has a fake, or on timeout.
//
// We can't reuse primitives.CollectAll because that stores one R per player.
// Here we need (playerID, drawingID) → fake.
type fakeCollectPrimitive struct {
	round       int
	drawingsKey string
	// by[player][drawingID] = FakePrompt
	by map[engine.PlayerID]map[DrawingID]FakePrompt
}

func newFakeCollect(round int) engine.AnyPrimitive {
	return &erasedFakeCollect{inner: &fakeCollectPrimitive{
		round:       round,
		drawingsKey: phaseName("drawing_submit", round),
		by:          map[engine.PlayerID]map[DrawingID]FakePrompt{},
	}}
}

// erasedFakeCollect adapts the primitive to engine.AnyPrimitive directly —
// we didn't need the generic Primitive[R] path because this primitive's
// result type is known to this package.
type erasedFakeCollect struct{ inner *fakeCollectPrimitive }

func (e *erasedFakeCollect) Name() string { return "fake_prompt_submit" }

func (e *erasedFakeCollect) Start(ctx *engine.PhaseContext) (engine.AnyDecision, error) {
	dec := e.maybeAdvance(ctx)
	// Surface drawings so clients can render them on the fake-prompt screen.
	// Sent regardless of advance decision — if we advance immediately (e.g.
	// single-player edge), clients still see them before voting starts.
	drawings, ok := engine.GetResult[primitives.CollectAllResult[Drawing]](ctx.State, e.inner.drawingsKey)
	if ok {
		summaries := make([]proto.DrawingSummary, 0, len(drawings.Items))
		for _, d := range drawings.Items {
			summaries = append(summaries, proto.DrawingSummary{
				DrawingID: string(d.ID),
				AuthorID:  string(d.AuthorID),
				Format:    d.Format,
				Data:      d.Data,
			})
		}
		raw, err := json.Marshal(proto.DrawingsPayload{Round: e.inner.round, Drawings: summaries})
		if err == nil {
			dec.Broadcast = append(dec.Broadcast, engine.Event{Type: proto.S2CDrawings, Payload: raw})
		}
	}
	return dec, nil
}

func (e *erasedFakeCollect) Handle(ctx *engine.PhaseContext, in engine.Input) (engine.AnyDecision, error) {
	p := e.inner
	drawings, ok := engine.GetResult[primitives.CollectAllResult[Drawing]](ctx.State, p.drawingsKey)
	if !ok {
		return engine.AnyDecision{}, fmt.Errorf("drawings missing from state")
	}

	var payload proto.SubmitFakePromptPayload
	if err := json.Unmarshal(in.Value, &payload); err != nil {
		return engine.AnyDecision{}, fmt.Errorf("parse fake: %w", err)
	}

	// Find the target drawing.
	var target Drawing
	found := false
	for _, d := range drawings.Items {
		if string(d.ID) == payload.DrawingID {
			target = d
			found = true
			break
		}
	}
	if !found {
		return engine.AnyDecision{}, fmt.Errorf("unknown drawing_id")
	}
	if target.AuthorID == in.PlayerID {
		return engine.AnyDecision{}, fmt.Errorf("cannot submit fake for your own drawing")
	}

	text, err := proto.CleanFakeText(payload.Text)
	if err != nil {
		return engine.AnyDecision{}, err
	}
	// §8: reject fakes equal to the true prompt (case-insensitive, trimmed).
	if strings.EqualFold(strings.TrimSpace(text), strings.TrimSpace(target.Prompt)) {
		return engine.AnyDecision{}, fmt.Errorf("fake matches true prompt; try another")
	}

	// Single submission per (player, drawing).
	if p.by[in.PlayerID] == nil {
		p.by[in.PlayerID] = map[DrawingID]FakePrompt{}
	}
	if _, dup := p.by[in.PlayerID][target.ID]; dup {
		return engine.AnyDecision{}, fmt.Errorf("already submitted fake for that drawing")
	}
	p.by[in.PlayerID][target.ID] = FakePrompt{
		ID:        FakePromptID(uuid.NewString()),
		DrawingID: target.ID,
		AuthorID:  in.PlayerID,
		Text:      text,
	}

	return e.maybeAdvance(ctx), nil
}

func (e *erasedFakeCollect) Timeout(ctx *engine.PhaseContext) (engine.AnyDecision, error) {
	// Advance with whatever we have; defaults are applied later by the reveal
	// phase (missing fakes become "..." and get collision-merged there).
	return e.finalize(ctx), nil
}

func (e *erasedFakeCollect) maybeAdvance(ctx *engine.PhaseContext) engine.AnyDecision {
	p := e.inner
	drawings, ok := engine.GetResult[primitives.CollectAllResult[Drawing]](ctx.State, p.drawingsKey)
	if !ok {
		return engine.AnyDecision{}
	}
	// Every connected non-author player must submit one fake per drawing
	// not authored by themselves.
	required := 0
	have := 0
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

func (e *erasedFakeCollect) finalize(ctx *engine.PhaseContext) engine.AnyDecision {
	p := e.inner
	drawings, _ := engine.GetResult[primitives.CollectAllResult[Drawing]](ctx.State, p.drawingsKey)
	byDrawing := map[DrawingID][]FakePrompt{}
	for _, m := range p.by {
		for did, fake := range m {
			byDrawing[did] = append(byDrawing[did], fake)
		}
	}
	// Include drawings with zero submissions so the reveal phase can apply
	// default "..." fakes for missing entries.
	for _, d := range drawings.Items {
		if _, ok := byDrawing[d.ID]; !ok {
			byDrawing[d.ID] = nil
		}
		for _, text := range generatedFakeTexts(ctx.State.Settings.GeneratedFakeCount, d.Prompt) {
			byDrawing[d.ID] = append(byDrawing[d.ID], FakePrompt{
				ID:        FakePromptID(uuid.NewString()),
				DrawingID: d.ID,
				Text:      text,
			})
		}
	}
	return engine.AnyDecision{
		AdvancePhase: true,
		Result: engine.StoredResult[FakeCollectionResult]{
			Phase:     ctx.Phase.Name,
			Primitive: e.Name(),
			Value:     FakeCollectionResult{ByDrawing: byDrawing},
		},
	}
}

func generatedFakeTexts(count int, truePrompt string) []string {
	if count <= 0 {
		return nil
	}
	out := make([]string, 0, count)
	seen := map[string]bool{normalize(truePrompt): true}
	for len(out) < count {
		for _, candidate := range NextPrompts(count * 2) {
			key := normalize(candidate)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, candidate)
			if len(out) == count {
				break
			}
		}
	}
	return out
}
