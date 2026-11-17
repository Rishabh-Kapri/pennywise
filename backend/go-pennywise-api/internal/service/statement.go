package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils/narration"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/storage"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	MaxStatementSizeBytes = 20 << 20
	statementListLimit    = 50
)

var (
	ErrStatementNotFound = errors.New("statement not found")
	ErrStatementNoFile   = errors.New("statement has no file")
)

type StatementService interface {
	// Upload creates the import and queues its statement.extract job in
	// cipher, returning at once. The same file uploaded again returns the
	// existing import.
	Upload(
		ctx context.Context,
		fileName string,
		pdf []byte,
		password string,
		defaultAccountID *uuid.UUID,
	) (*model.StatementImport, bool, error)
	List(ctx context.Context) ([]model.StatementImport, error)
	Reconcile(ctx context.Context, id uuid.UUID) (*model.StatementReconciliation, error)
	SetSectionAccount(
		ctx context.Context,
		id uuid.UUID,
		section int,
		accountID *uuid.UUID,
	) (*model.StatementReconciliation, error)
	CreateMissing(ctx context.Context, id uuid.UUID, req model.StatementCreateReq) (*model.StatementCreateResult, error)
	// LinkRow adds transactions of its section's account to a statement
	// row's links, beside any it has; UnlinkRow removes them all.
	LinkRow(ctx context.Context, id uuid.UUID, rowID string, txnIDs []uuid.UUID) (*model.StatementReconciliation, error)
	UnlinkRow(ctx context.Context, id uuid.UUID, rowID string) (*model.StatementReconciliation, error)
	// Link links every row to every transaction: one transaction paying
	// several rows (EMI lines), or one row paid by several. The rows must
	// be in one section.
	Link(ctx context.Context, id uuid.UUID, rowIDs []string, txnIDs []uuid.UUID) (*model.StatementReconciliation, error)
	// EditRow corrects a row read wrong, keeping what was read the first
	// time; RevertRow puts that back.
	EditRow(ctx context.Context, id uuid.UUID, rowID string, req model.StatementRowEditReq) (*model.StatementReconciliation, error)
	RevertRow(ctx context.Context, id uuid.UUID, rowID string) (*model.StatementReconciliation, error)
	// SetRowEMI records whether a row is part of an EMI the budget already
	// paid for, over what its text suggested; nil goes back to the text.
	SetRowEMI(ctx context.Context, id uuid.UUID, rowID string, emi *bool) (*model.StatementReconciliation, error)
	// AddDifference books what a linked row's transactions are short of the
	// row as a new unapproved transaction, linked to the row too.
	AddDifference(
		ctx context.Context,
		id uuid.UUID,
		rowID string,
		req model.StatementDifferenceReq,
	) (*model.StatementReconciliation, error)
	// Retry queues a failed import's dead jobs again.
	Retry(ctx context.Context, id uuid.UUID) (*model.StatementImport, error)
	Delete(ctx context.Context, id uuid.UUID) error
	// File opens the uploaded PDF. ErrStatementNoFile when the import has
	// none (uploaded before PDFs were kept).
	File(ctx context.Context, id uuid.UUID) (*model.StatementImport, io.ReadSeekCloser, error)
	// ImportFromEmail is the statement.import job: a statement PDF fetched
	// from email becomes an import like an upload, without a typed password
	// or account. The same file imported again returns the existing import.
	ImportFromEmail(ctx context.Context, p model.StatementImportPayload) (*model.StatementImport, error)
	// SetStatementPassword saves the password that opens accountID's
	// statement PDFs, sealed. An empty password removes it.
	SetStatementPassword(ctx context.Context, accountID uuid.UUID, password string) error
	// StatementPasswordAccounts lists the accounts that have one saved.
	StatementPasswordAccounts(ctx context.Context) ([]uuid.UUID, error)
	// SaveExtraction is the statement.save job: it saves what cipher read
	// onto the import and maps its sections to accounts.
	SaveExtraction(ctx context.Context, p model.StatementSavePayload) error
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
	broker             job.Broker
	// jobStore reads the import's jobs; nil on the memory job driver, where
	// an import's progress and failure aren't visible.
	jobStore job.JobStore
	// files keeps the uploaded PDFs, in the same store as receipts.
	files storage.Store
	// sources holds the bank text behind transactions: the references used
	// for matching, and the statement lines recorded for matched rows.
	sources repository.TransactionSourceRepository
	// passwords holds each account's statement password, sealed by box. box
	// is nil when LLM_CREDENTIALS_KEY is unset; passwords can't be saved then.
	passwords repository.StatementPasswordRepository
	box       *utils.SecretBox
	dbTxn     repository.DBTxn
}

func NewStatementService(
	repo repository.StatementImportRepository,
	transactionRepo repository.TransactionRepository,
	accountRepo repository.AccountRepository,
	payeeRepo repository.PayeesRepository,
	categoryRepo repository.CategoryRepository,
	transactionService TransactionService,
	dbTxn repository.DBTxn,
	broker job.Broker,
	jobStore job.JobStore,
	files storage.Store,
	sources repository.TransactionSourceRepository,
	passwords repository.StatementPasswordRepository,
	box *utils.SecretBox,
) StatementService {
	return &statementService{
		dbTxn:              dbTxn,
		repo:               repo,
		transactionRepo:    transactionRepo,
		accountRepo:        accountRepo,
		payeeRepo:          payeeRepo,
		categoryRepo:       categoryRepo,
		transactionService: transactionService,
		broker:             broker,
		jobStore:           jobStore,
		files:              files,
		sources:            sources,
		passwords:          passwords,
		box:                box,
	}
}

func (s *statementService) Upload(
	ctx context.Context,
	fileName string,
	pdf []byte,
	password string,
	defaultAccountID *uuid.UUID,
) (*model.StatementImport, bool, error) {
	return s.create(ctx, fileName, pdf, password, defaultAccountID)
}

