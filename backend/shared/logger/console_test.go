package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testPayload struct {
	EmailText string  `json:"emailText"`
	Amount    float64 `json:"amount"`
}

type testStringer struct{}

func (testStringer) String() string { return "test-stringer" }

// newTestLogger returns a logger writing uncolored console output to buf.
func newTestLogger(buf *bytes.Buffer, level slog.Level) *slog.Logger {
	return slog.New(newConsoleHandler(buf, consoleOptions{Level: level, AddSource: true}))
}

var timePrefix = regexp.MustCompile(`^\d{2}:\d{2}:\d{2}\.\d{3} `)

func firstLine(t *testing.T, buf *bytes.Buffer) string {
	t.Helper()
	line, _, _ := strings.Cut(buf.String(), "\n")
	require.Regexp(t, timePrefix, line, "every line starts with HH:MM:SS.mmm")
	return timePrefix.ReplaceAllString(line, "")
}

func TestConsole_InlineAttributes(t *testing.T) {
	var buf bytes.Buffer
	newTestLogger(&buf, slog.LevelDebug).Info("listing transactions",
		"limit", 30, "sort", "DESC", "note", "", "query", "blue tokai", "ok", true, "took", 312*time.Millisecond)

	line := firstLine(t, &buf)
	assert.True(t, strings.HasPrefix(line, "INF listing transactions  limit=30 sort=DESC "), line)
	assert.Contains(t, line, `note=""`)
	assert.Contains(t, line, `query="blue tokai"`, "values with spaces are quoted")
	assert.Contains(t, line, "ok=true")
	assert.Contains(t, line, "took=312ms")
	assert.Equal(t, 1, strings.Count(buf.String(), "\n"), "simple values stay on one line")
}

func TestConsole_Levels(t *testing.T) {
	cases := map[slog.Level]string{
		slog.LevelDebug: "DBG", slog.LevelInfo: "INF", slog.LevelWarn: "WRN", slog.LevelError: "ERR",
	}
	for level, label := range cases {
		var buf bytes.Buffer
		newTestLogger(&buf, slog.LevelDebug).Log(context.Background(), level, "m")
		assert.True(t, strings.HasPrefix(firstLine(t, &buf), label+" m"), label)
	}

	var buf bytes.Buffer
	l := newTestLogger(&buf, slog.LevelInfo)
	l.Debug("hidden")
	assert.Empty(t, buf.String(), "records below the level are dropped")
}

func TestConsole_JSONAndStructsBecomeIndentedBlocks(t *testing.T) {
	want := "{\n  \"emailText\": \"hello\",\n  \"amount\": -12.5\n}"
	cases := map[string]any{
		"json string":     `{"emailText":"hello","amount":-12.5}`,
		"struct":          testPayload{EmailText: "hello", Amount: -12.5},
		"pointer":         &testPayload{EmailText: "hello", Amount: -12.5},
		"json.RawMessage": json.RawMessage(`{"emailText":"hello","amount":-12.5}`),
		"map":             map[string]any{"amount": -12.5, "emailText": "hello"},
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			newTestLogger(&buf, slog.LevelDebug).Info("payload", "body", value)

			lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
			require.Greater(t, len(lines), 1, buf.String())
			assert.NotContains(t, lines[0], "body", "blocks are not repeated inline")
			block := strings.Join(lines[1:], "\n")
			assert.True(t, strings.HasPrefix(block, "    body: {"), block)
			if name == "map" {
				assert.Contains(t, block, `      "amount": -12.5`)
				return
			}
			assert.Equal(t, "    body: "+strings.ReplaceAll(want, "\n", "\n    "), block)
		})
	}
}

func TestConsole_ErrorsAndStringersKeepTheirText(t *testing.T) {
	var buf bytes.Buffer
	newTestLogger(&buf, slog.LevelDebug).Warn("geocode failed",
		"err", errors.New("nominatim: context deadline exceeded"), "who", testStringer{})

	line := firstLine(t, &buf)
	assert.Contains(t, line, `err="nominatim: context deadline exceeded"`)
	assert.Contains(t, line, "who=test-stringer")
	assert.Equal(t, 1, strings.Count(buf.String(), "\n"))
}

func TestConsole_LongAndMultilineStringsMoveToABlock(t *testing.T) {
	var buf bytes.Buffer
	email := "Dear Customer,\nRs. 2440.95 has been debited"
	newTestLogger(&buf, slog.LevelDebug).Info("parsed", "email", email, "long", strings.Repeat("x", 150))

	out := buf.String()
	assert.Contains(t, out, "    email: Dear Customer,\n    Rs. 2440.95 has been debited\n", "shown as written, not escaped")
	assert.Contains(t, out, "    long:\n      "+strings.Repeat("x", 150)+"\n")
	assert.NotContains(t, out, `\n`)
}

