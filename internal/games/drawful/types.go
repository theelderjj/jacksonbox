// Package drawful is the Jrawful-style game mode. See docs/DESIGN.md §8.
// Phase primitives are registered with the engine registry in Register().
package drawful

import "github.com/jj/trivia/internal/engine"

type DrawingID string
type FakePromptID string

// Drawing is a per-player artifact in a given round.
type Drawing struct {
	ID       DrawingID
	AuthorID engine.PlayerID
	Prompt   string // hidden during voting
	Format   string // "strokes" | "png"
	Data     string // see §10 caps
}

// FakePrompt is a decoy authored by a non-drawer for a specific drawing.
type FakePrompt struct {
	ID        FakePromptID
	DrawingID DrawingID
	AuthorID  engine.PlayerID
	Text      string // trimmed, max 80
}

// Vote: one per non-authoring player per drawing.
type Vote struct {
	VoterID   engine.PlayerID
	DrawingID DrawingID
	ChoiceID  string // FakePromptID or "TRUE"
}

// --- Phase results (each implements engine.PhaseResult via StoredResult). ---

// PromptDistributionResult: map of drawing author -> the true prompt they drew.
// Consumed by reveal and scoring.
type PromptDistributionResult struct {
	ByAuthor map[engine.PlayerID]string
}

// DrawingCollectionResult: one Drawing per player. Consumed by fake and vote.
type DrawingCollectionResult struct {
	ByDrawing map[DrawingID]Drawing
	ByAuthor  map[engine.PlayerID]DrawingID
}

// FakeCollectionResult: fakes grouped by drawing.
// Collision merge (§8) happens in the reveal phase — this stage keeps
// authorship granular so split-credit scoring can recover who contributed.
type FakeCollectionResult struct {
	ByDrawing map[DrawingID][]FakePrompt
}

// VoteCollectionResult: every submitted vote, keyed for easy tally.
type VoteCollectionResult struct {
	ByDrawing map[DrawingID][]Vote
}

// RevealResult is what the reveal phase broadcasts and stores.
// Scoring consumes it to build deltas.
type RevealResult struct {
	ByDrawing map[DrawingID]DrawingReveal
}

// DrawingReveal contains merged choices ready for scoring.
type DrawingReveal struct {
	DrawingID  DrawingID
	AuthorID   engine.PlayerID
	TruePrompt string
	Choices    []Choice
}

// Choice is the voting option shown for a given drawing. After collision
// merge, AuthorIDs may contain more than one player (credit splits evenly).
type Choice struct {
	ChoiceID  string // stable ID used in the Vote.ChoiceID
	Text      string
	AuthorIDs []engine.PlayerID // nil/empty for the true prompt
	IsTrue    bool
	Voters    []engine.PlayerID
}
