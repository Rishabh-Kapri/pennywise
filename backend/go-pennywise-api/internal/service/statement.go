package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	MaxStatementSizeBytes = 20 << 20
	MaxStatementPages     = 60

	// statementChunkRunes sizes one extraction request: about two dense
	// statement pages, small enough for a local model's context and for its
	// output (every row echoed back as JSON) to fit the response budget.
	statementChunkRunes = 9000
	// statementChunkTimeout bounds one chunk across the whole provider chain.
	statementChunkTimeout = 10 * time.Minute
	statementRunTimeout   = time.Hour
	statementListLimit    = 50
)

var ErrStatementNotFound = errors.New("statement not found")

type StatementService interface {
	// Upload reads the PDF's text and queues extraction, returning at once.
	// The same file uploaded again returns the existing import.
	Upload(ctx context.Context, fileName string, pdf []byte, password string, defaultAccountID *uuid.UUID) (*model.StatementImport, bool, error)
	List(ctx context.Context) ([]model.StatementImport, error)
	Reconcile(ctx context.Context, id uuid.UUID) (*model.StatementReconciliation, error)
	SetSectionAccount(ctx context.Context, id uuid.UUID, section int, accountID *uuid.UUID) (*model.StatementReconciliation, error)
	CreateMissing(ctx context.Context, id uuid.UUID, req model.StatementCreateReq) (*model.StatementCreateResult, error)
	Retry(ctx context.Context, id uuid.UUID) (*model.StatementImport, error)
	Delete(ctx context.Context, id uuid.UUID) error
	// FailInterrupted is called once at startup; see the repository method.
	FailInterrupted(ctx context.Context)
}

type statementService struct {
	repo               repository.StatementImportRepository
	transactionRepo    repository.TransactionRepository
	accountRepo        repository.AccountRepository
	payeeRepo          repository.PayeesRepository
	categoryRepo       repository.CategoryRepository
	transactionService TransactionService
	cipherClient       CipherClient
	// extractSlot serialises extraction across uploads: several PDFs dropped
	// at once would otherwise all queue on the same local model anyway, each
	// holding its timeout while it waits.
	extractSlot chan struct{}
}

func NewStatementService(
	repo repository.StatementImportRepository,
	transactionRepo repository.TransactionRepository,
	accountRepo repository.AccountRepository,
	payeeRepo repository.PayeesRepository,
	categoryRepo repository.CategoryRepository,
	transactionService TransactionService,
	cipherClient CipherClient,
) StatementService {
	return &statementService{
		repo:               repo,
		transactionRepo:    transactionRepo,
		accountRepo:        accountRepo,
		payeeRepo:          payeeRepo,
		categoryRepo:       categoryRepo,
		transactionService: transactionService,
		cipherClient:       cipherClient,
		extractSlot:        make(chan struct{}, 1),
	}
}

func (s *statementService) Upload(
	ctx context.Context,
	fileName string,
	pdf []byte,
	password string,
	defaultAccountID *uuid.UUID,
) (*model.StatementImport, bool, error) {
	budgetID := utils.MustBudgetID(ctx)

	if !bytes.HasPrefix(bytes.TrimLeft(pdf, "\x00\t\r\n "), []byte("%PDF-")) {
		return nil, false, errs.New(errs.CodeInvalidArgument, "file is not a PDF")
	}
	if defaultAccountID != nil {
		if _, err := s.accountRepo.GetById(ctx, nil, budgetID, *defaultAccountID); err != nil {
			return nil, false, errs.New(errs.CodeInvalidArgument, "account not found")
		}
	}

	sum := sha256.Sum256(pdf)
	hash := hex.EncodeToString(sum[:])
	if existing, err := s.repo.FindByHash(ctx, budgetID, hash); err != nil {
		return nil, false, errs.Wrap(errs.CodeInternalError, "error checking for a duplicate statement", err)
	} else if existing != nil {
		return existing, true, nil
	}

	pages, err := extractPDFPages(ctx, pdf, password)
	if err != nil {
		return nil, false, err
	}
	if len(pages) > MaxStatementPages {
		return nil, false, errs.New(errs.CodeInvalidArgument,
			"statement has %d pages, the limit is %d", len(pages), MaxStatementPages)
	}

	imp, err := s.repo.Create(ctx, model.StatementImport{
		BudgetID:         budgetID,
		FileName:         fileName,
		FileHash:         hash,
		Status:           model.StatementImportProcessing,
		PageCount:        len(pages),
		DefaultAccountID: defaultAccountID,
		// Pages are rejoined with the form feed pdftotext split them on, so a
		// retry re-chunks the stored text exactly as the first run did.
		SourceText: strings.Join(pages, "\f"),
	})
	if err != nil {
		return nil, false, errs.Wrap(errs.CodeInternalError, "error saving statement", err)
	}

	s.startExtraction(ctx, budgetID, *imp)
	return imp, false, nil
}

