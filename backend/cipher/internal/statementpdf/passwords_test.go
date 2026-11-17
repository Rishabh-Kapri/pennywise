package statementpdf

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
)

// fakePdftoppm stands in for pdftoppm: it renders one page when given the
// password "right" or when the PDF isn't locked, and fails like poppler does
// on a wrong or missing password otherwise.
func fakePdftoppm(t *testing.T, locked bool) {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "pdftoppm")
	lockedFlag := "0"
	if locked {
		lockedFlag = "1"
	}
	body := `#!/bin/sh
pw=""
while [ $# -gt 0 ]; do
  case "$1" in
    -upw) pw="$2"; shift 2;;
    -opw|-r) shift 2;;
    -png) shift;;
    *) last="$1"; shift;;
  esac
done
if [ "` + lockedFlag + `" = "1" ] && [ "$pw" != "right" ]; then
  echo "Command Line Error: Incorrect password" >&2; exit 1
fi
: > "$last-1.png"
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	old := pdftoppmBinary
	pdftoppmBinary = script
	t.Cleanup(func() { pdftoppmBinary = old })
}

func TestRenderWithPasswordsPicksTheOneThatOpens(t *testing.T) {
	fakePdftoppm(t, true)
	pages, idx, err := Render(context.Background(), t.TempDir(), []byte("%PDF"), []string{"wrong", "right", "other"})
	if err != nil || idx != 1 || len(pages) != 1 {
		t.Fatalf("pages=%v idx=%d err=%v", pages, idx, err)
	}
}

func TestRenderWithPasswordsUnlockedCreditsNoOne(t *testing.T) {
	fakePdftoppm(t, false)
	_, idx, err := Render(context.Background(), t.TempDir(), []byte("%PDF"), []string{"right"})
	if err != nil || idx != -1 {
		t.Fatalf("idx=%d err=%v, want -1", idx, err)
	}
}

func TestRenderWithPasswordsNoneWork(t *testing.T) {
	fakePdftoppm(t, true)
	_, _, err := Render(context.Background(), t.TempDir(), []byte("%PDF"), []string{"a", "b"})
	if !errs.HasCode(err, errs.CodeStatementPasswordRequired) {
		t.Fatalf("err = %v, want password required", err)
	}
}

// fakePdftotext prints two pages, each ended by a form feed like pdftotext.
func fakePdftotext(t *testing.T) {
	t.Helper()
	script := filepath.Join(t.TempDir(), "pdftotext")
	body := "#!/bin/sh\nprintf 'page one\\fpage two\\f'\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	old := pdftotextBinary
	pdftotextBinary = script
	t.Cleanup(func() { pdftotextBinary = old })
}

func TestTextSplitsPages(t *testing.T) {
	fakePdftotext(t)
	pages, idx, err := Text(context.Background(), []byte("%PDF"), nil)
	if err != nil || idx != -1 || len(pages) != 2 || pages[0] != "page one" || pages[1] != "page two" {
		t.Fatalf("pages=%q idx=%d err=%v", pages, idx, err)
	}
}
