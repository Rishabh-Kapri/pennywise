package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestLogger_LogsBodiesWhenDebugEnabled(t *testing.T) {
	t.Setenv("GIN_MODE", gin.TestMode)
	gin.SetMode(gin.TestMode)

	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() {
		slog.SetDefault(previous)
	})

	router := gin.New()
	router.Use(RequestLogger())
	router.POST("/echo", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"received": string(body)})
	})

	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(`{"name":"logan"}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, res.Code)
	}

	entry := readLogEntry(t, logs.String())
	if got := entry["request_body"]; got != `{"name":"logan"}` {
		t.Fatalf("expected request body to be logged, got %v", got)
	}

	responseBody, ok := entry["response_body"].(string)
	if !ok {
		t.Fatalf("expected response body to be logged, got %v", entry["response_body"])
	}
	if !strings.Contains(responseBody, `"received":"{\"name\":\"logan\"}"`) {
		t.Fatalf("expected response body to include echoed payload, got %s", responseBody)
	}
	if entry["status"] != float64(http.StatusCreated) {
		t.Fatalf("expected status %d, got %v", http.StatusCreated, entry["status"])
	}
}

func TestRequestLogger_OmitsBodiesWithoutDebugLogging(t *testing.T) {
	t.Setenv("GIN_MODE", gin.TestMode)
	gin.SetMode(gin.TestMode)

	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() {
		slog.SetDefault(previous)
	})

	router := gin.New()
	router.Use(RequestLogger())
	router.POST("/echo", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"received": string(body)})
	})

	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(`{"name":"logan"}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}

	entry := readLogEntry(t, logs.String())
	if _, ok := entry["request_body"]; ok {
		t.Fatalf("expected request body to be omitted, got %v", entry["request_body"])
	}
	if _, ok := entry["response_body"]; ok {
		t.Fatalf("expected response body to be omitted, got %v", entry["response_body"])
	}
}

func readLogEntry(t *testing.T, raw string) map[string]any {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatal("expected at least one log line")
	}

	entry := map[string]any{}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &entry); err != nil {
		t.Fatalf("failed to decode log line: %v", err)
	}

	return entry
}
