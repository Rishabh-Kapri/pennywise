package llmAdapter

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/progress"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/statementpdf"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	sharedUtils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

// statementContextWindow fits one page of statement text (about 3,000 tokens)
// and its rows as JSON (up to the adapter's max tokens). Ollama's default
// window (4096) cuts the answer short.
const statementContextWindow = 16384

// ExtractStatement implements [ports.StatementImport]. It reads the PDF's text
// and sends it to the model one page at a time, so a long statement never
// outgrows the model's output limit.
func (a *adapter) ExtractStatement(
	ctx context.Context,
	pdf []byte,
	passwords []string,
) (sharedModel.StatementExtractResponse, error) {
	progress.Report(ctx, "reading text")
	pages, opened, err := statementpdf.Text(ctx, pdf, passwords)
	if err != nil {
		return sharedModel.StatementExtractResponse{}, err
	}

	var (
		sections []sharedModel.StatementSection
		previous string // the account the last page ended in
		total    = len(pages)
	)
	progress.ReportCount(ctx, "reading pages with "+a.cfg.Model, 0, total)
	for i, text := range pages {
		if err := ctx.Err(); err != nil {
			return sharedModel.StatementExtractResponse{}, err
		}
		page := i + 1
		if strings.TrimSpace(text) == "" {
			// A scanned page has no text layer; an OCR reader is the one for it.
			progress.ReportCount(ctx, "reading pages with "+a.cfg.Model, page, total)
			continue
		}
		reply, _, err := a.ask(ctx, errs.CodeStatementExtractFailed,
			statementExtractionPrompt, statementPageInput(page, total, previous, text),
			func(r *sharedModel.ChatRequest) {
				r.Temperature = 0
				r.ContextWindow = statementContextWindow
				// Transcription, not analysis: at a reasoning model's default
				// effort the thinking used most of the output budget and the
				// row list was cut off mid-JSON.
				r.ReasoningEffort = "low"
			})
		if err != nil {
			return sharedModel.StatementExtractResponse{}, errs.Wrap(
				errs.CodeStatementExtractFailed, fmt.Sprintf("could not read page %d", page), err,
			)
		}
		parsed, err := sharedUtils.UnmarshalResponse[rawStatementExtraction]([]byte(reply))
		if err != nil {
			return sharedModel.StatementExtractResponse{}, errs.Wrap(
				errs.CodeStatementExtractFailed, fmt.Sprintf("page %d: unreadable model output", page), err,
			)
		}
		for _, s := range parsed.toSections(page) {
			sections = mergeSection(sections, s)
			previous = sectionName(s)
		}
		progress.ReportCount(ctx, "reading pages with "+a.cfg.Model, page, total)
	}

	rows := 0
	for _, s := range sections {
		rows += len(s.Rows)
	}
	// Counts only: the text is the user's statement.
	logger.Logger(ctx).Info("statement read by model",
		"model", a.cfg.Model, "pages", total, "sections", len(sections), "rows", rows)

	return sharedModel.StatementExtractResponse{Sections: sections, PasswordIndex: opened}, nil
}

// sectionName tells the model the account the last page ended in, in the
// prompt's own field names so it can copy them onto a continued section.
func sectionName(s sharedModel.StatementSection) string {
	return fmt.Sprintf("accountLabel %q, accountNumber %q", s.AccountLabel, s.AccountNumber)
}

// mergeSection adds s to sections: a page that continues an account the model
// already returned adds its rows to that section, and fills in what earlier
// pages didn't print. The closing balance is the last one printed.
func mergeSection(sections []sharedModel.StatementSection, s sharedModel.StatementSection) []sharedModel.StatementSection {
	for i := range sections {
		have := &sections[i]
		if have.AccountLabel != s.AccountLabel || have.AccountNumber != s.AccountNumber {
			continue
		}
		have.Rows = append(have.Rows, s.Rows...)
		if have.Currency == "" {
			have.Currency = s.Currency
		}
		if have.Kind == "" {
			have.Kind = s.Kind
		}
		fillString(&have.StatementDate, s.StatementDate)
		fillString(&have.DueDate, s.DueDate)
		fillNumber(&have.TotalDue, s.TotalDue)
		fillNumber(&have.MinimumDue, s.MinimumDue)
		fillNumber(&have.CreditLimit, s.CreditLimit)
		have.Rewards = mergeRewards(have.Rewards, s.Rewards)
		if have.PeriodStart == nil {
			have.PeriodStart = s.PeriodStart
		}
		if have.PeriodEnd == nil {
			have.PeriodEnd = s.PeriodEnd
		}
		if have.OpeningBalance == nil {
			have.OpeningBalance = s.OpeningBalance
		}
		if s.ClosingBalance != nil {
			have.ClosingBalance = s.ClosingBalance
		}
		return sections
	}
	return append(sections, s)
}