func (s *statementService) ImportFromEmail(
	ctx context.Context,
	p model.StatementImportPayload,
) (*model.StatementImport, error) {
	imp, _, err := s.create(ctx, p.FileName, p.Data, "", nil)
	return imp, err
}

// statementFileExt is how a statement upload is stored: ".pdf", or ".csv" for
// a bank's export (a .csv name and text content); false for anything else.
func statementFileExt(fileName string, data []byte) (string, bool) {
	switch {
	case bytes.HasPrefix(bytes.TrimLeft(data, "\x00\t\r\n "), []byte("%PDF-")):
		return ".pdf", true
	case strings.EqualFold(path.Ext(fileName), ".csv") && utf8.Valid(data) && !bytes.ContainsRune(data, 0):
		return ".csv", true
	}
	return "", false
}

// create stores the PDF or CSV, queues its statement.extract and creates the
// import. The extract gets the typed password, if any, sealed; cipher tries it
// first on a locked PDF, then the accounts' saved passwords.
func (s *statementService) create(
	ctx context.Context,
	fileName string,
	pdf []byte,
	password string,
	defaultAccountID *uuid.UUID,
) (*model.StatementImport, bool, error) {
	budgetID := utils.MustBudgetID(ctx)

	ext, ok := statementFileExt(fileName, pdf)
	if !ok {
		return nil, false, errs.New(errs.CodeInvalidArgument, "file is not a PDF or a CSV")
	}
	if defaultAccountID != nil {
		if _, err := s.accountRepo.GetById(ctx, budgetID, *defaultAccountID); err != nil {
			return nil, false, errs.New(errs.CodeInvalidArgument, "account not found")
		}
	}
	if s.broker == nil {
		return nil, false, errs.New(errs.CodeJobEnqueueFailed, "reading statements needs the job queue (JOB_DRIVER)")
	}

	sum := sha256.Sum256(pdf)
	hash := hex.EncodeToString(sum[:])
	if existing, err := s.repo.FindByHash(ctx, budgetID, hash); err != nil {
		return nil, false, errs.Wrap(errs.CodeInternalError, "error checking for a duplicate statement", err)
	} else if existing != nil {
		return existing, true, nil
	}

	// The job is queued first so the row can name it. Its key is the new
	// import, not the file: a failed import's file may be uploaded again.
	importID := uuid.New()
	input := model.StatementInput{ImportID: importID, FileName: fileName, Data: pdf}
	if password != "" {
		if s.box == nil {
			return nil, false, errs.New(errs.CodeAdapterNotConfigured,
				"reading a locked statement needs LLM_CREDENTIALS_KEY on the API")
		}
		sealed, err := s.box.Seal(password)
		if err != nil {
			return nil, false, errs.Wrap(errs.CodeInternalError, "error sealing password", err)
		}
		input.SealedPassword = sealed
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, false, errs.Wrap(errs.CodeInternalError, "error encoding statement", err)
	}
	queued, err := s.broker.Enqueue(
		ctx,
		model.JobTypeStatementExtract,
		payload,
		job.WithQueue(model.QueueCipher),
		job.WithBudgetID(&budgetID),
		job.WithIdempotencyKey("statement:"+importID.String()),
	)
	if err != nil {
		return nil, false, errs.Wrap(errs.CodeJobEnqueueFailed, "error queueing "+model.JobTypeStatementExtract, err)
	}

	// The PDF is kept so the user can open it later. Its key is the import,
	// not the file name, which the user chose and may repeat.
	filePath, err := s.files.Save(
		path.Join("statements", budgetID.String()),
		importID.String()+ext,
		bytes.NewReader(pdf),
	)
	if err != nil {
		return nil, false, errs.Wrap(errs.CodeInternalError, "error storing statement", err)
	}

	// If this fails the job still runs, and statement.save finds no import to
	// save onto, so it does nothing.
	imp, err := s.repo.Create(ctx, model.StatementImport{
		ID:               importID,
		BudgetID:         budgetID,
		FileName:         fileName,
		FileHash:         hash,
		Status:           model.StatementImportProcessing,
		JobID:            &queued.JobID,
		DefaultAccountID: defaultAccountID,
		FilePath:         filePath,
	})
	if err != nil {
		s.deleteFile(ctx, filePath)
		return nil, false, errs.Wrap(errs.CodeInternalError, "error saving statement", err)
	}
	return imp, false, nil
}

func (s *statementService) List(ctx context.Context) ([]model.StatementImport, error) {
	imports, err := s.repo.List(ctx, utils.MustBudgetID(ctx), statementListLimit)
	if err != nil {
		return nil, err
	}
	for i := range imports {
		s.applyJobState(ctx, &imports[i])
	}
	s.fillProgress(ctx, imports)
	return imports, nil
}

// fillProgress reconciles each ready import in the list to count where its
// rows stand. List rows carry no sections, so each import is loaded again. A
// failure leaves that import without progress; the list still loads.
func (s *statementService) fillProgress(ctx context.Context, imports []model.StatementImport) {
	budgetID := utils.MustBudgetID(ctx)
	var names *ledgerNameIndex
	for i := range imports {
		if imports[i].Status != model.StatementImportReady {
			continue
		}
		if names == nil {
			var err error
			if names, err = s.ledgerNames(ctx, budgetID); err != nil {
				logger.Logger(ctx).Warn("statement progress not counted", "error", err)
				return
			}
		}
		imp, err := s.load(ctx, budgetID, imports[i].ID)
		if err != nil {
			logger.Logger(ctx).Warn("statement progress not counted", "statementId", imports[i].ID, "error", err)
			continue
		}
		rec, err := s.reconcileImport(ctx, budgetID, imp, names)
		if err != nil {
			logger.Logger(ctx).Warn("statement progress not counted", "statementId", imports[i].ID, "error", err)
			continue
		}
		imports[i].Progress = statementProgress(rec.Sections)
	}
}

