package catalog

import "testing"

func TestDefaultCatalogIncludesJrawful(t *testing.T) {
	t.Parallel()

	game, ok := FindGame("jrawful")
	if !ok {
		t.Fatal("expected jrawful in catalog")
	}
	if game.Status != GameStatusAvailable {
		t.Fatalf("expected jrawful available, got %s", game.Status)
	}
}

func TestPlannedGamesPresentButNotSelectable(t *testing.T) {
	t.Parallel()

	if _, ok := FindGame("chain_reaction"); !ok {
		t.Fatal("expected chain_reaction in catalog")
	}
	if err := CanSelectGame("chain_reaction", 4); err == nil {
		t.Fatal("expected planned game selection to fail")
	}
}

func TestCanSelectGameRejectsUnknownAndPlayerCount(t *testing.T) {
	t.Parallel()

	if err := CanSelectGame("missing", 3); err == nil {
		t.Fatal("expected unknown game rejection")
	}
	if err := CanSelectGame("jrawful", 2); err == nil {
		t.Fatal("expected min-player rejection")
	}
	if err := CanSelectGame("jrawful", 21); err == nil {
		t.Fatal("expected max-player rejection")
	}
	if err := CanSelectGame("jrawful", 3); err != nil {
		t.Fatalf("expected jrawful selectable at 3 players, got %v", err)
	}
	if err := CanSelectGame("fake_artist", 4); err != nil {
		t.Fatalf("expected fake_artist selectable at 4 players, got %v", err)
	}
	if err := CanSelectGame("draw_duel", 3); err != nil {
		t.Fatalf("expected draw_duel selectable at 3 players, got %v", err)
	}
	if err := CanSelectGame("word_storm", 2); err != nil {
		t.Fatalf("expected word_storm selectable at 2 players, got %v", err)
	}
	if err := CanSelectGame("price_is_right", 2); err != nil {
		t.Fatalf("expected price_is_right selectable at 2 players, got %v", err)
	}
	if err := CanSelectGame("mafia", 6); err != nil {
		t.Fatalf("expected mafia selectable at 6 players, got %v", err)
	}
}

func TestCatalogOrderStable(t *testing.T) {
	t.Parallel()

	got := DefaultCatalog()
	want := []string{"jrawful", "fake_artist", "draw_duel", "word_storm", "chain_reaction", "reaction_duel", "price_is_right", "split_vote", "mafia"}
	if len(got) != len(want) {
		t.Fatalf("catalog length mismatch: want %d got %d", len(want), len(got))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("catalog order[%d]: want %s got %s", i, id, got[i].ID)
		}
	}
}
