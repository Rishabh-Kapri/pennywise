package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"runtime"
	"testing"
)

func TestContextLoggerCaller(t *testing.T) {
	for _, level := range []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError} {
		t.Run(level.String(), func(t *testing.T) {
			var output bytes.Buffer
			log := &ContextLogger{
				Logger: slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{
					AddSource: true,
					Level:     slog.LevelDebug,
				})).With("service", "test"),
				ctx: context.Background(),
			}
			log = log.WithGroup("request").With("id", "123")
			var file string
			var line int
			switch level {
			case slog.LevelDebug:
				_, file, line, _ = runtime.Caller(0)
				log.Debug("message", "error", "failure")
			case slog.LevelInfo:
				_, file, line, _ = runtime.Caller(0)
				log.Info("message", "error", "failure")
			case slog.LevelWarn:
				_, file, line, _ = runtime.Caller(0)
				log.Warn("message", "error", "failure")
			case slog.LevelError:
				_, file, line, _ = runtime.Caller(0)
				log.Error("message", "error", "failure")
			}
			var record struct {
				Source  slog.Source       `json:"source"`
				Level   string            `json:"level"`
				Message string            `json:"msg"`
				Service string            `json:"service"`
				Request map[string]string `json:"request"`
			}
			if err := json.Unmarshal(output.Bytes(), &record); err != nil {
				t.Fatal(err)
			}
			if record.Source.File != file || record.Source.Line != line+1 {
				t.Fatalf("source = %s:%d, want %s:%d", record.Source.File, record.Source.Line, file, line+1)
			}
			if record.Level != level.String() || record.Message != "message" || record.Service != "test" || record.Request["id"] != "123" || record.Request["error"] != "failure" {
				t.Fatalf("unexpected log record: %+v", record)
			}
		})
	}
}

func TestContextLoggerDisabledLevel(t *testing.T) {
	var output bytes.Buffer
	log := &ContextLogger{
		Logger: slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelInfo})),
		ctx:    context.Background(),
	}
	log.Debug("disabled")
	if output.Len() != 0 {
		t.Fatalf("disabled log was emitted: %s", output.String())
	}
}