func statementProgress(sections []model.StatementSectionReconciliation) *model.StatementProgress {
	p := &model.StatementProgress{}
	for _, sec := range sections {
		if sec.AccountID == nil {
			p.Unmapped++
		}
		p.Missing += len(sec.Missing)
		p.Matched += len(sec.Matched)
		p.LedgerOnly += len(sec.LedgerOnly)
		p.Covered += len(sec.Covered)
	}
	p.Reconciled = len(sections) > 0 && p.Unmapped == 0 && p.Missing == 0 && p.LedgerOnly == 0
	return p
}

// applyJobState fills a processing import's progress from its jobs, and marks
// it failed when one of them has stopped for good. Clients learn about live
// changes from the job updates on the websocket; this is what a page sees
// when it loads.
func (s *statementService) applyJobState(ctx context.Context, imp *model.StatementImport) {
	if imp.Status != model.StatementImportProcessing || imp.JobID == nil || s.jobStore == nil {
		return
	}
	jobs, err := s.importJobs(ctx, *imp.JobID)
	if err != nil {
		logger.Logger(ctx).Warn("could not read statement jobs", "statementId", imp.ID, "error", err)
		return
	}
	for _, j := range jobs {
		switch j.Status {
		case job.DeadJobStatus, job.CancelledJobStatus:
			message := j.LastError
			if message == "" {
				message = "reading the statement was stopped"
			}
			if err := s.repo.Fail(ctx, imp.ID, message); err != nil {
				logger.Logger(ctx).Warn("could not mark statement failed", "statementId", imp.ID, "error", err)
				return
			}
			imp.Status = model.StatementImportFailed
			imp.Error = &message
			return
		case job.QueuedJobStatus, job.RunningJobStatus:
			imp.Stage = "waiting"
			if j.Type == model.JobTypeStatementSave {
				imp.Stage = "saving rows"
			}
			var p job.Progress
			if len(j.Progress) > 0 && json.Unmarshal(j.Progress, &p) == nil && p.Message != "" {
				imp.Stage = p.Message
				imp.PagesProcessed, imp.PageCount = p.Current, p.Total
			}
		}
	}
}

// importJobs returns the import's statement.extract job and the jobs it
// queued, extract first.
func (s *statementService) importJobs(ctx context.Context, jobID uuid.UUID) ([]job.Job, error) {
	extract, err := s.jobStore.GetJobByID(ctx, jobID)
	if err != nil {
		return nil, err
	}
	rest, err := s.jobStore.ListByRoot(ctx, jobID)
	if err != nil {
		return nil, err
	}
	out := []job.Job{*extract}
	for _, j := range rest {
		if j.ID != extract.ID {
			out = append(out, j)
		}
	}
	return out, nil
}

func (s *statementService) Retry(ctx context.Context, id uuid.UUID) (*model.StatementImport, error) {
	budgetID := utils.MustBudgetID(ctx)
	imp, err := s.load(ctx, budgetID, id)
	if err != nil {
		return nil, err
	}
	if imp.Status != model.StatementImportFailed {
		return nil, errs.New(errs.CodeInvalidArgument, "only a failed statement can be retried")
	}
	if imp.JobID == nil {
		return nil, errs.New(errs.CodeInvalidArgument, "this statement was read before jobs; upload it again")
	}
	if s.jobStore == nil {
		return nil, errs.New(
			errs.CodeInvalidArgument,
			"statements can't be retried without a job store; upload it again",
		)
	}

	jobs, err := s.importJobs(ctx, *imp.JobID)
	if err != nil {
		return nil, err
	}
	retried := 0
	for _, j := range jobs {
		if j.Status != job.DeadJobStatus {
			continue
		}
		if _, err := s.jobStore.RetryDead(ctx, j.ID, jobActor(ctx)); err != nil {
			return nil, err
		}
		retried++
	}
	if retried == 0 {
		return nil, errs.New(errs.CodeInvalidArgument, "nothing to retry; upload the statement again")
	}

	if err := s.repo.Restart(ctx, budgetID, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errs.New(errs.CodeInvalidArgument, "only a failed statement can be retried")
		}
		return nil, err
	}
	return s.load(ctx, budgetID, id)
}

func (s *statementService) Delete(ctx context.Context, id uuid.UUID) error {
	budgetID := utils.MustBudgetID(ctx)
	imp, err := s.load(ctx, budgetID, id)
	if err != nil {
		return err
	}
	err = s.repo.Delete(ctx, budgetID, id)
	if errors.Is(err, repository.ErrNotFound) {
		return ErrStatementNotFound
	}
	if err != nil {
		return err
	}
	s.deleteFile(ctx, imp.FilePath)
	// A deleted statement is no longer evidence for anything.
	if s.sources != nil {
		if err := s.sources.ReplaceStatementLines(ctx, budgetID, id, nil); err != nil {
			logger.Logger(ctx).Warn("statement sources not removed", "statementId", id, "error", err)
		}
	}
	return nil
}

// recordSources links each matched statement row to its transaction as a
// STATEMENT_LINE source, replacing what this statement recorded before. A
// failure is logged, not returned: the statement itself is saved either way.
func (s *statementService) recordSources(ctx context.Context, id uuid.UUID) {
	if s.sources == nil {
		return
	}
	rec, err := s.Reconcile(ctx, id)
	if err != nil || rec.Import.Status != model.StatementImportReady {
		if err != nil {
			logger.Logger(ctx).Warn("statement sources not recorded", "statementId", id, "error", err)
		}
		return
	}
	lines := statementLines(rec.Sections)
	err = s.dbTxn.Do(ctx, func(ctx context.Context) error {
		return s.sources.ReplaceStatementLines(ctx, rec.Import.BudgetID, id, lines)
	})
	if err != nil {
		logger.Logger(ctx).Warn("statement sources not recorded", "statementId", id, "error", err)
		return
	}
	logger.Logger(ctx).Info("statement sources recorded", "statementId", id, "lines", len(lines))
}

