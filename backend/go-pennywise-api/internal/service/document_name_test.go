package service

import (
	"strings"
	"testing"
	"time"
)

func TestFormatDocumentName(t *testing.T) {
	cases := []struct {
		name    string
		payee   string
		date    string
		index   int
		want    string
	}{
		{"plain", "McDonalds", "2026-08-19", 1, "McDonalds_20260819-1"},
		{"punctuation stripped", "McDonald's", "2026-08-19", 1, "McDonalds_20260819-1"},
		{"spaces removed, case kept", "Blue Tokai Coffee", "2026-01-02", 2, "BlueTokaiCoffee_20260102-2"},
		{"slashes cannot escape the key", "Zomato/Swiggy", "2026-08-19", 1, "ZomatoSwiggy_20260819-1"},
		{"unicode and emoji dropped", "Café ☕ Nero", "2026-12-31", 3, "CafNero_20261231-3"},
		{"missing payee falls back", "", "2026-08-19", 1, "Receipt_20260819-1"},
		{"payee of only symbols falls back", "###", "2026-08-19", 1, "Receipt_20260819-1"},
		{"rfc3339 date accepted", "Uber", "2026-08-19T10:30:00Z", 1, "Uber_20260819-1"},
		{"index floors at one", "Uber", "2026-08-19", 0, "Uber_20260819-1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatDocumentName(tc.payee, tc.date, tc.index); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// An unparseable date must still yield a usable name rather than an empty or
// malformed one, since a bad date should not block an upload.
func TestFormatDocumentNameFallsBackToToday(t *testing.T) {
	got := formatDocumentName("Uber", "not-a-date", 1)
	want := "Uber_" + time.Now().Format("20060102") + "-1"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// The name becomes an object key; a separator would create an unintended
// "directory" in the bucket.
func TestFormatDocumentNameHasNoPathSeparators(t *testing.T) {
	got := formatDocumentName("a/b\\c", "2026-08-19", 1)
	if strings.ContainsAny(got, `/\`) {
		t.Errorf("name %q contains a path separator", got)
	}
}
