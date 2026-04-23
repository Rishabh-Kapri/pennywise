package model

import (
	"fmt"

	"github.com/google/uuid"
)

// Helper functions
func ptrToString(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%q", *s)
}

func ptrToUUIDString(u *uuid.UUID) string {
	if u == nil {
		return "<nil>"
	}
	return u.String()
}

func ptrToFloat64String(f *float64) string {
	if f == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%.2f", *f)
}

func ptrToBoolString(b *bool) string {
	if b == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%t", *b)
}
