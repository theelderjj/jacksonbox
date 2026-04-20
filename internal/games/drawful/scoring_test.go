package drawful

import (
	"testing"

	"github.com/jj/trivia/internal/engine"
	"github.com/jj/trivia/internal/primitives"
)

// buildState prepares a GameState pre-populated with a reveal for one drawing,
// ready for the scoring phase. Keeps test arithmetic legible.
func buildState(t *testing.T, reveal RevealResult, scores map[engine.PlayerID]int) *engine.GameState {
	t.Helper()
	s := &engine.GameState{
		Players:   map[engine.PlayerID]*engine.Player{},
		PhaseData: map[string]engine.PhaseResult{},
		Scores:    map[engine.PlayerID]int{},
		Round:     1,
	}
	for id, v := range scores {
		s.Players[id] = &engine.Player{ID: id, Connected: true}
		s.Scores[id] = v
	}
	s.PhaseData[phaseName("reveal", 1)] = engine.StoredResult[RevealResult]{
		Phase: phaseName("reveal", 1), Primitive: "reveal", Value: reveal,
	}
	return s
}

func runScoring(t *testing.T, s *engine.GameState) {
	t.Helper()
	prim := newScoring(1)
	ctx := &engine.PhaseContext{
		State: s, Phase: engine.Phase{Name: phaseName("scoring", 1)},
	}
	d, err := prim.Start(ctx)
	if err != nil {
		t.Fatalf("scoring: %v", err)
	}
	if !d.AdvancePhase {
		t.Fatal("scoring must advance")
	}
}

func TestScoring_DrawerAndGuesserForTrue(t *testing.T) {
	t.Log("Scenario: drawing by A; voters B and C both pick TRUE.")
	t.Log("Expected: A earns 2×1000=2000 (drawer payout, one per TRUE voter). B and C each earn 1000 (guesser payout for picking TRUE).")
	// Drawing D by author A. Voters B and C both pick TRUE.
	// A earns 2*1000 = 2000. B and C each earn 1000.
	reveal := RevealResult{ByDrawing: map[DrawingID]DrawingReveal{
		"d1": {
			DrawingID: "d1", AuthorID: "A", TruePrompt: "dog",
			Choices: []Choice{
				{ChoiceID: "TRUE", Text: "dog", IsTrue: true,
					Voters: []engine.PlayerID{"B", "C"}},
			},
		},
	}}
	s := buildState(t, reveal, map[engine.PlayerID]int{"A": 0, "B": 0, "C": 0})
	runScoring(t, s)

	if s.Scores["A"] != 2000 {
		t.Errorf("A: want 2000, got %d", s.Scores["A"])
	}
	if s.Scores["B"] != 1000 {
		t.Errorf("B: want 1000, got %d", s.Scores["B"])
	}
	if s.Scores["C"] != 1000 {
		t.Errorf("C: want 1000, got %d", s.Scores["C"])
	}
}

func TestScoring_FakerFooledSingleAuthor(t *testing.T) {
	t.Log("Scenario: A draws `dog`; B's fake `cat` fools C (C voted for it); nobody voted TRUE.")
	t.Log("Expected: B earns 500 (faker-fooled payout, one per voter tricked). A earns 0 (no TRUE voters, no drawer payout). C earns 0 (picked a fake).")
	// B's fake "cat" fools C. B earns 500; no one gets drawer points (no TRUE voters).
	reveal := RevealResult{ByDrawing: map[DrawingID]DrawingReveal{
		"d1": {
			DrawingID: "d1", AuthorID: "A", TruePrompt: "dog",
			Choices: []Choice{
				{ChoiceID: "TRUE", Text: "dog", IsTrue: true, Voters: nil},
				{ChoiceID: "f1", Text: "cat", AuthorIDs: []engine.PlayerID{"B"},
					Voters: []engine.PlayerID{"C"}},
			},
		},
	}}
	s := buildState(t, reveal, map[engine.PlayerID]int{"A": 0, "B": 0, "C": 0})
	runScoring(t, s)

	if s.Scores["A"] != 0 {
		t.Errorf("A: want 0, got %d", s.Scores["A"])
	}
	if s.Scores["B"] != 500 {
		t.Errorf("B: want 500, got %d", s.Scores["B"])
	}
	if s.Scores["C"] != 0 {
		t.Errorf("C: want 0, got %d", s.Scores["C"])
	}
}

