package model

import "testing"

func TestParseStatementTime(t *testing.T) {
	for in, want := range map[string]string{
		"06/08/2026| 20:49":      "20:49",
		"03/09/2026 16:45:49":    "16:45",
		"03/09/2026 04:45:49 PM": "16:45",
		"12:05 am":               "00:05",
		"8:05 p.m.":              "20:05",
		"06/08/2026":             "",
		"25:10":                  "",
		"Swiggy order 1234":      "",
	} {
		if got := ParseStatementTime(in); got != want {
			t.Errorf("ParseStatementTime(%q) = %q, want %q", in, got, want)
		}
	}
}
