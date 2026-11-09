package llmAdapter

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	prompts "github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/context"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

func TestReplyText(t *testing.T) {
	reply := func(stop sharedModel.StopReason, texts ...string) *sharedModel.ChatResponse {
		res := &sharedModel.ChatResponse{StopReason: stop}
		for _, text := range texts {
			res.Message.Content = append(res.Message.Content, sharedModel.ContentBlock{Type: "text", Text: text})
		}
		return res
	}
	tests := []struct {
		name    string
		res     *sharedModel.ChatResponse
		want    string
		wantErr string
	}{
		{"complete", reply(sharedModel.StopReasonEndTurn, `{"amount":1}`), `{"amount":1}`, ""},
		{"skips an empty block", reply(sharedModel.StopReasonEndTurn, "", `{"amount":1}`), `{"amount":1}`, ""},
		{"cut off mid-JSON", reply(sharedModel.StopReasonMaxTokens, `{"amount":`), "", "token limit"},
		{"blank text", reply(sharedModel.StopReasonEndTurn, "  "), "", "empty response"},
		{"no blocks", reply(sharedModel.StopReasonEndTurn), "", "empty response"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := replyText(tt.res, "m", errs.CodeMessageExtractFailed)
			if tt.wantErr == "" {
				if err != nil || got != tt.want {
					t.Fatalf("got %q, %v", got, err)
				}
				return
			}
			if !errs.HasCode(err, errs.CodeMessageExtractFailed) || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("want %s, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestToEntities(t *testing.T) {
	want := time.Date(2026, 11, 4, 0, 19, 43, 0, time.UTC)
	tests := []struct {
		name   string
		json   string
		amount float64
		at     time.Time
		acct   string
	}{
		{"normal", `{"account":"HDFC 1","amount":-650.5,"date":"2026-11-04","time":"00:19:43"}`, -650.5, want, "HDFC 1"},
		{"amount as a string, day first", `{"account":"HDFC 1","amount":"13.04","date":"04-11-2026","time":"00:19:43"}`, 13.04, want, "HDFC 1"},
		{"older account field, no time", `{"account_card":"4567","amount":1,"date":"2026-11-04"}`, 1, want.Truncate(24 * time.Hour), "4567"},
		{"month name with dashes", `{"account":"HDFC 1","amount":-650.5,"date":"04-Nov-2026","time":"00:19:43"}`, -650.5, want, "HDFC 1"},
		{"not a transaction", `{}`, 0, time.Time{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var e extraction
			if err := json.Unmarshal([]byte(tt.json), &e); err != nil {
				t.Fatal(err)
			}
			got, err := toEntities(e)
			if err != nil {
				t.Fatal(err)
			}
			if got.Amount != tt.amount || !got.Timestamp.Equal(tt.at) || got.Account != tt.acct {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestToEntitiesRejectsAnUnreadableDate(t *testing.T) {
	if _, err := toEntities(extraction{Date: "sometime"}); err == nil {
		t.Fatal("want an error, so the next model can try")
	}
}

// Models copy the prompt's examples, so each one has to be something Extract
// can read, with exactly the fields the prompt asks for.
func TestExtractionPromptExamplesAreValid(t *testing.T) {
	want := []string{"account", "amount", "currency", "date", "merchant", "time"}
	found := 0
	for line := range strings.Lines(prompts.ExtractionPrompt) {
		_, output, ok := strings.Cut(line, "Output: ")
		if !ok || !strings.HasPrefix(output, "{") {
			continue
		}
		found++
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(output), &fields); err != nil {
			t.Fatalf("invalid JSON: %s: %v", output, err)
		}
		if len(fields) != len(want) {
			t.Fatalf("fields in %s, want %v", output, want)
		}
		for _, name := range want {
			if _, ok := fields[name]; !ok {
				t.Fatalf("%s is missing %s", output, name)
			}
		}
		var e extraction
		if err := json.Unmarshal([]byte(output), &e); err != nil {
			t.Fatal(err)
		}
		if _, err := toEntities(e); err != nil {
			t.Fatalf("%s: %v", output, err)
		}
	}
	if found == 0 {
		t.Fatal("no examples found")
	}
}
