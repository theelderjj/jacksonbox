package drawful

import (
	"testing"

	"github.com/jj/trivia/internal/engine"
)

// TestBuildRevealSteps_OrdersAndAttributes pins the shape of the reveal
// animation queue: eliminations come first (in deterministic ChoiceID order),
// then a final step carrying per-drawing deltas + awards. Leader-driven
// reveal no longer injects timing-only gap steps between drawings. Points
// math mirrors phase_scoring.go.
func TestBuildRevealSteps_OrdersAndAttributes(t *testing.T) {
	t.Log("Scenario: round has 2 drawings; d1 has 2 fakes + 1 true (1 voter picked fake, 1 voter picked true); d2 has 1 fake that nobody voted for.")
	t.Log("Expected: step queue is [d1.eliminate(f1), d1.eliminate(f2), d1.final{awards: truth voter + drawer + faker}, d2.eliminate(g1), d2.final{awards: empty}]. Deterministic order by drawing ID then by choice ID.")

	const round = 1
	revealKey := phaseName("reveal", round)

	state := &engine.GameState{
		Players:   map[engine.PlayerID]*engine.Player{},
		PhaseData: map[string]engine.PhaseResult{},
		Scores:    map[engine.PlayerID]int{},
	}
	state.PhaseData[revealKey] = engine.StoredResult[RevealResult]{
		Phase: revealKey,
		Value: RevealResult{ByDrawing: map[DrawingID]DrawingReveal{
			"d1": {
				AuthorID: "A",
				Choices: []Choice{
					{ChoiceID: "TRUE", IsTrue: true, AuthorIDs: []engine.PlayerID{"A"}, Voters: []engine.PlayerID{"B"}},
					// Intentionally inserted in reverse of expected ChoiceID
					// order so the sort inside BuildRevealSteps is exercised.
					{ChoiceID: "f2", IsTrue: false, AuthorIDs: []engine.PlayerID{"C"}, Voters: []engine.PlayerID{}},
					{ChoiceID: "f1", IsTrue: false, AuthorIDs: []engine.PlayerID{"B"}, Voters: []engine.PlayerID{"D"}},
				},
			},
			"d2": {
				AuthorID: "B",
				Choices: []Choice{
					{ChoiceID: "TRUE", IsTrue: true, AuthorIDs: []engine.PlayerID{"B"}, Voters: []engine.PlayerID{}},
					{ChoiceID: "g1", IsTrue: false, AuthorIDs: []engine.PlayerID{"A"}, Voters: []engine.PlayerID{}},
				},
			},
		}},
	}

	steps := BuildRevealSteps(state, round)
	if len(steps) == 0 {
		t.Fatal("no steps produced")
	}

	// Flatten to a compact shape so the assertion reads like the scenario.
	type shape struct {
		kind string
		drw  string
		chc  string
	}
	got := make([]shape, 0, len(steps))
	for _, s := range steps {
		got = append(got, shape{s.Kind, s.DrawingID, s.ChoiceID})
	}
	want := []shape{
		{"eliminate", "d1", "f1"},
		{"eliminate", "d1", "f2"},
		{"final", "d1", ""},
		{"eliminate", "d2", "g1"},
		{"final", "d2", ""},
	}
	if len(got) != len(want) {
		t.Fatalf("step count: want %d, got %d: %+v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("step[%d]: want %+v, got %+v", i, want[i], got[i])
		}
	}

	// Sanity: d1 final carries attribution for drawer (truth guessed by B),
	// guesser B, and faker B (fooled D). The faker payout is integer-divided
	// across merged authors — here single-authored, so no division loss.
	final := steps[2] // the d1 final
	if final.Kind != "final" || final.DrawingID != "d1" {
		t.Fatalf("want d1 final at index 2, got %+v", final)
	}
	if final.Deltas["A"] != PointsDrawerPerCorrectVote {
		t.Errorf("drawer A: want %d, got %d", PointsDrawerPerCorrectVote, final.Deltas["A"])
	}
	if final.Deltas["B"] != PointsGuesserForTrue+PointsFakerPerFooledVote {
		t.Errorf("B (guesser + faker): want %d, got %d",
			PointsGuesserForTrue+PointsFakerPerFooledVote, final.Deltas["B"])
	}
	// Three awards: drawer_truth, guesser_truth, faker_fool.
	if len(final.Awards) != 3 {
		t.Errorf("award count: want 3, got %d (%+v)", len(final.Awards), final.Awards)
	}

	// d2 final: no voters anywhere, so deltas empty, awards empty.
	final2 := steps[4]
	if final2.Kind != "final" || final2.DrawingID != "d2" {
		t.Fatalf("want d2 final at index 4, got %+v", final2)
	}
	if len(final2.Deltas) != 0 {
		t.Errorf("d2 deltas: want empty, got %+v", final2.Deltas)
	}
	if len(final2.Awards) != 0 {
		t.Errorf("d2 awards: want empty, got %+v", final2.Awards)
	}
}

