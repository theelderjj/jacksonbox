package proto

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeEnvelope_OK(t *testing.T) {
	t.Log("Scenario: a well-formed wire frame with all four required envelope fields (v, id, type, ts) plus empty payload.")
	t.Log("Expected: DecodeEnvelope returns the populated Envelope with no error.")
	raw := []byte(`{"v":1,"id":"c1","type":"ping","ts":1,"payload":{}}`)
	e, err := DecodeEnvelope(raw)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if e.V != 1 || e.ID != "c1" || e.Type != "ping" {
		t.Fatalf("unexpected: %+v", e)
	}
}

func TestDecodeEnvelope_MissingFields(t *testing.T) {
	t.Log("Scenario: three malformed envelopes, each missing one required field (v, id, or type).")
	t.Log("Expected: DecodeEnvelope rejects each — envelope fields are load-bearing for routing and idempotency.")
	cases := []string{
		`{"id":"c1","type":"ping"}`,                 // missing v
		`{"v":1,"type":"ping"}`,                     // missing id
		`{"v":1,"id":"c1"}`,                         // missing type
	}
	for _, c := range cases {
		t.Logf("  case: %s", c)
		if _, err := DecodeEnvelope([]byte(c)); err == nil {
			t.Errorf("expected err for %s", c)
		}
	}
}

func TestDecodeEnvelope_SizeCap(t *testing.T) {
	t.Logf("Scenario: inbound frame is MaxFrameBytes+1 (%d bytes) — 1 byte over the hard cap.", MaxFrameBytes+1)
	t.Log("Expected: DecodeEnvelope rejects before any JSON parsing — cap protects the server from memory-blowup frames.")
	huge := make([]byte, MaxFrameBytes+1)
	if _, err := DecodeEnvelope(huge); err == nil {
		t.Fatal("expected size-cap error")
	}
}

func TestValidatePayload_SubmitDrawing(t *testing.T) {
	t.Log("Scenario: two submit_drawing payloads — one with format=`strokes` (allowed), one with format=`bogus` (not).")
	t.Log("Expected: the allowed format passes validation; the unknown format is rejected.")
	raw, _ := json.Marshal(SubmitDrawingPayload{Format: "strokes", Data: "{}"})
	if err := ValidatePayload(C2SSubmitDraw, raw); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	raw, _ = json.Marshal(SubmitDrawingPayload{Format: "bogus", Data: "x"})
	if err := ValidatePayload(C2SSubmitDraw, raw); err == nil {
		t.Fatal("expected format rejection")
	}
}

func TestCleanName_Rules(t *testing.T) {
	t.Log("Scenario: three display-name inputs — empty, 100 chars (over cap), and `  hello  ` (trimmable).")
	t.Log("Expected: empty rejected, oversize rejected, trimmable returns `hello` with no error.")
	if _, err := CleanName(""); err == nil {
		t.Fatal("empty name should fail")
	}
	if _, err := CleanName(strings.Repeat("a", 100)); err == nil {
		t.Fatal("oversize name should fail")
	}
	n, err := CleanName("  hello  ")
	if err != nil || n != "hello" {
		t.Fatalf("want trimmed hello, got %q err=%v", n, err)
	}
	t.Logf("Clean of `  hello  ` → %q", n)
}
