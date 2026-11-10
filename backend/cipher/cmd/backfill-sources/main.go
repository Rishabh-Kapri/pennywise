// Command backfill-sources moves the bank text stored on transactions
// (raw_bank_text) into transaction_sources and learns counterparty_aliases
// from it.
//
// Everything runs in one database transaction: either every source and alias
// is written or nothing is. Re-running is safe: every existing source is
// parsed again from its text and updated only if the parsed fields changed,
// so a parser fix reaches existing rows by running this again. Existing
// aliases (budget_id, channel, merchant) are left as they are.
//
// Parsing is local and rule-based (shared/utils/narration); no text leaves the
// process. Alias embeddings are not computed here. Only statement lines can be
// parsed: alert emails are skipped, and their existing sources are left as they
// are, because the rules that read them are gone.
//
//	go run ./cmd/backfill-sources [-budget <uuid>] [-dry-run]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils/narration"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/factory"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// advisoryLockKey keeps two runs from interleaving.
const advisoryLockKey = "backfill-sources"

func main() {
	budgetFlag := flag.String("budget", "", "backfill only this budget (default: every budget)")
	dryRun := flag.Bool("dry-run", false, "do all the work, print the report, then roll back")
	flag.Parse()

	ctx := context.Background()
	log := logger.Logger(ctx)

	var budgetID *uuid.UUID
	if *budgetFlag != "" {
		id, err := uuid.Parse(*budgetFlag)
		if err != nil {
			logger.Fatal("invalid -budget: %v", err)
		}
		budgetID = &id
	}

	cfg := config.Load()
	db, err := factory.ConnectDB(ctx, factory.Config{Driver: cfg.DatabaseDriver, URL: cfg.DatabaseURL})
	if err != nil {
		logger.Fatal(err.Error())
	}
	defer db.Close()
	if db.Pool == nil {
		logger.Fatal("backfill-sources needs the postgres driver")
	}

	report, err := run(ctx, db.Pool, budgetID, *dryRun)
	if err != nil {
		log.Error("backfill failed, nothing was written", "error", err)
		db.Close()
		os.Exit(1)
	}
	report.print()
}

// txnRow is a transaction that has bank text.
type txnRow struct {
	ID         uuid.UUID
	BudgetID   uuid.UUID
	RawText    string
	PayeeID    *uuid.UUID
	CategoryID *uuid.UUID
	Date       string
	// CardAccount: the transaction is on a credit card account.
	CardAccount bool
	// Statement is the statement import row this transaction was created
	// from, nil for emails.
	Statement *statementRow
}

// statementRow is one row of a statement import (statement_imports.sections),
// the columns the bank printed next to the narration.
type statementRow struct {
	ImportID      uuid.UUID
	Amount        string
	Direction     string // "debit" / "credit"
	Date          string // YYYY-MM-DD
	Currency      string
	AccountNumber string
}

func run(ctx context.Context, pool *pgxpool.Pool, budgetID *uuid.UUID, dryRun bool) (*report, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after Commit

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, advisoryLockKey); err != nil {
		return nil, fmt.Errorf("lock: %w", err)
	}

	rows, err := loadTransactions(ctx, tx, budgetID)
	if err != nil {
		return nil, err
	}

	rep := newReport(dryRun)
	rep.Transactions = len(rows)

	parsed := make([]parsedTxn, 0, len(rows))
	for _, r := range rows {
		p := narration.ParseWith(r.RawText, narration.Hint{CardAccount: r.CardAccount})
		if p.Kind == narration.KindEmail {
			rep.EmailsSkipped++
			continue
		}
		if p.Kind == narration.KindStatementLine && r.Statement != nil {
			fillFromStatement(&p, *r.Statement)
			rep.StatementMatched++
		}
		parsed = append(parsed, parsedTxn{txnRow: r, Parsed: p})
		rep.countParsed(r, p)
	}

	if err := insertSources(ctx, tx, parsed, rep); err != nil {
		return nil, err
	}

	aliases, conflicts := learnAliases(parsed)
	rep.Conflicts = conflicts
	if rep.PayeeNames, err = payeeNames(ctx, tx, conflicts); err != nil {
		return nil, err
	}
	if err := insertAliases(ctx, tx, aliases, rep); err != nil {
		return nil, err
	}

	if dryRun {
		return rep, nil // deferred Rollback discards everything
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return rep, nil
}