// statementLines are the sources a reconciled statement gives its
// transactions. An exact match is recorded; so is any pair the user linked by
// hand, even when the amounts differ: they said the row is that transaction.
// A near match the matcher guessed isn't: the pair may be wrong.
func statementLines(sections []model.StatementSectionReconciliation) []model.StatementLineSource {
	var lines []model.StatementLineSource
	for _, section := range sections {
		hint := accountNumberSuffix(section.AccountNumber)
		for _, m := range section.Matched {
			if m.Kind != model.StatementMatchExact && !m.Linked {
				continue
			}
			line := statementLineSource(m, hint, section.Currency)
			lines = append(lines, line)
			// Transactions linked to the row with it (a booked difference)
			// come from the same statement line.
			for _, other := range m.Others {
				extra := line
				extra.TransactionID = other.ID
				lines = append(lines, extra)
			}
		}
	}
	return lines
}

// statementLineSource is the source m's rows give a transaction. A
// transaction keeps one statement line, so rows linked to it together
// (several EMI lines) are recorded as one: their text on a line each and
// their amounts added up.
func statementLineSource(m model.StatementMatch, accountHint, currency string) model.StatementLineSource {
	r := m.Row
	raw := statementRawText(r)
	amount := r.Amount
	isCredit := r.Direction == model.StatementCredit
	if len(m.OtherRows) > 0 {
		total := r.SignedAmount()
		for _, o := range m.OtherRows {
			raw += "\n" + statementRawText(o)
			total += o.SignedAmount()
		}
		amount = roundCents(math.Abs(total))
		if total != 0 {
			isCredit = total > 0
		}
	}
	direction := narration.Debit
	if isCredit {
		direction = narration.Credit
	}
	return model.StatementLineSource{
		TransactionID: m.Transaction.ID,
		RawText:       raw,
		Channel:       r.Channel,
		Merchant:      r.Merchant,
		VPA:           r.VPA,
		Ref:           r.Reference,
		AccountHint:   accountHint,
		Direction:     direction,
		Amount:        amount,
		Currency:      currency,
		OccurredOn:    r.Date,
		OccurredAt:    statementRowTime(r),
		RewardsPoints: r.RewardPoints,
	}
}

func statementRawText(r model.StatementRow) string {
	if r.RawDescription != "" {
		return r.RawDescription
	}
	return r.Description
}

// statementRowTime is the row's date and printed time in the bank's zone, so
// a transaction made or matched from it shows when it happened. nil when the
// row printed no time.
func statementRowTime(r model.StatementRow) *time.Time {
	if r.Time == "" {
		return nil
	}
	t, err := time.ParseInLocation("2006-01-02 15:04", r.Date+" "+r.Time, narration.BankZone)
	if err != nil {
		return nil
	}
	return &t
}

func (s *statementService) File(ctx context.Context, id uuid.UUID) (*model.StatementImport, io.ReadSeekCloser, error) {
	imp, err := s.load(ctx, utils.MustBudgetID(ctx), id)
	if err != nil {
		return nil, nil, err
	}
	if imp.FilePath == "" {
		return nil, nil, ErrStatementNoFile
	}
	f, err := s.files.Open(imp.FilePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, ErrStatementNoFile
	}
	if err != nil {
		return nil, nil, errs.Wrap(errs.CodeInternalError, "error opening statement", err)
	}
	return imp, f, nil
}

// deleteFile removes a stored PDF. A failure is only logged: the row is
// already gone or never written, and an orphaned file harms nothing.
func (s *statementService) deleteFile(ctx context.Context, filePath string) {
	if filePath == "" {
		return
	}
	if err := s.files.Delete(filePath); err != nil {
		logger.Logger(ctx).Warn("statement file not deleted", "path", filePath, "error", err)
	}
}

func (s *statementService) SaveExtraction(ctx context.Context, p model.StatementSavePayload) error {
	budgetID := utils.MustBudgetID(ctx)
	imp, err := s.load(ctx, budgetID, p.ImportID)
	if errors.Is(err, ErrStatementNotFound) {
		// Deleted while it was read, or its row was never saved.
		logger.Logger(ctx).Info("statement import gone, rows not saved", "statementId", p.ImportID)
		return nil
	}
	if err != nil {
		return err
	}
	if imp.Status != model.StatementImportProcessing {
		return nil
	}

	sections, dropped := cleanStatementSections(p.Sections)
	if len(sections) == 0 {
		return s.repo.Fail(ctx, imp.ID, "no transactions were found in this statement")
	}
	// The account whose saved password opened the PDF stands in for one
	// picked at upload.
	if imp.DefaultAccountID == nil && p.OpenedWith != nil {
		imp.DefaultAccountID = p.OpenedWith
	}
	s.autoMapAccounts(ctx, *imp, sections)
	if err := s.repo.Complete(ctx, imp.ID, sections, dropped); err != nil {
		return err
	}
	s.recordSources(ctx, imp.ID)
	return nil
}

func (s *statementService) SetStatementPassword(ctx context.Context, accountID uuid.UUID, password string) error {
	budgetID := utils.MustBudgetID(ctx)
	if _, err := s.accountRepo.GetById(ctx, budgetID, accountID); err != nil {
		return errs.New(errs.CodeInvalidArgument, "account not found")
	}
	if s.passwords == nil {
		return errs.New(errs.CodeAdapterNotConfigured, "statement passwords need a database")
	}
	if password == "" {
		return s.passwords.Delete(ctx, budgetID, accountID)
	}
	if s.box == nil {
		return errs.New(errs.CodeAdapterNotConfigured, "saving passwords needs LLM_CREDENTIALS_KEY on the API")
	}
	sealed, err := s.box.Seal(password)
	if err != nil {
		return errs.Wrap(errs.CodeInternalError, "error sealing password", err)
	}
	return s.passwords.Set(ctx, budgetID, accountID, sealed)
}

