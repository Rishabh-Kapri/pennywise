// statement-try reads one statement PDF on your machine with the same readers
// and the same checks the statement.extract job uses, and says in plain words
// what happened at each step: what was read, whether the numbers check out,
// whether the second read ran, and which read was kept.
//
//	go run ./cmd/statement-try statement.pdf
//	go run ./cmd/statement-try -first llm -second glm statement.pdf
//	go run ./cmd/statement-try -second off -out result.json statement.pdf
//
// Options go before the PDF (Go stops reading them at the first file name).
//
// Nothing is stored and no budget is involved. The models are called directly
// (Ollama at -ollama, or OpenRouter with OPENROUTER_API_KEY). The rows are
// printed to your terminal and, with -out, written to a file: keep that file
// out of git.
//
// The decision mirrors readStatement in internal/worker/handlers.go; keep the
// two in step.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/llm"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/agent/llm/providers"
	llmAdapter "github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/adapters/llm"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/adapters/localocr"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/progress"
	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/statementcheck"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/otelSDK"
)

type options struct {
	first, second string
	ocrModel      string
	llmModel      string
	llmVia        string
	ollamaURL     string
	passwords     []string
	out           string
	showRows      bool
	verbose       bool
}

func main() {
	var o options
	var password string
	flag.StringVar(
		&o.first,
		"first",
		"glm",
		"the first read: glm (glm-ocr looks at page pictures) or llm (a chat model reads the PDF's text)",
	)
	flag.StringVar(
		&o.second,
		"second",
		"llm",
		"the second read, used only when the first doesn't check out: glm, llm or off",
	)
	flag.StringVar(&o.ocrModel, "ocr-model", "glm-ocr", "Ollama model for glm")
	flag.StringVar(
		&o.llmModel,
		"llm-model",
		"qwen3.5:9b",
		"model for llm, e.g. qwen3.5:9b, or anthropic/claude-haiku-4.5 with -llm-via openrouter",
	)
	flag.StringVar(
		&o.llmVia,
		"llm-via",
		"ollama",
		"where the llm model runs: ollama or openrouter (needs OPENROUTER_API_KEY)",
	)
	flag.StringVar(&o.ollamaURL, "ollama", envOr("OLLAMA_URL", "http://192.168.1.34:11434"), "Ollama address")
	flag.StringVar(
		&password,
		"password",
		os.Getenv("PDF_PASSWORD"),
		"PDF password, if it's locked (or set PDF_PASSWORD)",
	)
	flag.StringVar(&o.out, "out", "", "also write both reads and the decision to this JSON file")
	flag.BoolVar(&o.showRows, "rows", true, "print the rows that were read")
	flag.BoolVar(
		&o.verbose,
		"v",
		false,
		"also print the readers' own logs (each model call, and how glm-ocr's tables were parsed)",
	)
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: statement-try [flags] statement.pdf")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}
	if !o.verbose {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	}
	if password != "" {
		o.passwords = []string{password}
	}
	pdf, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		fail(err)
	}
	if err := run(context.Background(), o, pdf); err != nil {
		fail(err)
	}
}

// read is one reader's attempt and what the check made of it.
type read struct {
	Reader  string                         `json:"reader"`
	Seconds float64                        `json:"seconds"`
	Error   string                         `json:"error,omitempty"`
	Result  model.StatementExtractResponse `json:"result"`
	Check   statementcheck.Result          `json:"check"`
}