func (s *statementService) List(ctx context.Context) ([]model.StatementImport, error) {
	return s.repo.List(ctx, utils.MustBudgetID(ctx), statementListLimit)
}

func (s *statementService) Retry(ctx context.Context, id uuid.UUID) (*model.StatementImport, error) {
	budgetID := utils.MustBudgetID(ctx)
	if err := s.repo.Restart(ctx, budgetID, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errs.New(errs.CodeInvalidArgument, "only a failed statement can be retried")
		}
		return nil, err
	}
	imp, err := s.load(ctx, budgetID, id)
	if err != nil {
		return nil, err
	}
	s.startExtraction(ctx, budgetID, *imp)
	return imp, nil
}

func (s *statementService) Delete(ctx context.Context, id uuid.UUID) error {
	err := s.repo.Delete(ctx, utils.MustBudgetID(ctx), id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrStatementNotFound
	}
	return err
}

func (s *statementService) FailInterrupted(ctx context.Context) {
	n, err := s.repo.FailInterrupted(ctx, "extraction was interrupted by a server restart; retry it")
	if err != nil {
		logger.Logger(ctx).Warn("could not mark interrupted statement imports", "error", err)
		return
	}
	if n > 0 {
		logger.Logger(ctx).Info("marked interrupted statement imports failed", "count", n)
	}
}

func (s *statementService) load(ctx context.Context, budgetID, id uuid.UUID) (*model.StatementImport, error) {
	imp, err := s.repo.GetByID(ctx, budgetID, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrStatementNotFound
	}
	if err != nil {
		return nil, err
	}
	return imp, nil
}

// startExtraction runs the model over the statement in the background: a
// multi-page statement is several LLM round-trips, far past a proxy's request
// timeout. Progress is written as each chunk finishes and clients poll.
func (s *statementService) startExtraction(ctx context.Context, budgetID uuid.UUID, imp model.StatementImport) {
	// Detach from the request: this outlives the response by design.
	bgCtx := utils.WithRequestMetadata(context.Background(), utils.RequestMetadataFromContext(ctx))
	bgCtx = utils.WithInternalAuthToken(bgCtx, utils.InternalAuthTokenFromContext(ctx))
	bgCtx = utils.WithBudgetID(bgCtx, budgetID)

	go func() {
		runCtx, cancel := context.WithTimeout(bgCtx, statementRunTimeout)
		defer cancel()
		log := logger.Logger(runCtx).With("statementId", imp.ID)

		select {
		case s.extractSlot <- struct{}{}:
			defer func() { <-s.extractSlot }()
		case <-runCtx.Done():
			s.fail(runCtx, imp.ID, "timed out waiting for another statement to finish")
			return
		}

		if err := s.runExtraction(runCtx, imp); err != nil {
			log.Warn("statement extraction failed", "error", err)
			// The run context may be what expired; record the failure anyway.
			s.fail(bgCtx, imp.ID, err.Error())
			return
		}
		log.Info("statement extraction finished")
	}()
}

func (s *statementService) fail(ctx context.Context, id uuid.UUID, message string) {
	if err := s.repo.Fail(ctx, id, message); err != nil {
		logger.Logger(ctx).Error("could not record statement failure", "statementId", id, "error", err)
	}
}