func (s *statementService) StatementPasswordAccounts(ctx context.Context) ([]uuid.UUID, error) {
	if s.passwords == nil {
		return []uuid.UUID{}, nil
	}
	sealed, err := s.passwords.ListByBudget(ctx, utils.MustBudgetID(ctx))
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, 0, len(sealed))
	for id := range sealed {
		out = append(out, id)
	}
	return out, nil
}

func (s *statementService) FailInterrupted(ctx context.Context) {
	n, err := s.repo.FailInterrupted(ctx, "extraction was interrupted by a server restart; upload it again")
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
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrStatementNotFound
	}
	if err != nil {
		return nil, err
	}
	return imp, nil
}

// autoMapAccounts points each section at a ledger account: by the account
// number's last four digits (accounts.suffix), else the account chosen at
// upload — for a single-section statement always, otherwise only when exactly
// one section is still unmapped, since guessing between several would be
// worse than asking.
func (s *statementService) autoMapAccounts(
	ctx context.Context,
	imp model.StatementImport,
	sections []model.StatementSection,
) {
	// The printed number wins over the account picked at upload: a statement
	// says which card it is, a dropdown can be left on the wrong one.
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
		if _, err := s.accountRepo.GetById(ctx, budgetID, *accountID); err != nil {
			return nil, errs.New(errs.CodeInvalidArgument, "account not found")
		}
	}
	imp.Sections[section].AccountID = accountID
	if err := s.repo.UpdateSections(ctx, budgetID, id, imp.Sections); err != nil {
		return nil, err
	}
	s.recordSources(ctx, id)
	return s.Reconcile(ctx, id)
}

// Reconcile compares each section with its account's ledger as it is now.
func (s *statementService) Reconcile(ctx context.Context, id uuid.UUID) (*model.StatementReconciliation, error) {
	budgetID := utils.MustBudgetID(ctx)
	imp, err := s.load(ctx, budgetID, id)
	if err != nil {
		return nil, err
	}
	s.applyJobState(ctx, imp)
	if imp.Status != model.StatementImportReady {
		return &model.StatementReconciliation{Import: *imp, Sections: []model.StatementSectionReconciliation{}}, nil
	}
	names, err := s.ledgerNames(ctx, budgetID)
	if err != nil {
		return nil, err
	}
	result, err := s.reconcileImport(ctx, budgetID, imp, names)
	if err != nil {
		return nil, err
	}
	s.suggestFromPast(ctx, budgetID, result, names)
	return result, nil
}

// reconcileImport is Reconcile for a ready import, with the budget's names
// already loaded.
func (s *statementService) reconcileImport(
	ctx context.Context,
	budgetID uuid.UUID,
	imp *model.StatementImport,
	names *ledgerNameIndex,
) (*model.StatementReconciliation, error) {
	result := &model.StatementReconciliation{
		Import:   *imp,
		Sections: make([]model.StatementSectionReconciliation, 0, len(imp.Sections)),
	}

	usedByAccount := map[uuid.UUID]map[uuid.UUID]bool{}
	for i, section := range imp.Sections {
		start, end := statementPeriod(section)
		rec := model.StatementSectionReconciliation{
			Index:              i,
			AccountLabel:       section.AccountLabel,
			AccountNumber:      section.AccountNumber,
			Kind:               section.Kind,
			Currency:           section.Currency,
			AccountID:          section.AccountID,
			PeriodStart:        start,
			PeriodEnd:          end,
			OpeningBalance:     section.OpeningBalance,
			ClosingBalance:     section.ClosingBalance,
			BalanceCheck:       statementBalanceCheck(section),
			StatementDate:      section.StatementDate,
			DueDate:            section.DueDate,
			TotalDue:           section.TotalDue,
			MinimumDue:         section.MinimumDue,
			CreditLimit:        section.CreditLimit,
			Rewards:            section.Rewards,
			EMIPlans:           section.EMIPlans,
			RewardPointsOnRows: rowRewardPoints(section.Rows),
			Charges:            statementCharges(section.Rows),
			Cashback:           statementCashback(section.Rows),
			RowCount:           len(section.Rows),
			Matched:            []model.StatementMatch{},
			LedgerOnly:         []model.StatementLedgerTxn{},
			Covered:            []model.StatementRow{},
		}
		// EMI instalments are set aside before matching: the purchase they
		// pay off is already in the budget. One the user linked is matched.
		matchable := make([]model.StatementRow, 0, len(section.Rows))
		for _, r := range section.Rows {
			// Classified again so imports saved under older rules read the
			// same as new ones; it's only text matching.
			classifyStatementRow(&r, section.Kind == model.StatementKindCard)
			stripEMIOffer(&r)
			if setAsideAsEMI(r) {
				rec.Covered = append(rec.Covered, r)
			} else {
				matchable = append(matchable, r)
			}
		}
		rec.Missing = matchable

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
			ledgerStart := ledgerPeriodStart(section, start, end)
			rec.Matched, rec.Missing, rec.LedgerOnly = reconcileStatementRows(matchable, txns, ledgerStart, end, used)
		}
		for j := range rec.Missing {
			if kind := rec.Missing[j].ChargeKind; kind != "" {
				rec.Missing[j].SuggestedCategoryID = chargeCategory(kind, names.pickable)
			}
		}
		result.Sections = append(result.Sections, rec)
	}
	return result, nil
}

