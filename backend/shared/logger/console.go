package logger

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

// consoleHandler renders records for a developer's terminal:
//
//	11:31:04.123 INF listing transactions  limit=30 sort=DESC   cid=c0ffee12 budget=4e490d26  handler/transaction.go:115
//	    filter: {
//	      "AccountIDs": ["8d9e7e78-..."],
//	      "Limit": 30
//	    }
//
// What you logged comes first; the ambient request context (correlation id,
// budget, user, caller) follows, dimmed and shortened; the source location is
// last. JSON strings, structs and maps, multi-line and very long values are
// printed as indented blocks under the line instead of escaped inline.
type consoleHandler struct {
	mu        *sync.Mutex
	w         io.Writer
	level     slog.Leveler
	color     bool
	addSource bool

	attrs  []slog.Attr // from WithAttrs, keys already prefixed with their group
	prefix string      // current group path, "a.b."
}

type consoleOptions struct {
	Level     slog.Leveler
	Color     bool
	AddSource bool
}

func newConsoleHandler(w io.Writer, opts consoleOptions) *consoleHandler {
	level := opts.Level
	if level == nil {
		level = slog.LevelInfo
	}
	return &consoleHandler{mu: &sync.Mutex{}, w: w, level: level, color: opts.Color, addSource: opts.AddSource}
}

func (h *consoleHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *consoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.attrs = append(append([]slog.Attr{}, h.attrs...), prefixAttrs(h.prefix, attrs)...)
	return &clone
}

func (h *consoleHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	clone.prefix = h.prefix + name + "."
	return &clone
}

// ANSI styles.
const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiDim    = "\x1b[2m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiBlue   = "\x1b[34m"
	ansiCyan   = "\x1b[36m"
)

func (h *consoleHandler) style(s, codes string) string {
	if !h.color || s == "" {
		return s
	}
	return codes + s + ansiReset
}

// contextKeys are attached by Logger(ctx) on every line; they are shown last,
// dimmed, under a short name.
var contextKeys = map[string]string{
	"correlation_id":    "cid",
	"budget_id":         "budget",
	"user_id":           "user",
	"caller_service":    "from",
	"origin_service":    "origin",
	"verified_internal": "internal",
}

// hiddenKeys add nothing when every line in a terminal comes from one process.
var hiddenKeys = map[string]bool{"service": true}

// Values longer than this, or spanning lines, move to a block under the line.
const inlineLimit = 120

type renderedAttr struct {
	key, value string
	block      bool
	isErr      bool
}

func (h *consoleHandler) Handle(_ context.Context, r slog.Record) error {
	var inline, blocks []renderedAttr
	ctxVals := map[string]string{}
	add := func(a slog.Attr) {
		flattenAttr("", a, func(key string, v slog.Value) {
			if hiddenKeys[key] {
				return
			}
			if short, ok := contextKeys[key]; ok {
				ctxVals[short] = shortID(v.String())
				return
			}
			ra := renderValue(key, v)
			if ra.block {
				blocks = append(blocks, ra)
			} else {
				inline = append(inline, ra)
			}
		})
	}
	for _, a := range h.attrs {
		add(a)
	}
	r.Attrs(func(a slog.Attr) bool {
		if h.prefix != "" {
			a.Key = h.prefix + a.Key
		}
		add(a)
		return true
	})

	var buf bytes.Buffer
	buf.WriteString(h.style(r.Time.Format("15:04:05.000"), ansiDim))
	buf.WriteByte(' ')
	buf.WriteString(h.levelLabel(r.Level))
	buf.WriteByte(' ')
	if r.Level >= slog.LevelError {
		buf.WriteString(h.style(r.Message, ansiBold+ansiRed))
	} else {
		buf.WriteString(h.style(r.Message, ansiBold))
	}

	if len(inline) > 0 {
		buf.WriteString(" ")
	}
	for _, a := range inline {
		buf.WriteByte(' ')
		buf.WriteString(h.style(a.key+"=", ansiCyan))
		if a.isErr {
			buf.WriteString(h.style(a.value, ansiRed))
		} else {
			buf.WriteString(a.value)
		}
	}

	if trailer := h.trailer(ctxVals, r.PC); trailer != "" {
		buf.WriteString("   ")
		buf.WriteString(h.style(trailer, ansiDim))
	}
	buf.WriteByte('\n')

	for _, b := range blocks {
		buf.WriteString("    ")
		buf.WriteString(h.style(b.key+":", ansiCyan))
		body := b.value
		if b.isErr {
			body = h.style(body, ansiRed)
		}
		if strings.Contains(b.value, "\n") {
			buf.WriteByte(' ')
			buf.WriteString(strings.ReplaceAll(body, "\n", "\n    "))
		} else {
			buf.WriteString("\n      ")
			buf.WriteString(body)
		}
		buf.WriteByte('\n')
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(buf.Bytes())
	return err
}

func (h *consoleHandler) levelLabel(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return h.style("ERR", ansiBold+ansiRed)
	case level >= slog.LevelWarn:
		return h.style("WRN", ansiBold+ansiYellow)
	case level >= slog.LevelInfo:
		return h.style("INF", ansiGreen)
	default:
		return h.style("DBG", ansiBlue)
	}
}

