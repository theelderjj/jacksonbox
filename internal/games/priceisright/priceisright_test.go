package priceisright

import (
	"encoding/json"
	"testing"

	"github.com/jj/trivia/internal/engine"
)

func TestWinningGuessesClosestWithoutGoingOver(t *testing.T) {
	t.Parallel()

	winners := winningGuesses(15000, map[string]int{
		"alice": 14900,
		"bob":   15100,
		"cara":  12000,
	})
	if len(winners) != 1 || winners[0] != "alice" {
		t.Fatalf("expected alice to win, got %+v", winners)
	}
}

func TestWinningGuessesTie(t *testing.T) {
	t.Parallel()

	winners := winningGuesses(15000, map[string]int{
		"alice": 14900,
		"bob":   14900,
		"cara":  12000,
	})
	if len(winners) != 2 || winners[0] != "alice" || winners[1] != "bob" {
		t.Fatalf("expected tie between alice and bob, got %+v", winners)
	}
}

func TestEffectiveThresholdAndSelection(t *testing.T) {
	t.Parallel()

	state := &engine.GameState{
		RoomID: "PRICE",
		Players: map[engine.PlayerID]*engine.Player{
			"a": {ID: "a", Name: "Alice", Connected: true},
			"b": {ID: "b", Name: "Bob", Connected: true},
		},
		Settings: engine.GameSettings{
			GameOptions: map[string]any{
				"minimum_price_dollars": 50,
				"threshold_mode":        "times_ten",
			},
		},
	}

	prompt := buildPrompt(state, 2)
	if prompt.ThresholdCents != 50000 {
		t.Fatalf("round 2 threshold: want 50000, got %d", prompt.ThresholdCents)
	}
	if prompt.ActualPriceCents <= prompt.ThresholdCents {
		t.Fatalf("selected product should exceed threshold: price=%d threshold=%d", prompt.ActualPriceCents, prompt.ThresholdCents)
	}
}

func TestRealSnapshotHasTenThousandProducts(t *testing.T) {
	t.Parallel()

	if got := len(products); got != 10000 {
		t.Fatalf("snapshot catalog size: want 10000, got %d", got)
	}
}

func TestRealSnapshotEntriesHaveImagesAndPrices(t *testing.T) {
	t.Parallel()

	for idx, product := range products {
		if product.ID == "" {
			t.Fatalf("product %d missing id", idx)
		}
		if product.Name == "" {
			t.Fatalf("product %d missing name", idx)
		}
		if product.PriceCents <= 0 {
			t.Fatalf("product %d has non-positive price %d", idx, product.PriceCents)
		}
		if product.ImageURL == "" {
			t.Fatalf("product %d missing image url", idx)
		}
	}
}

func TestRealSnapshotPriceMix(t *testing.T) {
	t.Parallel()

	over1000 := 0
	over5000 := 0
	for _, product := range products {
		if product.PriceCents >= 100000 {
			over1000++
		}
		if product.PriceCents >= 500000 {
			over5000++
		}
	}
	if over1000 != 3000 {
		t.Fatalf("products >= $1000: want 3000, got %d", over1000)
	}
	if over5000 != 1000 {
		t.Fatalf("products >= $5000: want 1000, got %d", over5000)
	}
}

func TestRealSnapshotCategoryDiversity(t *testing.T) {
	t.Parallel()

	var rawProducts []struct {
		Category   string `json:"category"`
		Source     string `json:"source"`
		PriceCents int    `json:"price_cents"`
	}
	if err := json.Unmarshal(productsSnapshotJSON, &rawProducts); err != nil {
		t.Fatalf("decode snapshot json: %v", err)
	}

	categories := map[string]struct{}{}
	expensiveCategories := map[string]struct{}{}
	for _, product := range rawProducts {
		if product.Category != "" {
			categories[product.Category] = struct{}{}
			if product.PriceCents >= 500000 {
				expensiveCategories[product.Category] = struct{}{}
			}
		}
	}
	if len(categories) < 12 {
		t.Fatalf("category diversity: want at least 12 categories, got %d", len(categories))
	}
	if len(expensiveCategories) < 5 {
		t.Fatalf("expensive category diversity: want at least 5 categories, got %d", len(expensiveCategories))
	}
}