func loadTransactions(ctx context.Context, tx pgx.Tx, budgetID *uuid.UUID) ([]txnRow, error) {
	rows, err := tx.Query(ctx, `
		SELECT t.id, t.budget_id, t.raw_bank_text, t.payee_id, t.category_id, t.date,
		       a.type = 'creditCard',
		       st.import_id, st.amount, st.direction, st.date, st.currency, st.account_number
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		-- The statement row this transaction was imported from: same account,
		-- date, narration and amount.
		LEFT JOIN LATERAL (
			SELECT si.id AS import_id,
			       r->>'amount' AS amount, r->>'direction' AS direction, r->>'date' AS date,
			       sec->>'currency' AS currency, sec->>'accountNumber' AS account_number
			FROM statement_imports si,
			     jsonb_array_elements(si.sections) sec,
			     jsonb_array_elements(sec->'rows') r
			WHERE si.budget_id = t.budget_id AND si.deleted = FALSE
			  AND sec->>'accountId' = t.account_id::text
			  AND r->>'date' = t.date
			  AND r->>'description' = t.raw_bank_text
			  AND jsonb_typeof(r->'amount') = 'number'
			  AND (r->>'amount')::numeric = abs(t.amount)
			ORDER BY si.created_at DESC
			LIMIT 1
		) st ON TRUE
		WHERE t.deleted = FALSE
		  AND t.raw_bank_text IS NOT NULL AND btrim(t.raw_bank_text) <> ''
		  -- The live pipeline's sources aren't ours to re-parse.
		  AND NOT EXISTS (
			SELECT 1 FROM transaction_sources own
			WHERE own.transaction_id = t.id AND own.origin = 'PIPELINE'
		  )
		  AND ($1::uuid IS NULL OR t.budget_id = $1)
		ORDER BY t.budget_id, t.date, t.id`, budgetID)
	if err != nil {
		return nil, fmt.Errorf("load transactions: %w", err)
	}
	defer rows.Close()

	var out []txnRow
	for rows.Next() {
		var r txnRow
		var importID *uuid.UUID
		var amount, direction, date, currency, accountNumber *string
		if err := rows.Scan(&r.ID, &r.BudgetID, &r.RawText, &r.PayeeID, &r.CategoryID, &r.Date, &r.CardAccount,
			&importID, &amount, &direction, &date, &currency, &accountNumber); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		if importID != nil {
			r.Statement = &statementRow{
				ImportID: *importID, Amount: deref(amount), Direction: deref(direction),
				Date: deref(date), Currency: deref(currency), AccountNumber: deref(accountNumber),
			}
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load transactions: %w", err)
	}
	return out, nil
}

// fillFromStatement adds what the statement's other columns state, for fields
// the narration itself didn't give.
func fillFromStatement(p *narration.Parsed, st statementRow) {
	if p.Amount == "" {
		p.Amount = st.Amount
	}
	if p.Currency == "" {
		p.Currency = strings.ToUpper(st.Currency)
	}
	if p.Direction == "" {
		switch strings.ToLower(st.Direction) {
		case "debit":
			p.Direction = narration.Debit
		case "credit":
			p.Direction = narration.Credit
		}
	}
	if p.OccurredOn == nil {
		if d, err := time.ParseInLocation(time.DateOnly, st.Date, ist); err == nil {
			p.OccurredOn = &d
		}
	}
	if p.AccountHint == "" && len(st.AccountNumber) >= 4 {
		p.AccountHint = st.AccountNumber[len(st.AccountNumber)-4:]
	}
}

var ist = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		return time.FixedZone("IST", 5*3600+1800)
	}
	return loc
}()

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

type parsedTxn struct {
	txnRow
	narration.Parsed
}

