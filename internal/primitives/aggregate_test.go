package primitives

import (
	"errors"
	"testing"

	"github.com/jj/trivia/internal/engine"
)

func TestAggregate_HappyPath(t *testing.T) {
	t.Log("Scenario: an Aggregate primitive's fold returns value=42 and a single `tick` broadcast, no error.")
	t.Log("Expected: Start advances the phase in one shot, Result stores 42, Broadcast contains exactly one event.")
	agg := NewAggregate("agg", func(s *engine.GameState) (int, []engine.Event, error) {
		return 42, []engine.Event{{Type: "tick"}}, nil
	})
	s, _ := testState(1)
	ctx := stubCtx(s)
	d, err := agg.Start(ctx)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if !d.AdvancePhase {
		t.Fatal("aggregate must advance in Start")
	}
	if d.Result != 42 {
		t.Fatalf("want 42, got %v", d.Result)
	}
	if len(d.Broadcast) != 1 {
		t.Fatalf("want 1 event, got %d", len(d.Broadcast))
	}
}

func TestAggregate_ErrorPropagates(t *testing.T) {
	t.Log("Scenario: the fold function returns an error.")
	t.Log("Expected: Start surfaces the error to the caller rather than silently advancing — the actor's recover path will handle it.")
	agg := NewAggregate("agg", func(*engine.GameState) (int, []engine.Event, error) {
		return 0, nil, errors.New("boom")
	})
	s, _ := testState(1)
	_, err := agg.Start(stubCtx(s))
	if err == nil {
		t.Fatal("expected err")
	}
}

func TestAggregate_HandleRejectsInputs(t *testing.T) {
	t.Log("Scenario: a client-origin Input arrives during an Aggregate phase (shouldn't happen — Aggregate is pure server-side).")
	t.Log("Expected: Handle returns an error — Aggregate phases do not accept player inputs.")
	agg := NewAggregate("agg", func(*engine.GameState) (int, []engine.Event, error) {
		return 0, nil, nil
	})
	s, ids := testState(1)
	_, err := agg.Handle(stubCtx(s), engine.Input{PlayerID: ids[0]})
	if err == nil {
		t.Fatal("aggregate should reject Handle")
	}
}

func TestScoreTransform_AppliesDeltas(t *testing.T) {
	t.Log("Scenario: current scores A=200, B=0; fold returns deltas A=+100, B=+50.")
	t.Log("Expected: post-phase state has A=300 and B=50 (deltas added, not replaced).")
	st := NewScoreTransform("score", func(s *engine.GameState) (map[engine.PlayerID]int, []engine.Event, error) {
		return map[engine.PlayerID]int{
			"A": 100, "B": 50,
		}, nil, nil
	})
	s, _ := testState(2)
	s.Scores["A"] = 200
	s.Scores["B"] = 0

	d, err := st.Start(stubCtx(s))
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if !d.AdvancePhase {
		t.Fatal("score transform must advance")
	}
	if got := s.Scores["A"]; got != 300 {
		t.Fatalf("A: want 300, got %d", got)
	}
	if got := s.Scores["B"]; got != 50 {
		t.Fatalf("B: want 50, got %d", got)
	}
}

func TestScoreTransform_NoNegative(t *testing.T) {
	t.Log("Scenario: current score A=10; fold returns a negative delta A=-50 that would drop the score below zero.")
	t.Log("Expected: final score clamps to 0 — scores never go negative, per spec §7.")
	st := NewScoreTransform("score", func(*engine.GameState) (map[engine.PlayerID]int, []engine.Event, error) {
		return map[engine.PlayerID]int{"A": -50}, nil, nil
	})
	s, _ := testState(1)
	s.Scores["A"] = 10
	if _, err := st.Start(stubCtx(s)); err != nil {
		t.Fatalf("start: %v", err)
	}
	if s.Scores["A"] != 0 {
		t.Fatalf("expected clamp to 0, got %d", s.Scores["A"])
	}
}
