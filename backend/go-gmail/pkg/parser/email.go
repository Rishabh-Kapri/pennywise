package parser

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"time"
)

type EmailParser struct {
	patterns *Patterns
}

type Patterns struct {
	EmailRegex   *regexp.Regexp
	AmountRegex  *regexp.Regexp
	TypeRegex    *regexp.Regexp
	DateRegex    *regexp.Regexp
	NewlineRegex *regexp.Regexp
}

type EmailDetails struct {
	Text            string  `json:"email_text"`
	Date            string  `json:"date"`
	Amount          float64 `json:"amount"`
	TransactionType string  `json:"transaction_type"`
	Account         string  `json:"account"`
	Payee           string  `json:"payee"`
	Type            string  `json:"type"` // used for predictions
}

func NewEmailParser() *EmailParser {
	return &EmailParser{
		patterns: &Patterns{
			EmailRegex:   regexp.MustCompile(`(?i)(Dear\s+(Customer|Card Member|Card Holder).*?)\,*(\s|\S)+(\d{2}-\d{2}-\d{4}|\d{2}-\d{2}-\d{2})`),
			AmountRegex:  regexp.MustCompile(`(?i)(Rs\.?\s?)([\d,]+\.\d+)`),
			TypeRegex:    regexp.MustCompile(`(?i)(credited|debited)`),
			DateRegex:    regexp.MustCompile(`\d{2}-\d{2}-\d{4}|\d{2}-\d{2}-\d{2}`),
			NewlineRegex: regexp.MustCompile(`\n`),
		},
	}
}

func (s *EmailParser) extractDate(emailDetails *EmailDetails) error {
	dateMatch := s.patterns.DateRegex.FindString(emailDetails.Text)
	if len(dateMatch) == 0 {
		return errors.New("No date pattern found")
	}
	formattedDate := ""
	var parsed time.Time
	var err error
	if len(dateMatch) == 10 {
		parsed, err = time.Parse("02-01-2006", dateMatch)
	} else if len(dateMatch) == 8 {
		parsed, err = time.Parse("02-01-06", dateMatch)
	}
	if err != nil {
		return fmt.Errorf("Date parse error: %s", err)
	}
	formattedDate = parsed.Format("2006-01-02")
	emailDetails.Date = formattedDate

	return nil
}

func (s *EmailParser) extractAmount(emailDetails *EmailDetails) error {
	amountMatch := s.patterns.AmountRegex.FindStringSubmatch(emailDetails.Text)
	if len(amountMatch) == 0 {
		return errors.New("No amount pattern found")
	}
	amount, err := strconv.ParseFloat(amountMatch[2], 10)
	if err != nil {
		return fmt.Errorf("Amount parse error: %s", err)
	}
	if emailDetails.TransactionType == "debited" {
		emailDetails.Amount = -amount
	} else {
		emailDetails.Amount = amount
	}
	return nil
}

func (s *EmailParser) extractType(emailDetails *EmailDetails) error {
	typeMatch := s.patterns.TypeRegex.FindString(emailDetails.Text)
	emailDetails.TransactionType = "debited"
	if len(typeMatch) > 0 {
		emailDetails.TransactionType = typeMatch
	}
	return nil
}

func (s *EmailParser) ParseEmail(html string) (*EmailDetails, error) {
	match := s.patterns.EmailRegex.FindStringSubmatch(html)
	if len(match) == 0 {
		return nil, errors.New("No transaction pattern found")
	}
	emailDetails := &EmailDetails{
		Text:            s.patterns.NewlineRegex.ReplaceAllString(match[0], " ") + ".",
		Date:            "",
		Amount:          0,
		TransactionType: "debited",
	}

	if err := s.extractDate(emailDetails); err != nil {
		return nil, err
	}

	if err := s.extractType(emailDetails); err != nil {
		return nil, err
	}

	if err := s.extractAmount(emailDetails); err != nil {
		return nil, err
	}

	return emailDetails, nil
}