func (s *statementService) runExtraction(ctx context.Context, imp model.StatementImport) error {
	pages := strings.Split(imp.SourceText, "\f")
	chunks := chunkStatementPages(pages, statementChunkRunes)
	if len(chunks) == 0 {
		return ErrStatementNoText
	}

	results := make([][]model.StatementSection, 0, len(chunks))
	previous := ""
	for _, chunk := range chunks {
		sections, err := s.extractChunk(ctx, chunk, previous)
		if err != nil && chunk.LastPage > chunk.FirstPage {
			// Most often the model ran out of output budget on a dense
			// chunk; one page at a time is less to transcribe per call.
			logger.Logger(ctx).Warn("statement chunk failed, retrying page by page",
				"firstPage", chunk.FirstPage, "lastPage", chunk.LastPage, "error", err)
			sections, err = s.extractPageByPage(ctx, pages, chunk, previous)
		}
		if err != nil {
			return err
		}
		if len(sections) > 0 {
			results = append(results, sections)
			last := sections[len(sections)-1]
			previous = strings.TrimSpace(last.AccountLabel + " " + last.AccountNumber)
		}
		if err := s.repo.UpdateProgress(ctx, imp.ID, chunk.LastPage); err != nil {
			logger.Logger(ctx).Warn("could not record statement progress", "error", err)
		}
	}

	sections, dropped := cleanStatementSections(mergeExtractedSections(results))
	if len(sections) == 0 {
		return errors.New("no transactions were found in this statement")
	}
	s.autoMapAccounts(ctx, imp, sections)
	return s.repo.Complete(ctx, imp.ID, sections, dropped)
}

func (s *statementService) extractChunk(
	ctx context.Context,
	chunk statementChunk,
	previous string,
) ([]model.StatementSection, error) {
	chunkCtx, cancel := context.WithTimeout(ctx, statementChunkTimeout)
	defer cancel()
	res, err := s.cipherClient.ExtractStatement(chunkCtx, model.StatementExtractRequest{
		Text:            chunk.Text,
		FirstPage:       chunk.FirstPage,
		LastPage:        chunk.LastPage,
		PreviousSection: previous,
	})
	if err != nil {
		return nil, fmt.Errorf("pages %d-%d: %w", chunk.FirstPage, chunk.LastPage, err)
	}
	if res == nil {
		return nil, nil
	}
	return res.Sections, nil
}

// extractPageByPage re-runs a failed chunk one page per call, carrying the
// account forward between pages as the chunk loop does.
func (s *statementService) extractPageByPage(
	ctx context.Context,
	pages []string,
	chunk statementChunk,
	previous string,
) ([]model.StatementSection, error) {
	var all []model.StatementSection
	for page := chunk.FirstPage; page <= chunk.LastPage; page++ {
		single := chunkStatementPages(pagesOnly(pages, page), statementChunkRunes)
		if len(single) == 0 {
			continue
		}
		sections, err := s.extractChunk(ctx, single[0], previous)
		if err != nil {
			return nil, err
		}
		if len(sections) > 0 {
			last := sections[len(sections)-1]
			previous = strings.TrimSpace(last.AccountLabel + " " + last.AccountNumber)
		}
		all = append(all, sections...)
	}
	return all, nil
}

// pagesOnly blanks every page but one, keeping page numbers intact for
// chunkStatementPages (which skips empty pages).
func pagesOnly(pages []string, page int) []string {
	out := make([]string, len(pages))
	if page >= 1 && page <= len(pages) {
		out[page-1] = pages[page-1]
	}
	return out
}

// autoMapAccounts points each section at a ledger account: by the account
// number's last four digits (accounts.suffix), else the account chosen at
// upload — for a single-section statement always, otherwise only when exactly
// one section is still unmapped, since guessing between several would be
// worse than asking.
func (s *statementService) autoMapAccounts(ctx context.Context, imp model.StatementImport, sections []model.StatementSection) {
	if len(sections) == 1 && imp.DefaultAccountID != nil {
		sections[0].AccountID = imp.DefaultAccountID
		return
	}
	unmapped := []int{}
	for i := range sections {
		if suffix := accountNumberSuffix(sections[i].AccountNumber); suffix != "" {
			if acc, err := s.accountRepo.GetBySuffix(ctx, imp.BudgetID, suffix); err == nil && acc != nil {
				id := acc.ID
				sections[i].AccountID = &id
				continue
			}
		}
		unmapped = append(unmapped, i)
	}
	if len(unmapped) == 1 && imp.DefaultAccountID != nil {
		sections[unmapped[0]].AccountID = imp.DefaultAccountID
	}
}

