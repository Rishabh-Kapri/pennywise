package model

import (
	"fmt"

	"github.com/google/uuid"
)

// ptrToString returns "<nil>" if the pointer is nil
func ptrToString(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%q", *s)
}

// ptrToUUIDString returns "<nil>" if the pointer is nil
func ptrToUUIDString(u *uuid.UUID) string {
	if u == nil {
		return "<nil>"
	}
	return u.String()
}

// ptrToInt64String returns "<nil>" if the pointer is nil
func ptrToFloat64String(f *float64) string {
	if f == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%.2f", *f)
}

// ptrToBoolString returns "<nil>" if the pointer is nil
func ptrToBoolString(b *bool) string {
	if b == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%t", *b)
}
