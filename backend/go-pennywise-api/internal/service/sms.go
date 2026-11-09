package service

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
)

// SmsMessage is one bank SMS forwarded by the Android app.
type SmsMessage struct {
	Sender     string
	Body       string
	ReceivedAt time.Time
	// Source is how the app got it: "live" or "inbox".
	Source string
}

// SmsDecisionEntry is one line of the decisions log (JSON Lines).
type SmsDecisionEntry struct {
	LoggedAt   time.Time        `json:"loggedAt"`
	ReceivedAt time.Time        `json:"receivedAt"`
	Sender     string           `json:"sender"`
	Source     string           `json:"source,omitempty"`
	Body       string           `json:"body"`
	Verdict    *DecisionVerdict `json:"verdict,omitempty"`
	Error      string           `json:"error,omitempty"`
	// Cost is this call's provider-reported cost (0 when none was reported);
	// TotalCost is the running sum over the whole file.
	Cost      float64 `json:"cost"`
	TotalCost float64 `json:"totalCost"`
}

type SmsService interface {
	// Ingest classifies the SMS with cipher's decisions endpoint and appends
	// the result, failures included, to the decisions log.
	Ingest(ctx context.Context, msg SmsMessage) (*SmsDecisionEntry, error)
}

// smsService is a test harness until the SMS queue exists: every message is
// sent to the decisions model and logged with its cost. Re-imports are not
// deduplicated, so each one is billed again.
type smsService struct {
	decisions DecisionsClient
	logPath   string

	mu          sync.Mutex
	totalLoaded bool
	totalCost   float64
}

func NewSmsService(decisions DecisionsClient, logPath string) SmsService {
	return &smsService{decisions: decisions, logPath: logPath}
}

func (s *smsService) Ingest(ctx context.Context, msg SmsMessage) (*SmsDecisionEntry, error) {
	verdict, callErr := s.decisions.TestDecisions(ctx, DecisionMessage{
		Body:   msg.Body,
		Source: "sms",
		Sender: msg.Sender,
	})

	entry := SmsDecisionEntry{
		LoggedAt:   time.Now().UTC(),
		ReceivedAt: msg.ReceivedAt,
		Sender:     msg.Sender,
		Source:     msg.Source,
		Body:       msg.Body,
		Verdict:    verdict,
	}
	if callErr != nil {
		entry.Error = callErr.Error()
	}
	if verdict != nil && verdict.Usage != nil && verdict.Usage.Cost != nil {
		entry.Cost = *verdict.Usage.Cost
	}

	if err := s.append(&entry); err != nil {
		logger.Logger(ctx).Error("sms decisions log write failed", "path", s.logPath, "error", err)
		return nil, err
	}
	if callErr != nil {
		return &entry, fmt.Errorf("decisions: %w", callErr)
	}
	return &entry, nil
}

// append writes entry with the updated running total. The total is read
// back from the file once, so it survives restarts.
func (s *smsService) append(entry *SmsDecisionEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.totalLoaded {
		total, err := sumCosts(s.logPath)
		if err != nil {
			return err
		}
		s.totalCost, s.totalLoaded = total, true
	}

	entry.TotalCost = s.totalCost + entry.Cost
	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(s.logPath), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(s.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}

	s.totalCost = entry.TotalCost
	return nil
}

// sumCosts adds up the cost of every entry already in the log. A missing
// file is an empty log; unreadable lines are skipped.
func sumCosts(path string) (float64, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	defer f.Close()

	var total float64
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var e struct {
			Cost float64 `json:"cost"`
		}
		if json.Unmarshal(scanner.Bytes(), &e) == nil {
			total += e.Cost
		}
	}
	return total, scanner.Err()
}
