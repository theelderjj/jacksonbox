package drawduel

import (
	"testing"

	"github.com/jj/trivia/internal/engine"
	"github.com/jj/trivia/internal/proto"
)

func TestBuildSetupRotatesArtistsByRound(t *testing.T) {
	t.Parallel()

	state := &engine.GameState{
		Players: map[engine.PlayerID]*engine.Player{
			"z": {ID: "z", Name: "Cara", Connected: true},
			"a": {ID: "a", Name: "Alice", Connected: true},
			"b": {ID: "b", Name: "Bob", Connected: true},
		},
	}

	round1 := buildSetup(state, 1)
	if round1.ArtistAName != "Alice" || round1.ArtistBName != "Bob" {
		t.Fatalf("round 1 artists: want Alice/Bob got %s/%s", round1.ArtistAName, round1.ArtistBName)
	}
	if len(round1.JudgeIDs) != 1 || round1.JudgeIDs[0] != "z" {
		t.Fatalf("round 1 judges: want [z], got %+v", round1.JudgeIDs)
	}

	round2 := buildSetup(state, 2)
	if round2.ArtistAName != "Cara" || round2.ArtistBName != "Alice" {
		t.Fatalf("round 2 artists: want Cara/Alice got %s/%s", round2.ArtistAName, round2.ArtistBName)
	}
}

func TestResolveRevealAwardsWinnerAndCorrectJudges(t *testing.T) {
	t.Parallel()

	state := &engine.GameState{
		Players: map[engine.PlayerID]*engine.Player{
			"a": {ID: "a", Name: "Alice", Connected: true},
			"b": {ID: "b", Name: "Bob", Connected: true},
			"c": {ID: "c", Name: "Cara", Connected: true},
		},
		PhaseData: map[string]engine.PhaseResult{},
		Scores:    map[engine.PlayerID]int{},
	}

	engine.PutResult(state, "draw_duel_prompt_r1", "drawduel.prompt.r1", SetupResult{
		Round:       1,
		Prompt:      "Draw a tiny disaster.",
		ArtistAID:   "a",
		ArtistAName: "Alice",
		ArtistBID:   "b",
		ArtistBName: "Bob",
		JudgeIDs:    []string{"c"},
	})
	engine.PutResult(state, "draw_duel_draw_r1", "drawduel.draw.r1", DrawingResult{
		ByPlayer: map[string]proto.DrawingSummary{
			"a": {DrawingID: "a", AuthorID: "a", Format: "strokes", Data: `{"strokes":[]}`},
			"b": {DrawingID: "b", AuthorID: "b", Format: "strokes", Data: `{"strokes":[]}`},
		},
	})
	engine.PutResult(state, "draw_duel_vote_r1", "drawduel.vote.r1", VoteResult{
		ByJudge: map[string]string{"c": "b"},
	})

	reveal, deltas, scores, err := resolveReveal(state, 1)
	if err != nil {
		t.Fatalf("resolveReveal: %v", err)
	}
	if reveal.WinnerArtistID != "b" {
		t.Fatalf("winner artist: want b got %s", reveal.WinnerArtistID)
	}
	if deltas["b"] != 1000 || deltas["c"] != 500 {
		t.Fatalf("unexpected deltas: %+v", deltas)
	}
	if scores["b"] != 1000 || scores["c"] != 500 {
		t.Fatalf("unexpected scores: %+v", scores)
	}
}
