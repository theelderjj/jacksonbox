package proto

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ValidatePayload performs per-type payload validation. This is the
// "schema validation" layer from §3's trust boundary. Returning nil here
// means the room actor can trust the payload shape; it only needs to
// worry about game semantics.
func ValidatePayload(typ string, raw json.RawMessage) error {
	switch typ {
	case C2SJoinRoom:
		var p JoinRoomPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		roomID := strings.TrimSpace(p.RoomID)
		if len(roomID) < 1 || len(roomID) > 24 {
			return fmt.Errorf("room_id must be 1..24 chars")
		}
		for _, r := range roomID {
			if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_') {
				return fmt.Errorf("room_id must be alphanumeric, dash, or underscore")
			}
		}
		name, err := cleanName(p.Name)
		if err != nil {
			return err
		}
		_ = name // caller will re-clean; we only validate here
		return nil

	case C2SLeaveRoom, C2SPing:
		return nil

	case C2SReady:
		var p ReadyPayload
		return json.Unmarshal(raw, &p)

	case C2SSubmitDraw:
		var p SubmitDrawingPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		switch p.Format {
		case "strokes", "png":
			// content validation is deeper; primitive decoder handles it
		default:
			return fmt.Errorf("format must be strokes or png")
		}
		if len(p.Data) == 0 {
			return fmt.Errorf("data required")
		}
		return nil

	case C2SSubmitFake:
		var p SubmitFakePromptPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		if p.DrawingID == "" {
			return fmt.Errorf("drawing_id required")
		}
		if _, err := CleanFakeText(p.Text); err != nil {
			return err
		}
		return nil

	case C2SSubmitVote:
		var p SubmitVotePayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		if p.DrawingID == "" {
			return fmt.Errorf("drawing_id required")
		}
		if p.ChoiceID == "" {
			return fmt.Errorf("choice_id required")
		}
		return nil

	case C2SSubmitTap:
		var p SubmitTapPayload
		return json.Unmarshal(raw, &p)

	case C2SSubmitPriceGuess:
		var p SubmitPriceGuessPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		if p.GuessCents < 0 {
			return fmt.Errorf("guess_cents must be >= 0")
		}
		if p.GuessCents > 10_000_000_00 {
			return fmt.Errorf("guess_cents too large")
		}
		return nil

	case C2SSubmitSplitSetup:
		var p SubmitSplitSetupPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		if strings.TrimSpace(p.OptionA) == "" || strings.TrimSpace(p.OptionB) == "" {
			return fmt.Errorf("both options are required")
		}
		return nil

	case C2SSubmitSplitChoice:
		var p SubmitSplitChoicePayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		if p.ChoiceID != "A" && p.ChoiceID != "B" {
			return fmt.Errorf("choice_id must be A or B")
		}
		return nil

	case C2SSubmitFakeArtistGuess:
		var p SubmitFakeArtistGuessPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		if strings.TrimSpace(p.Prompt) == "" {
			return fmt.Errorf("prompt required")
		}
		if utf8.RuneCountInString(strings.TrimSpace(p.Prompt)) > 80 {
			return fmt.Errorf("prompt too long (max 80)")
		}
		return nil

	case C2SSubmitWordList:
		var p SubmitWordListPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		if len(p.Words) == 0 {
			return fmt.Errorf("at least one word required")
		}
		if len(p.Words) > 20 {
			return fmt.Errorf("too many words (max 20)")
		}
		for _, word := range p.Words {
			word = strings.TrimSpace(word)
			if word == "" {
				continue
			}
			if utf8.RuneCountInString(word) > 32 {
				return fmt.Errorf("word too long (max 32)")
			}
		}
		return nil

	case C2SSetPause:
		var p SetPausePayload
		return json.Unmarshal(raw, &p)

	case C2SSetPencilsDown:
		var p SetPencilsDownPayload
		return json.Unmarshal(raw, &p)

	case C2SAdvanceReveal:
		var p AdvanceRevealPayload
		return json.Unmarshal(raw, &p)

	case C2SUpdateSettings:
		var p UpdateSettingsPayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		if p.RoundCount < 1 || p.RoundCount > 60 {
			return fmt.Errorf("round_count must be between 1 and 60")
		}
		if p.GeneratedFakeCount < 0 || p.GeneratedFakeCount > 6 {
			return fmt.Errorf("generated_fake_count must be between 0 and 6")
		}
		if p.DrawingSeconds < 10 || p.DrawingSeconds > 300 {
			return fmt.Errorf("drawing_seconds must be between 10 and 300")
		}
		if p.FakePromptSeconds < 10 || p.FakePromptSeconds > 300 {
			return fmt.Errorf("fake_prompt_seconds must be between 10 and 300")
		}
		if p.VotingSeconds < 10 || p.VotingSeconds > 180 {
			return fmt.Errorf("voting_seconds must be between 10 and 180")
		}
		return nil

	case C2SRerollPrompt:
		var p RerollPromptPayload
		return json.Unmarshal(raw, &p)

	case C2SSelectGame:
		var p SelectGamePayload
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		id := strings.TrimSpace(p.GameID)
		if len(id) < 1 || len(id) > 48 {
			return fmt.Errorf("game_id must be 1..48 chars")
		}
		return nil

	case C2SStartGame:
		var p StartGamePayload
		return json.Unmarshal(raw, &p)

	case C2SReturnToPicker:
		var p ReturnToPickerPayload
		return json.Unmarshal(raw, &p)

	default:
		return fmt.Errorf("unknown type %q", typ)
	}
}

// CleanName enforces §10 player-name rules: 1..24 chars, printable Unicode,
// control chars stripped.
func CleanName(raw string) (string, error) { return cleanName(raw) }

func cleanName(raw string) (string, error) {
	var b strings.Builder
	for _, r := range raw {
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	s := strings.TrimSpace(b.String())
	if utf8.RuneCountInString(s) < 1 {
		return "", fmt.Errorf("name empty")
	}
	if utf8.RuneCountInString(s) > 24 {
		return "", fmt.Errorf("name too long (max 24)")
	}
	return s, nil
}

// CleanFakeText enforces §10 fake-prompt rules: 1..80 chars after trim.
// Returns the cleaned version.
func CleanFakeText(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	n := utf8.RuneCountInString(s)
	if n < 1 {
		return "", fmt.Errorf("fake prompt empty")
	}
	if n > 80 {
		return "", fmt.Errorf("fake prompt too long (max 80)")
	}
	return s, nil
}
