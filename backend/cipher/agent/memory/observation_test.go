package memory

import (
	"strings"
	"testing"
	"time"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

func mustLoad(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Skipf("timezone %s unavailable: %v", name, err)
	}
	return loc
}

func timedMessage(seq int, role sharedModel.Role, text string, at time.Time) sharedModel.AgentMessage {
	return sharedModel.AgentMessage{
		Sequence:  seq,
		Role:      role,
		Content:   []sharedModel.ContentBlock{{Type: "text", Text: text}},
		CreatedAt: at,
	}
}

// The observer records a timestamp per observation and the transcript was its
// only possible source. Without one it invented the same clock time on every
// run.
func TestFormatMessagesForObserverCarriesRealTimestamps(t *testing.T) {
	loc := mustLoad(t, "Asia/Kolkata")
	// 03:30 UTC is 09:00 in Kolkata: proves the zone is applied, not just passed.
	stored := time.Date(2026, 7, 14, 3, 30, 0, 0, time.UTC)
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)

	messages := []sharedModel.AgentMessage{
		timedMessage(1, sharedModel.RoleUser, "what did I spend on groceries", stored),
		// No CreatedAt: produced during the current run, so "now" applies.
		timedMessage(2, sharedModel.RoleAssistant, "You spent 4200.", time.Time{}),
	}

	transcript := formatMessagesForObserver(messages, nil, now, loc)

	if !strings.Contains(transcript, "1. [2026-07-14 09:00] User:") {
		t.Errorf("stored timestamp missing or not in Asia/Kolkata:\n%s", transcript)
	}
	if !strings.Contains(transcript, "2. [2026-08-01 17:30] Assistant:") {
		t.Errorf("current-run message should fall back to now:\n%s", transcript)
	}
}

func TestRepairObservationTimes(t *testing.T) {
	loc := mustLoad(t, "Asia/Kolkata")
	earliest := time.Date(2026, 8, 1, 17, 0, 0, 0, loc)
	latest := time.Date(2026, 8, 1, 17, 30, 0, 0, loc)

	tests := []struct {
		name     string
		date     string
		clock    string
		wantDate string
		wantTime string
	}{
		{
			// The reported bug: a plausible-looking constant the model invented.
			name: "invented time outside the window is clamped",
			date: "2026-08-01", clock: "14:00",
			wantDate: "2026-08-01", wantTime: "17:00",
		},
		{
			name: "time inside the window is left alone",
			date: "2026-08-01", clock: "17:15",
			wantDate: "2026-08-01", wantTime: "17:15",
		},
		{
			name: "future time is clamped to the latest message",
			date: "2026-08-02", clock: "09:00",
			wantDate: "2026-08-01", wantTime: "17:30",
		},
		{
			name: "unparseable falls back to the latest message",
			date: "sometime", clock: "recently",
			wantDate: "2026-08-01", wantTime: "17:30",
		},
		{
			name: "empty falls back to the latest message",
			date: "", clock: "",
			wantDate: "2026-08-01", wantTime: "17:30",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			observations := []sharedModel.AgentObservations{{Date: tc.date, Time: tc.clock}}
			repairObservationTimes(observations, earliest, latest, loc)

			if observations[0].Date != tc.wantDate || observations[0].Time != tc.wantTime {
				t.Errorf("got %s %s, want %s %s",
					observations[0].Date, observations[0].Time, tc.wantDate, tc.wantTime)
			}
		})
	}
}

// Boundary values are legitimate: an observation about the first or last
// message must not be rewritten.
func TestRepairObservationTimesKeepsBoundaries(t *testing.T) {
	loc := mustLoad(t, "Asia/Kolkata")
	earliest := time.Date(2026, 8, 1, 17, 0, 30, 0, loc)
	latest := time.Date(2026, 8, 1, 17, 30, 0, 0, loc)

	observations := []sharedModel.AgentObservations{
		{Date: "2026-08-01", Time: "17:00"},
		{Date: "2026-08-01", Time: "17:30"},
	}
	if repaired := repairObservationTimes(observations, earliest, latest, loc); repaired != 0 {
		t.Errorf("repaired %d boundary timestamps, want 0", repaired)
	}
}

func TestObservedTimeBounds(t *testing.T) {
	loc := mustLoad(t, "Asia/Kolkata")
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)

	messages := []sharedModel.AgentMessage{
		// Below lastSequence: already observed, must not widen the window.
		timedMessage(1, sharedModel.RoleUser, "old", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
		timedMessage(2, sharedModel.RoleUser, "a", time.Date(2026, 8, 1, 11, 0, 0, 0, time.UTC)),
		timedMessage(3, sharedModel.RoleAssistant, "b", time.Date(2026, 8, 1, 11, 30, 0, 0, time.UTC)),
	}

	earliest, latest := observedTimeBounds(messages, 1, now, loc)

	if got := earliest.UTC(); !got.Equal(time.Date(2026, 8, 1, 11, 0, 0, 0, time.UTC)) {
		t.Errorf("earliest = %v, want 2026-08-01T11:00Z", got)
	}
	if got := latest.UTC(); !got.Equal(time.Date(2026, 8, 1, 11, 30, 0, 0, time.UTC)) {
		t.Errorf("latest = %v, want 2026-08-01T11:30Z", got)
	}
}
