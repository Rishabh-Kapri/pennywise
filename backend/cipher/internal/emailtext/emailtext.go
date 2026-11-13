// Package emailtext turns an email's HTML into text.
package emailtext

import (
	"regexp"
	"strings"

	"jaytaylor.com/html2text"
)

var (
	options    = html2text.Options{PrettyTables: false, OmitLinks: true, TextOnly: true}
	blankLines = regexp.MustCompile(`\n{3,}`)
)

// Plain is the email as one line of text: what models read, and what dedupe
// hashes and lookup keys are made from. Stored transactions and learned
// aliases came from text made this way, so changing it changes their keys.
func Plain(html string) (string, error) {
	text, err := html2text.FromString(html, options)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(strings.ReplaceAll(text, "\n", "")), nil
}

// Display is the email as a person reads it, with its line breaks. It mustn't
// go into hashes or lookup keys.
func Display(html string) (string, error) {
	text, err := html2text.FromString(html, options)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(blankLines.ReplaceAllString(text, "\n\n")), nil
}