const insertSourceSQL = `
	INSERT INTO transaction_sources (
		budget_id, transaction_id, kind, raw_text,
		channel, merchant, vpa, ref, account_hint, direction,
		amount, currency, occurred_on, occurred_at, statement_import_id
	) VALUES (
		$1, $2, $3, $4,
		$5, $6, $7, $8, $9, $10,
		$11::text::numeric, $12, $13::date, $14, $15
	)
	-- An existing row is re-parsed: its parsed fields are replaced, but only
	-- when they changed. raw_text and rewards are kept, and a statement link
	-- is only ever added, never cleared.
	ON CONFLICT (transaction_id, kind) DO UPDATE SET
		channel = EXCLUDED.channel,
		merchant = EXCLUDED.merchant,
		vpa = EXCLUDED.vpa,
		ref = EXCLUDED.ref,
		account_hint = EXCLUDED.account_hint,
		direction = EXCLUDED.direction,
		amount = EXCLUDED.amount,
		currency = EXCLUDED.currency,
		occurred_on = EXCLUDED.occurred_on,
		occurred_at = EXCLUDED.occurred_at,
		statement_import_id = COALESCE(EXCLUDED.statement_import_id, transaction_sources.statement_import_id),
		updated_at = now()
	WHERE (transaction_sources.channel, transaction_sources.merchant, transaction_sources.vpa,
	       transaction_sources.ref, transaction_sources.account_hint, transaction_sources.direction,
	       transaction_sources.amount, transaction_sources.currency, transaction_sources.occurred_on,
	       transaction_sources.occurred_at, transaction_sources.statement_import_id)
	      IS DISTINCT FROM
	      (EXCLUDED.channel, EXCLUDED.merchant, EXCLUDED.vpa,
	       EXCLUDED.ref, EXCLUDED.account_hint, EXCLUDED.direction,
	       EXCLUDED.amount, EXCLUDED.currency, EXCLUDED.occurred_on,
	       EXCLUDED.occurred_at,
	       COALESCE(EXCLUDED.statement_import_id, transaction_sources.statement_import_id))
	RETURNING (xmax = 0) AS inserted`

func insertSources(ctx context.Context, tx pgx.Tx, parsed []parsedTxn, rep *report) error {
	batch := &pgx.Batch{}
	for _, p := range parsed {
		var occurredOn any
		if p.OccurredOn != nil {
			occurredOn = p.OccurredOn.Format(time.DateOnly)
		}
		var importID *uuid.UUID
		if p.Statement != nil {
			importID = &p.Statement.ImportID
		}
		batch.Queue(insertSourceSQL,
			p.BudgetID, p.ID, p.Kind, p.RawText,
			null(p.Channel), null(p.Merchant), null(p.VPA), null(p.Ref), null(p.AccountHint), null(p.Direction),
			null(p.Amount), null(p.Currency), occurredOn, p.OccurredAt, importID,
		)
	}
	results := tx.SendBatch(ctx, batch)
	for _, p := range parsed {
		var inserted bool
		err := results.QueryRow().Scan(&inserted)
		switch {
		case errors.Is(err, pgx.ErrNoRows): // already current: nothing returned
			rep.SourcesExisting++
		case err != nil:
			results.Close()
			return fmt.Errorf("insert source for transaction %s: %w", p.ID, err)
		case inserted:
			rep.SourcesInserted++
		default:
			rep.SourcesReparsed++
		}
	}
	return results.Close()
}

// alias is one learned "CHANNEL | MERCHANT" -> payee mapping.
type alias struct {
	BudgetID   uuid.UUID
	Channel    string
	Merchant   string
	PayeeID    uuid.UUID
	CategoryID *uuid.UUID
	HitCount   int
	LastSeen   time.Time
}

// conflict is a key whose transactions went to more than one payee. The
// majority payee was learned; the user may want to look at the rest.
type conflict struct {
	Key    string
	Payees map[uuid.UUID]int
}

type aliasKey struct {
	BudgetID          uuid.UUID
	Channel, Merchant string
}

