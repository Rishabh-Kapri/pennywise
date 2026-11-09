package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

// maxStatementChunkRunes bounds one extraction request. The API chunks by page
// well under this; the cap guards the prompt against a caller that does not.
const maxStatementChunkRunes = 24000

const statementExtractionPrompt = `You extract transactions from the text of a bank account or credit card statement. The text came from a PDF with its column layout preserved, so a table row is one line (occasionally wrapped onto the next).

Return every transaction line on these pages, grouped by the account it belongs to.

For each account section:
- accountLabel: the account as the statement names it (bank, product, masked number), e.g. "HDFC Savings XX1234".
- accountNumber: the account or card number exactly as printed, masked digits included (e.g. "XXXX1234"). Empty if not shown on these pages.
- currency: ISO code if stated (e.g. "INR").
- periodStart / periodEnd: the statement period as YYYY-MM-DD, only if printed. Otherwise null.
- openingBalance / closingBalance: only if printed on these pages, as plain numbers. Otherwise null.

For each transaction row:
- date: the transaction date as YYYY-MM-DD. Statements from India print DD/MM/YYYY or DD-MM-YY; convert them. Use the statement period to resolve a missing year.
- description: the narration exactly as printed, wrapped lines joined with a space.
- payee: a short clean name of the merchant or counterparty ("Swiggy", "Amazon", "Rahul Sharma"): drop UPI ids, reference numbers, city names and payment-rail prefixes. For transfers between the holder's own accounts or a card bill payment, use what the statement says (e.g. "Credit Card Payment").
- amount: a positive number, no currency symbol or thousands separators.
- direction: "debit" if money left the account holder (purchase, withdrawal, fee, interest, tax, EMI), "credit" if money reached them (deposit, salary, refund, reversal, cashback, payment received on a card). Decide from separate Debit/Credit or Withdrawal/Deposit columns, "Dr"/"Cr" markers, or a leading "+"/"-"; never from the description alone when a marker is present.
- balance: the running balance printed on that row, as a positive number, or null if the statement has no balance column.
- reference: a cheque/reference number if printed in its own column, else "".
- page: the page number the row appears on (pages are marked "=== Page N ===").

Do not output: opening/closing balance lines, totals, subtotals, "brought/carried forward" lines, reward point summaries, EMI/loan summary tables, GST summaries, or column headers. A row is a transaction only if it has a date and an amount.

If a page continues an account from an earlier page without repeating its header, put its rows in a section with the same accountLabel and accountNumber as given under "Previous section". If there are no transactions, return {"sections": []}.

Respond with JSON only:
{"sections": [{"accountLabel": "", "accountNumber": "", "currency": "", "periodStart": null, "periodEnd": null, "openingBalance": null, "closingBalance": null, "rows": [{"date": "", "description": "", "payee": "", "amount": 0, "direction": "debit", "balance": null, "reference": "", "page": 1}]}]}`

// ExtractStatement pulls the transaction rows out of one chunk of statement
// text. It returns what the model produced; the API validates every row and
// reconciles balances, since it is the side that knows the ledger.
func (s *predictionService) ExtractStatement(
	ctx context.Context,
	req sharedModel.StatementExtractRequest,
) (*sharedModel.StatementExtractResponse, error) {
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return nil, errs.New(errs.CodeInvalidArgument, "statement text is required")
	}
	if runes := []rune(text); len(runes) > maxStatementChunkRunes {
		return nil, errs.New(errs.CodeInvalidArgument,
			"statement chunk is %d characters, limit is %d", len(runes), maxStatementChunkRunes)
	}

	res, err := s.chatWithFallback(ctx, "statement:extract", func(model string) sharedModel.ChatRequest {
		return sharedModel.ChatRequest{
			Model: model,
			Messages: []sharedModel.AgentMessage{
				{
					Role:    sharedModel.RoleSystem,
					Content: []sharedModel.ContentBlock{{Type: "text", Text: statementExtractionPrompt}},
				},
				{
					Role:    sharedModel.RoleUser,
					Content: []sharedModel.ContentBlock{{Type: "text", Text: statementExtractionInput(req, text)}},
				},
			},
			Temperature: 0,
			MaxTokens:   cloudMaxOutputTokens,
			Stream:      false,
			Format:      "json",
			// Transcription, not analysis: at a reasoning model's default
			// effort the thinking used most of the output budget and the row
			// list was cut off mid-JSON.
			ReasoningEffort: "low",
		}
	})
	if err != nil {
		return nil, err
	}
	if res == nil || len(res.Message.Content) == 0 {
		return nil, errs.New(errs.CodeInternalError, "statement extraction: no content returned")
	}
	if res.StopReason == sharedModel.StopReasonMaxTokens {
		// The API retries a multi-page chunk one page at a time on failure.
		return nil, errs.New(errs.CodeInternalError,
			"statement extraction: output truncated at the token limit (pages %d-%d)", req.FirstPage, req.LastPage)
	}

	parsed, err := utils.UnmarshalResponse[rawStatementExtraction]([]byte(res.Message.Content[0].Text))
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "statement extraction: unparseable model output", err)
	}
	return parsed.toModel(), nil
}

