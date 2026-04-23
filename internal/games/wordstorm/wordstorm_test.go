package wordstorm

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jj/trivia/internal/engine"
	"github.com/jj/trivia/internal/proto"
)

func TestWordPointsGrowFifteenPercentPastFourLetters(t *testing.T) {
	t.Parallel()

	if got := wordPoints("cat"); got != 300 {
		t.Fatalf("cat points = %d, want 300", got)
	}
	if got := wordPoints("camel"); got != 575 {
		t.Fatalf("camel points = %d, want 575", got)
	}
	if got := wordPoints("cabinet"); got != 1065 {
		t.Fatalf("cabinet points = %d, want 1065", got)
	}
}

func TestSubmitRejectsAlreadyEnteredWordsImmediately(t *testing.T) {
	state := testState()
	engine.PutResult(state, "word_prompt", regKeyPrompt, PromptResult{
		Letters:       []LetterWindow{{Letter: "c", Index: 0}},
		LetterSeconds: 20,
	})
	prim := newSubmit()
	now := time.Now()
	ctx := &engine.PhaseContext{
		State:    state,
		Phase:    engine.Phase{Duration: 20 * time.Second},
		Deadline: now.Add(20 * time.Second),
		Now:      func() time.Time { return now },
	}

	raw, _ := json.Marshal(proto.SubmitWordListPayload{Words: []string{"cat"}})
	if _, err := prim.Handle(ctx, engine.Input{PlayerID: "p1", Value: raw}); err != nil {
		t.Fatalf("first submit error: %v", err)
	}
	raw, _ = json.Marshal(proto.SubmitWordListPayload{Words: []string{"cat"}})
	if _, err := prim.Handle(ctx, engine.Input{PlayerID: "p2", Value: raw}); err != nil {
		t.Fatalf("duplicate submit error: %v", err)
	}
	decision, err := prim.Timeout(ctx)
	if err != nil {
		t.Fatalf("timeout error: %v", err)
	}
	result := decision.Result
	if len(result.Entries) != 2 {
		t.Fatalf("entries length = %d, want 2", len(result.Entries))
	}
	if result.Entries[1].Status != "duplicate" {
		t.Fatalf("second entry status = %q, want duplicate", result.Entries[1].Status)
	}
	if len(prim.claimed) != 1 {
		t.Fatalf("claimed ledger size = %d, want exactly one committed claim", len(prim.claimed))
	}
	if claim := prim.claimed["cat"]; claim.PlayerID != "p1" || claim.Sequence != 1 {
		t.Fatalf("claim = %#v, want first player and sequence 1", claim)
	}
}

func TestMadeUpCurrentLetterWordStillClaimsFirstUse(t *testing.T) {
	state := testState()
	engine.PutResult(state, "word_prompt", regKeyPrompt, PromptResult{
		Letters:       []LetterWindow{{Letter: "c", Index: 0}},
		LetterSeconds: 20,
	})
	prim := newSubmit()
	now := time.Now()
	ctx := &engine.PhaseContext{
		State:    state,
		Phase:    engine.Phase{Duration: 20 * time.Second},
		Deadline: now.Add(20 * time.Second),
		Now:      func() time.Time { return now },
	}

	raw, _ := json.Marshal(proto.SubmitWordListPayload{Words: []string{"czzzzz"}})
	if _, err := prim.Handle(ctx, engine.Input{PlayerID: "p1", Value: raw}); err != nil {
		t.Fatalf("first made-up submit error: %v", err)
	}
	raw, _ = json.Marshal(proto.SubmitWordListPayload{Words: []string{"czzzzz"}})
	if _, err := prim.Handle(ctx, engine.Input{PlayerID: "p2", Value: raw}); err != nil {
		t.Fatalf("duplicate made-up submit error: %v", err)
	}

	decision, err := prim.Timeout(ctx)
	if err != nil {
		t.Fatalf("timeout error: %v", err)
	}
	if decision.Result.Entries[0].Status != "pending" {
		t.Fatalf("first made-up status = %q, want pending before scoring", decision.Result.Entries[0].Status)
	}
	if decision.Result.Entries[1].Status != "duplicate" {
		t.Fatalf("duplicate made-up status = %q, want duplicate", decision.Result.Entries[1].Status)
	}
}

