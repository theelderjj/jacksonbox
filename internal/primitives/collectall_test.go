package primitives

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jj/trivia/internal/engine"
)

// testState builds a minimal GameState with n connected players.
func testState(n int) (*engine.GameState, []engine.PlayerID) {
	s := &engine.GameState{
		Players:   map[engine.PlayerID]*engine.Player{},
		PhaseData: map[string]engine.PhaseResult{},
		Scores:    map[engine.PlayerID]int{},
	}
	ids := make([]engine.PlayerID, 0, n)
	for i := 0; i < n; i++ {
		id := engine.PlayerID(string(rune('A' + i)))
		s.Players[id] = &engine.Player{ID: id, Connected: true}
		ids = append(ids, id)
	}
	return s, ids
}

func stubCtx(s *engine.GameState) *engine.PhaseContext {
	return &engine.PhaseContext{
		State:     s,
		Phase:     engine.Phase{Name: "test"},
		Deadline:  time.Now().Add(time.Second),
		Now:       time.Now,
		Broadcast: func(engine.Event) {},
	}
}

type item struct {
	Value int `json:"value"`
}

func intDecoder(in engine.Input, _ *engine.GameState) (item, error) {
	var it item
	if err := json.Unmarshal(in.Value, &it); err != nil {
		return item{}, err
	}
	if it.Value < 0 {
		return item{}, errors.New("negative")
	}
	return it, nil
}

func TestCollectAll_Happy(t *testing.T) {
	t.Log("Scenario: 3 connected players each submit one valid item in order.")
	t.Log("Expected: Start does not advance; each of the first N-1 submissions holds; the Nth submission advances the phase.")
	s, ids := testState(3)
	c := NewCollectAll[item]("test", intDecoder, nil, nil)
	ctx := stubCtx(s)

	if d, _ := c.Start(ctx); d.AdvancePhase {
		t.Fatal("should not advance on Start with active players")
	}
	for i, id := range ids {
		raw, _ := json.Marshal(item{Value: i + 1})
		in := engine.Input{PlayerID: id, Value: raw}
		d, err := c.Handle(ctx, in)
		if err != nil {
			t.Fatalf("handle #%d err: %v", i, err)
		}
		if i < len(ids)-1 && d.AdvancePhase {
			t.Fatalf("advanced too early at #%d", i)
		}
		if i == len(ids)-1 && !d.AdvancePhase {
			t.Fatal("expected final submission to advance")
		}
	}
}

func TestCollectAll_TimeoutAdvancesWithPartial(t *testing.T) {
	t.Log("Scenario: 3 players are connected but only player A submits before the phase deadline.")
	t.Log("Expected: Timeout advances the phase anyway and the stored result contains exactly 1 item (A's).")
	s, ids := testState(3)
	c := NewCollectAll[item]("test", intDecoder, nil, nil)
	ctx := stubCtx(s)

	raw, _ := json.Marshal(item{Value: 7})
	_, _ = c.Handle(ctx, engine.Input{PlayerID: ids[0], Value: raw})

	d, err := c.Timeout(ctx)
	if err != nil {
		t.Fatalf("timeout err: %v", err)
	}
	if !d.AdvancePhase {
		t.Fatal("timeout must advance")
	}
	if len(d.Result.Items) != 1 {
		t.Fatalf("want 1 item, got %d", len(d.Result.Items))
	}
}

func TestCollectAll_DuplicateRejected(t *testing.T) {
	t.Log("Scenario: the same player submits twice in the same collection phase.")
	t.Log("Expected: second submission returns an error — exactly one item per player is allowed.")
	s, ids := testState(2)
	c := NewCollectAll[item]("test", intDecoder, nil, nil)
	ctx := stubCtx(s)

	raw, _ := json.Marshal(item{Value: 1})
	_, err := c.Handle(ctx, engine.Input{PlayerID: ids[0], Value: raw})
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}
	_, err = c.Handle(ctx, engine.Input{PlayerID: ids[0], Value: raw})
	if err == nil {
		t.Fatal("expected duplicate rejection")
	}
}

func TestCollectAll_InvalidRejected(t *testing.T) {
	t.Log("Scenario: a submission passes JSON parsing but fails the decoder's domain validation (negative value).")
	t.Log("Expected: Handle returns the decoder's error and does not record the submission.")
	s, ids := testState(1)
	c := NewCollectAll[item]("test", intDecoder, nil, nil)
	ctx := stubCtx(s)

	raw, _ := json.Marshal(item{Value: -1})
	_, err := c.Handle(ctx, engine.Input{PlayerID: ids[0], Value: raw})
	if err == nil {
		t.Fatal("expected decoder rejection")
	}
}

func TestCollectAll_ShortCircuitOnAllSubmitted(t *testing.T) {
	t.Log("Scenario: every eligible player has submitted before the phase deadline elapses.")
	t.Log("Expected: the final Handle returns AdvancePhase=true — we don't wait out the timer when we already have everyone.")
	s, ids := testState(2)
	c := NewCollectAll[item]("test", intDecoder, nil, nil)
	ctx := stubCtx(s)

	raw, _ := json.Marshal(item{Value: 1})
	_, _ = c.Handle(ctx, engine.Input{PlayerID: ids[0], Value: raw})
	d, _ := c.Handle(ctx, engine.Input{PlayerID: ids[1], Value: raw})
	if !d.AdvancePhase {
		t.Fatal("expected advance when all submitted")
	}
}

func TestCollectAll_StartAdvancesOnEmptyEligible(t *testing.T) {
	t.Log("Scenario: phase starts with zero eligible players (empty room / everyone filtered out).")
	t.Log("Expected: Start advances immediately — otherwise the phase would hang until timeout for no reason.")
	s := &engine.GameState{
		Players:   map[engine.PlayerID]*engine.Player{},
		PhaseData: map[string]engine.PhaseResult{},
		Scores:    map[engine.PlayerID]int{},
	}
	c := NewCollectAll[item]("test", intDecoder, nil, nil)
	ctx := stubCtx(s)
	d, _ := c.Start(ctx)
	if !d.AdvancePhase {
		t.Fatal("empty room should advance immediately")
	}
}

func TestCollectAll_EligibilityFilters(t *testing.T) {
	t.Log("Scenario: an eligibility predicate permits only player A; B attempts to submit anyway, then A submits.")
	t.Log("Expected: B's submission is rejected; A's submission advances the phase (she was the only eligible one).")
	s, ids := testState(3)
	// Only first player eligible.
	eligible := func(pid engine.PlayerID, _ *engine.GameState) bool {
		return pid == ids[0]
	}
	c := NewCollectAll[item]("test", intDecoder, eligible, nil)
	ctx := stubCtx(s)

	raw, _ := json.Marshal(item{Value: 1})
	_, err := c.Handle(ctx, engine.Input{PlayerID: ids[1], Value: raw})
	if err == nil {
		t.Fatal("expected eligibility rejection for ids[1]")
	}
	d, err := c.Handle(ctx, engine.Input{PlayerID: ids[0], Value: raw})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !d.AdvancePhase {
		t.Fatal("advance expected when all eligibles submitted")
	}
}
