package fakeartist

import (
	"strings"
	"testing"

	"github.com/jj/trivia/internal/engine"
)

func TestMajorityAccusation(t *testing.T) {
	accused, majority := majorityAccusation(map[string]string{
		"a": "p2",
		"b": "p2",
		"c": "p1",
		"d": "p2",
	})
	if !majority {
		t.Fatalf("expected a majority accusation")
	}
	if accused != "p2" {
		t.Fatalf("expected p2 to be accused, got %q", accused)
	}
}

func TestMergeStrokeDataAppendsFrames(t *testing.T) {
	left := `{"strokes":[{"points":[[1,1]],"color":"#111","width":4}]}`
	right := `{"strokes":[{"points":[[2,2]],"color":"#222","width":5}]}`
	merged := mergeStrokeData(left, right)
	want := `{"strokes":[{"color":"#111","points":[[1,1]],"width":4},{"color":"#222","points":[[2,2]],"width":5}]}`
	if merged != want {
		t.Fatalf("unexpected merged strokes:\nwant %s\ngot  %s", want, merged)
	}
}

func TestBuildRoleResultRepeatsDrawOrderTwice(t *testing.T) {
	state := &engine.GameState{
		Players: map[engine.PlayerID]*engine.Player{
			"a": {ID: "a", Name: "Alice", Connected: true},
			"b": {ID: "b", Name: "Bob", Connected: true},
			"c": {ID: "c", Name: "Cara", Connected: true},
			"d": {ID: "d", Name: "Drew", Connected: true},
		},
	}

	result := buildRoleResult(state, 1)
	if len(result.Order) != 4 {
		t.Fatalf("unique player order: want 4, got %d", len(result.Order))
	}
	if len(result.DrawOrder) != 8 {
		t.Fatalf("draw order length: want 8, got %d", len(result.DrawOrder))
	}
	for idx, playerID := range result.Order {
		if result.DrawOrder[idx] != playerID {
			t.Fatalf("first pass turn %d: want %q, got %q", idx, playerID, result.DrawOrder[idx])
		}
		if result.DrawOrder[idx+len(result.Order)] != playerID {
			t.Fatalf("second pass turn %d: want %q, got %q", idx, playerID, result.DrawOrder[idx+len(result.Order)])
		}
	}
}

func TestPromptForPlayerHidesPromptFromFakeArtist(t *testing.T) {
	role := RoleResult{
		Round:  2,
		Prompt: "roller coaster",
		FakeID: "fake",
	}

	fakePrompt := promptForPlayer(role, "fake")
	if fakePrompt == "" {
		t.Fatal("fake prompt should contain instructions")
	}
	if fakePrompt == "Draw this together: roller coaster" {
		t.Fatal("fake artist should not receive the real drawing prompt")
	}
	if strings.Contains(fakePrompt, "roller coaster") {
		t.Fatalf("fake artist instructions leaked the prompt: %q", fakePrompt)
	}

	realPrompt := promptForPlayer(role, "town")
	if realPrompt != "Draw this together: roller coaster" {
		t.Fatalf("real player prompt mismatch: got %q", realPrompt)
	}
}
