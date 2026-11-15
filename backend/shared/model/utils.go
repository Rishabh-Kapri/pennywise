package model

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Pagination struct {
	PrevCursor string `json:"prevCursor,omitempty"`
	NextCursor string `json:"nextCursor,omitempty"`
}

type Cursor struct {
	Date       string
	UpdatedAt  time.Time
	ID         uuid.UUID
	PointsNext bool
}

type PaginatedResponse[T any] struct {
	Data       []T        `json:"data"`
	Total      int        `json:"total"`
	Pagination Pagination `json:"pagination"`
	// Totals sums every row the filters match, not just this page. Only lists
	// of money (transactions) set it.
	Totals *AmountTotals `json:"totals,omitempty"`
}

// AmountTotals is money in and money out, both positive.
type AmountTotals struct {
	Inflow  float64 `json:"inflow"`
	Outflow float64 `json:"outflow"`
}

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