type payeeVotes struct {
	Count      int
	Categories map[uuid.UUID]int
	LastSeen   time.Time
}

// learnAliases picks, for each key, the payee most of its transactions were
// filed under (ties: the most recently seen), and that payee's most common
// category among them.
func learnAliases(parsed []parsedTxn) ([]alias, []conflict) {
	votes := map[aliasKey]map[uuid.UUID]*payeeVotes{}
	for _, p := range parsed {
		if p.LookupKey() == "" || p.PayeeID == nil {
			continue
		}
		k := aliasKey{p.BudgetID, p.Channel, p.Merchant}
		if votes[k] == nil {
			votes[k] = map[uuid.UUID]*payeeVotes{}
		}
		v := votes[k][*p.PayeeID]
		if v == nil {
			v = &payeeVotes{Categories: map[uuid.UUID]int{}}
			votes[k][*p.PayeeID] = v
		}
		v.Count++
		if p.CategoryID != nil {
			v.Categories[*p.CategoryID]++
		}
		if seen := seenAt(p); seen.After(v.LastSeen) {
			v.LastSeen = seen
		}
	}

	keys := make([]aliasKey, 0, len(votes))
	for k := range votes {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.BudgetID != b.BudgetID {
			return a.BudgetID.String() < b.BudgetID.String()
		}
		if a.Channel != b.Channel {
			return a.Channel < b.Channel
		}
		return a.Merchant < b.Merchant
	})

	var aliases []alias
	var conflicts []conflict
	for _, k := range keys {
		byPayee := votes[k]
		var bestPayee uuid.UUID
		var best *payeeVotes
		for payee, v := range byPayee {
			if best == nil || v.Count > best.Count ||
				(v.Count == best.Count && v.LastSeen.After(best.LastSeen)) ||
				(v.Count == best.Count && v.LastSeen.Equal(best.LastSeen) && payee.String() < bestPayee.String()) {
				bestPayee, best = payee, v
			}
		}
		a := alias{
			BudgetID: k.BudgetID, Channel: k.Channel, Merchant: k.Merchant,
			PayeeID: bestPayee, CategoryID: topCategory(best.Categories),
			HitCount: best.Count, LastSeen: best.LastSeen,
		}
		aliases = append(aliases, a)
		if len(byPayee) > 1 {
			c := conflict{Key: k.Channel + " | " + k.Merchant, Payees: map[uuid.UUID]int{}}
			for payee, v := range byPayee {
				c.Payees[payee] = v.Count
			}
			conflicts = append(conflicts, c)
		}
	}
	return aliases, conflicts
}

func topCategory(counts map[uuid.UUID]int) *uuid.UUID {
	var top *uuid.UUID
	for id, n := range counts {
		if top == nil || n > counts[*top] || (n == counts[*top] && id.String() < top.String()) {
			id := id
			top = &id
		}
	}
	return top
}

// seenAt is when the source says the payment happened, else the
// transaction's own date.
func seenAt(p parsedTxn) time.Time {
	if p.OccurredAt != nil {
		return *p.OccurredAt
	}
	if p.OccurredOn != nil {
		return *p.OccurredOn
	}
	t, _ := time.Parse(time.DateOnly, p.Date)
	return t
}

const insertAliasSQL = `
	INSERT INTO counterparty_aliases (
		budget_id, channel, merchant, payee_id, category_id, source, hit_count, last_seen_at
	) VALUES ($1, $2, $3, $4, $5, 'AUTO_LEARNED', $6, $7)
	ON CONFLICT (budget_id, channel, merchant) DO NOTHING`

