package proto

import (
	"encoding/json"
	"testing"
)

func TestValidatePayloadJoinRoomAllowsArbitraryRoomCode(t *testing.T) {
	t.Parallel()

	raw, _ := json.Marshal(JoinRoomPayload{RoomID: "PARTY_01", Name: "Alice"})
	if err := ValidatePayload(C2SJoinRoom, raw); err != nil {
		t.Fatalf("expected valid join_room payload, got %v", err)
	}
}

func TestValidatePayloadSelectGame(t *testing.T) {
	t.Parallel()

	good, _ := json.Marshal(SelectGamePayload{GameID: "jrawful"})
	if err := ValidatePayload(C2SSelectGame, good); err != nil {
		t.Fatalf("expected valid select_game payload, got %v", err)
	}

	bad, _ := json.Marshal(SelectGamePayload{GameID: ""})
	if err := ValidatePayload(C2SSelectGame, bad); err == nil {
		t.Fatal("expected empty game_id to be rejected")
	}
}

func TestValidatePayloadStartAndReturnToPicker(t *testing.T) {
	t.Parallel()

	start, _ := json.Marshal(StartGamePayload{})
	if err := ValidatePayload(C2SStartGame, start); err != nil {
		t.Fatalf("expected valid start_game payload, got %v", err)
	}

	returnToPicker, _ := json.Marshal(ReturnToPickerPayload{})
	if err := ValidatePayload(C2SReturnToPicker, returnToPicker); err != nil {
		t.Fatalf("expected valid return_to_picker payload, got %v", err)
	}
}

func TestValidatePayloadPriceGuess(t *testing.T) {
	t.Parallel()

	good, _ := json.Marshal(SubmitPriceGuessPayload{GuessCents: 14999})
	if err := ValidatePayload(C2SSubmitPriceGuess, good); err != nil {
		t.Fatalf("expected valid submit_price_guess payload, got %v", err)
	}

	bad, _ := json.Marshal(SubmitPriceGuessPayload{GuessCents: -1})
	if err := ValidatePayload(C2SSubmitPriceGuess, bad); err == nil {
		t.Fatal("expected negative guess to be rejected")
	}
}

func TestValidatePayloadSubmitWordList(t *testing.T) {
	t.Parallel()

	good, _ := json.Marshal(SubmitWordListPayload{Words: []string{"cat", "camel"}})
	if err := ValidatePayload(C2SSubmitWordList, good); err != nil {
		t.Fatalf("expected valid submit_word_list payload, got %v", err)
	}

	empty, _ := json.Marshal(SubmitWordListPayload{Words: nil})
	if err := ValidatePayload(C2SSubmitWordList, empty); err == nil {
		t.Fatal("expected empty word list to be rejected")
	}

	tooLong, _ := json.Marshal(SubmitWordListPayload{Words: []string{"supercalifragilisticexpialidociousword"}})
	if err := ValidatePayload(C2SSubmitWordList, tooLong); err == nil {
		t.Fatal("expected overlong word to be rejected")
	}
}