// rawStatementExtraction mirrors the prompt's schema with lenient numbers:
// models sometimes quote amounts or keep thousands separators ("1,234.50"),
// and one such value must not fail the whole chunk.
type rawStatementExtraction struct {
	Sections []struct {
		AccountLabel   string      `json:"accountLabel"`
		AccountNumber  string      `json:"accountNumber"`
		Currency       string      `json:"currency"`
		PeriodStart    *string     `json:"periodStart"`
		PeriodEnd      *string     `json:"periodEnd"`
		OpeningBalance *flexNumber `json:"openingBalance"`
		ClosingBalance *flexNumber `json:"closingBalance"`
		Rows           []struct {
			Date        string      `json:"date"`
			Description string      `json:"description"`
			Payee       string      `json:"payee"`
			Amount      flexNumber  `json:"amount"`
			Direction   string      `json:"direction"`
			Balance     *flexNumber `json:"balance"`
			Reference   flexString  `json:"reference"`
			Page        flexNumber  `json:"page"`
		} `json:"rows"`
	} `json:"sections"`
}

func (r rawStatementExtraction) toModel() *sharedModel.StatementExtractResponse {
	out := &sharedModel.StatementExtractResponse{Sections: make([]sharedModel.StatementSection, 0, len(r.Sections))}
	for _, rs := range r.Sections {
		section := sharedModel.StatementSection{
			AccountLabel:   strings.TrimSpace(rs.AccountLabel),
			AccountNumber:  strings.TrimSpace(rs.AccountNumber),
			Currency:       strings.ToUpper(strings.TrimSpace(rs.Currency)),
			PeriodStart:    nonEmpty(rs.PeriodStart),
			PeriodEnd:      nonEmpty(rs.PeriodEnd),
			OpeningBalance: rs.OpeningBalance.ptr(),
			ClosingBalance: rs.ClosingBalance.ptr(),
			Rows:           make([]sharedModel.StatementRow, 0, len(rs.Rows)),
		}
		for _, row := range rs.Rows {
			section.Rows = append(section.Rows, sharedModel.StatementRow{
				Date:        strings.TrimSpace(row.Date),
				Description: strings.Join(strings.Fields(row.Description), " "),
				Payee:       strings.TrimSpace(row.Payee),
				Amount:      float64(row.Amount),
				Direction:   sharedModel.StatementDirection(strings.ToLower(strings.TrimSpace(row.Direction))),
				Balance:     row.Balance.ptr(),
				Reference:   strings.TrimSpace(string(row.Reference)),
				Page:        int(row.Page),
			})
		}
		out.Sections = append(out.Sections, section)
	}
	return out
}

func nonEmpty(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	v := strings.TrimSpace(*s)
	return &v
}

// flexNumber decodes a JSON number or a numeric string. Anything unparseable
// decodes as 0, which the API then rejects as an invalid row and counts.
type flexNumber float64

func (f *flexNumber) UnmarshalJSON(data []byte) error {
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
	*f = flexNumber(math.Abs(v))
	return nil
}

func (f *flexNumber) ptr() *float64 {
	if f == nil {
		return nil
	}
	v := float64(*f)
	return &v
}

// flexString accepts a string or a bare number (reference columns are often
// numeric and models drop the quotes).
type flexString string

func (s *flexString) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "null" {
		return nil
	}
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		*s = flexString(str)
		return nil
	}
	*s = flexString(text)
	return nil
}

func statementExtractionInput(req sharedModel.StatementExtractRequest, text string) string {
	var b strings.Builder
	if req.FirstPage > 0 {
		fmt.Fprintf(&b, "Pages %d to %d of the statement.\n", req.FirstPage, req.LastPage)
	}
	if req.PreviousSection != "" {
		fmt.Fprintf(&b, "Previous section: %s\n", req.PreviousSection)
	} else {
		b.WriteString("Previous section: none (these are the first pages)\n")
	}
	b.WriteString("\nStatement text:\n")
	b.WriteString(text)
	return b.String()
}