func TestScoring_FakerFooledMultipleVoters(t *testing.T) {
	t.Log("Scenario: A draws; B writes one fake prompt; C and D both pick it.")
	t.Log("Expected: B gets paid once per fooled player, so 2 fooled voters = 1000 total fake-prompt points.")
	reveal := RevealResult{ByDrawing: map[DrawingID]DrawingReveal{
		"d1": {
			DrawingID: "d1", AuthorID: "A", TruePrompt: "dog",
			Choices: []Choice{
				{ChoiceID: "TRUE", Text: "dog", IsTrue: true, Voters: nil},
				{ChoiceID: "f1", Text: "cat", AuthorIDs: []engine.PlayerID{"B"},
					Voters: []engine.PlayerID{"C", "D"}},
			},
		},
	}}
	s := buildState(t, reveal, map[engine.PlayerID]int{"A": 0, "B": 0, "C": 0, "D": 0})
	runScoring(t, s)

	if s.Scores["B"] != 1000 {
		t.Errorf("B: want 1000, got %d", s.Scores["B"])
	}
	if s.Scores["A"] != 0 || s.Scores["C"] != 0 || s.Scores["D"] != 0 {
		t.Errorf("non-fakers should not score here: A=%d C=%d D=%d", s.Scores["A"], s.Scores["C"], s.Scores["D"])
	}
}

func TestScoring_MergedChoiceSplitsCredit(t *testing.T) {
	t.Log("Scenario: B and C both submitted `cat` (collision-merged into one choice, two authors); voter D picks it.")
	t.Log("Expected: faker payout = 500 × voters(1) / authors(2) = 250 each for B and C. Integer split; any remainder drops per §8.")
	// B and C both submitted "cat"; merged into one choice with two authors.
	// Voter D picks it (voter count = 1). Faker payout = 500*1/2 = 250 each
	// (integer split; remainder drops per §8).
	reveal := RevealResult{ByDrawing: map[DrawingID]DrawingReveal{
		"d1": {
			DrawingID: "d1", AuthorID: "A", TruePrompt: "dog",
			Choices: []Choice{
				{ChoiceID: "TRUE", Text: "dog", IsTrue: true},
				{ChoiceID: "f-merged", Text: "cat",
					AuthorIDs: []engine.PlayerID{"B", "C"},
					Voters:    []engine.PlayerID{"D"}},
			},
		},
	}}
	s := buildState(t, reveal, map[engine.PlayerID]int{
		"A": 0, "B": 0, "C": 0, "D": 0,
	})
	runScoring(t, s)

	if s.Scores["B"] != 250 {
		t.Errorf("B: want 250, got %d", s.Scores["B"])
	}
	if s.Scores["C"] != 250 {
		t.Errorf("C: want 250, got %d", s.Scores["C"])
	}
}

