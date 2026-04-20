package drawful

import (
	"testing"

	"github.com/jj/trivia/internal/engine"
	"github.com/jj/trivia/internal/primitives"
)

// TestReveal_CollisionMergesFakes verifies the §8 collision-merge rule:
// two players submitting "CAT" and "cat" end up sharing one voting choice.
func TestReveal_CollisionMergesFakes(t *testing.T) {
	t.Log("Scenario: drawing d1 by A (true prompt `dog`); B submits fake `CAT`, C submits fake `cat`; A votes for f2 (`cat`).")
	t.Log("Expected: reveal produces 2 choices total — 1 merged fake (authors={B,C}, voters={A}) + 1 true prompt. Case/whitespace-insensitive merge per §8.")
	Register()
	const r = 1
	s := &engine.GameState{
		Players: map[engine.PlayerID]*engine.Player{
			"A": {ID: "A", Connected: true},
			"B": {ID: "B", Connected: true},
			"C": {ID: "C", Connected: true},
		},
		PhaseData: map[string]engine.PhaseResult{},
		Scores:    map[engine.PlayerID]int{},
	}
	s.PhaseData[phaseName("drawing_submit", r)] = engine.StoredResult[primitives.CollectAllResult[Drawing]]{
		Phase: phaseName("drawing_submit", r),
		Value: primitives.CollectAllResult[Drawing]{Items: map[engine.PlayerID]Drawing{
			"A": {ID: "d1", AuthorID: "A", Prompt: "dog"},
		}},
	}
	s.PhaseData[phaseName("fake_prompt_submit", r)] = engine.StoredResult[FakeCollectionResult]{
		Phase: phaseName("fake_prompt_submit", r),
		Value: FakeCollectionResult{ByDrawing: map[DrawingID][]FakePrompt{
			"d1": {
				{ID: "f1", DrawingID: "d1", AuthorID: "B", Text: "CAT"},
				{ID: "f2", DrawingID: "d1", AuthorID: "C", Text: "cat"},
			},
		}},
	}
	s.PhaseData[phaseName("voting", r)] = engine.StoredResult[VoteCollectionResult]{
		Phase: phaseName("voting", r),
		Value: VoteCollectionResult{ByDrawing: map[DrawingID][]Vote{
			"d1": {{VoterID: "A", DrawingID: "d1", ChoiceID: "f2"}},
		}},
	}

	prim := newReveal(r)
	d, err := prim.Start(&engine.PhaseContext{
		State: s, Phase: engine.Phase{Name: phaseName("reveal", r)},
	})
	if err != nil {
		t.Fatalf("reveal: %v", err)
	}
	if !d.AdvancePhase {
		t.Fatal("reveal must advance")
	}

	stored, ok := engine.GetResult[RevealResult](s, phaseName("reveal", r))
	// We stored it implicitly via the room; for the test we pull from the decision.
	_ = stored
	_ = ok

	sr, ok := d.Result.(engine.StoredResult[RevealResult])
	if !ok {
		t.Fatalf("unexpected result type %T", d.Result)
	}
	drv := sr.Value.ByDrawing["d1"]
	// Expect 2 choices: 1 merged fake + 1 true.
	if len(drv.Choices) != 2 {
		t.Fatalf("want 2 choices, got %d", len(drv.Choices))
	}
	for _, c := range drv.Choices {
		if c.IsTrue {
			continue
		}
		if len(c.AuthorIDs) != 2 {
			t.Fatalf("merged choice should have 2 authors, got %d", len(c.AuthorIDs))
		}
		if len(c.Voters) != 1 || c.Voters[0] != "A" {
			t.Fatalf("merged choice should have 1 voter (A), got %v", c.Voters)
		}
	}
}