func run(ctx context.Context, o options, pdf []byte) error {
	report := struct {
		First  *read  `json:"first"`
		Second *read  `json:"second,omitempty"`
		Kept   string `json:"kept"`
		Why    string `json:"why"`
	}{}

	title("Step 1: first read, with " + describe(o, o.first))
	first, err := readWith(ctx, o, o.first, pdf)
	if err != nil {
		return err
	}
	report.First = first
	if first.Error != "" {
		// The job fails here too: there's nothing to fall back from.
		return fmt.Errorf("the first read failed, so the job would fail: %s", first.Error)
	}
	show(first, o.showRows)

	title("Step 2: do the numbers check out?")
	explainCheck(first.Check)

	keep := func(r *read, why string) error {
		report.Kept, report.Why = r.Reader, why
		title("Result")
		fmt.Printf("Kept the %s read, whole. %s\n", r.Reader, why)
		fmt.Println("Nothing is mixed between the two reads: the kept one is used as it is.")
		return writeOut(o.out, report)
	}

	if first.Check.OK() {
		fmt.Println("\nThey do, so the second read is skipped.")
		return keep(first, "It checked out.")
	}
	if o.second == "off" {
		fmt.Println("\nThey don't, but the second read is turned off.")
		return keep(first, "The second read is off.")
	}

	title("Step 3: second read, with " + describe(o, o.second))
	fmt.Println("It reads the whole statement again from scratch. It doesn't see the first read.")
	second, err := readWith(ctx, o, o.second, pdf)
	if err != nil {
		return err
	}
	report.Second = second
	if second.Error != "" {
		fmt.Println("It failed:", second.Error)
		return keep(first, "The second read failed.")
	}
	show(second, o.showRows)
	fmt.Println()
	explainCheck(second.Check)

	title("Step 4: which read is better?")
	pickSecond := statementcheck.Better(second.Check, first.Check)
	fmt.Println(compare(second.Check, first.Check))
	if pickSecond {
		return keep(second, "The second read was better.")
	}
	return keep(first, "The second read wasn't better.")
}

func readWith(ctx context.Context, o options, which string, pdf []byte) (*read, error) {
	reader, err := newReader(o, which)
	if err != nil {
		return nil, err
	}
	ctx = progress.With(ctx, func(s progress.Step) {
		if s.Total > 0 {
			fmt.Printf("  … %s (%d of %d)\n", s.Name, s.Current, s.Total)
		} else {
			fmt.Printf("  … %s\n", s.Name)
		}
	})
	started := time.Now()
	res, err := reader.ExtractStatement(ctx, pdf, o.passwords)
	r := &read{Reader: which, Seconds: time.Since(started).Seconds()}
	if err != nil {
		r.Error = err.Error()
		return r, nil
	}
	r.Result = res
	r.Check = statementcheck.Check(res)
	fmt.Printf("  took %.0fs\n", r.Seconds)
	return r, nil
}

func newReader(o options, which string) (ports.StatementImport, error) {
	tel, err := otelSDK.NewTelemetry(
		context.Background(),
		otelSDK.Config{ServiceName: "statement-try", OtelSdkDisabled: true},
	)
	if err != nil {
		return nil, err
	}
	ollama := func() (*llm.ObservedLLM, error) {
		c, err := providers.NewUserOllamaClient(o.ollamaURL, "", http.DefaultClient)
		if err != nil {
			return nil, err
		}
		return llm.NewObservedLLM(c, tel, "local"), nil
	}
	switch which {
	case "glm":
		client, err := ollama()
		if err != nil {
			return nil, err
		}
		return localocr.NewOCRAdapter(client, localocr.OCRConfig{Provider: "ollama", Model: o.ocrModel})
	case "llm":
		var client *llm.ObservedLLM
		provider := o.llmVia
		switch o.llmVia {
		case "ollama":
			if client, err = ollama(); err != nil {
				return nil, err
			}
		case "openrouter":
			key := os.Getenv("OPENROUTER_API_KEY")
			if key == "" {
				return nil, fmt.Errorf("-llm-via openrouter needs OPENROUTER_API_KEY")
			}
			c, err := providers.NewOpenRouterClientWithKey(key)
			if err != nil {
				return nil, err
			}
			client = llm.NewObservedLLM(c, tel, "cloud")
		default:
			return nil, fmt.Errorf("-llm-via must be ollama or openrouter, not %q", o.llmVia)
		}
		return llmAdapter.NewLLMAdapter(llmAdapter.Config{
			Client:      client,
			Provider:    provider,
			Model:       o.llmModel,
			CallTimeout: 10 * time.Minute,
		}), nil
	}
	return nil, fmt.Errorf("a reader is glm or llm, not %q", which)
}

func describe(o options, which string) string {
	if which == "glm" {
		return o.ocrModel + " (looks at a picture of each page)"
	}
	return o.llmModel + " on " + o.llmVia + " (reads the PDF's text, one page at a time)"
}