func (s *statementService) SetSectionAccount(
	ctx context.Context,
	id uuid.UUID,
	section int,
	accountID *uuid.UUID,
) (*model.StatementReconciliation, error) {
	budgetID := utils.MustBudgetID(ctx)
	imp, err := s.load(ctx, budgetID, id)
	if err != nil {
		return nil, err
	}
	if section < 0 || section >= len(imp.Sections) {
		return nil, errs.New(errs.CodeInvalidArgument, "section %d does not exist", section)
	}
	if accountID != nil {
		if _, err := s.accountRepo.GetById(ctx, nil, budgetID, *accountID); err != nil {
			return nil, errs.New(errs.CodeInvalidArgument, "account not found")
		}
	}
	imp.Sections[section].AccountID = accountID
	if err := s.repo.UpdateSections(ctx, budgetID, id, imp.Sections); err != nil {
		return nil, err
	}
	return s.Reconcile(ctx, id)
}

// Reconcile compares each section with its account's ledger as it is now.
func (s *statementService) Reconcile(ctx context.Context, id uuid.UUID) (*model.StatementReconciliation, error) {
	budgetID := utils.MustBudgetID(ctx)
	imp, err := s.load(ctx, budgetID, id)
	if err != nil {
		return nil, err
	}

	result := &model.StatementReconciliation{
		Import:   *imp,
		Sections: make([]model.StatementSectionReconciliation, 0, len(imp.Sections)),
	}
	if imp.Status != model.StatementImportReady {
		return result, nil
	}

	names, err := s.ledgerNames(ctx, budgetID)
	if err != nil {
		return nil, err
	}

	usedByAccount := map[uuid.UUID]map[uuid.UUID]bool{}
	for i, section := range imp.Sections {
		start, end := statementPeriod(section)
		rec := model.StatementSectionReconciliation{
			Index:          i,
			AccountLabel:   section.AccountLabel,
			AccountNumber:  section.AccountNumber,
			Currency:       section.Currency,
			AccountID:      section.AccountID,
			PeriodStart:    start,
			PeriodEnd:      end,
			OpeningBalance: section.OpeningBalance,
			ClosingBalance: section.ClosingBalance,
			BalanceCheck:   statementBalanceCheck(section),
			RowCount:       len(section.Rows),
			Matched:        []model.StatementMatch{},
			Missing:        section.Rows,
			LedgerOnly:     []model.StatementLedgerTxn{},
		}
		if rec.Missing == nil {
			rec.Missing = []model.StatementRow{}
		}

		if section.AccountID != nil && start != "" {
			rec.AccountName = names.accounts[*section.AccountID]
			txns, err := s.ledgerWindow(ctx, budgetID, *section.AccountID, start, end, names)
			if err != nil {
				return nil, err
			}
			used := usedByAccount[*section.AccountID]
			if used == nil {
				used = map[uuid.UUID]bool{}
				usedByAccount[*section.AccountID] = used
			}
			rec.Matched, rec.Missing, rec.LedgerOnly = reconcileStatementRows(section.Rows, txns, start, end, used)
		}
		result.Sections = append(result.Sections, rec)
	}
	return result, nil
}

type ledgerNameIndex struct {
	accounts   map[uuid.UUID]string
	payees     map[uuid.UUID]string
	categories map[uuid.UUID]string
}

func (s *statementService) ledgerNames(ctx context.Context, budgetID uuid.UUID) (*ledgerNameIndex, error) {
	idx := &ledgerNameIndex{
		accounts:   map[uuid.UUID]string{},
		payees:     map[uuid.UUID]string{},
		categories: map[uuid.UUID]string{},
	}
	accounts, err := s.accountRepo.GetAll(ctx, budgetID)
	if err != nil {
		return nil, errs.Wrap(errs.CodeAccountLookupFailed, "error loading accounts", err)
	}
	for _, a := range accounts {
		idx.accounts[a.ID] = a.Name
	}
	payees, err := s.payeeRepo.GetAll(ctx, budgetID)
	if err != nil {
		return nil, errs.Wrap(errs.CodePayeeLookupFailed, "error loading payees", err)
	}
	for _, p := range payees {
		idx.payees[p.ID] = p.Name
	}
	categories, err := s.categoryRepo.GetAll(ctx, budgetID)
	if err != nil {
		return nil, errs.Wrap(errs.CodeInternalError, "error loading categories", err)
	}
	for _, c := range categories {
		idx.categories[c.ID] = c.Name
	}
	return idx, nil
}

