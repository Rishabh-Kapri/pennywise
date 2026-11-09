// Package sanitize turns an email's HTML into the plain text the pipeline
// reads, leaving out what a reader wouldn't see. Hidden text and link targets are
// where a sender can address the model instead of the user.
package sanitize

import (
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

const maxChars = 8000

var (
	urls        = regexp.MustCompile(`https?://\S+`)
	spaces      = regexp.MustCompile(`[ \t\r\f\v\x{00A0}]+`)
	lineBreaks  = regexp.MustCompile(` ?\n[ \n]*`)
	hiddenStyle = regexp.MustCompile(
		`(?i)display\s*:\s*none|visibility\s*:\s*hidden|font-size\s*:\s*0(px|pt|em|rem|%)?\s*(;|!|$)`,
	)
)

var skipTags = map[string]bool{"script": true, "style": true, "head": true, "template": true}

var newlineTags = map[string]bool{
	"p": true, "div": true, "tr": true, "br": true, "li": true, "table": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
}

// Text returns the visible text of raw. It returns "" if raw can't be parsed.
func Text(raw string) string {
	doc, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return ""
	}
	var b strings.Builder
	walk(doc, &b)

	text := urls.ReplaceAllString(b.String(), "")
	text = spaces.ReplaceAllString(text, " ")
	text = lineBreaks.ReplaceAllString(text, "\n")
	text = strings.TrimSpace(text)

	if chars := []rune(text); len(chars) > maxChars {
		text = string(chars[:maxChars])
	}
	return text
}

// walk writes n's text to b, skipping hidden elements. Comments are never
// written, because only text nodes are.
func walk(n *html.Node, b *strings.Builder) {
	if n.Type == html.TextNode {
		b.WriteString(n.Data)
		return
	}
	if n.Type == html.ElementNode && isHidden(n) {
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, b)
	}
	if n.Type == html.ElementNode {
		switch {
		case newlineTags[n.Data]:
			b.WriteByte('\n')
		case n.Data == "td" || n.Data == "th":
			b.WriteByte(' ') // keeps "Amount" and "500" apart
		}
	}
}

func isHidden(n *html.Node) bool {
	if skipTags[n.Data] {
		return true
	}
	for _, a := range n.Attr {
		if a.Key == "hidden" || (a.Key == "style" && hiddenStyle.MatchString(a.Val)) {
			return true
		}
	}
	return false
}
