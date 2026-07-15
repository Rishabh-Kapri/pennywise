package utils

import "strings"

func StripMarkdownFence(text string) string {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "```") {
		return text
	}

	// Drop the opening fence line, including optional language labels like
	// ```json. If the response is malformed and has no newline, leave it as-is.
	firstNewline := strings.IndexByte(text, '\n')
	if firstNewline == -1 {
		return text
	}
	text = strings.TrimSpace(text[firstNewline+1:])

	if endFence := strings.LastIndex(text, "```"); endFence != -1 {
		text = strings.TrimSpace(text[:endFence])
	}

	return text
}
