package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
)

// DemoService logs users into a shared, pre-seeded demo account so the app
// can be used without a Google login. Only wired up when DEMO_MODE is true.
type DemoService interface {
	LoginAsDemo(ctx context.Context) (*model.AuthUserResponse, string, string, error)
}

type demoService struct {
	authService    AuthService
	authRepo       repository.AuthRepository
	googleProvider repository.GoogleProviderRepository
	budgetRepo     repository.BudgetRepository
	catGroupRepo   repository.CategoryGroupRepository
	categoryRepo   repository.CategoryRepository
	payeeRepo      repository.PayeesRepository
	accountRepo    repository.AccountRepository
	payeeRuleRepo  repository.PayeeRuleRepository
	dbTxn          repository.DBTxn
	bulk           repository.BulkInserter
}

func NewDemoService(
	authService AuthService,
	authRepo repository.AuthRepository,
	googleProvider repository.GoogleProviderRepository,
	budgetRepo repository.BudgetRepository,
	catGroupRepo repository.CategoryGroupRepository,
	categoryRepo repository.CategoryRepository,
	payeeRepo repository.PayeesRepository,
	accountRepo repository.AccountRepository,
	payeeRuleRepo repository.PayeeRuleRepository,
	dbTxn repository.DBTxn,
	bulk repository.BulkInserter,
) DemoService {
	return &demoService{
		dbTxn:          dbTxn,
		bulk:           bulk,
		authService:    authService,
		authRepo:       authRepo,
		googleProvider: googleProvider,
		budgetRepo:     budgetRepo,
		catGroupRepo:   catGroupRepo,
		categoryRepo:   categoryRepo,
		payeeRepo:      payeeRepo,
		accountRepo:    accountRepo,
		payeeRuleRepo:  payeeRuleRepo,
	}
}

// LoginAsDemo returns tokens for the demo user, seeding the user and its
// budget data on first use. The demo user is persistent: subsequent logins
// reuse it and any edits made to its data survive.
func (s *demoService) LoginAsDemo(ctx context.Context) (*model.AuthUserResponse, string, string, error) {
	userWithCreds, err := s.googleProvider.GetUserByGoogleIDAndClientType(
		ctx,
		demoGoogleID,
		model.GoogleOAuthClientTypeWeb,
	)
	if errors.Is(err, repository.ErrUserNotFound) {
		// detach from the request context: seeding can outlive an impatient client
		userWithCreds, err = s.seedDemoUser(context.WithoutCancel(ctx))
	}
	if err != nil {
		return nil, "", "", err
	}

	return s.authService.StartSession(ctx, userWithCreds)
}

