package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
)

// Statement text comes from poppler's pdftotext in -layout mode: it keeps a
// table row on one line with its columns aligned, which is what lets a model
// tell a debit column from a credit column. Pure-Go extractors emit text runs
// in content-stream order, which scrambles exactly those tables.

var (
	// ErrStatementPasswordRequired: the PDF is encrypted and no (or the wrong)
	// password was given. Indian banks password-protect most statements.
	ErrStatementPasswordRequired = errors.New("this statement is password protected: enter its password")
	// ErrStatementNoText: the PDF has no text layer, i.e. it is a scan.
	ErrStatementNoText = errors.New("no text found in this PDF; scanned statements are not supported yet")
)

const pdfTextTimeout = 60 * time.Second

// pdftotextBinary is a variable so tests can point it at a stub.
var pdftotextBinary = "pdftotext"

// extractPDFPages returns the text of each page. The password, when given, is
// passed on pdftotext's command line for the life of the process; it is never
// stored or logged.
func extractPDFPages(ctx context.Context, pdf []byte, password string) ([]string, error) {
	if _, err := exec.LookPath(pdftotextBinary); err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "pdftotext is not installed on the server (poppler-utils)", err)
	}

	// pdftotext needs a seekable file; the temp file is removed before return.
	f, err := os.CreateTemp("", "statement-*.pdf")
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "could not stage statement", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(pdf); err != nil {
		f.Close()
		return nil, errs.Wrap(errs.CodeInternalError, "could not stage statement", err)
	}
	if err := f.Close(); err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "could not stage statement", err)
	}

	runCtx, cancel := context.WithTimeout(ctx, pdfTextTimeout)
	defer cancel()

	args := []string{"-layout", "-enc", "UTF-8"}
	if password != "" {
		// A user password opens the file; an owner password also lifts copy
		// restrictions. Statements use the former, but either may be what the
		// user knows, and poppler accepts each for its own role.
		args = append(args, "-upw", password, "-opw", password)
	}
	args = append(args, f.Name(), "-")

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(runCtx, pdftotextBinary, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.ToLower(stderr.String())
		if strings.Contains(msg, "password") || strings.Contains(msg, "encrypt") {
			return nil, ErrStatementPasswordRequired
		}
		if runCtx.Err() != nil {
			return nil, errs.New(errs.CodeInternalError, "reading the PDF took too long")
		}
		return nil, fmt.Errorf("could not read this PDF: %s", firstLine(stderr.String()))
	}

	return splitPDFPages(stdout.String())
}

// splitPDFPages splits pdftotext output on its form feeds. A trailing form
// feed ends the last page, so the final empty element is not a page.
func splitPDFPages(text string) ([]string, error) {
	pages := strings.Split(text, "\f")
	if n := len(pages); n > 0 && strings.TrimSpace(pages[n-1]) == "" {
		pages = pages[:n-1]
	}
	hasText := false
	for _, p := range pages {
		if strings.TrimSpace(p) != "" {
			hasText = true
			break
		}
	}
	if !hasText {
		return nil, ErrStatementNoText
	}
	return pages, nil
}

// statementChunk is a run of consecutive pages sent to the model together.
type statementChunk struct {
	FirstPage int
	LastPage  int
	Text      string
}

// chunkStatementPages groups pages into chunks of at most maxRunes, each page
// headed "=== Page N ===" so rows can report where they came from. A page is
// never split: a row cut in half would be extracted twice or not at all. A
// single page over the limit is sent alone and truncated.
func chunkStatementPages(pages []string, maxRunes int) []statementChunk {
	var chunks []statementChunk
	var b strings.Builder
	current := statementChunk{}
	runes := 0

	flush := func() {
		if current.FirstPage == 0 {
			return
		}
		current.Text = b.String()
		chunks = append(chunks, current)
		b.Reset()
		current = statementChunk{}
		runes = 0
	}

	for i, page := range pages {
		page = trimLayoutPage(page)
		if page == "" {
			continue
		}
		pageNo := i + 1
		block := fmt.Sprintf("=== Page %d ===\n%s\n", pageNo, page)
		blockRunes := len([]rune(block))
		if blockRunes > maxRunes {
			block = string([]rune(block)[:maxRunes])
			blockRunes = maxRunes
		}
		if runes > 0 && runes+blockRunes > maxRunes {
			flush()
		}
		if current.FirstPage == 0 {
			current.FirstPage = pageNo
		}
		current.LastPage = pageNo
		b.WriteString(block)
		runes += blockRunes
	}
	flush()
	return chunks
}

// trimLayoutPage drops trailing spaces and blank-line runs. -layout pads
// lines to the page width; that whitespace costs tokens and carries nothing.
func trimLayoutPage(page string) string {
	lines := strings.Split(page, "\n")
	out := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		line = strings.TrimRight(line, " \t\r")
		if line == "" {
			if blank {
				continue
			}
			blank = true
		} else {
			blank = false
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
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
