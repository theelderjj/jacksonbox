package drawful

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// §10: stroke payload caps. Coordinates are 0..1000; up to 5000 points total.
const (
	maxStrokePoints = 5000
	minCoord        = 0.0
	maxCoord        = 1000.0
	maxPNGDecoded   = 64 * 1024
)

type strokesPayload struct {
	Strokes []struct {
		Points [][2]float64 `json:"points"`
		Color  string       `json:"color"`
		Width  float64      `json:"width"`
	} `json:"strokes"`
}

// ValidateDrawing enforces §10's stroke/PNG caps. Returns nil on success.
// Kept separate from the gateway payload validator because the content
// inspection is heavier and belongs near the game that needs it.
func ValidateDrawing(format, data string) error {
	switch format {
	case "strokes":
		return validateStrokes(data)
	case "png":
		return validatePNG(data)
	default:
		return fmt.Errorf("unknown drawing format %q", format)
	}
}

func validateStrokes(data string) error {
	var p strokesPayload
	if err := json.Unmarshal([]byte(data), &p); err != nil {
		return fmt.Errorf("strokes parse: %w", err)
	}
	total := 0
	for _, s := range p.Strokes {
		total += len(s.Points)
		for _, pt := range s.Points {
			x, y := pt[0], pt[1]
			if x < minCoord || x > maxCoord || y < minCoord || y > maxCoord {
				return fmt.Errorf("point out of range: (%v,%v)", x, y)
			}
		}
	}
	if total > maxStrokePoints {
		return fmt.Errorf("too many stroke points: %d > %d", total, maxStrokePoints)
	}
	return nil
}

func validatePNG(data string) error {
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return fmt.Errorf("png base64 decode: %w", err)
	}
	if len(raw) > maxPNGDecoded {
		return fmt.Errorf("png too large: %d bytes decoded", len(raw))
	}
	// Minimal header check — full image parsing is overkill for the invite-only threat model.
	if len(raw) < 8 ||
		raw[0] != 0x89 || raw[1] != 'P' || raw[2] != 'N' || raw[3] != 'G' {
		return fmt.Errorf("not a PNG")
	}
	return nil
}