// seedDemoUser creates the demo auth user with a fully populated budget in a
// SINGLE database transaction: an interrupted or failed seed rolls back to
// nothing, so retries can never pollute the database with partial data. Bulk
// COPY is used for transactions and monthly budgets to keep the whole seed to
// a handful of round trips (important on remote databases). Concurrent first
// logins are safe: the loser of the auth_providers unique constraint rolls
// back entirely and falls back to looking up the winner's user.
func (s *demoService) seedDemoUser(ctx context.Context) (*model.UserWithCredentials, error) {
	log := logger.Logger(ctx)
	log.Info("seeding demo user", "googleId", demoGoogleID)
	start := time.Now()

	var userWithCreds *model.UserWithCredentials
	var authUser *model.AuthUser
	err := s.dbTxn.Do(ctx, func(ctx context.Context) error {
		var err error
		authUser, err = s.authRepo.CreateUser(ctx)
		if err != nil {
			return fmt.Errorf("error creating auth user: %w", err)
		}

		budget, categoryIDs, err := newBudget(ctx, budgetRepos{
			budgets:    s.budgetRepo,
			catGroups:  s.catGroupRepo,
			categories: s.categoryRepo,
			payees:     s.payeeRepo,
		}, demoBudgetName, authUser.ID, demoTemplateGroups(), true)
		if err != nil {
			return fmt.Errorf("error creating demo budget: %w", err)
		}
		payeeIDs, accountIDs, err := s.seedAccountsAndPayees(ctx, budget.ID)
		if err != nil {
			return err
		}
		payeeIDs[demoStartingBal] = budget.Metadata.StartingBalPayeeID
		if err := s.seedPayeeRules(ctx, budget.ID, payeeIDs, categoryIDs); err != nil {
			return err
		}

		now := time.Now()
		txnRows, carryover, err := buildDemoSeedRows(now, payeeIDs, accountIDs, categoryIDs)
		if err != nil {
			return err
		}
		txnCopy := make([][]any, len(txnRows))
		for i, r := range txnRows {
			txnCopy[i] = []any{
				r.id, budget.ID, r.date, r.payeeID, r.categoryID,
				r.accountID, r.amount, "", r.transferAccountID, r.transferTransactionID,
			}
		}
		_, err = s.bulk.BulkInsert(ctx, "transactions",
			[]string{
				"id", "budget_id", "date", "payee_id", "category_id",
				"account_id", "amount", "note", "transfer_account_id", "transfer_transaction_id",
			},
			txnCopy,
		)
		if err != nil {
			return fmt.Errorf("error bulk inserting transactions: %w", err)
		}

		monthlyRows, err := buildDemoMonthlyRows(now, categoryIDs, carryover)
		if err != nil {
			return err
		}
		monthlyCopy := make([][]any, len(monthlyRows))
		for i, r := range monthlyRows {
			monthlyCopy[i] = []any{r.month, budget.ID, r.categoryID, r.budgeted, r.carryover}
		}
		_, err = s.bulk.BulkInsert(ctx, "monthly_budgets",
			[]string{"month", "budget_id", "category_id", "budgeted", "carryover_balance"},
			monthlyCopy,
		)
		if err != nil {
			return fmt.Errorf("error bulk inserting monthly budgets: %w", err)
		}

		predictionRows, err := buildDemoPredictionRows(txnRows, payeeIDs, accountIDs, categoryIDs)
		if err != nil {
			return err
		}
		predictionCopy := make([][]any, len(predictionRows))
		for i, r := range predictionRows {
			predictionCopy[i] = []any{
				budget.ID, r.txnID, r.emailText, r.reasoning, r.metadata,
				r.amount, r.extractedAccount, r.extractedMerchant,
				r.predictedPayeeID, r.predictedCategoryID,
				r.payeeConf, r.catConf, string(r.source),
				r.hasUserCorrected, r.actualPayeeID, r.actualCategoryID, r.createdAt,
			}
		}
		_, err = s.bulk.BulkInsert(ctx, "cipher_predictions",
			[]string{
				"budget_id", "transaction_id", "email_text", "llm_reasoning", "metadata",
				"amount", "extracted_account", "extracted_payee",
				"predicted_payee_id", "predicted_category_id",
				"payee_confidence", "category_confidence", "source",
				"has_user_corrected", "actual_payee_id", "actual_category_id", "created_at",
			},
			predictionCopy,
		)
		if err != nil {
			return fmt.Errorf("error bulk inserting cipher predictions: %w", err)
		}

		userWithCreds, err = s.googleProvider.Create(
			ctx, authUser.ID,
			demoGoogleID, model.GoogleOAuthClientTypeWeb, demoUserName, "", demoUserEmail,
		)
		if err != nil {
			return fmt.Errorf("error creating demo provider: %w", err)
		}
		return nil
	})
	if err != nil {
		// a concurrent login may have won the auth_providers unique
		// constraint; our transaction rolled back, use the winner's user
		existing, lookupErr := s.googleProvider.GetUserByGoogleIDAndClientType(
			ctx,
			demoGoogleID,
			model.GoogleOAuthClientTypeWeb,
		)
		if lookupErr != nil {
			return nil, fmt.Errorf("demoService.seedDemoUser; %w", err)
		}
		return existing, nil
	}
	userWithCreds.AuthUser = authUser

	log.Info("demo user seeded",
		"authUserId", authUser.ID,
		"durationMs", time.Since(start).Milliseconds(),
	)
	return userWithCreds, nil
}

// seedPayeeRules maps UPI handles / bank narration strings to seeded payees
// and categories so the demo shows rule-based classification in action.
func (s *demoService) seedPayeeRules(
	ctx context.Context,
	budgetID uuid.UUID,
	payeeIDs, categoryIDs map[string]uuid.UUID,
) error {
	for _, rule := range demoPayeeRules() {
		payeeID, ok := payeeIDs[rule.payee]
		if !ok {
			return fmt.Errorf("demo seed; rule payee %q not found", rule.payee)
		}
		categoryID, ok := categoryIDs[rule.category]
		if !ok {
			return fmt.Errorf("demo seed; rule category %q not found", rule.category)
		}
		err := s.payeeRuleRepo.CreatePayeeRule(ctx, model.PayeeRule{
			BudgetID:    budgetID,
			PayeeID:     payeeID,
			CategoryID:  &categoryID,
			MatchString: rule.matchString,
			MatchType:   rule.matchType,
		})
		if err != nil {
			return fmt.Errorf("error creating payee rule %q: %w", rule.matchString, err)
		}
	}
	return nil
}

// seedAccountsAndPayees creates the demo accounts, each with its transfer
// payee, and the regular demo payees.
func (s *demoService) seedAccountsAndPayees(
	ctx context.Context,
	budgetID uuid.UUID,
) (payeeIDs, accountIDs map[string]uuid.UUID, err error) {
	payeeIDs = make(map[string]uuid.UUID)
	accountIDs = make(map[string]uuid.UUID)
	for _, account := range demoAccounts() {
		account.BudgetID = budgetID
		created, transferPayee, err := createAccount(ctx, s.accountRepo, s.payeeRepo, account)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating account %q: %w", account.Name, err)
		}
		accountIDs[account.Name] = created.ID
		payeeIDs[transferPayee.Name] = transferPayee.ID
	}

	for _, name := range demoPayeeNames() {
		payee, err := s.payeeRepo.Create(ctx, model.Payee{Name: name, BudgetID: budgetID})
		if err != nil {
			return nil, nil, fmt.Errorf("error creating payee %q: %w", name, err)
		}
		payeeIDs[name] = payee.ID
	}
	return payeeIDs, accountIDs, nil
}
