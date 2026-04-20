// Package proto defines the wire protocol. See docs/DESIGN.md §9.
// Kept deliberately thin — no heavy JSON Schema library; payload validation
// is hand-rolled per message type and colocated with the type.
package proto

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ProtocolVersion is baked into every envelope. Mismatch closes with 4001;
// client reloads. Single-deployer rationale is in §9 / changelog.
const ProtocolVersion = 1

// MaxFrameBytes mirrors the gateway cap in §10. Frames larger than this
// trigger WS close code 1009.
const MaxFrameBytes = 256 * 1024

// Envelope is the shape of every frame, both directions.
type Envelope struct {
	V       int             `json:"v"`
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	TS      int64           `json:"ts"`
	Payload json.RawMessage `json:"payload"`
}

// ErrBadEnvelope indicates the frame failed parse/shape checks at the gateway.
var ErrBadEnvelope = errors.New("proto: bad envelope")

// DecodeEnvelope parses and performs shape-only validation. Payload is
// inspected later by the per-type decoder.
func DecodeEnvelope(raw []byte) (Envelope, error) {
	if len(raw) > MaxFrameBytes {
		return Envelope{}, fmt.Errorf("%w: frame too large (%d bytes)", ErrBadEnvelope, len(raw))
	}
	var e Envelope
	if err := json.Unmarshal(raw, &e); err != nil {
		return Envelope{}, fmt.Errorf("%w: %v", ErrBadEnvelope, err)
	}
	if e.V == 0 {
		return Envelope{}, fmt.Errorf("%w: missing v", ErrBadEnvelope)
	}
	if e.ID == "" {
		return Envelope{}, fmt.Errorf("%w: missing id", ErrBadEnvelope)
	}
	if e.Type == "" {
		return Envelope{}, fmt.Errorf("%w: missing type", ErrBadEnvelope)
	}
	if len(e.Payload) == 0 {
		// Allow null payload for messages like leave_room / ping.
		e.Payload = json.RawMessage(`{}`)
	}
	return e, nil
}

// EncodeEnvelope marshals a server-originated envelope. Server IDs are
// assigned by the caller; ts is server-current millis.
func EncodeEnvelope(id, typ string, ts int64, payload any) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(Envelope{V: ProtocolVersion, ID: id, Type: typ, TS: ts, Payload: raw})
}