func TestConsole_RequestContextIsShortenedAndTrailing(t *testing.T) {
	var buf bytes.Buffer
	budget := uuid.MustParse("4e490d26-b89b-4285-b20a-bbaa7ed10cd5")
	user := uuid.MustParse("9b2c0d11-1111-4222-8333-444455556666")
	l := newTestLogger(&buf, slog.LevelDebug).With(
		"service", "pennywise-api",
		"correlation_id", "6f1c2a9e-0000-4000-8000-000000000001",
		"caller_service", "cipher", "origin_service", "gmail-watch",
		"verified_internal", true,
		"budget_id", budget.String(), "user_id", user.String(),
	)
	l.Info("created", "txn", "t1")

	line := firstLine(t, &buf)
	assert.True(t, strings.HasPrefix(line, "INF created  txn=t1   cid=6f1c2a9e  budget=4e490d26  user=9b2c0d11  from=cipher  origin=gmail-watch  internal  "), line)
	assert.NotContains(t, line, "service=", "the service name is the same on every line of a process")
	assert.NotContains(t, line, budget.String())
}

func TestConsole_OriginIsHiddenWhenItIsTheCaller(t *testing.T) {
	var buf bytes.Buffer
	newTestLogger(&buf, slog.LevelDebug).With("caller_service", "web", "origin_service", "web").Info("m")
	line := firstLine(t, &buf)
	assert.Contains(t, line, "from=web")
	assert.NotContains(t, line, "origin=")
}

func TestConsole_GroupsPrefixKeys(t *testing.T) {
	var buf bytes.Buffer
	newTestLogger(&buf, slog.LevelDebug).
		WithGroup("http").With("method", "GET").
		Info("request", "status", 200, slog.Group("timing", "db", 4*time.Millisecond))

	line := firstLine(t, &buf)
	assert.Contains(t, line, "http.method=GET")
	assert.Contains(t, line, "http.status=200")
	assert.Contains(t, line, "http.timing.db=4ms")
}

func TestConsole_WithAttrsDoesNotLeakIntoTheParent(t *testing.T) {
	var buf bytes.Buffer
	parent := newTestLogger(&buf, slog.LevelDebug)
	_ = parent.With("child", 1)
	parent.Info("m")
	assert.NotContains(t, buf.String(), "child")
}

// The source must be the code that logged, not the logger package.
func TestContextLogger_ReportsTheCallersSource(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(newTestLogger(&buf, slog.LevelDebug))

	ctx := utils.WithBudgetID(context.Background(), uuid.MustParse("4e490d26-b89b-4285-b20a-bbaa7ed10cd5"))
	log := Logger(ctx)
	log.Debug("d")
	log.Info("i")
	log.With("k", "v").Warn("w")
	log.Error("e")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 4)
	for _, line := range lines {
		assert.Regexp(t, `logger/console_test\.go:\d+$`, line)
		assert.NotContains(t, line, "logger/logger.go")
		assert.Contains(t, line, "budget=4e490d26")
	}
}

func TestContextLogger_JSONOutputGetsTheCallersSourceToo(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{AddSource: true})))

	Logger(context.Background()).Info("hello")

	var rec struct {
		Source struct{ File string } `json:"source"`
	}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &rec))
	assert.True(t, strings.HasSuffix(rec.Source.File, "logger/console_test.go"), rec.Source.File)
}

func TestConsole_ColorOnlyWhenAsked(t *testing.T) {
	var plain, colored bytes.Buffer
	slog.New(newConsoleHandler(&plain, consoleOptions{Level: slog.LevelInfo})).Error("boom", "err", errors.New("x"))
	slog.New(newConsoleHandler(&colored, consoleOptions{Level: slog.LevelInfo, Color: true})).Error("boom", "err", errors.New("x"))

	assert.NotContains(t, plain.String(), "\x1b[")
	assert.Contains(t, colored.String(), ansiRed)
	assert.Contains(t, colored.String(), ansiReset)
}

func TestConsole_ConcurrentWritesDoNotInterleave(t *testing.T) {
	var buf bytes.Buffer
	l := newTestLogger(&buf, slog.LevelDebug)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			l.With("worker", i).Info("tick", "body", `{"a":1}`)
		}(i)
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	require.Len(t, lines, 50*4, "each record is one line plus a 3-line block")
	for i := 0; i < len(lines); i += 4 {
		assert.Regexp(t, timePrefix, lines[i])
		assert.Equal(t, `    body: {`, lines[i+1])
		assert.Equal(t, `      "a": 1`, lines[i+2])
		assert.Equal(t, `    }`, lines[i+3])
	}
}

// Two loggers derived from one parent must not share its attribute slice: with
// spare capacity, appending for one sibling would overwrite the other's attrs.
// 13 attrs is a size where append leaves spare capacity (40-byte Attr, 576-byte class).
func TestConsole_SiblingLoggersKeepTheirOwnAttributes(t *testing.T) {
	var buf bytes.Buffer
	args := make([]any, 0, 26)
	for i := 0; i < 13; i++ {
		args = append(args, "k"+string(rune('a'+i)), i)
	}
	parent := newTestLogger(&buf, slog.LevelDebug).With(args...)
	first := parent.With("child", "one")
	_ = parent.With("child", "two")

	first.Info("m")
	assert.Contains(t, buf.String(), "child=one")
	assert.NotContains(t, buf.String(), "child=two")
}
