package localocr

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/llm"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/progress"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

const execTimeout = 60 * time.Second

// ocrContextWindow fits a page and its table. A 200 dpi A4 page is about 4,000
// prompt tokens and a full page of rows about 5,000 more; Ollama's default
// window (4096) leaves almost nothing for the answer.
const ocrContextWindow = 16384

// pdftotextBinary is a variable so tests can point it at a stub.
var (
	pdftotextBinary = "pdftotext"
	pdftoppmBinary  = "pdftoppm"
)

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

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return "unknown error"
	}
	return s
}

// runPoppler runs a poppler tool (pdftoppm, pdftotext, ...) on the PDF and
// returns what it wrote to stdout. The PDF is staged in a temp file, removed
// before return. The command line is: binary opts [password flags] <pdf file> trailing.
// The password is passed on the command line for the life of the process; it
// is never stored or logged.
func runPoppler(
	ctx context.Context,
	binary string,
	pdf []byte,
	password string,
	opts []string,
	trailing ...string,
) ([]byte, error) {
	if _, err := exec.LookPath(binary); err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, binary+" is not installed (poppler-utils)", err)
	}

	f, err := os.CreateTemp("", "statement-*.pdf")
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "could not stage statement", err)
	}
	// remove file after we are done
	defer os.Remove(f.Name())

	if _, err := f.Write(pdf); err != nil {
		f.Close()
		return nil, errs.Wrap(errs.CodeInternalError, "could not stage statement", err)
	}
	if err := f.Close(); err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "could not stage statement", err)
	}

	runCtx, cancel := context.WithTimeout(ctx, execTimeout)
	defer cancel()

	args := append([]string{}, opts...)
	if password != "" {
		args = append(args, "-upw", password, "-opw", password)
	}
	args = append(args, f.Name())
	args = append(args, trailing...)

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(runCtx, binary, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.ToLower(stderr.String())
		if strings.Contains(msg, "password") || strings.Contains(msg, "encrypt") {
			return nil, errs.Wrap(errs.CodeStatementPasswordRequired, "file is password protected", err)
		}
		if runCtx.Err() != nil {
			return nil, errs.New(errs.CodeInternalError, "reading the PDF took too long")
		}
		return nil, errs.New(errs.CodeStatementExtractFailed, "could not read this PDF: %s", firstLine(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// renderPages renders every page of the PDF as a PNG in dir and returns their
// paths in page order. The caller owns dir and removes it.
func renderPages(ctx context.Context, dir string, pdf []byte, password string) ([]string, error) {
	// pdftoppm writes <prefix>-1.png, <prefix>-2.png, ... and pads the number
	// once there are 10 or more pages (page-01.png), so order by the number.
	prefix := filepath.Join(dir, "page")
	if _, err := runPoppler(ctx, pdftoppmBinary, pdf, password, []string{"-png", "-r", "200"}, prefix); err != nil {
		return nil, err
	}
	return listPages(dir)
}

// renderWithPasswords renders the PDF with no password, then with each of
// passwords in turn, and returns which one opened it (-1: none was needed).
// No password is tried first so an unlocked PDF isn't credited to an account.
func renderWithPasswords(ctx context.Context, dir string, pdf []byte, passwords []string) ([]string, int, error) {
	tries := append([]string{""}, passwords...)
	var lastErr error
	for i, pw := range tries {
		if i > 0 && pw == "" {
			continue
		}
		pages, err := renderPages(ctx, dir, pdf, pw)
		if err == nil {
			return pages, i - 1, nil
		}
		if !errs.HasCode(err, errs.CodeStatementPasswordRequired) {
			return nil, -1, err
		}
		lastErr = err
	}
	if len(passwords) == 0 {
		return nil, -1, lastErr
	}
	return nil, -1, errs.New(errs.CodeStatementPasswordRequired,
		"the PDF is locked and none of the saved passwords opened it; save its password on the account")
}

func listPages(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "could not list rendered pages", err)
	}

	type page struct {
		num  int
		path string
	}
	pages := make([]page, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, "page-") || !strings.HasSuffix(name, ".png") {
			continue
		}
		num, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(name, "page-"), ".png"))
		if err != nil {
			continue
		}
		pages = append(pages, page{num, filepath.Join(dir, name)})
	}
	if len(pages) == 0 {
		return nil, errs.New(errs.CodeStatementExtractFailed, "the PDF has no pages")
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].num < pages[j].num })

	paths := make([]string, len(pages))
	for i, p := range pages {
		paths[i] = p.path
	}
	return paths, nil
}

