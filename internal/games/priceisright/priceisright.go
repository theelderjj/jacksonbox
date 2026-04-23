package priceisright

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jj/trivia/internal/engine"
	"github.com/jj/trivia/internal/proto"
)

const (
	regKeyPrompt = "priceisright.prompt"
	regKeyGuess  = "priceisright.guess"
	regKeyReveal = "priceisright.reveal"

	defaultGuessSeconds  = 20
	defaultRevealSeconds = 6
	modeFixedThreshold   = "fixed"
	modeTimesTen         = "times_ten"
)

type phaseConfig struct {
	Round int `json:"round"`
}

type Product struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PriceCents int    `json:"price_cents"`
	ImageURL   string `json:"image_url"`
	Accent     string `json:"accent"`
}

type PromptResult struct {
	Round              int    `json:"round"`
	ProductID          string `json:"product_id"`
	ProductName        string `json:"product_name"`
	ImageURL           string `json:"image_url"`
	ActualPriceCents   int    `json:"actual_price_cents"`
	ThresholdCents     int    `json:"threshold_cents"`
	ThresholdMode      string `json:"threshold_mode"`
	ThresholdBaseCents int    `json:"threshold_base_cents"`
}

type GuessResult struct {
	Guesses map[string]int `json:"guesses"`
}

var registerOnce sync.Once

func Register() {
	registerOnce.Do(func() {
		engine.Register(regKeyPrompt, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, err := readRound(cfg)
			if err != nil {
				return nil, err
			}
			return engine.Erase(newPrompt(round)), nil
		})
		engine.Register(regKeyGuess, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, err := readRound(cfg)
			if err != nil {
				return nil, err
			}
			return engine.Erase(newGuess(round)), nil
		})
		engine.Register(regKeyReveal, func(cfg json.RawMessage) (engine.AnyPrimitive, error) {
			round, err := readRound(cfg)
			if err != nil {
				return nil, err
			}
			return engine.Erase(newReveal(round)), nil
		})
	})
}

func BuildPhases(state *engine.GameState) []engine.Phase {
	rounds := state.Settings.RoundCount
	if rounds <= 0 {
		rounds = 3
	}
	guessSeconds := state.Settings.VotingSeconds
	if guessSeconds <= 0 {
		guessSeconds = defaultGuessSeconds
	}
	phases := make([]engine.Phase, 0, rounds*3)
	for round := 1; round <= rounds; round++ {
		cfg, _ := json.Marshal(phaseConfig{Round: round})
		phases = append(phases,
			engine.Phase{
				Name:      fmt.Sprintf("price_prompt_r%d", round),
				Primitive: regKeyPrompt,
				Config:    cfg,
			},
			engine.Phase{
				Name:      fmt.Sprintf("price_guess_r%d", round),
				Primitive: regKeyGuess,
				Duration:  time.Duration(guessSeconds) * time.Second,
				Config:    cfg,
				DependsOn: []string{fmt.Sprintf("price_prompt_r%d", round)},
			},
			engine.Phase{
				Name:      fmt.Sprintf("price_reveal_r%d", round),
				Primitive: regKeyReveal,
				Duration:  defaultRevealSeconds * time.Second,
				Config:    cfg,
				DependsOn: []string{fmt.Sprintf("price_prompt_r%d", round), fmt.Sprintf("price_guess_r%d", round)},
			},
		)
	}
	return phases
}

type promptPrimitive struct {
	round int
}

func newPrompt(round int) *promptPrimitive { return &promptPrimitive{round: round} }
func (p *promptPrimitive) Name() string    { return fmt.Sprintf("priceisright.prompt.r%d", p.round) }

func (p *promptPrimitive) Start(ctx *engine.PhaseContext) (engine.Decision[PromptResult], error) {
	result := buildPrompt(ctx.State, p.round)
	raw, _ := json.Marshal(proto.PricePromptPayload{
		Round:              result.Round,
		ProductID:          result.ProductID,
		ProductName:        result.ProductName,
		ImageURL:           result.ImageURL,
		ThresholdCents:     result.ThresholdCents,
		ThresholdMode:      result.ThresholdMode,
		ThresholdBaseCents: result.ThresholdBaseCents,
	})
	return engine.Decision[PromptResult]{
		AdvancePhase: true,
		Broadcast:    []engine.Event{{Type: proto.S2CPricePrompt, Payload: raw}},
		Result:       result,
	}, nil
}

func (*promptPrimitive) Handle(*engine.PhaseContext, engine.Input) (engine.Decision[PromptResult], error) {
	return engine.Decision[PromptResult]{AdvancePhase: false}, nil
}

func (p *promptPrimitive) Timeout(ctx *engine.PhaseContext) (engine.Decision[PromptResult], error) {
	return p.Start(ctx)
}

type guessPrimitive struct {
	round   int
	guesses map[string]int
}

func newGuess(round int) *guessPrimitive {
	return &guessPrimitive{round: round, guesses: map[string]int{}}
}

func (p *guessPrimitive) Name() string { return fmt.Sprintf("priceisright.guess.r%d", p.round) }