// ledgerWindow loads the account's transactions over the statement period
// widened by the match window on both sides. Rejected rows are not part of the
// ledger and are left out.
func (s *statementService) ledgerWindow(
	ctx context.Context,
	budgetID, accountID uuid.UUID,
	start, end string,
	names *ledgerNameIndex,
) ([]model.StatementLedgerTxn, error) {
	from := shiftDate(start, -statementMatchWindowDays)
	to := shiftDate(end, statementMatchWindowDays)
	txns, err := s.transactionRepo.GetAll(ctx, budgetID, &model.TransactionFilter{
		AccountIDs: []uuid.UUID{accountID},
		StartDate:  &from,
		EndDate:    &to,
	})
	if err != nil {
		return nil, errs.Wrap(errs.CodeTransactionLookupFailed, "error loading transactions", err)
	}
	out := make([]model.StatementLedgerTxn, 0, len(txns))
	for _, t := range txns {
		if t.Status == model.TransactionStatusRejected {
			continue
		}
		lt := model.StatementLedgerTxn{
			ID:     t.ID,
			Date:   ledgerDate(t.Date.String()),
			Amount: t.Amount,
			Note:   t.Note,
			Status: t.Status,
		}
		if t.PayeeID != nil {
			lt.PayeeName = names.payees[*t.PayeeID]
		}
		if t.CategoryID != nil {
			lt.CategoryName = names.categories[*t.CategoryID]
		}
		out = append(out, lt)
	}
	return out, nil
}

func shiftDate(date string, days int) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return t.AddDate(0, 0, days).Format("2006-01-02")
}

// CreateMissing creates ledger transactions for the chosen statement rows, as
// UNAPPROVED so they land in the normal review flow. Each row is checked
// against a fresh reconciliation first — a row that has gained a match since
// the page loaded is skipped — and inserted in its own database transaction,
// so one bad row does not undo the rest. A dedupe hash derived from the row
// makes a double submit (or an overlapping statement) a skip, not a duplicate.
func (s *statementService) CreateMissing(
	ctx context.Context,
	id uuid.UUID,
	req model.StatementCreateReq,
) (*model.StatementCreateResult, error) {
	budgetID := utils.MustBudgetID(ctx)
	if len(req.Items) == 0 {
		return nil, errs.New(errs.CodeInvalidArgument, "no rows selected")
	}

	rec, err := s.Reconcile(ctx, id)
	if err != nil {
		return nil, err
	}
	if rec.Import.Status != model.StatementImportReady {
		return nil, errs.New(errs.CodeInvalidArgument, "statement is not ready")
	}

	type missingRow struct {
		row       model.StatementRow
		accountID *uuid.UUID
		hash      string
	}
	missing := map[string]missingRow{}
	for _, section := range rec.Sections {
		occurrences := map[string]int{}
		for _, row := range section.Missing {
			missing[row.ID] = missingRow{row: row, accountID: section.AccountID}
		}
		// Hash over every row of the section, not only the missing ones, so
		// the occurrence count of identical rows is stable between calls.
		for _, row := range rec.Import.Sections[section.Index].Rows {
			if m, ok := missing[row.ID]; ok && section.AccountID != nil {
				key := statementRowKey(*section.AccountID, row)
				m.hash = statementRowHash(key, occurrences[key])
				missing[row.ID] = m
			}
			if section.AccountID != nil {
				occurrences[statementRowKey(*section.AccountID, row)]++
			}
		}
	}

	payees, err := s.payeeRepo.GetAll(ctx, budgetID)
	if err != nil {
		return nil, errs.Wrap(errs.CodePayeeLookupFailed, "error loading payees", err)
	}
	payeeByName := map[string]uuid.UUID{}
	for _, p := range payees {
		key := strings.ToLower(strings.TrimSpace(p.Name))
		// Prefer an ordinary payee over a transfer payee of the same name.
		if _, exists := payeeByName[key]; !exists || p.TransferAccountID == nil {
			payeeByName[key] = p.ID
		}
	}

	result := &model.StatementCreateResult{
		Created: []uuid.UUID{},
		Skipped: []string{},
		Failed:  []model.StatementCreateFailure{},
	}
	for _, item := range req.Items {
		m, ok := missing[item.RowID]
		if !ok {
			result.Skipped = append(result.Skipped, item.RowID)
			continue
		}
		if m.accountID == nil {
			result.Failed = append(result.Failed, model.StatementCreateFailure{
				RowID: item.RowID, Error: "choose the account this section belongs to first",
			})
			continue
		}

		created, err := s.createFromRow(ctx, budgetID, *m.accountID, m.row, m.hash, item, payeeByName)
		switch {
		case err == nil && created != nil:
			result.Created = append(result.Created, *created)
		case err == nil || isUniqueViolation(err):
			result.Skipped = append(result.Skipped, item.RowID)
		default:
			result.Failed = append(result.Failed, model.StatementCreateFailure{RowID: item.RowID, Error: err.Error()})
		}
	}
	return result, nil
}

