// Package localocr reads a statement with a local OCR model (glm-ocr on
// Ollama). Each page comes back as JSON in a schema's shape (Ollama's
// structured outputs), and the rules in rows.go turn it into rows. Page 1
// decides bank or card, which picks the schema. The statement's header
// (account number, period, balances, dues, points summary) isn't read yet.
package localocr

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/llm"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/progress"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/statementpdf"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

// ocrContextWindow fits a page and its rows. A 200 dpi A4 page is about 4,000
// prompt tokens and a full page of rows about 5,000 more; Ollama's default
// window (4096) leaves almost nothing for the answer.
const ocrContextWindow = 16384

type OCRConfig struct {
	Provider  string
	Model     string
	MaxTokens int
}

type adapter struct {
	client llm.LLM
	cfg    OCRConfig
}

func NewOCRAdapter(client *llm.ObservedLLM, cfg OCRConfig) (ports.StatementImport, error) {
	if client.Type != "local" {
		return nil, errs.New(
			errs.CodeAdapterNotConfigured,
			"can't use localocr with non-local provider: %s", client.Type,
		)
	}
	return &adapter{client, cfg}, nil
}

// ExtractStatement implements [ports.StatementImport].
func (a *adapter) ExtractStatement(
	ctx context.Context,
	pdf []byte,
	passwords []string,
) (sharedModel.StatementExtractResponse, error) {
	// The page images show the statement, so the folder goes even on failure.
	dir, err := os.MkdirTemp("", "statement-pages-*")
	if err != nil {
		return sharedModel.StatementExtractResponse{}, errs.Wrap(errs.CodeInternalError, "could not stage statement", err)
	}
	defer os.RemoveAll(dir)

	progress.Report(ctx, "rendering pages")
	pages, opened, err := statementpdf.Render(ctx, dir, pdf, passwords)
	if err != nil {
		return sharedModel.StatementExtractResponse{}, err
	}
	if len(pages) == 0 {
		return sharedModel.StatementExtractResponse{}, errs.New(errs.CodeStatementExtractFailed, "the PDF has no pages")
	}

	kind := a.kind(ctx, pdf, passwords, pages[0])
	layout := bankLayout
	if kind == sharedModel.StatementKindCard {
		layout = cardLayout
	}

	var read []pageRow
	progress.ReportCount(ctx, "reading pages", 0, len(pages))
	for i, path := range pages {
		if err := ctx.Err(); err != nil {
			return sharedModel.StatementExtractResponse{}, err
		}
		answer, err := a.ocr(ctx, path, layout.prompt, layout.schema)
		if err != nil {
			return sharedModel.StatementExtractResponse{}, errs.Wrap(
				errs.CodeStatementExtractFailed, fmt.Sprintf("could not read page %d", i+1), err)
		}
		var got struct {
			Transactions []pageRow `json:"transactions"`
		}
		if err := json.Unmarshal([]byte(answer), &got); err != nil {
			return sharedModel.StatementExtractResponse{}, errs.Wrap(
				errs.CodeStatementExtractFailed, fmt.Sprintf("page %d isn't the JSON asked for", i+1), err)
		}
		for _, r := range got.Transactions {
			r.page = i + 1
			read = append(read, r)
		}
		progress.ReportCount(ctx, "reading pages", i+1, len(pages))
	}

	rows := layout.rows(read)
	// Counts only: the rows are the user's statement.
	logger.Logger(ctx).Info("statement pages read",
		"pages", len(pages), "kind", kind, "read", len(read), "rows", len(rows))

	res := sharedModel.StatementExtractResponse{Sections: []sharedModel.StatementSection{}, PasswordIndex: opened}
	if len(rows) > 0 {
		res.Sections = append(res.Sections, sharedModel.StatementSection{Kind: kind, Rows: rows})
	}
	return res, nil
}

// cardRe is what only a card statement's first page says.
var cardRe = regexp.MustCompile(`(?i)credit\s*card\s*(?:no|number|statement)|total\s+amount\s+due|minimum\s+(?:amount\s+)?dues?\b`)

// kind reads page 1 for card or bank: the PDF's own text when it has some,
// else the OCR model's reading of it. A page that says neither is a bank's.
func (a *adapter) kind(ctx context.Context, pdf []byte, passwords []string, firstPage string) string {
	text := ""
	if pages, _, err := statementpdf.Text(ctx, pdf, passwords); err == nil {
		text = pages[0]
	}
	if strings.TrimSpace(text) == "" {
		var err error
		if text, err = a.ocr(ctx, firstPage, "Text Recognition:", nil); err != nil {
			logger.Logger(ctx).Warn("statement first page not read as text", "error", err)
		}
	}
	if cardRe.MatchString(text) {
		return sharedModel.StatementKindCard
	}
	return sharedModel.StatementKindBank
}

// ocr sends one page image with prompt; format, when set, is the JSON schema
// the answer must follow.
func (a *adapter) ocr(ctx context.Context, path, prompt string, format any) (string, error) {
	img, err := os.ReadFile(path)
	if err != nil {
		return "", errs.Wrap(errs.CodeInternalError, "could not read rendered page", err)
	}
	res, err := a.client.Chat(ctx, sharedModel.ChatRequest{
		Provider: a.cfg.Provider,
		Model:    a.cfg.Model,
		Messages: []sharedModel.AgentMessage{{
			Role:    sharedModel.RoleUser,
			Content: []sharedModel.ContentBlock{{Type: "text", Text: prompt}},
			Images:  []string{base64.StdEncoding.EncodeToString(img)},
		}},
		Format:        format,
		MaxTokens:     a.cfg.MaxTokens,
		ContextWindow: ocrContextWindow,
	}, nil)
	if err != nil {
		return "", err
	}
	if res == nil || len(res.Message.Content) == 0 {
		return "", errs.New(errs.CodeStatementExtractFailed, "the OCR model returned nothing")
	}
	// A cut-off answer is JSON with no end. Fail loudly instead of reporting
	// a page short.
	if res.StopReason == sharedModel.StopReasonMaxTokens {
		return "", errs.New(errs.CodeStatementExtractFailed,
			"the OCR model's answer was cut off after %d tokens (prompt %d tokens)",
			res.Usage.OutputTokens, res.Usage.InputTokens)
	}
	return res.Message.Content[0].Text, nil
}
