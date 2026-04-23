package splitthevote

import (
	"testing"

	"github.com/jj/trivia/internal/engine"
)

func TestTargetCounts(t *testing.T) {
	t.Parallel()

	a, b := targetCounts(4, "strict_split")
	if a != 2 || b != 2 {
		t.Fatalf("strict split 4 players: want 2/2 got %d/%d", a, b)
	}

	a, b = targetCounts(5, "odd_one")
	if a != 4 || b != 1 {
		t.Fatalf("odd one 5 players: want 4/1 got %d/%d", a, b)
	}

	a, b = targetCounts(1, "strict_split")
	if a != 0 || b != 1 {
		t.Fatalf("strict split 1 voter: want 0/1 got %d/%d", a, b)
	}
}

func TestResolveRoundAwardsSplitterAndVotersWhenAchieved(t *testing.T) {
	t.Parallel()

	state := &engine.GameState{
		Players: map[engine.PlayerID]*engine.Player{
			"a": {ID: "a", Name: "Alice", Connected: true},
			"b": {ID: "b", Name: "Bob", Connected: true},
			"c": {ID: "c", Name: "Carol", Connected: true},
			"d": {ID: "d", Name: "Dan", Connected: true},
		},
		PhaseData: map[string]engine.PhaseResult{},
		Scores:    map[engine.PlayerID]int{},
	}

	engine.PutResult(state, "split_setup_r1", "splitthevote.setup.r1", SetupResult{
		Round:        1,
		SplitterID:   "a",
		SplitterName: "Alice",
		Prompt:       "Who brings the better energy?",
		OptionA:      "Morning person",
		OptionB:      "Night owl",
		TargetA:      2,
		TargetB:      2,
		TargetMode:   "strict_split",
		ShowTarget:   true,
	})
	engine.PutResult(state, "split_vote_r1", "splitthevote.vote.r1", VoteResult{
		ByPlayer: map[string]string{
			"a": "A",
			"b": "A",
			"c": "B",
			"d": "B",
		},
		CountA: 2,
		CountB: 2,
	})

	reveal, scores, deltas, err := resolveRound(state, 1)
	if err != nil {
		t.Fatalf("resolveRound: %v", err)
	}
	if !reveal.Achieved {
		t.Fatal("expected round to achieve target")
	}
	if deltas["a"] != 2000 {
		t.Fatalf("splitter should get 2000 total (1500+500), got %d", deltas["a"])
	}
	if deltas["b"] != 500 || deltas["c"] != 500 || deltas["d"] != 500 {
		t.Fatalf("expected each voter to get 500, got %+v", deltas)
	}
	if scores["a"] != 2000 {
		t.Fatalf("unexpected cumulative score for splitter: %+v", scores)
	}
}

func TestResolveRoundAwardsOneVoterWhenSplitterExcluded(t *testing.T) {
	t.Parallel()

	state := &engine.GameState{
		Players: map[engine.PlayerID]*engine.Player{
			"a": {ID: "a", Name: "Alice", Connected: true},
			"b": {ID: "b", Name: "Bob", Connected: true},
		},
		PhaseData: map[string]engine.PhaseResult{},
		Scores:    map[engine.PlayerID]int{},
	}

	engine.PutResult(state, "split_setup_r1", "splitthevote.setup.r1", SetupResult{
		Round:        1,
		SplitterID:   "a",
		SplitterName: "Alice",
		Prompt:       "Which side wins?",
		OptionA:      "Left",
		OptionB:      "Right",
		TargetA:      0,
		TargetB:      1,
		TargetMode:   "strict_split",
		ShowTarget:   false,
	})
	engine.PutResult(state, "split_vote_r1", "splitthevote.vote.r1", VoteResult{
		ByPlayer: map[string]string{
			"b": "B",
		},
		CountA: 0,
		CountB: 1,
	})

	reveal, scores, deltas, err := resolveRound(state, 1)
	if err != nil {
		t.Fatalf("resolveRound: %v", err)
	}
	if !reveal.Achieved {
		t.Fatal("expected one-voter round to achieve target")
	}
	if deltas["a"] != 1500 {
		t.Fatalf("splitter should get 1500, got %d", deltas["a"])
	}
	if deltas["b"] != 500 {
		t.Fatalf("voter should get 500, got %d", deltas["b"])
	}
	if scores["a"] != 1500 || scores["b"] != 500 {
		t.Fatalf("unexpected cumulative scores: %+v", scores)
	}
}
