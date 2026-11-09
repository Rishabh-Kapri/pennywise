package service

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDecisions struct {
	costs []*float64 // returned in order, one per call
	err   error
	got   []DecisionMessage
}

func (f *fakeDecisions) TestDecisions(_ context.Context, msg DecisionMessage) (*DecisionVerdict, error) {
	f.got = append(f.got, msg)
	if f.err != nil {
		return nil, f.err
	}
	cost := f.costs[0]
	f.costs = f.costs[1:]
	return &DecisionVerdict{
		IsTransaction: true,
		Type:          "debit",
		Usage:         &DecisionUsage{Model: "m", InputTokens: 100, OutputTokens: 5, Cost: cost},
	}, nil
}

func costPtr(v float64) *float64 { return &v }

func readLog(t *testing.T, path string) []SmsDecisionEntry {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	var entries []SmsDecisionEntry
	s := bufio.NewScanner(f)
	for s.Scan() {
		var e SmsDecisionEntry
		require.NoError(t, json.Unmarshal(s.Bytes(), &e))
		entries = append(entries, e)
	}
	return entries
}

var sms = SmsMessage{Sender: "AX-HDFCBK", Body: "INR 1111.00 debited", ReceivedAt: time.Date(2026, 10, 30, 10, 0, 0, 0, time.UTC), Source: "inbox"}

func TestSmsService_SendsSmsToDecisions(t *testing.T) {
	dec := &fakeDecisions{costs: []*float64{costPtr(0.001)}}
	svc := NewSmsService(dec, filepath.Join(t.TempDir(), "log.jsonl"))

	_, err := svc.Ingest(context.Background(), sms)

	require.NoError(t, err)
	assert.Equal(t, []DecisionMessage{{Body: "INR 1111.00 debited", Source: "sms", Sender: "AX-HDFCBK"}}, dec.got)
}

func TestSmsService_LogsEachCallWithRunningTotal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "log.jsonl")
	dec := &fakeDecisions{costs: []*float64{costPtr(0.001), nil, costPtr(0.002)}}
	svc := NewSmsService(dec, path)

	for range 3 {
		_, err := svc.Ingest(context.Background(), sms)
		require.NoError(t, err)
	}

	entries := readLog(t, path)
	require.Len(t, entries, 3)
	assert.InDelta(t, 0.001, entries[0].TotalCost, 1e-12)
	assert.Equal(t, 0.0, entries[1].Cost, "no reported cost counts as zero")
	assert.InDelta(t, 0.001, entries[1].TotalCost, 1e-12)
	assert.InDelta(t, 0.003, entries[2].TotalCost, 1e-12)
	assert.Equal(t, "inbox", entries[2].Source)
	assert.Equal(t, 100, entries[2].Verdict.Usage.InputTokens)
}

func TestSmsService_TotalSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log.jsonl")
	_, err := NewSmsService(&fakeDecisions{costs: []*float64{costPtr(0.5)}}, path).Ingest(context.Background(), sms)
	require.NoError(t, err)

	entry, err := NewSmsService(&fakeDecisions{costs: []*float64{costPtr(0.25)}}, path).Ingest(context.Background(), sms)

	require.NoError(t, err)
	assert.InDelta(t, 0.75, entry.TotalCost, 1e-12)
}

func TestSmsService_FailedCallIsLoggedAndReturned(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log.jsonl")
	svc := NewSmsService(&fakeDecisions{err: errors.New("cipher down")}, path)

	_, err := svc.Ingest(context.Background(), sms)

	require.ErrorContains(t, err, "cipher down")
	entries := readLog(t, path)
	require.Len(t, entries, 1)
	assert.Equal(t, "cipher down", entries[0].Error)
	assert.Equal(t, 0.0, entries[0].TotalCost)
}
