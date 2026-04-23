package catalog

import "fmt"

type GameStatus string

const (
	GameStatusAvailable GameStatus = "available"
	GameStatusPlanned   GameStatus = "planned"
)

type GameDefinition struct {
	ID               string
	Name             string
	Summary          string
	MinPlayers       int
	MaxPlayers       int
	EstimatedMinutes int
	Tags             []string
	Status           GameStatus
}

func DefaultCatalog() []GameDefinition {
	return []GameDefinition{
		{
			ID:               "jrawful",
			Name:             "Jrawful",
			Summary:          "Draw strange prompts, fake the answers, and vote for the truth.",
			MinPlayers:       3,
			MaxPlayers:       20,
			EstimatedMinutes: 20,
			Tags:             []string{"drawing", "bluffing", "voting"},
			Status:           GameStatusAvailable,
		},
		{
			ID:               "fake_artist",
			Name:             "Fake Artist",
			Summary:          "Everyone draws the same prompt, except one player who has to fake it.",
			MinPlayers:       4,
			MaxPlayers:       12,
			EstimatedMinutes: 10,
			Tags:             []string{"drawing", "deduction", "roles"},
			Status:           GameStatusAvailable,
		},
		{
			ID:               "draw_duel",
			Name:             "Draw Duel",
			Summary:          "Two artists draw head-to-head while everyone else judges.",
			MinPlayers:       3,
			MaxPlayers:       20,
			EstimatedMinutes: 8,
			Tags:             []string{"drawing", "voting", "quick"},
			Status:           GameStatusAvailable,
		},
		{
			ID:               "word_storm",
			Name:             "Word Storm",
			Summary:          "Race to submit unique words that match the category.",
			MinPlayers:       2,
			MaxPlayers:       20,
			EstimatedMinutes: 8,
			Tags:             []string{"word", "speed", "party"},
			Status:           GameStatusAvailable,
		},
		{
			ID:               "chain_reaction",
			Name:             "Chain Reaction",
			Summary:          "Build a last-letter word chain before the turn timer runs out.",
			MinPlayers:       2,
			MaxPlayers:       20,
			EstimatedMinutes: 8,
			Tags:             []string{"word", "turns", "quick"},
			Status:           GameStatusPlanned,
		},
		{
			ID:               "reaction_duel",
			Name:             "Reaction Duel",
			Summary:          "Wait for green. Tap too early and you pay for it.",
			MinPlayers:       2,
			MaxPlayers:       20,
			EstimatedMinutes: 5,
			Tags:             []string{"reaction", "speed", "quick"},
			Status:           GameStatusAvailable,
		},
		{
			ID:               "price_is_right",
			Name:             "Price is Right",
			Summary:          "Guess the listing price. Closest guess without going over wins the round.",
			MinPlayers:       2,
			MaxPlayers:       20,
			EstimatedMinutes: 8,
			Tags:             []string{"guessing", "prices", "party"},
			Status:           GameStatusAvailable,
		},
		{
			ID:               "split_vote",
			Name:             "Split the Vote",
			Summary:          "Set up a prompt, split the room, and try to land the target distribution.",
			MinPlayers:       2,
			MaxPlayers:       20,
			EstimatedMinutes: 8,
			Tags:             []string{"social", "voting", "mind-games"},
			Status:           GameStatusAvailable,
		},
		{
			ID:               "mafia",
			Name:             "Mafia",
			Summary:          "Secret roles, night actions, day debates, and suspicious friends.",
			MinPlayers:       6,
			MaxPlayers:       20,
			EstimatedMinutes: 30,
			Tags:             []string{"deduction", "roles", "social"},
			Status:           GameStatusAvailable,
		},
	}
}

func FindGame(id string) (GameDefinition, bool) {
	for _, game := range DefaultCatalog() {
		if game.ID == id {
			return game, true
		}
	}
	return GameDefinition{}, false
}

func AvailableForPlayerCount(count int) []GameDefinition {
	out := make([]GameDefinition, 0, len(DefaultCatalog()))
	for _, game := range DefaultCatalog() {
		if game.Status != GameStatusAvailable {
			continue
		}
		if count < game.MinPlayers || count > game.MaxPlayers {
			continue
		}
		out = append(out, game)
	}
	return out
}

func CanSelectGame(id string, playerCount int) error {
	game, ok := FindGame(id)
	if !ok {
		return fmt.Errorf("unknown game %q", id)
	}
	if game.Status != GameStatusAvailable {
		return fmt.Errorf("%s is coming soon", game.Name)
	}
	if playerCount < game.MinPlayers {
		return fmt.Errorf("%s needs at least %d players", game.Name, game.MinPlayers)
	}
	if playerCount > game.MaxPlayers {
		return fmt.Errorf("%s supports at most %d players", game.Name, game.MaxPlayers)
	}
	return nil
}