func (*guessPrimitive) Start(*engine.PhaseContext) (engine.Decision[GuessResult], error) {
	return engine.Decision[GuessResult]{AdvancePhase: false}, nil
}

func (p *guessPrimitive) Handle(ctx *engine.PhaseContext, in engine.Input) (engine.Decision[GuessResult], error) {
	var payload proto.SubmitPriceGuessPayload
	if err := json.Unmarshal(in.Value, &payload); err != nil {
		return engine.Decision[GuessResult]{}, err
	}
	if payload.GuessCents < 0 {
		return engine.Decision[GuessResult]{}, fmt.Errorf("guess must be positive")
	}
	playerID := string(in.PlayerID)
	p.guesses[playerID] = payload.GuessCents
	if len(p.guesses) >= connectedPlayerCount(ctx.State) {
		return engine.Decision[GuessResult]{
			AdvancePhase: true,
			Result:       GuessResult{Guesses: cloneGuessMap(p.guesses)},
		}, nil
	}
	return engine.Decision[GuessResult]{AdvancePhase: false}, nil
}

func (p *guessPrimitive) Timeout(*engine.PhaseContext) (engine.Decision[GuessResult], error) {
	return engine.Decision[GuessResult]{
		AdvancePhase: true,
		Result:       GuessResult{Guesses: cloneGuessMap(p.guesses)},
	}, nil
}

type revealPrimitive struct {
	round    int
	resolved bool
	cached   proto.PriceResultPayload
}

func newReveal(round int) *revealPrimitive { return &revealPrimitive{round: round} }
func (p *revealPrimitive) Name() string    { return fmt.Sprintf("priceisright.reveal.r%d", p.round) }

func (p *revealPrimitive) Start(ctx *engine.PhaseContext) (engine.Decision[proto.PriceResultPayload], error) {
	if !p.resolved {
		reveal, deltas, scores, err := scoreRound(ctx.State, p.round)
		if err != nil {
			return engine.Decision[proto.PriceResultPayload]{}, err
		}
		p.cached = reveal
		p.resolved = true
		revealJSON, _ := json.Marshal(reveal)
		roundJSON, _ := json.Marshal(proto.RoundResultPayload{
			Round:  p.round,
			Deltas: deltas,
			Scores: scores,
		})
		return engine.Decision[proto.PriceResultPayload]{
			Broadcast: []engine.Event{
				{Type: proto.S2CPriceResult, Payload: revealJSON},
				{Type: proto.S2CRoundResult, Payload: roundJSON},
			},
		}, nil
	}
	return engine.Decision[proto.PriceResultPayload]{AdvancePhase: false}, nil
}

func (*revealPrimitive) Handle(*engine.PhaseContext, engine.Input) (engine.Decision[proto.PriceResultPayload], error) {
	return engine.Decision[proto.PriceResultPayload]{AdvancePhase: false}, nil
}

func (p *revealPrimitive) Timeout(*engine.PhaseContext) (engine.Decision[proto.PriceResultPayload], error) {
	return engine.Decision[proto.PriceResultPayload]{
		AdvancePhase: true,
		Result:       p.cached,
	}, nil
}

func buildPrompt(state *engine.GameState, round int) PromptResult {
	mode := thresholdMode(state.Settings.GameOptions)
	baseCents := thresholdBaseCents(state.Settings.GameOptions)
	product, thresholdCents := chooseProduct(state, round, baseCents, mode)
	imageURL := product.ImageURL
	if strings.TrimSpace(imageURL) == "" {
		imageURL = imageDataURL(product)
	}
	return PromptResult{
		Round:              round,
		ProductID:          product.ID,
		ProductName:        product.Name,
		ImageURL:           imageURL,
		ActualPriceCents:   product.PriceCents,
		ThresholdCents:     thresholdCents,
		ThresholdMode:      mode,
		ThresholdBaseCents: baseCents,
	}
}

func chooseProduct(state *engine.GameState, round, baseCents int, mode string) (Product, int) {
	threshold := effectiveThreshold(baseCents, mode, round)
	if product, ok := ebayProduct(context.Background(), state, round, threshold); ok {
		return product, threshold
	}
	filtered := eligibleProducts(threshold)
	if len(filtered) == 0 {
		threshold = maxSupportedThreshold(baseCents, mode, round)
		filtered = eligibleProducts(threshold)
	}
	seed := hashSeed(state.RoomID, round)
	index := 0
	if len(filtered) > 0 {
		index = seed % len(filtered)
	}
	return filtered[index], threshold
}

func eligibleProducts(threshold int) []Product {
	filtered := make([]Product, 0, len(products))
	for _, product := range products {
		if product.PriceCents > threshold {
			filtered = append(filtered, product)
		}
	}
	if len(filtered) == 0 {
		return append([]Product(nil), products...)
	}
	return filtered
}

func maxSupportedThreshold(baseCents int, mode string, round int) int {
	maxThreshold := baseCents
	for currentRound := 1; currentRound <= round; currentRound++ {
		threshold := effectiveThreshold(baseCents, mode, currentRound)
		hasAny := false
		for _, product := range products {
			if product.PriceCents > threshold {
				hasAny = true
				break
			}
		}
		if !hasAny {
			break
		}
		maxThreshold = threshold
	}
	return maxThreshold
}

