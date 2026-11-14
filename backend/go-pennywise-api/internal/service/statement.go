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
	"path"
	"strings"
	"time"

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

// create stores the PDF, queues its statement.extract and creates the
// import. The extract gets the typed password, if any, and every saved
// account password, which cipher tries in turn on a locked PDF.
func (s *statementService) create(
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
	payload, err := json.Marshal(model.StatementInput{
		ImportID:   importID,
		Data:       pdf,
		Password:   password,
		Candidates: s.passwordCandidates(ctx, budgetID),
	})
	if err != nil {
		return nil, false, errs.Wrap(errs.CodeInternalError, "error encoding statement", err)
	}
	queued, err := s.broker.Enqueue(
		ctx,
		model.JobTypeStatementExtract,
		payload,
		job.WithQueue(model.QueueCipher),
		job.WithBudgetID(&budgetID),
		job.WithIdempotencyKey("statement:"+hash),
	)
	if err != nil {
		return nil, false, errs.Wrap(errs.CodeJobEnqueueFailed, "error queueing "+model.JobTypeStatementExtract, err)
	}

	// The PDF is kept so the user can open it later. Its key is the import,
	// not the file name, which the user chose and may repeat.
	filePath, err := s.files.Save(
		path.Join("statements", budgetID.String()),
		importID.String()+".pdf",
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
	return imports, nil
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

// recordSources links each exactly matched statement row to its transaction
// as a STATEMENT_LINE source, replacing what this statement recorded before.
// Near matches (amount differs) aren't recorded: the pair may be wrong. A
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
	var lines []model.StatementLineSource
	for _, section := range rec.Sections {
		hint := accountNumberSuffix(section.AccountNumber)
		for _, m := range section.Matched {
			if m.Kind != model.StatementMatchExact {
				continue
			}
			lines = append(lines, statementLineSource(m, hint, section.Currency))
		}
	}
	err = s.dbTxn.Do(ctx, func(ctx context.Context) error {
		return s.sources.ReplaceStatementLines(ctx, rec.Import.BudgetID, id, lines)
	})
	if err != nil {
		logger.Logger(ctx).Warn("statement sources not recorded", "statementId", id, "error", err)
		return
	}
	logger.Logger(ctx).Info("statement sources recorded", "statementId", id, "lines", len(lines))
}

func statementLineSource(m model.StatementMatch, accountHint, currency string) model.StatementLineSource {
	r := m.Row
	raw := r.RawDescription
	if raw == "" {
		raw = r.Description
	}
	direction := narration.Debit
	if r.Direction == model.StatementCredit {
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
		Amount:        r.Amount,
		Currency:      currency,
		OccurredOn:    r.Date,
		RewardsPoints: r.RewardPoints,
	}
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

// passwordCandidates opens the budget's saved statement passwords. One that
// won't open (the key changed) is skipped: the PDF may not need it.
func (s *statementService) passwordCandidates(ctx context.Context, budgetID uuid.UUID) []model.StatementPasswordCandidate {
	if s.passwords == nil || s.box == nil {
		return nil
	}
	sealed, err := s.passwords.ListByBudget(ctx, budgetID)
	if err != nil {
		logger.Logger(ctx).Warn("statement passwords not loaded", "error", err)
		return nil
	}
	out := make([]model.StatementPasswordCandidate, 0, len(sealed))
	for accountID, box := range sealed {
		pw, err := s.box.Open(box)
		if err != nil {
			logger.Logger(ctx).Warn("statement password not opened", "accountId", accountID, "error", err)
			continue
		}
		out = append(out, model.StatementPasswordCandidate{AccountID: accountID, Password: pw})
	}
	return out
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
			RowCount:           len(section.Rows),
			Matched:            []model.StatementMatch{},
			LedgerOnly:         []model.StatementLedgerTxn{},
			Covered:            []model.StatementRow{},
		}
		// EMI instalments are set aside before matching: the purchase they
		// pay off is already in the budget.
		matchable := make([]model.StatementRow, 0, len(section.Rows))
		for _, r := range section.Rows {
			if coveredByEMI(r) {
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
			rec.Matched, rec.Missing, rec.LedgerOnly = reconcileStatementRows(matchable, txns, start, end, used)
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