// TestBuildRevealSteps_MissingRevealReturnsNil guards the
// "reveal hasn't been stored yet" case — e.g., a test that builds steps
// without having run the reveal primitive first. Should no-op silently so
// production callers don't panic if the hook is invoked at the wrong time.
func TestBuildRevealSteps_MissingRevealReturnsNil(t *testing.T) {
	state := &engine.GameState{
		PhaseData: map[string]engine.PhaseResult{},
	}
	if steps := BuildRevealSteps(state, 1); steps != nil {
		t.Fatalf("want nil on missing reveal, got %+v", steps)
	}
}

// TestBuildRevealSteps_SplitsFakerCreditAcrossMergedAuthors pins the
// integer-division behavior from phase_scoring.go: fake credit is split
// across the merged authors, with remainder dropped.
func TestBuildRevealSteps_SplitsFakerCreditAcrossMergedAuthors(t *testing.T) {
	t.Log("Scenario: 1 drawing with a merged-fake choice (authors = {B, C}) that 3 voters fell for.")
	t.Log("Expected: per-author payout = (PointsFakerPerFooledVote * 3) / 2, remainder dropped.")

	const round = 1
	state := &engine.GameState{
		PhaseData: map[string]engine.PhaseResult{},
	}
	state.PhaseData[phaseName("reveal", round)] = engine.StoredResult[RevealResult]{
		Phase: phaseName("reveal", round),
		Value: RevealResult{ByDrawing: map[DrawingID]DrawingReveal{
			"d1": {
				AuthorID: "A",
				Choices: []Choice{
					{ChoiceID: "TRUE", IsTrue: true, AuthorIDs: []engine.PlayerID{"A"}, Voters: []engine.PlayerID{}},
					{ChoiceID: "f1", IsTrue: false, AuthorIDs: []engine.PlayerID{"B", "C"}, Voters: []engine.PlayerID{"D", "E", "F"}},
				},
			},
		}},
	}
	steps := BuildRevealSteps(state, round)
	// Final step is the second in the queue (1 eliminate + 1 final; no gap
	// because single drawing).
	var final *struct {
		deltas map[string]int
		awards int
	}
	for _, s := range steps {
		if s.Kind == "final" {
			final = &struct {
				deltas map[string]int
				awards int
			}{s.Deltas, len(s.Awards)}
			break
		}
	}
	if final == nil {
		t.Fatal("no final step")
	}
	expectPer := (PointsFakerPerFooledVote * 3) / 2
	if final.deltas["B"] != expectPer {
		t.Errorf("faker B: want %d, got %d", expectPer, final.deltas["B"])
	}
	if final.deltas["C"] != expectPer {
		t.Errorf("faker C: want %d, got %d", expectPer, final.deltas["C"])
	}
	// 2 awards (one per merged author). Drawer+guesser get nothing — no
	// voters picked truth.
	if final.awards != 2 {
		t.Errorf("award count: want 2, got %d", final.awards)
	}
}