// ocrPage sends one page image to the OCR model and returns what it read
// (GLM-OCR answers "Table Recognition:" with HTML tables).
// OCR prompts GLM-OCR understands: tables come back as HTML, text as lines.
const (
	promptTables = "Table Recognition:"
	promptText   = "Text Recognition:"
)

func (a *adapter) ocrPage(ctx context.Context, path, prompt string) (string, error) {
	img, err := os.ReadFile(path)
	if err != nil {
		return "", errs.Wrap(errs.CodeInternalError, "could not read rendered page", err)
	}

	req := sharedModel.ChatRequest{
		Provider: a.cfg.Provider,
		Model:    a.cfg.Model,
		Messages: []sharedModel.AgentMessage{
			{
				Role:    sharedModel.RoleUser,
				Content: []sharedModel.ContentBlock{{Type: "text", Text: prompt}},
				Images:  []string{base64.StdEncoding.EncodeToString(img)},
			},
		},
		Temperature:   0,
		Stream:        false,
		MaxTokens:     a.cfg.MaxTokens,
		ContextWindow: ocrContextWindow,
	}
	res, err := a.client.Chat(ctx, req, nil)
	if err != nil {
		return "", err
	}
	if res == nil || len(res.Message.Content) == 0 {
		return "", errs.New(errs.CodeStatementExtractFailed, "the OCR model returned nothing")
	}
	// A cut-off answer is a table with no end, which parses to no rows. Fail
	// loudly instead of reporting an empty statement.
	if res.StopReason == sharedModel.StopReasonMaxTokens {
		return "", errs.New(errs.CodeStatementExtractFailed,
			"the OCR model's answer was cut off after %d tokens (prompt %d tokens)",
			res.Usage.OutputTokens, res.Usage.InputTokens)
	}
	return res.Message.Content[0].Text, nil
}

// Extract impelements [ports.StatementImport]
func (a *adapter) Extract(
	ctx context.Context,
	pdf []byte,
	passwords []string,
) (sharedModel.StatementExtractResponse, error) {
	// The page images show the statement, so the folder goes even on failure.
	dir, err := os.MkdirTemp("", "statement-pages-*")
	if err != nil {
		return sharedModel.StatementExtractResponse{}, errs.Wrap(
			errs.CodeInternalError,
			"could not stage statement",
			err,
		)
	}
	defer os.RemoveAll(dir)

	progress.Report(ctx, "rendering pages")
	pages, opened, err := renderWithPasswords(ctx, dir, pdf, passwords)
	if err != nil {
		return sharedModel.StatementExtractResponse{}, err
	}

	total := len(pages)
	var parser tableParser
	progress.ReportCount(ctx, "reading pages", 0, total)
	for i, path := range pages {
		if err := ctx.Err(); err != nil {
			return sharedModel.StatementExtractResponse{}, err
		}
		tables, err := a.ocrPage(ctx, path, promptTables)
		if err != nil {
			return sharedModel.StatementExtractResponse{}, errs.Wrap(
				errs.CodeStatementExtractFailed, fmt.Sprintf("could not read page %d", i+1), err,
			)
		}
		if i == 0 {
			// The header decides how rows read (a minus sign, a date with no
			// year), so it comes before page 1's rows.
			parser.header = a.readHeader(ctx, path, tables)
		}
		parser.parsePage(i+1, tables)
		progress.ReportCount(ctx, "reading pages", i+1, total)
	}

	res := parser.result()
	res.PasswordIndex = opened
	// Only counts and found/not-found are logged: the text is the user's
	// statement.
	h := parser.header
	logger.Logger(ctx).Info("statement pages read",
		"pages", total, "rows", len(parser.rows), "kind", h.kind,
		"account_number_found", h.accountNumber != "", "period_found", h.periodStart != "",
		"rewards_found", h.rewards != nil)
	return res, nil
}

// readHeader reads page 1 as text as well as tables: the account number and
// period are usually prose, the balances a summary box. A failed text read
// costs only the header, not the statement.
func (a *adapter) readHeader(ctx context.Context, path, tables string) statementHeader {
	progress.Report(ctx, "reading account details")
	lines := headerLines(tables)
	text, err := a.ocrPage(ctx, path, promptText)
	if err != nil {
		logger.Logger(ctx).Warn("statement first page not read as text", "error", err)
	} else {
		lines = append(headerLines(text), lines...)
	}
	return parseHeader(lines)
}