func scoreRound(state *engine.GameState, round int) (proto.PriceResultPayload, map[string]int, map[string]int, error) {
	prompt, ok := engine.GetResult[PromptResult](state, fmt.Sprintf("price_prompt_r%d", round))
	if !ok {
		return proto.PriceResultPayload{}, nil, nil, fmt.Errorf("missing prompt result")
	}
	guesses, ok := engine.GetResult[GuessResult](state, fmt.Sprintf("price_guess_r%d", round))
	if !ok {
		return proto.PriceResultPayload{}, nil, nil, fmt.Errorf("missing guess result")
	}

	winnerIDs := winningGuesses(prompt.ActualPriceCents, guesses.Guesses)
	deltas := map[string]int{}
	if len(winnerIDs) > 0 {
		points := 1000 / len(winnerIDs)
		if points < 1 {
			points = 1
		}
		for _, winnerID := range winnerIDs {
			deltas[winnerID] = points
			state.Scores[engine.PlayerID(winnerID)] += points
		}
	}

	scores := map[string]int{}
	for playerID, score := range state.Scores {
		scores[string(playerID)] = score
	}
	return proto.PriceResultPayload{
		Round:            round,
		ProductID:        prompt.ProductID,
		ProductName:      prompt.ProductName,
		ImageURL:         prompt.ImageURL,
		ActualPriceCents: prompt.ActualPriceCents,
		ThresholdCents:   prompt.ThresholdCents,
		ThresholdMode:    prompt.ThresholdMode,
		Guesses:          cloneGuessMap(guesses.Guesses),
		WinnerIDs:        winnerIDs,
	}, deltas, scores, nil
}

func winningGuesses(actualCents int, guesses map[string]int) []string {
	bestDelta := actualCents + 1
	winners := []string{}
	for playerID, guess := range guesses {
		if guess > actualCents {
			continue
		}
		delta := actualCents - guess
		switch {
		case delta < bestDelta:
			bestDelta = delta
			winners = []string{playerID}
		case delta == bestDelta:
			winners = append(winners, playerID)
		}
	}
	slices.Sort(winners)
	return winners
}

func thresholdMode(opts map[string]any) string {
	if opts == nil {
		return modeFixedThreshold
	}
	if raw, ok := opts["threshold_mode"].(string); ok && raw == modeTimesTen {
		return raw
	}
	return modeFixedThreshold
}

func thresholdBaseCents(opts map[string]any) int {
	if opts == nil {
		return 500
	}
	switch raw := opts["minimum_price_dollars"].(type) {
	case int:
		if raw > 0 {
			return raw * 100
		}
	case float64:
		if raw > 0 {
			return int(raw) * 100
		}
	}
	return 500
}

func effectiveThreshold(baseCents int, mode string, round int) int {
	threshold := baseCents
	if mode != modeTimesTen || round <= 1 {
		return threshold
	}
	for i := 1; i < round; i++ {
		threshold *= 10
	}
	return threshold
}

func connectedPlayerCount(state *engine.GameState) int {
	count := 0
	for _, player := range state.Players {
		if player.Connected {
			count++
		}
	}
	return count
}

func cloneGuessMap(in map[string]int) map[string]int {
	out := make(map[string]int, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func readRound(cfg json.RawMessage) (int, error) {
	if len(cfg) == 0 {
		return 1, nil
	}
	var out phaseConfig
	if err := json.Unmarshal(cfg, &out); err != nil {
		return 0, err
	}
	if out.Round < 1 {
		out.Round = 1
	}
	return out.Round, nil
}

func hashSeed(roomID string, round int) int {
	total := round * 37
	for _, ch := range roomID {
		total += int(ch)
	}
	if total < 0 {
		return -total
	}
	return total
}

func imageDataURL(product Product) string {
	label := svgLabel(product.Name)
	svg := fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="900" viewBox="0 0 1200 900"><rect width="1200" height="900" fill="#f7f4ef"/><rect x="80" y="70" width="1040" height="760" rx="40" fill="#ffffff" stroke="#d7dce5" stroke-width="8"/><rect x="140" y="130" width="920" height="480" rx="28" fill="%s"/><rect x="220" y="220" width="760" height="300" rx="32" fill="rgba(255,255,255,0.2)"/><circle cx="320" cy="300" r="64" fill="rgba(255,255,255,0.28)"/><rect x="210" y="660" width="780" height="28" rx="14" fill="#1f2937"/><text x="160" y="735" fill="#111827" font-family="Verdana, Arial, sans-serif" font-size="56" font-weight="700">%s</text><text x="160" y="785" fill="#4b5563" font-family="Verdana, Arial, sans-serif" font-size="28">Marketplace pick</text></svg>`,
		product.Accent,
		xmlEscape(label),
	)
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
}

func svgLabel(name string) string {
	parts := strings.Fields(name)
	if len(parts) <= 4 {
		return name
	}
	return strings.Join(parts[:4], " ") + "..."
}

func xmlEscape(in string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;")
	return replacer.Replace(in)
}
