// Package statementpdf opens statement PDFs with poppler-utils: page images
// for OCR readers, page text for LLM readers. It tries the passwords so the
// readers never see them.
package statementpdf

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
)

const execTimeout = 60 * time.Second

// The poppler binaries are variables so tests can point them at stubs.
var (
	pdftotextBinary = "pdftotext"
	pdftoppmBinary  = "pdftoppm"
)

// Render renders every page of the PDF as a PNG in dir and returns their
// paths in page order, and which of passwords opened it (-1: none was
// needed). The caller owns dir and removes it.
func Render(ctx context.Context, dir string, pdf []byte, passwords []string) ([]string, int, error) {
	var pages []string
	opened, err := withPasswords(passwords, func(pw string) error {
		var err error
		pages, err = renderPages(ctx, dir, pdf, pw)
		return err
	})
	return pages, opened, err
}

// Text returns each page's text with its column layout kept, in page order,
// and which of passwords opened the PDF (-1: none was needed).
func Text(ctx context.Context, pdf []byte, passwords []string) ([]string, int, error) {
	var out []byte
	opened, err := withPasswords(passwords, func(pw string) error {
		var err error
		out, err = runPoppler(ctx, pdftotextBinary, pdf, pw, []string{"-layout", "-enc", "UTF-8"}, "-")
		return err
	})
	if err != nil {
		return nil, -1, err
	}
	// pdftotext ends each page with a form feed.
	pages := strings.Split(strings.TrimSuffix(string(out), "\f"), "\f")
	if len(pages) == 1 && strings.TrimSpace(pages[0]) == "" {
		return nil, -1, errs.New(errs.CodeStatementExtractFailed, "the PDF has no text")
	}
	return pages, opened, nil
}

// withPasswords runs open with no password, then with each of passwords in
// turn, and returns which one worked (-1: none was needed). No password is
// tried first so an unlocked PDF isn't credited to an account.
func withPasswords(passwords []string, open func(password string) error) (int, error) {
	tries := append([]string{""}, passwords...)
	var lastErr error
	for i, pw := range tries {
		if i > 0 && pw == "" {
			continue
		}
		err := open(pw)
		if err == nil {
			return i - 1, nil
		}
		if !errs.HasCode(err, errs.CodeStatementPasswordRequired) {
			return -1, err
		}
		lastErr = err
	}
	if len(passwords) == 0 {
		return -1, lastErr
	}
	return -1, errs.New(errs.CodeStatementPasswordRequired,
		"the PDF is locked and none of the saved passwords opened it; save its password on the account")
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
