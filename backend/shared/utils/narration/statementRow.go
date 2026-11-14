package narration

import (
	"regexp"
	"strings"
	"time"
)

// StatementLine is one statement row's narration, read.
type StatementLine struct {
	Parsed
	// Clean is the narration without its trailing value date and reference
	// ("... Value Dt 02/10/2026 Ref 123456789012"), which only repeat what
	// the row's own columns say.
	Clean string
	// ValueDate is the printed value date as YYYY-MM-DD, "" when none.
	ValueDate string
}

var (
	valueDateRe = regexp.MustCompile(`(?i)\bvalue\s*(?:dt|date)\.?\s*[:\-]?\s*(\d{1,2}[/.\-]\d{1,2}[/.\-]\d{2,4})`)
	// refRe is a labelled reference anywhere in bank text: "Ref 7046...",
	// "Ref No: AB12...", "RRN 1234...", "UTR: HDFCN...", "Txn ID 98...".
	refRe = regexp.MustCompile(`(?i)\b(?:ref(?:erence)?|rrn|utr|txn\s*(?:id|no))\.?\s*(?:no\.?|number|#)?\s*[:\-]?\s*([A-Z0-9]{6,30})\b`)
)

var valueDateLayouts = []string{"02/01/2006", "02-01-2006", "02.01.2006", "02/01/06", "02-01-06", "2/1/2006"}

// ParseStatementRow reads a statement row's narration (description cell).
// The OCR joins a wrapped narration's lines with spaces, which can land
// inside a UPI address or IFSC; a UPI line that doesn't parse is retried with
// those spaces removed.
func ParseStatementRow(raw string, hint Hint) StatementLine {
	text := clean(raw)
	line := StatementLine{Clean: text}

	cut := len(text)
	if loc := valueDateRe.FindStringSubmatchIndex(text); loc != nil {
		line.ValueDate = parseValueDate(text[loc[2]:loc[3]])
		cut = loc[0]
	}
	// A reference ends the narration only when nothing but the value date
	// follows it; one in the middle is part of the text.
	var tailRef string
	for _, loc := range refRe.FindAllStringSubmatchIndex(text, -1) {
		rest := strings.TrimSpace(text[loc[1]:])
		if rest != "" && !valueDateRe.MatchString(rest) {
			continue
		}
		if !hasDigits(text[loc[2]:loc[3]], 4) {
			continue
		}
		tailRef = strings.ToUpper(text[loc[2]:loc[3]])
		if loc[0] < cut {
			cut = loc[0]
		}
		break
	}
	line.Clean = strings.TrimRight(strings.TrimSpace(text[:cut]), " -/,")
	if line.Clean == "" {
		line.Clean = text
	}

	line.Parsed = ParseWith(line.Clean, hint)
	if line.Channel == "" && strings.HasPrefix(strings.ToUpper(line.Clean), "UPI") && strings.Contains(line.Clean, "@") {
		if p := ParseWith(despaceUPI(line.Clean), hint); p.Channel == ChannelUPI {
			line.Parsed = p
		}
	}
	if line.Ref == "" {
		line.Ref = tailRef
	}
	return line
}

// despaceUPI removes wrap spaces from the machine-written part of a UPI
// narration: the address and everything after it (IFSC, RRN). The name before
// the address keeps its spaces.
func despaceUPI(s string) string {
	at := strings.Index(s, "@")
	head, tail := s[:at], s[at:]
	start := strings.LastIndex(head, "-") + 1
	return head[:start] + strings.ReplaceAll(head[start:], " ", "") + strings.ReplaceAll(tail, " ", "")
}

func parseValueDate(s string) string {
	for _, l := range valueDateLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return ""
}

func hasDigits(s string, n int) bool {
	c := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			c++
		}
	}
	return c >= n
}
