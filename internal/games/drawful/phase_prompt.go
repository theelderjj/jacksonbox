package drawful

import (
	"encoding/json"
	"fmt"

	"github.com/jj/trivia/internal/engine"
	"github.com/jj/trivia/internal/primitives"
	"github.com/jj/trivia/internal/proto"
)

// newPromptDistribute is an Aggregate phase that assigns one unique prompt
// per connected player, stores them under its phase key, and emits one
// S2CPromptIssued event per player. The client filters on player_id.
func newPromptDistribute() engine.AnyPrimitive {
	return engine.Erase(primitives.NewAggregate("prompt_distribute",
		func(s *engine.GameState) (PromptDistributionResult, []engine.Event, error) {
			active := s.ActivePlayers()
			if len(active) == 0 {
				return PromptDistributionResult{ByAuthor: map[engine.PlayerID]string{}}, nil, nil
			}
			prompts := NextPrompts(len(active))
			by := make(map[engine.PlayerID]string, len(active))
			events := make([]engine.Event, 0, len(active))
			for i, pid := range active {
				by[pid] = prompts[i]
				raw, _ := json.Marshal(struct {
					PlayerID string `json:"player_id"`
					Prompt   string `json:"prompt"`
				}{string(pid), prompts[i]})
				events = append(events, engine.Event{Type: proto.S2CPromptIssued, Payload: raw})
			}
			return PromptDistributionResult{ByAuthor: by}, events, nil
		}))
}

func RerollPrompt(state *engine.GameState, phase string, pid engine.PlayerID) (string, []engine.Event, error) {
	prompts, ok := engine.GetResult[PromptDistributionResult](state, phase)
	if !ok {
		return "", nil, fmt.Errorf("prompt distribution missing")
	}
	if _, assigned := prompts.ByAuthor[pid]; !assigned {
		return "", nil, fmt.Errorf("no prompt assigned to player")
	}
	next := NextPrompts(1)[0]
	prompts.ByAuthor[pid] = next
	state.PhaseData[phase] = engine.StoredResult[PromptDistributionResult]{
		Phase:     phase,
		Primitive: "prompt_distribute",
		Value:     prompts,
	}
	raw, _ := json.Marshal(proto.PromptIssuedPayload{
		PlayerID: string(pid),
		Prompt:   next,
	})
	return next, []engine.Event{{Type: proto.S2CPromptIssued, Payload: raw}}, nil
}
