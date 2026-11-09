package model

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatVector renders an embedding as "[0.1, 0.2, ...]", the text form
// cipher sends embeddings in. It is also valid JSON.
func FormatVector(vec []float64) string {
	parts := make([]string, len(vec))
	for i, v := range vec {
		parts[i] = fmt.Sprintf("%.8f", v)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// ParseVector reads the text form produced by FormatVector.
func ParseVector(s string) ([]float64, error) {
	var vec []float64
	if err := json.Unmarshal([]byte(s), &vec); err != nil {
		return nil, fmt.Errorf("parse embedding vector: %w", err)
	}
	return vec, nil
}