// suggestFromPast fills each missing row's payee and category with what the
// budget gave the same line on an earlier statement (a "10% Swiggy CashBack"
// filed under Swiggy and Cashbacks last month). It only suggests: the user
// still picks. A failure is logged and the rows keep their own guesses.
func (s *statementService) suggestFromPast(
	ctx context.Context,
	budgetID uuid.UUID,
	rec *model.StatementReconciliation,
	names *ledgerNameIndex,
) {
	if s.sources == nil {
		return
	}
	var keys []string
	for _, section := range rec.Sections {
		for _, r := range section.Missing {
			if k := model.StatementTextKey(statementRowText(r)); k != "" {
				keys = append(keys, k)
			}
		}
	}
	if len(keys) == 0 {
		return
	}
	past, err := s.sources.PastStatementChoices(ctx, budgetID, keys)
	if err != nil {
		logger.Logger(ctx).Warn("past statement choices not loaded", "error", err)
		return
	}
	for i := range rec.Sections {
		for j := range rec.Sections[i].Missing {
			r := &rec.Sections[i].Missing[j]
			c, ok := past[model.StatementTextKey(statementRowText(*r))]
			if !ok {
				continue
			}
			r.SuggestedPayeeName = c.PayeeName
			if c.CategoryID != nil {
				if _, pickable := names.pickable[*c.CategoryID]; pickable {
					r.SuggestedCategoryID = c.CategoryID
				}
			}
		}
	}
}

// statementRowText is the text a statement line source is recorded with.
func statementRowText(r model.StatementRow) string {
	if r.RawDescription != "" {
		return r.RawDescription
	}
	return r.Description
}

// ledgerPeriodStart is where a section's "not on the statement" list starts.
// A card's printed cycle often names the previous statement's date as its
// first day ("3 Apr – 3 May"), a day that statement already billed: unless a
// row of this one is dated that day, the list starts the day after, so a
// monthly EMI on the 3rd isn't listed twice.
func ledgerPeriodStart(section model.StatementSection, start, end string) string {
	if section.Kind != model.StatementKindCard || start == "" || end == "" || start == end {
		return start
	}
	from, err1 := time.Parse("2006-01-02", start)
	to, err2 := time.Parse("2006-01-02", end)
	if err1 != nil || err2 != nil || from.Day() != to.Day() {
		return start
	}
	for _, r := range section.Rows {
		if r.Date == start {
			return start
		}
	}
	return from.AddDate(0, 0, 1).Format("2006-01-02")
}

type ledgerNameIndex struct {
	accounts   map[uuid.UUID]string
	payees     map[uuid.UUID]string
	categories map[uuid.UUID]string
	// pickable are the categories a suggestion may use: not hidden or deleted.
	pickable map[uuid.UUID]string
}

