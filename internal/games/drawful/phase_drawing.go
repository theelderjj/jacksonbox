package drawful

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/jj/trivia/internal/engine"
	"github.com/jj/trivia/internal/primitives"
	"github.com/jj/trivia/internal/proto"
)

// newDrawingCollect wires CollectAll[Drawing] with:
//   - payload decoder that enforces §10 content caps
//   - eligibility: every player who was assigned a prompt this round
func newDrawingCollect(round int) engine.AnyPrimitive {
	promptKey := phaseName("prompt_distribute", round)

	decode := func(in engine.Input, s *engine.GameState) (Drawing, error) {
		var p proto.SubmitDrawingPayload
		if err := json.Unmarshal(in.Value, &p); err != nil {
			return Drawing{}, fmt.Errorf("parse drawing: %w", err)
		}
		if err := ValidateDrawing(p.Format, p.Data); err != nil {
			return Drawing{}, err
		}
		prompts, ok := engine.GetResult[PromptDistributionResult](s, promptKey)
		if !ok {
			return Drawing{}, fmt.Errorf("no prompt distribution in state")
		}
		truePrompt, assigned := prompts.ByAuthor[in.PlayerID]
		if !assigned {
			return Drawing{}, fmt.Errorf("no prompt assigned to player")
		}
		return Drawing{
			ID:       DrawingID(uuid.NewString()),
			AuthorID: in.PlayerID,
			Prompt:   truePrompt,
			Format:   p.Format,
			Data:     p.Data,
		}, nil
	}

	eligible := func(pid engine.PlayerID, s *engine.GameState) bool {
		prompts, ok := engine.GetResult[PromptDistributionResult](s, promptKey)
		if !ok {
			return false
		}
		_, assigned := prompts.ByAuthor[pid]
		return assigned
	}

	return engine.Erase(primitives.NewCollectAll[Drawing](
		"drawing_submit",
		decode,
		eligible,
		nil, // progress-tick events are a nice-to-have; v1.1
	))
}
