package telemetry

import (
	"encoding/json"
	"unicode/utf8"
)

// JSON keeps span payloads bounded without turning them into invalid JSON.
// A large value becomes a small preview object, so Langfuse can still parse it.
func JSON(value any, limit int) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	if limit <= 0 || len(encoded) <= limit {
		return string(encoded), nil
	}

	previewSize := limit - 100
	if previewSize < 0 {
		previewSize = 0
	}
	preview := encoded[:previewSize]
	for len(preview) > 0 {
		if !utf8.Valid(preview) {
			preview = preview[:len(preview)-1]
			continue
		}
		bounded, err := json.Marshal(struct {
			Truncated     bool   `json:"truncated"`
			OriginalBytes int    `json:"original_bytes"`
			Preview       string `json:"preview"`
		}{true, len(encoded), string(preview)})
		if err != nil {
			return "", err
		}
		if len(bounded) <= limit {
			return string(bounded), nil
		}
		preview = preview[:len(preview)-1]
	}
	return `{"truncated":true}`, nil
}