func (s *statementService) ledgerNames(ctx context.Context, budgetID uuid.UUID) (*ledgerNameIndex, error) {
	idx := &ledgerNameIndex{
		accounts:   map[uuid.UUID]string{},
		payees:     map[uuid.UUID]string{},
		categories: map[uuid.UUID]string{},
		pickable:   map[uuid.UUID]string{},
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
		if !c.Hidden && !c.Deleted {
			idx.pickable[c.ID] = c.Name
		}
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
	if s.sources != nil && len(out) > 0 {
		ids := make([]uuid.UUID, len(out))
		for i, t := range out {
			ids[i] = t.ID
		}
		refs, err := s.sources.RefsByTransaction(ctx, budgetID, ids)
		if err != nil {
			// References only sharpen matching; without them it still works.
			logger.Logger(ctx).Warn("transaction references not loaded", "error", err)
		}
		for i := range out {
			out[i].Refs = refs[out[i].ID]
		}
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
	if len(result.Created) > 0 {
		s.recordSources(ctx, id)
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
	err := s.dbTxn.Do(ctx, func(ctx context.Context) error {
		payeeID, created, err := s.resolvePayee(ctx, budgetID, row, item, payeeByName)
		if err != nil {
			return err
		}
		newPayee = created
		// The narration isn't the user's note: the statement line is kept as
		// the transaction's source (recordSources) once it matches the row.
		// raw_bank_text keeps it too, for learning when the user approves.
		description := row.Description
		txn := model.Transaction{
			BudgetID:    budgetID,
			Date:        model.Date(row.Date),
			AccountID:   &accountID,
			PayeeID:     &payeeID,
			CategoryID:  item.CategoryID,
			Amount:      row.SignedAmount(),
			Note:        strings.TrimSpace(item.Note),
			Status:      model.TransactionStatusUnapproved,
			RawBankText: &description,
			TagIDs:      []uuid.UUID{},
		}
		if hash != "" {
			txn.DedupeHash = &hash
		}
		txns, err := s.transactionService.CreateInTx(ctx, txn)
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
	created, err := s.payeeRepo.Create(ctx, model.Payee{BudgetID: budgetID, Name: name})
	if err != nil {
		return uuid.Nil, nil, errs.Wrap(errs.CodeInternalError, "error creating payee "+name, err)
	}
	return created.ID, created, nil
}

// findRow returns the section and index of the row with rowID.
func findRow(imp *model.StatementImport, rowID string) (section, row int, ok bool) {
	for i := range imp.Sections {
		for j := range imp.Sections[i].Rows {
			if imp.Sections[i].Rows[j].ID == rowID {
				return i, j, true
			}
		}
	}
	return 0, 0, false
}

func (s *statementService) LinkRow(
	ctx context.Context,
	id uuid.UUID,
	rowID string,
	txnIDs []uuid.UUID,
) (*model.StatementReconciliation, error) {
	return s.Link(ctx, id, []string{rowID}, txnIDs)
}

func (s *statementService) Link(
	ctx context.Context,
	id uuid.UUID,
	rowIDs []string,
	txnIDs []uuid.UUID,
) (*model.StatementReconciliation, error) {
	budgetID := utils.MustBudgetID(ctx)
	if len(rowIDs) == 0 || len(txnIDs) == 0 {
		return nil, errs.New(errs.CodeInvalidArgument, "pick at least one row and one transaction")
	}
	rec, err := s.Reconcile(ctx, id)
	if err != nil {
		return nil, err
	}
	if rec.Import.Status != model.StatementImportReady {
		return nil, errs.New(errs.CodeInvalidArgument, "statement is not ready")
	}
	imp := &rec.Import
	si := -1
	rowIdx := make([]int, 0, len(rowIDs))
	for _, rowID := range rowIDs {
		sec, ri, ok := findRow(imp, rowID)
		if !ok {
			return nil, errs.New(errs.CodeInvalidArgument, "row %s is not on this statement", rowID)
		}
		if si >= 0 && sec != si {
			return nil, errs.New(errs.CodeInvalidArgument, "the rows are on different accounts of the statement")
		}
		si = sec
		rowIdx = append(rowIdx, ri)
	}
	if imp.Sections[si].AccountID == nil {
		return nil, errs.New(errs.CodeInvalidArgument, "choose the account this section belongs to first")
	}

	// Only transactions the page shows: the section's account's in the
	// statement's window. One matched to another row moves to these; one
	// linked to another row joins its group.
	section := rec.Sections[si]
	offered := map[uuid.UUID]bool{}
	for _, t := range section.LedgerOnly {
		offered[t.ID] = true
	}
	for _, m := range section.Matched {
		offered[m.Transaction.ID] = true
		for _, o := range m.Others {
			offered[o.ID] = true
		}
	}
	for _, txnID := range txnIDs {
		if !offered[txnID] {
			return nil, errs.New(errs.CodeInvalidArgument,
				"a transaction isn't one of this account's transactions for the statement's period")
		}
	}
	for _, ri := range rowIdx {
		row := &imp.Sections[si].Rows[ri]
		for _, txnID := range txnIDs {
			if !slices.Contains(row.LinkedTransactionIDs, txnID) {
				row.LinkedTransactionIDs = append(row.LinkedTransactionIDs, txnID)
			}
		}
	}
	if err := s.repo.UpdateSections(ctx, budgetID, id, imp.Sections); err != nil {
		return nil, err
	}
	s.recordSources(ctx, id)
	return s.Reconcile(ctx, id)
}

func (s *statementService) EditRow(
	ctx context.Context,
	id uuid.UUID,
	rowID string,
	req model.StatementRowEditReq,
) (*model.StatementReconciliation, error) {
	budgetID := utils.MustBudgetID(ctx)
	imp, err := s.load(ctx, budgetID, id)
	if err != nil {
		return nil, err
	}
	if imp.Status != model.StatementImportReady {
		return nil, errs.New(errs.CodeInvalidArgument, "statement is not ready")
	}
	si, ri, ok := findRow(imp, rowID)
	if !ok {
		return nil, errs.New(errs.CodeInvalidArgument, "row %s is not on this statement", rowID)
	}
	row := &imp.Sections[si].Rows[ri]
	original := model.StatementRowOriginal{
		Date: row.Date, Time: row.Time, Description: row.Description, Amount: row.Amount, Direction: row.Direction,
	}
	if req.Time != nil {
		t := strings.TrimSpace(*req.Time)
		if t != "" && model.ParseStatementTime(t) == "" {
			return nil, errs.New(errs.CodeInvalidArgument, "time must be HH:MM")
		}
		row.Time = model.ParseStatementTime(t)
	}
	if req.Date != nil {
		d, err := time.Parse("2006-01-02", strings.TrimSpace(*req.Date))
		if err != nil {
			return nil, errs.New(errs.CodeInvalidArgument, "date must be YYYY-MM-DD")
		}
		row.Date = d.Format("2006-01-02")
	}
	if req.Description != nil {
		desc := strings.TrimSpace(*req.Description)
		if desc == "" {
			return nil, errs.New(errs.CodeInvalidArgument, "the description can't be empty")
		}
		row.Description = desc
	}
	if req.Amount != nil {
		if *req.Amount <= 0 || math.IsInf(*req.Amount, 0) || math.IsNaN(*req.Amount) {
			return nil, errs.New(errs.CodeInvalidArgument, "the amount must be more than 0; set the direction for money in or out")
		}
		row.Amount = roundCents(*req.Amount)
	}
	if req.Direction != nil {
		if *req.Direction != model.StatementDebit && *req.Direction != model.StatementCredit {
			return nil, errs.New(errs.CodeInvalidArgument, "direction must be debit or credit")
		}
		row.Direction = *req.Direction
	}
	changed := row.Date != original.Date || row.Time != original.Time || row.Description != original.Description ||
		row.Amount != original.Amount || row.Direction != original.Direction
	if !changed {
		return s.Reconcile(ctx, id)
	}
	// The first edit keeps what was read; later ones keep that too.
	if row.Edited == nil {
		row.Edited = &original
	}
	if err := s.repo.UpdateSections(ctx, budgetID, id, imp.Sections); err != nil {
		return nil, err
	}
	s.recordSources(ctx, id)
	return s.Reconcile(ctx, id)
}

func (s *statementService) RevertRow(
	ctx context.Context,
	id uuid.UUID,
	rowID string,
) (*model.StatementReconciliation, error) {
	budgetID := utils.MustBudgetID(ctx)
	imp, err := s.load(ctx, budgetID, id)
	if err != nil {
		return nil, err
	}
	si, ri, ok := findRow(imp, rowID)
	if !ok {
		return nil, errs.New(errs.CodeInvalidArgument, "row %s is not on this statement", rowID)
	}
	row := &imp.Sections[si].Rows[ri]
	if row.Edited == nil {
		return s.Reconcile(ctx, id)
	}
	row.Date, row.Time, row.Description = row.Edited.Date, row.Edited.Time, row.Edited.Description
	row.Amount, row.Direction = row.Edited.Amount, row.Edited.Direction
	row.Edited = nil
	if err := s.repo.UpdateSections(ctx, budgetID, id, imp.Sections); err != nil {
		return nil, err
	}
	s.recordSources(ctx, id)
	return s.Reconcile(ctx, id)
}

func (s *statementService) SetRowEMI(
	ctx context.Context,
	id uuid.UUID,
	rowID string,
	emi *bool,
) (*model.StatementReconciliation, error) {
	budgetID := utils.MustBudgetID(ctx)
	imp, err := s.load(ctx, budgetID, id)
	if err != nil {
		return nil, err
	}
	if imp.Status != model.StatementImportReady {
		return nil, errs.New(errs.CodeInvalidArgument, "statement is not ready")
	}
	si, ri, ok := findRow(imp, rowID)
	if !ok {
		return nil, errs.New(errs.CodeInvalidArgument, "row %s is not on this statement", rowID)
	}
	row := &imp.Sections[si].Rows[ri]
	row.EMIOverride = emi
	if emi != nil && *emi {
		// set aside: a link would pair it with something it doesn't pay for
		row.LinkedTransactionIDs = nil
	}
	if err := s.repo.UpdateSections(ctx, budgetID, id, imp.Sections); err != nil {
		return nil, err
	}
	s.recordSources(ctx, id)
	return s.Reconcile(ctx, id)
}

func (s *statementService) UnlinkRow(
	ctx context.Context,
	id uuid.UUID,
	rowID string,
) (*model.StatementReconciliation, error) {
	budgetID := utils.MustBudgetID(ctx)
	imp, err := s.load(ctx, budgetID, id)
	if err != nil {
		return nil, err
	}
	si, ri, ok := findRow(imp, rowID)
	if !ok {
		return nil, errs.New(errs.CodeInvalidArgument, "row %s is not on this statement", rowID)
	}
	// The row's group goes with it: every row sharing one of its linked
	// transactions, and the rows sharing theirs.
	rows := imp.Sections[si].Rows
	group := map[uuid.UUID]bool{}
	for _, t := range rows[ri].LinkedTransactionIDs {
		group[t] = true
	}
	cleared := false
	for grew := true; grew; {
		grew = false
		for k := range rows {
			if len(rows[k].LinkedTransactionIDs) == 0 {
				continue
			}
			shares := k == ri
			for _, t := range rows[k].LinkedTransactionIDs {
				shares = shares || group[t]
			}
			if !shares {
				continue
			}
			for _, t := range rows[k].LinkedTransactionIDs {
				if !group[t] {
					group[t] = true
					grew = true
				}
			}
			rows[k].LinkedTransactionIDs = nil
			cleared = true
		}
	}
	if cleared {
		if err := s.repo.UpdateSections(ctx, budgetID, id, imp.Sections); err != nil {
			return nil, err
		}
		s.recordSources(ctx, id)
	}
	return s.Reconcile(ctx, id)
}

func (s *statementService) AddDifference(
	ctx context.Context,
	id uuid.UUID,
	rowID string,
	req model.StatementDifferenceReq,
) (*model.StatementReconciliation, error) {
	budgetID := utils.MustBudgetID(ctx)
	rec, err := s.Reconcile(ctx, id)
	if err != nil {
		return nil, err
	}
	imp := &rec.Import
	si, ri, ok := findRow(imp, rowID)
	if !ok {
		return nil, errs.New(errs.CodeInvalidArgument, "row %s is not on this statement", rowID)
	}
	var match *model.StatementMatch
	for i := range rec.Sections[si].Matched {
		if m := &rec.Sections[si].Matched[i]; m.Row.ID == rowID && m.Linked {
			match = m
		}
	}
	if match == nil || match.Difference == 0 || imp.Sections[si].AccountID == nil {
		return nil, errs.New(errs.CodeInvalidArgument, "that row is not linked to a transaction with a different amount")
	}

	row := imp.Sections[si].Rows[ri]
	payees, err := s.payeeRepo.GetAll(ctx, budgetID)
	if err != nil {
		return nil, errs.Wrap(errs.CodePayeeLookupFailed, "error loading payees", err)
	}
	payeeByName := map[string]uuid.UUID{}
	for _, p := range payees {
		key := strings.ToLower(strings.TrimSpace(p.Name))
		if _, exists := payeeByName[key]; !exists || p.TransferAccountID == nil {
			payeeByName[key] = p.ID
		}
	}

	// A second call finds no difference left and is refused above.
	var createdID uuid.UUID
	err = s.dbTxn.Do(ctx, func(ctx context.Context) error {
		item := model.StatementCreateItem{RowID: rowID, PayeeID: req.PayeeID, PayeeName: req.PayeeName}
		payeeID, _, err := s.resolvePayee(ctx, budgetID, row, item, payeeByName)
		if err != nil {
			return err
		}
		// No note: the statement line is recorded as its source once the row
		// adds up (recordSources), like the transaction it tops up.
		txns, err := s.transactionService.CreateInTx(ctx, model.Transaction{
			BudgetID:   budgetID,
			Date:       model.Date(row.Date),
			AccountID:  imp.Sections[si].AccountID,
			PayeeID:    &payeeID,
			CategoryID: req.CategoryID,
			Amount:     match.Difference,
			Status:     model.TransactionStatusUnapproved,
			TagIDs:     []uuid.UUID{},
		})
		if err != nil {
			return err
		}
		createdID = txns[0].ID
		return nil
	})
	if err != nil {
		return nil, err
	}

	linked := append([]uuid.UUID{}, row.LinkedTransactionIDs...)
	imp.Sections[si].Rows[ri].LinkedTransactionIDs = append(linked, createdID)
	if err := s.repo.UpdateSections(ctx, budgetID, id, imp.Sections); err != nil {
		return nil, err
	}
	s.recordSources(ctx, id)
	return s.Reconcile(ctx, id)
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