// show prints what a read found: each account's header, then its rows.
func show(r *read, rows bool) {
	if len(r.Result.Sections) == 0 {
		fmt.Println("  Found nothing.")
		return
	}
	for i, s := range r.Result.Sections {
		fmt.Printf("\n  Account %d: %s %s", i+1, orDash(s.AccountLabel), s.AccountNumber)
		if s.Kind != "" {
			fmt.Printf(" (%s)", s.Kind)
		}
		fmt.Println()
		field("period", str(s.PeriodStart)+" to "+str(s.PeriodEnd))
		field("statement date", str(s.StatementDate))
		field("opening balance", num(s.OpeningBalance))
		field("closing balance", num(s.ClosingBalance))
		if s.Kind != model.StatementKindBank {
			field("due date", str(s.DueDate))
			field("total due", num(s.TotalDue))
			field("minimum due", num(s.MinimumDue))
			field("credit limit", num(s.CreditLimit))
			field("reward points", rewards(s.Rewards))
		}
		if len(s.EMIPlans) > 0 {
			field("EMI plans", fmt.Sprint(len(s.EMIPlans)))
		}
		field("rows", fmt.Sprint(len(s.Rows)))
		if !rows {
			continue
		}
		for _, row := range s.Rows {
			sign := "-"
			if row.Direction == model.StatementCredit {
				sign = "+"
			}
			extra := ""
			if row.Balance != nil {
				extra += fmt.Sprintf("  bal %.2f", *row.Balance)
			}
			if row.RewardPoints != nil {
				extra += fmt.Sprintf("  pts %d", *row.RewardPoints)
			}
			if row.Page > 0 {
				extra += fmt.Sprintf("  p.%d", row.Page)
			}
			fmt.Printf("    %-10s %s%10.2f  %-40.40s%s\n", row.Date, sign, row.Amount, row.Description, extra)
		}
	}
}

// explainCheck says, per the check's own reasons, why a read did or didn't pass.
func explainCheck(c statementcheck.Result) {
	fmt.Printf(
		"  %d rows; %d of %d accounts could be checked against printed numbers.\n",
		c.Rows,
		c.Verified,
		c.Sections,
	)
	fmt.Println("  An account checks out when opening balance + rows = closing balance (or total due on a card),")
	fmt.Println("  or when every row's printed balance follows from the row before it.")
	if c.OK() {
		fmt.Println("  ✓ Everything checks out.")
		return
	}
	for _, reason := range c.Reasons {
		fmt.Println("  ✗", reason)
	}
}

func compare(a, b statementcheck.Result) string {
	switch {
	case a.OK() != b.OK():
		return "Only one of them checks out; that one wins."
	case (a.Rows > 0) != (b.Rows > 0):
		return "Only one of them found any rows; that one wins."
	case a.GoodRows != b.GoodRows:
		return fmt.Sprintf("Rows in accounts that check out: second %d, first %d. More wins.", a.GoodRows, b.GoodRows)
	case len(a.Reasons) != len(b.Reasons):
		return fmt.Sprintf("Problems: second %d, first %d. Fewer wins.", len(a.Reasons), len(b.Reasons))
	default:
		return fmt.Sprintf("Rows: second %d, first %d. More wins; a tie keeps the first.", a.Rows, b.Rows)
	}
}

func writeOut(path string, report any) error {
	if path == "" {
		return nil
	}
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return err
	}
	fmt.Println("\nWrote", path, "(it holds the statement's rows: keep it out of git)")
	return nil
}

func rewards(r *model.StatementRewards) string {
	if r == nil {
		return ""
	}
	var parts []string
	for _, p := range []struct {
		name string
		v    *int
	}{{"opening", r.Opening}, {"earned", r.Earned}, {"redeemed", r.Redeemed}, {"expired", r.Expired}, {"closing", r.Closing}} {
		if p.v != nil {
			parts = append(parts, fmt.Sprintf("%s %d", p.name, *p.v))
		}
	}
	return strings.Join(parts, ", ")
}

func title(s string) { fmt.Printf("\n== %s ==\n", s) }

func field(name, value string) {
	if strings.Trim(value, " to") == "" {
		value = "(not found)"
	}
	fmt.Printf("    %-16s %s\n", name+":", value)
}

func str(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func num(f *float64) string {
	if f == nil {
		return ""
	}
	return fmt.Sprintf("%.2f", *f)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