// mergeRewards fills what an earlier page's points summary left out.
func mergeRewards(have, more *sharedModel.StatementRewards) *sharedModel.StatementRewards {
	if have == nil {
		return more
	}
	if more == nil {
		return have
	}
	for _, f := range []struct{ dst, src **int }{
		{&have.Opening, &more.Opening},
		{&have.Earned, &more.Earned},
		{&have.Redeemed, &more.Redeemed},
		{&have.Expired, &more.Expired},
		{&have.Closing, &more.Closing},
	} {
		if *f.dst == nil {
			*f.dst = *f.src
		}
	}
	return have
}

// fillString and fillNumber keep what an earlier page printed and take a later
// page's value only when the earlier ones printed none.
func fillString(have **string, v *string) {
	if *have == nil {
		*have = v
	}
}

func fillNumber(have **float64, v *float64) {
	if *have == nil {
		*have = v
	}
}

func statementPageInput(page, total int, previous, text string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Page %d of %d of the statement.\n", page, total)
	if previous != "" {
		fmt.Fprintf(&b, "Previous section: %s\n", previous)
	} else {
		b.WriteString("Previous section: none (this is the first page)\n")
	}
	fmt.Fprintf(&b, "\nStatement text:\n=== Page %d ===\n%s", page, text)
	return b.String()
}

// rawStatementExtraction mirrors the prompt's schema with lenient numbers:
// models sometimes quote amounts or keep thousands separators ("1,234.50"),
// and one such value must not fail the whole page.
type rawStatementExtraction struct {
	Sections []struct {
		AccountLabel   string        `json:"accountLabel"`
		AccountNumber  string        `json:"accountNumber"`
		Kind           string        `json:"kind"`
		Currency       string        `json:"currency"`
		PeriodStart    *string       `json:"periodStart"`
		PeriodEnd      *string       `json:"periodEnd"`
		StatementDate  *string       `json:"statementDate"`
		OpeningBalance *statementNum `json:"openingBalance"`
		ClosingBalance *statementNum `json:"closingBalance"`
		DueDate        *string       `json:"dueDate"`
		TotalDue       *statementNum `json:"totalDue"`
		MinimumDue     *statementNum `json:"minimumDue"`
		CreditLimit    *statementNum `json:"creditLimit"`
		Rewards        *struct {
			Opening  *statementPoints `json:"opening"`
			Earned   *statementPoints `json:"earned"`
			Redeemed *statementPoints `json:"redeemed"`
			Expired  *statementPoints `json:"expired"`
			Closing  *statementPoints `json:"closing"`
		} `json:"rewards"`
		Rows []struct {
			Date         string           `json:"date"`
			Time         statementText    `json:"time"`
			Description  string           `json:"description"`
			Payee        string           `json:"payee"`
			Amount       statementNum     `json:"amount"`
			Direction    string           `json:"direction"`
			Balance      *statementNum    `json:"balance"`
			Reference    statementText    `json:"reference"`
			RewardPoints *statementPoints `json:"rewardPoints"`
		} `json:"rows"`
	} `json:"sections"`
}