func insertAliases(ctx context.Context, tx pgx.Tx, aliases []alias, rep *report) error {
	if len(aliases) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, a := range aliases {
		var lastSeen any
		if !a.LastSeen.IsZero() {
			lastSeen = a.LastSeen
		}
		batch.Queue(insertAliasSQL, a.BudgetID, a.Channel, a.Merchant, a.PayeeID, a.CategoryID, a.HitCount, lastSeen)
	}
	results := tx.SendBatch(ctx, batch)
	for _, a := range aliases {
		tag, err := results.Exec()
		if err != nil {
			results.Close()
			return fmt.Errorf("insert alias %s | %s: %w", a.Channel, a.Merchant, err)
		}
		if tag.RowsAffected() == 1 {
			rep.AliasesInserted++
		} else {
			rep.AliasesExisting++
		}
	}
	return results.Close()
}

// payeeNames names the payees in conflicts, for the report.
func payeeNames(ctx context.Context, tx pgx.Tx, conflicts []conflict) (map[uuid.UUID]string, error) {
	var ids []uuid.UUID
	for _, c := range conflicts {
		for id := range c.Payees {
			ids = append(ids, id)
		}
	}
	names := map[uuid.UUID]string{}
	if len(ids) == 0 {
		return names, nil
	}
	rows, err := tx.Query(ctx, `SELECT id, name FROM payees WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("load payee names: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, fmt.Errorf("scan payee name: %w", err)
		}
		names[id] = name
	}
	return names, rows.Err()
}

func null(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ---- report ----

type report struct {
	DryRun       bool
	Transactions int
	ByKind       map[string]int
	ByChannel    map[string]int
	NoKey        []uuid.UUID // transactions whose text gave no lookup key
	NoPayee      int         // keyed, but the transaction has no payee to learn
	// EmailsSkipped: transactions whose text is an alert email, not a statement line.
	EmailsSkipped int
	// StatementMatched: statement lines whose import row filled in amount,
	// direction, date, currency and account.
	StatementMatched int
	SourcesInserted  int
	SourcesExisting  int
	SourcesReparsed  int // parsed again with a different result, updated
	AliasesInserted  int
	AliasesExisting  int
	Conflicts        []conflict
	PayeeNames       map[uuid.UUID]string
}

func newReport(dryRun bool) *report {
	return &report{DryRun: dryRun, ByKind: map[string]int{}, ByChannel: map[string]int{}}
}

func (r *report) countParsed(row txnRow, p narration.Parsed) {
	r.ByKind[p.Kind]++
	if p.LookupKey() == "" {
		r.NoKey = append(r.NoKey, row.ID)
		return
	}
	r.ByChannel[p.Channel]++
	if row.PayeeID == nil {
		r.NoPayee++
	}
}

func (r *report) print() {
	if r.DryRun {
		fmt.Println("DRY RUN: everything below was rolled back")
	}
	fmt.Printf("transactions with bank text: %d\n", r.Transactions)
	fmt.Printf("  by kind:    %v\n", r.ByKind)
	fmt.Printf("  by channel: %v\n", r.ByChannel)
	fmt.Printf("  no lookup key: %d\n", len(r.NoKey))
	for i, id := range r.NoKey {
		if i == 10 {
			fmt.Printf("    ... and %d more\n", len(r.NoKey)-10)
			break
		}
		fmt.Printf("    %s\n", id)
	}
	fmt.Printf("  keyed but no payee: %d\n", r.NoPayee)
	fmt.Printf("  statement lines matched to their import row: %d of %d\n", r.StatementMatched, r.ByKind[narration.KindStatementLine])
	fmt.Printf("  alert emails skipped: %d\n", r.EmailsSkipped)
	fmt.Printf("sources: %d inserted, %d updated by re-parsing, %d unchanged\n",
		r.SourcesInserted, r.SourcesReparsed, r.SourcesExisting)
	fmt.Printf("aliases: %d inserted, %d already present\n", r.AliasesInserted, r.AliasesExisting)
	if len(r.Conflicts) > 0 {
		fmt.Printf("keys filed under several payees (majority learned): %d\n", len(r.Conflicts))
		for _, c := range r.Conflicts {
			fmt.Printf("  %s ->", c.Key)
			for id, n := range c.Payees {
				name := r.PayeeNames[id]
				if name == "" {
					name = id.String()
				}
				fmt.Printf(" %q×%d", name, n)
			}
			fmt.Println()
		}
	}
}