func TestWrongLetterDoesNotClaimWord(t *testing.T) {
	state := testState()
	engine.PutResult(state, "word_prompt", regKeyPrompt, PromptResult{
		Letters:       []LetterWindow{{Letter: "c", Index: 0}},
		LetterSeconds: 20,
	})
	prim := newSubmit()
	now := time.Now()
	ctx := &engine.PhaseContext{
		State:    state,
		Phase:    engine.Phase{Duration: 20 * time.Second},
		Deadline: now.Add(20 * time.Second),
		Now:      func() time.Time { return now },
	}

	raw, _ := json.Marshal(proto.SubmitWordListPayload{Words: []string{"banana"}})
	if _, err := prim.Handle(ctx, engine.Input{PlayerID: "p1", Value: raw}); err != nil {
		t.Fatalf("wrong-letter submit error: %v", err)
	}
	if len(prim.claimed) != 0 {
		t.Fatalf("wrong-letter word should not claim ledger, got %#v", prim.claimed)
	}
}

func TestScoreGameOnlyScoresAtEndAndCrossesInvalid(t *testing.T) {
	state := testState()
	engine.PutResult(state, "word_prompt", regKeyPrompt, PromptResult{
		Letters:             []LetterWindow{{Letter: "c", Index: 0}},
		LetterSeconds:       20,
		BasePointsPerLetter: 100,
		GrowthPercent:       15,
	})
	engine.PutResult(state, "word_submit", regKeySubmit, SubmitResult{
		Entries: []WordEntry{
			{PlayerID: "p1", PlayerName: "Alice", Word: "camel", Letter: "c", Status: "pending"},
			{PlayerID: "p1", PlayerName: "Alice", Word: "notaword", Letter: "c", Status: "pending"},
			{PlayerID: "p2", PlayerName: "Bob", Word: "cat", Letter: "c", Status: "pending"},
		},
	})

	result, deltas, scores, err := scoreGame(state)
	if err != nil {
		t.Fatalf("scoreGame returned error: %v", err)
	}

	if deltas["p1"] != 575 {
		t.Fatalf("p1 delta = %d, want 575", deltas["p1"])
	}
	if deltas["p2"] != 300 {
		t.Fatalf("p2 delta = %d, want 300", deltas["p2"])
	}
	if scores["p1"] != 575 || scores["p2"] != 300 {
		t.Fatalf("scores = %#v, want p1=575 p2=300", scores)
	}
	p1 := result.Results[0]
	if p1.Entries[1].Status != "invalid" {
		t.Fatalf("invalid entry status = %q, want invalid", p1.Entries[1].Status)
	}
	if p1.MadeUpCount != 1 {
		t.Fatalf("made up count = %d, want 1", p1.MadeUpCount)
	}
}

func TestNormalizeWord(t *testing.T) {
	t.Parallel()

	if got := normalizeWord(" Co-Bra! "); got != "cobra" {
		t.Fatalf("normalizeWord = %q, want cobra", got)
	}
}

func testState() *engine.GameState {
	return &engine.GameState{
		RoomID: "TEST",
		Players: map[engine.PlayerID]*engine.Player{
			"p1": {ID: "p1", Name: "Alice", Connected: true},
			"p2": {ID: "p2", Name: "Bob", Connected: true},
		},
		Settings:  engine.GameSettings{RoundCount: 1, GameOptions: map[string]any{"letter_seconds": 20}},
		PhaseData: map[string]engine.PhaseResult{},
		Scores:    map[engine.PlayerID]int{},
	}
}
