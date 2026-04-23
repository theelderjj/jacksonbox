package wordstorm

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/jj/trivia/internal/engine"
	"github.com/jj/trivia/internal/proto"
)

const (
	regKeyPrompt = "wordstorm.prompt"
	regKeySubmit = "wordstorm.submit"
	regKeyReveal = "wordstorm.reveal"

	defaultLetterSeconds = 20
	defaultRevealSeconds = 8
	basePointsPerLetter  = 100
)

type phaseConfig struct{}

type LetterWindow struct {
	Letter string `json:"letter"`
	Index  int    `json:"index"`
}

type PromptResult struct {
	Letters             []LetterWindow `json:"letters"`
	LetterSeconds       int            `json:"letter_seconds"`
	BasePointsPerLetter int            `json:"base_points_per_letter"`
	GrowthPercent       int            `json:"growth_percent"`
}

type WordEntry struct {
	PlayerID   string `json:"player_id"`
	PlayerName string `json:"player_name"`
	Word       string `json:"word"`
	Letter     string `json:"letter"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	Points     int    `json:"points,omitempty"`
}

type SubmitResult struct {
	Entries []WordEntry `json:"entries"`
}

type wordClaim struct {
	Word        string
	PlayerID    string
	PlayerName  string
	Letter      string
	ClientMsgID string
	ClaimedAt   time.Time
	Sequence    int
}

type PlayerResult struct {
	PlayerID    string      `json:"player_id"`
	PlayerName  string      `json:"player_name"`
	Entries     []WordEntry `json:"entries"`
	Score       int         `json:"score"`
	MadeUpCount int         `json:"made_up_count"`
}

type ResultPayload struct {
	Letters             []LetterWindow `json:"letters"`
	LetterSeconds       int            `json:"letter_seconds"`
	BasePointsPerLetter int            `json:"base_points_per_letter"`
	GrowthPercent       int            `json:"growth_percent"`
	Results             []PlayerResult `json:"results"`
	AcceptedWords       []string       `json:"accepted_words"`
}

var registerOnce sync.Once

func Register() {
	registerOnce.Do(func() {
		engine.Register(regKeyPrompt, func(json.RawMessage) (engine.AnyPrimitive, error) {
			return engine.Erase(newPrompt()), nil
		})
		engine.Register(regKeySubmit, func(json.RawMessage) (engine.AnyPrimitive, error) {
			return engine.Erase(newSubmit()), nil
		})
		engine.Register(regKeyReveal, func(json.RawMessage) (engine.AnyPrimitive, error) {
			return engine.Erase(newReveal()), nil
		})
	})
}

func BuildPhases(state *engine.GameState) []engine.Phase {
	letterCount := state.Settings.RoundCount
	if letterCount <= 0 {
		letterCount = 3
	}
	if letterCount > 10 {
		letterCount = 10
	}
	letterSeconds := letterSeconds(state)
	cfg, _ := json.Marshal(phaseConfig{})
	return []engine.Phase{
		{
			Name:      "word_prompt",
			Primitive: regKeyPrompt,
			Config:    cfg,
		},
		{
			Name:      "word_submit",
			Primitive: regKeySubmit,
			Duration:  time.Duration(letterCount*letterSeconds) * time.Second,
			Config:    cfg,
			DependsOn: []string{"word_prompt"},
		},
		{
			Name:      "word_reveal",
			Primitive: regKeyReveal,
			Duration:  defaultRevealSeconds * time.Second,
			Config:    cfg,
			DependsOn: []string{"word_prompt", "word_submit"},
		},
	}
}

type promptPrimitive struct{}

func newPrompt() *promptPrimitive { return &promptPrimitive{} }
func (*promptPrimitive) Name() string {
	return "wordstorm.prompt"
}

func (*promptPrimitive) Start(ctx *engine.PhaseContext) (engine.Decision[PromptResult], error) {
	result := PromptResult{
		Letters:             letterWindows(ctx.State),
		LetterSeconds:       letterSeconds(ctx.State),
		BasePointsPerLetter: basePointsPerLetter,
		GrowthPercent:       15,
	}
	raw, _ := json.Marshal(proto.WordPromptPayload{
		Letters:             protoWordLetters(result.Letters),
		LetterSeconds:       result.LetterSeconds,
		BasePointsPerLetter: result.BasePointsPerLetter,
		GrowthPercent:       result.GrowthPercent,
	})
	return engine.Decision[PromptResult]{
		AdvancePhase: true,
		Broadcast:    []engine.Event{{Type: proto.S2CWordPrompt, Payload: raw}},
		Result:       result,
	}, nil
}

func (*promptPrimitive) Handle(*engine.PhaseContext, engine.Input) (engine.Decision[PromptResult], error) {
	return engine.Decision[PromptResult]{AdvancePhase: false}, nil
}

func (p *promptPrimitive) Timeout(ctx *engine.PhaseContext) (engine.Decision[PromptResult], error) {
	return p.Start(ctx)
}

type submitPrimitive struct {
	entries  []WordEntry
	claimed  map[string]wordClaim
	sequence int
}

func newSubmit() *submitPrimitive {
	return &submitPrimitive{claimed: map[string]wordClaim{}}
}

func (*submitPrimitive) Name() string { return "wordstorm.submit" }

func (*submitPrimitive) Start(*engine.PhaseContext) (engine.Decision[SubmitResult], error) {
	return engine.Decision[SubmitResult]{AdvancePhase: false}, nil
}

func (s *submitPrimitive) Handle(ctx *engine.PhaseContext, in engine.Input) (engine.Decision[SubmitResult], error) {
	var payload proto.SubmitWordListPayload
	if err := json.Unmarshal(in.Value, &payload); err != nil {
		return engine.Decision[SubmitResult]{}, err
	}
	word := ""
	if len(payload.Words) > 0 {
		word = normalizeWord(payload.Words[0])
	}
	entry := s.evaluate(ctx, in, word)
	s.entries = append(s.entries, entry)
	entryJSON, _ := json.Marshal(proto.WordEntryPayload{
		PlayerID:   entry.PlayerID,
		PlayerName: entry.PlayerName,
		Word:       entry.Word,
		Letter:     entry.Letter,
		Status:     entry.Status,
		Message:    entry.Message,
		Points:     entry.Points,
	})
	return engine.Decision[SubmitResult]{
		AdvancePhase: false,
		Broadcast:    []engine.Event{{Type: proto.S2CWordEntry, Payload: entryJSON}},
	}, nil
}

func (s *submitPrimitive) Timeout(*engine.PhaseContext) (engine.Decision[SubmitResult], error) {
	return engine.Decision[SubmitResult]{
		AdvancePhase: true,
		Result:       SubmitResult{Entries: append([]WordEntry(nil), s.entries...)},
	}, nil
}

func (s *submitPrimitive) evaluate(ctx *engine.PhaseContext, in engine.Input, word string) WordEntry {
	player := ctx.State.Players[in.PlayerID]
	playerName := ""
	if player != nil {
		playerName = player.Name
	}
	letter := currentLetter(ctx)
	entry := WordEntry{
		PlayerID:   string(in.PlayerID),
		PlayerName: playerName,
		Word:       word,
		Letter:     letter,
	}
	switch {
	case word == "":
		entry.Status = "invalid"
		entry.Message = "Enter a word."
	case !strings.HasPrefix(word, letter):
		entry.Status = "invalid"
		entry.Message = fmt.Sprintf("Must start with %s.", strings.ToUpper(letter))
	default:
		entry = s.claimWord(ctx, in, entry)
	}
	return entry
}

// claimWord is the ACID-style transaction boundary for "first word wins":
// Atomic: validation and claim commit happen in one actor turn.
// Consistent: the ledger maps a normalized word to exactly one first claimant.
// Isolated: the room actor serializes Handle calls, so no two claims interleave.
// Durable: the claim remains in this primitive's in-memory ledger until scoring.
func (s *submitPrimitive) claimWord(ctx *engine.PhaseContext, in engine.Input, entry WordEntry) WordEntry {
	if prior, exists := s.claimed[entry.Word]; exists {
		entry.Status = "duplicate"
		if prior.PlayerID == string(in.PlayerID) {
			entry.Message = "Already entered."
		} else {
			entry.Message = fmt.Sprintf("Already entered by %s.", prior.PlayerName)
		}
		return entry
	}

	s.sequence++
	claimedAt := time.Now()
	if ctx.Now != nil {
		claimedAt = ctx.Now()
	}
	entry.Status = "pending"
	entry.Points = wordPoints(entry.Word)
	entry.Message = fmt.Sprintf("Tentative +%d points", entry.Points)
	s.claimed[entry.Word] = wordClaim{
		Word:        entry.Word,
		PlayerID:    entry.PlayerID,
		PlayerName:  entry.PlayerName,
		Letter:      entry.Letter,
		ClientMsgID: in.ClientMsgID,
		ClaimedAt:   claimedAt,
		Sequence:    s.sequence,
	}
	return entry
}

type revealPrimitive struct {
	resolved bool
	cached   ResultPayload
}

func newReveal() *revealPrimitive { return &revealPrimitive{} }
func (*revealPrimitive) Name() string {
	return "wordstorm.reveal"
}

func (r *revealPrimitive) Start(ctx *engine.PhaseContext) (engine.Decision[ResultPayload], error) {
	if !r.resolved {
		result, deltas, scores, err := scoreGame(ctx.State)
		if err != nil {
			return engine.Decision[ResultPayload]{}, err
		}
		r.cached = result
		r.resolved = true
		resultJSON, _ := json.Marshal(result)
		roundJSON, _ := json.Marshal(proto.RoundResultPayload{
			Round:  1,
			Deltas: deltas,
			Scores: scores,
		})
		return engine.Decision[ResultPayload]{
			Broadcast: []engine.Event{
				{Type: proto.S2CWordResult, Payload: resultJSON},
				{Type: proto.S2CRoundResult, Payload: roundJSON},
			},
		}, nil
	}
	return engine.Decision[ResultPayload]{AdvancePhase: false}, nil
}

func (*revealPrimitive) Handle(*engine.PhaseContext, engine.Input) (engine.Decision[ResultPayload], error) {
	return engine.Decision[ResultPayload]{AdvancePhase: false}, nil
}

func (r *revealPrimitive) Timeout(*engine.PhaseContext) (engine.Decision[ResultPayload], error) {
	return engine.Decision[ResultPayload]{AdvancePhase: true, Result: r.cached}, nil
}

func scoreGame(state *engine.GameState) (ResultPayload, map[string]int, map[string]int, error) {
	prompt, ok := engine.GetResult[PromptResult](state, "word_prompt")
	if !ok {
		return ResultPayload{}, nil, nil, fmt.Errorf("missing word prompt")
	}
	submitted, ok := engine.GetResult[SubmitResult](state, "word_submit")
	if !ok {
		return ResultPayload{}, nil, nil, fmt.Errorf("missing word submissions")
	}
	entriesByPlayer := map[string][]WordEntry{}
	deltas := map[string]int{}
	accepted := []string{}
	seenFinal := map[string]bool{}
	for _, original := range submitted.Entries {
		entry := original
		if entry.Status == "pending" {
			if isDictionaryWord(entry.Word) {
				entry.Status = "valid"
				entry.Points = wordPoints(entry.Word)
				entry.Message = fmt.Sprintf("+%d points", entry.Points)
				accepted = append(accepted, entry.Word)
				deltas[entry.PlayerID] += entry.Points
			} else {
				entry.Status = "invalid"
				entry.Message = "Not found in dictionary."
			}
		}
		if seenFinal[entry.Word] && entry.Status == "valid" {
			entry.Status = "duplicate"
			entry.Points = 0
			entry.Message = "Already entered."
		}
		if entry.Status == "valid" {
			seenFinal[entry.Word] = true
		}
		entriesByPlayer[entry.PlayerID] = append(entriesByPlayer[entry.PlayerID], entry)
	}
	results := make([]PlayerResult, 0, len(state.Players))
	for _, playerID := range sortedPlayerIDs(state) {
		player := state.Players[engine.PlayerID(playerID)]
		score := deltas[playerID]
		madeUpCount := 0
		for _, entry := range entriesByPlayer[playerID] {
			if entry.Status == "invalid" {
				madeUpCount++
			}
		}
		if score > 0 {
			state.Scores[engine.PlayerID(playerID)] += score
		}
		results = append(results, PlayerResult{
			PlayerID:    playerID,
			PlayerName:  player.Name,
			Entries:     entriesByPlayer[playerID],
			Score:       score,
			MadeUpCount: madeUpCount,
		})
	}
	sort.Strings(accepted)
	scores := map[string]int{}
	for playerID, score := range state.Scores {
		scores[string(playerID)] = score
	}
	return ResultPayload{
		Letters:             prompt.Letters,
		LetterSeconds:       prompt.LetterSeconds,
		BasePointsPerLetter: prompt.BasePointsPerLetter,
		GrowthPercent:       prompt.GrowthPercent,
		Results:             results,
		AcceptedWords:       accepted,
	}, deltas, scores, nil
}

func wordPoints(word string) int {
	length := len([]rune(word))
	points := float64(length * basePointsPerLetter)
	if length > 4 {
		points *= math.Pow(1.15, float64(length-4))
	}
	return int(math.Round(points))
}

func currentLetter(ctx *engine.PhaseContext) string {
	prompt, ok := engine.GetResult[PromptResult](ctx.State, "word_prompt")
	if !ok || len(prompt.Letters) == 0 {
		return "c"
	}
	elapsed := ctx.Phase.Duration - ctx.TimeLeft()
	if elapsed < 0 {
		elapsed = 0
	}
	index := int(elapsed / (time.Duration(prompt.LetterSeconds) * time.Second))
	if index < 0 {
		index = 0
	}
	if index >= len(prompt.Letters) {
		index = len(prompt.Letters) - 1
	}
	return prompt.Letters[index].Letter
}

func letterWindows(state *engine.GameState) []LetterWindow {
	count := state.Settings.RoundCount
	if count <= 0 {
		count = 3
	}
	if count > 10 {
		count = 10
	}
	sequence := []string{"c", "b", "s", "t", "m", "a", "f", "r", "p", "l", "d", "g", "h", "w"}
	out := make([]LetterWindow, count)
	offset := hashSeed(state.RoomID, 1) % len(sequence)
	for idx := 0; idx < count; idx++ {
		out[idx] = LetterWindow{Letter: sequence[(offset+idx)%len(sequence)], Index: idx}
	}
	return out
}

func letterSeconds(state *engine.GameState) int {
	seconds := intOption(state.Settings.GameOptions, "letter_seconds", defaultLetterSeconds)
	if seconds < 5 {
		return defaultLetterSeconds
	}
	if seconds > 60 {
		return 60
	}
	return seconds
}

func protoWordLetters(letters []LetterWindow) []proto.WordLetterWindow {
	out := make([]proto.WordLetterWindow, len(letters))
	for idx, letter := range letters {
		out[idx] = proto.WordLetterWindow{Letter: letter.Letter, Index: letter.Index}
	}
	return out
}

func isDictionaryWord(word string) bool {
	return dictionary[word]
}

func normalizeWord(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	var b strings.Builder
	for _, ch := range raw {
		if unicode.IsLetter(ch) {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

func sortedPlayerIDs(state *engine.GameState) []string {
	ids := make([]string, 0, len(state.Players))
	for playerID := range state.Players {
		ids = append(ids, string(playerID))
	}
	sort.Strings(ids)
	return ids
}

func intOption(opts map[string]any, key string, fallback int) int {
	if opts == nil {
		return fallback
	}
	switch raw := opts[key].(type) {
	case int:
		if raw > 0 {
			return raw
		}
	case float64:
		if raw > 0 {
			return int(raw)
		}
	}
	return fallback
}

func hashSeed(roomID string, round int) int {
	total := round * 53
	for _, ch := range roomID {
		total += int(ch)
	}
	if total < 0 {
		return -total
	}
	return total
}