// trailer is the dimmed context and source after the attributes, in a fixed order.
func (h *consoleHandler) trailer(ctxVals map[string]string, pc uintptr) string {
	var parts []string
	for _, k := range []string{"cid", "budget", "user", "from", "origin", "internal"} {
		v, ok := ctxVals[k]
		if !ok || (k == "origin" && v == ctxVals["from"]) {
			continue
		}
		if k == "internal" {
			parts = append(parts, "internal")
			continue
		}
		parts = append(parts, k+"="+v)
	}
	if h.addSource && pc != 0 {
		frame, _ := runtime.CallersFrames([]uintptr{pc}).Next()
		if frame.File != "" {
			parts = append(parts, fmt.Sprintf("%s:%d", shortPath(frame.File), frame.Line))
		}
	}
	return strings.Join(parts, "  ")
}

// flattenAttr walks groups, joining keys with dots.
func flattenAttr(prefix string, a slog.Attr, emit func(string, slog.Value)) {
	v := a.Value.Resolve()
	key := a.Key
	if prefix != "" && key != "" {
		key = prefix + "." + key
	} else if key == "" {
		key = prefix
	}
	if v.Kind() == slog.KindGroup {
		for _, ga := range v.Group() {
			flattenAttr(key, ga, emit)
		}
		return
	}
	if a.Equal(slog.Attr{}) {
		return
	}
	emit(key, v)
}

func prefixAttrs(prefix string, attrs []slog.Attr) []slog.Attr {
	if prefix == "" {
		return attrs
	}
	out := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		out[i] = slog.Attr{Key: prefix + a.Key, Value: a.Value}
	}
	return out
}

func renderValue(key string, v slog.Value) renderedAttr {
	ra := renderedAttr{key: key}
	switch v.Kind() {
	case slog.KindString:
		s := v.String()
		if pretty, ok := prettyJSONString(s); ok {
			ra.value, ra.block = pretty, true
			return ra
		}
		if strings.Contains(s, "\n") || len(s) > inlineLimit {
			ra.value, ra.block = strings.TrimRight(s, "\n"), true // shown as written
			return ra
		}
		ra.value = quoteIfNeeded(s)
	case slog.KindDuration:
		ra.value = v.Duration().String()
	case slog.KindTime:
		ra.value = v.Time().Format(time.RFC3339Nano)
	case slog.KindAny:
		val := v.Any()
		if err, ok := val.(error); ok {
			ra.value, ra.isErr = quoteIfNeeded(err.Error()), true
			return ra
		}
		if pretty, ok := prettyAny(val); ok {
			ra.value, ra.block = pretty, true
			return ra
		}
		ra.value = quoteIfNeeded(fmt.Sprintf("%+v", val))
	default:
		ra.value = v.String()
	}
	if strings.Contains(ra.value, "\n") || len(ra.value) > inlineLimit {
		ra.block = true
		if unquoted, err := strconv.Unquote(ra.value); err == nil {
			ra.value = unquoted
		}
	}
	return ra
}

// quoteIfNeeded leaves simple tokens bare and quotes anything with spaces,
// quotes or control characters, so key=value pairs stay unambiguous.
func quoteIfNeeded(s string) string {
	if s == "" {
		return `""`
	}
	for _, r := range s {
		if unicode.IsSpace(r) || r == '"' || r == '=' || !unicode.IsPrint(r) {
			return strconv.Quote(s)
		}
	}
	return s
}

// shortID trims UUIDs (and other long ids) to their first 8 characters.
func shortID(s string) string {
	if len(s) == 36 && strings.Count(s, "-") == 4 {
		return s[:8]
	}
	if len(s) > 12 {
		return s[:8] + "…"
	}
	return s
}

// shortPath keeps the last directory and file name: "service/transaction.go".
func shortPath(file string) string {
	dir, name := filepath.Split(file)
	return filepath.Join(filepath.Base(dir), name)
}