func (s *statementService) createFromRow(
	ctx context.Context,
	budgetID, accountID uuid.UUID,
	row model.StatementRow,
	hash string,
	item model.StatementCreateItem,
	payeeByName map[string]uuid.UUID,
) (*uuid.UUID, error) {
	var createdID *uuid.UUID
	var newPayee *model.Payee
	err := withTx(ctx, s.repo.GetDB(), func(tx pgx.Tx) error {
		payeeID, created, err := s.resolvePayee(ctx, tx, budgetID, row, item, payeeByName)
		if err != nil {
			return err
		}
		newPayee = created
		description := row.Description
		txn := model.Transaction{
			BudgetID:    budgetID,
			Date:        model.Date(row.Date),
			AccountID:   &accountID,
			PayeeID:     &payeeID,
			CategoryID:  item.CategoryID,
			Note:        description,
			Amount:      row.SignedAmount(),
			Status:      model.TransactionStatusUnapproved,
			RawBankText: &description,
			TagIDs:      []uuid.UUID{},
		}
		if hash != "" {
			txn.DedupeHash = &hash
		}
		txns, err := s.transactionService.CreateWithTx(ctx, tx, txn)
		if err != nil {
			return err
		}
		createdID = &txns[0].ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Remembered only once committed, so later rows with the same name reuse
	// it; a rolled-back payee would otherwise poison every later row.
	if newPayee != nil {
		payeeByName[strings.ToLower(newPayee.Name)] = newPayee.ID
	}
	return createdID, nil
}

// resolvePayee: an explicit payee id, else a payee whose name matches the
// requested (or suggested) name case-insensitively, else a new payee, which
// is returned so the caller can remember it after commit.
func (s *statementService) resolvePayee(
	ctx context.Context,
	tx pgx.Tx,
	budgetID uuid.UUID,
	row model.StatementRow,
	item model.StatementCreateItem,
	payeeByName map[string]uuid.UUID,
) (uuid.UUID, *model.Payee, error) {
	if item.PayeeID != nil {
		return *item.PayeeID, nil, nil
	}
	name := strings.TrimSpace(item.PayeeName)
	if name == "" {
		name = strings.TrimSpace(row.Payee)
	}
	if name == "" {
		name = row.Description
	}
	if id, ok := payeeByName[strings.ToLower(name)]; ok {
		return id, nil, nil
	}
	created, err := s.payeeRepo.Create(ctx, tx, model.Payee{BudgetID: budgetID, Name: name})
	if err != nil {
		return uuid.Nil, nil, errs.Wrap(errs.CodeInternalError, "error creating payee "+name, err)
	}
	return created.ID, created, nil
}

func statementRowKey(accountID uuid.UUID, row model.StatementRow) string {
	return fmt.Sprintf("%s|%s|%.2f|%s", accountID, row.Date, row.SignedAmount(), strings.ToLower(row.Description))
}

func statementRowHash(key string, occurrence int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("statement|%s|%d", key, occurrence)))
	return "stmt:" + hex.EncodeToString(sum[:16])
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