// toSections converts what the model returned for page into the model types.
// The API validates every row when it saves them.
func (r rawStatementExtraction) toSections(page int) []sharedModel.StatementSection {
	out := make([]sharedModel.StatementSection, 0, len(r.Sections))
	for _, rs := range r.Sections {
		section := sharedModel.StatementSection{
			AccountLabel:   strings.TrimSpace(rs.AccountLabel),
			AccountNumber:  strings.TrimSpace(rs.AccountNumber),
			Kind:           statementKind(rs.Kind),
			Currency:       strings.ToUpper(strings.TrimSpace(rs.Currency)),
			PeriodStart:    nonBlank(rs.PeriodStart),
			PeriodEnd:      nonBlank(rs.PeriodEnd),
			StatementDate:  nonBlank(rs.StatementDate),
			OpeningBalance: rs.OpeningBalance.ptr(),
			ClosingBalance: rs.ClosingBalance.ptr(),
			DueDate:        nonBlank(rs.DueDate),
			TotalDue:       rs.TotalDue.ptr(),
			MinimumDue:     rs.MinimumDue.ptr(),
			CreditLimit:    rs.CreditLimit.ptr(),
			Rows:           make([]sharedModel.StatementRow, 0, len(rs.Rows)),
		}
		if rw := rs.Rewards; rw != nil {
			rewards := sharedModel.StatementRewards{
				Opening:  rw.Opening.ptr(),
				Earned:   rw.Earned.ptr(),
				Redeemed: rw.Redeemed.ptr(),
				Expired:  rw.Expired.ptr(),
				Closing:  rw.Closing.ptr(),
			}
			if rewards != (sharedModel.StatementRewards{}) {
				section.Rewards = &rewards
			}
		}
		for _, row := range rs.Rows {
			section.Rows = append(section.Rows, sharedModel.StatementRow{
				Date:         strings.TrimSpace(row.Date),
				Time:         rowTime(string(row.Time), row.Date),
				Description:  strings.Join(strings.Fields(row.Description), " "),
				Payee:        strings.TrimSpace(row.Payee),
				Amount:       float64(row.Amount),
				Direction:    sharedModel.StatementDirection(strings.ToLower(strings.TrimSpace(row.Direction))),
				Balance:      row.Balance.ptr(),
				Reference:    strings.TrimSpace(string(row.Reference)),
				Page:         page,
				RewardPoints: row.RewardPoints.ptr(),
			})
		}
		out = append(out, section)
	}
	return out
}

// statementKind keeps only the kinds the model knows; anything else is "".
func statementKind(kind string) string {
	switch k := strings.ToLower(strings.TrimSpace(kind)); k {
	case sharedModel.StatementKindCard, sharedModel.StatementKindBank:
		return k
	}
	return ""
}

func nonBlank(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	v := strings.TrimSpace(*s)
	return &v
}

// statementNum decodes a JSON number or a numeric string. Anything unparseable
// decodes as 0, which the API then rejects as an invalid row and counts.
type statementNum float64

func (f *statementNum) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "null" {
		return nil
	}
	text = strings.Trim(text, `"`)
	text = strings.NewReplacer(",", "", " ", "", "₹", "", "$", "").Replace(text)
	text = strings.TrimSuffix(strings.TrimSuffix(text, "Dr"), "Cr")
	v, err := strconv.ParseFloat(text, 64)
	if err != nil {
		*f = 0
		return nil
	}
	*f = statementNum(math.Abs(v))
	return nil
}

func (f *statementNum) ptr() *float64 {
	if f == nil {
		return nil
	}
	v := float64(*f)
	return &v
}

// statementPoints decodes reward points: a JSON number or a numeric string
// ("1,200", "-12", "+40"), rounded to whole points. Unlike amounts the sign is
// kept: a reversal takes points back. Anything unreadable decodes as null.
type statementPoints struct{ v *int }

func (p *statementPoints) UnmarshalJSON(data []byte) error {
	text := strings.Trim(strings.TrimSpace(string(data)), `"`)
	text = strings.NewReplacer(",", "", " ", "", "+", "").Replace(text)
	if text == "" || text == "null" {
		return nil
	}
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return nil
	}
	v := int(math.Round(f))
	p.v = &v
	return nil
}

func (p *statementPoints) ptr() *int {
	if p == nil {
		return nil
	}
	return p.v
}

// statementText accepts a string or a bare number (reference columns are often
// numeric and models drop the quotes).
type statementText string

func (s *statementText) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "null" {
		return nil
	}
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		*s = statementText(str)
		return nil
	}
	*s = statementText(text)
	return nil
}

// rowTime is the row's time as "HH:MM": the model's time field, else a time
// it left in the date ("2026-08-06 20:49").
func rowTime(t, date string) string {
	if v := sharedModel.ParseStatementTime(t); v != "" {
		return v
	}
	return sharedModel.ParseStatementTime(date)
}