func TestScoring_MergedChoiceIntegerRemainderDrops(t *testing.T) {
	t.Log("Scenario: 3 authors share one merged fake; 1 voter picks it. Payout 500 × 1 ÷ 3 = 166.67.")
	t.Log("Expected: each author gets 166; remainder 2 points drops. Integer-only scoring ensures determinism and forbids round-up rescues.")
	// 3 authors, 1 voter → 500/3 = 166 each, remainder 2 drops.
	reveal := RevealResult{ByDrawing: map[DrawingID]DrawingReveal{
		"d1": {
			DrawingID: "d1", AuthorID: "A", TruePrompt: "dog",
			Choices: []Choice{
				{ChoiceID: "TRUE", Text: "dog", IsTrue: true},
				{ChoiceID: "f-merged", Text: "cat",
					AuthorIDs: []engine.PlayerID{"B", "C", "E"},
					Voters:    []engine.PlayerID{"D"}},
			},
		},
	}}
	s := buildState(t, reveal, map[engine.PlayerID]int{"A": 0, "B": 0, "C": 0, "D": 0, "E": 0})
	runScoring(t, s)

	for _, id := range []engine.PlayerID{"B", "C", "E"} {
		if s.Scores[id] != 166 {
			t.Errorf("%s: want 166, got %d", id, s.Scores[id])
		}
	}
}

func TestScoring_ZeroVoteRound(t *testing.T) {
	t.Log("Scenario: reveal has the drawing and a fake, but nobody voted on anything (empty voter lists).")
	t.Log("Expected: scores unchanged from pre-round totals (A=500, B=500). Zero activity → zero deltas → no phantom awards.")
	reveal := RevealResult{ByDrawing: map[DrawingID]DrawingReveal{
		"d1": {
			DrawingID: "d1", AuthorID: "A", TruePrompt: "dog",
			Choices: []Choice{
				{ChoiceID: "TRUE", IsTrue: true},
				{ChoiceID: "f1", Text: "cat", AuthorIDs: []engine.PlayerID{"B"}},
			},
		},
	}}
	s := buildState(t, reveal, map[engine.PlayerID]int{"A": 500, "B": 500})
	runScoring(t, s)

	if s.Scores["A"] != 500 || s.Scores["B"] != 500 {
		t.Errorf("zero-vote round should leave scores unchanged: A=%d B=%d",
			s.Scores["A"], s.Scores["B"])
	}
}

func TestScoring_MixedDrawerAndFaker(t *testing.T) {
	t.Log("Scenario: drawer A gets 2 TRUE voters (B, C); faker D fools 1 voter (E).")
	t.Log("Expected: A=2000 (drawer, 2 TRUE voters); B=1000, C=1000 (guesser payouts); D=500 (faker fooled one); E=0 (picked a fake).")
	// True prompt collects 2 TRUE voters. Faker fools 1. Everything counts.
	reveal := RevealResult{ByDrawing: map[DrawingID]DrawingReveal{
		"d1": {
			DrawingID: "d1", AuthorID: "A", TruePrompt: "dog",
			Choices: []Choice{
				{ChoiceID: "TRUE", Text: "dog", IsTrue: true,
					Voters: []engine.PlayerID{"B", "C"}},
				{ChoiceID: "f1", Text: "cat",
					AuthorIDs: []engine.PlayerID{"D"},
					Voters:    []engine.PlayerID{"E"}},
			},
		},
	}}
	s := buildState(t, reveal, map[engine.PlayerID]int{
		"A": 0, "B": 0, "C": 0, "D": 0, "E": 0,
	})
	runScoring(t, s)

	if s.Scores["A"] != 2000 {
		t.Errorf("A: want 2000, got %d", s.Scores["A"])
	}
	if s.Scores["B"] != 1000 || s.Scores["C"] != 1000 {
		t.Errorf("B/C: want 1000 each, got %d %d", s.Scores["B"], s.Scores["C"])
	}
	if s.Scores["D"] != 500 {
		t.Errorf("D: want 500, got %d", s.Scores["D"])
	}
	if s.Scores["E"] != 0 {
		t.Errorf("E: want 0, got %d", s.Scores["E"])
	}
}

// Compile-time check: CollectAllResult[Drawing] satisfies the type we store
// under PhaseData. If this compiles, the Jrawful <-> primitives contract holds.
var _ primitives.CollectAllResult[Drawing] = primitives.CollectAllResult[Drawing]{}
