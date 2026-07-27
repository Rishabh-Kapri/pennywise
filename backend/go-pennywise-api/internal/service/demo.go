package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
) DemoService {
	return &demoService{
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

	authUser := userWithCreds.AuthUser
	accessToken, err := s.authService.GenerateAccessToken(ctx, authUser.ID, authUser.TokenVersion)
	if err != nil {
		return nil, "", "", errs.Wrap(errs.CodeAuthCreateFailed, "failed to generate access token", err)
	}
	refreshToken, err := s.authService.GenerateRefreshToken(ctx, authUser.ID)
	if err != nil {
		return nil, "", "", errs.Wrap(errs.CodeAuthCreateFailed, "failed to generate refresh token", err)
	}
	if err := s.authRepo.SaveRefreshTokenHash(ctx, authUser.ID, refreshToken); err != nil {
		return nil, "", "", errs.Wrap(errs.CodeAuthCreateFailed, "failed to save refresh token", err)
	}

	resUser := model.AuthUserResponse{
		ID:      authUser.ID,
		Email:   userWithCreds.GoogleProvider.Email,
		Name:    userWithCreds.GoogleProvider.Name,
		Picture: userWithCreds.GoogleProvider.Picture,
	}
	return &resUser, accessToken, refreshToken, nil
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
	err := utils.WithTx(ctx, s.authRepo.GetDB(), func(tx pgx.Tx) error {
		var err error
		authUser, err = s.authRepo.CreateUser(ctx, tx)
		if err != nil {
			return fmt.Errorf("error creating auth user: %w", err)
		}

		budget, err := s.budgetRepo.Create(ctx, tx, demoBudgetName, authUser.ID)
		if err != nil {
			return fmt.Errorf("error creating demo budget: %w", err)
		}

		categoryIDs, metadata, err := s.seedCategories(ctx, tx, budget.ID)
		if err != nil {
			return err
		}
		payeeIDs, accountIDs, err := s.seedAccountsAndPayees(ctx, tx, budget.ID, metadata)
		if err != nil {
			return err
		}
		if err := s.seedPayeeRules(ctx, tx, budget.ID, payeeIDs, categoryIDs); err != nil {
			return err
		}

		err = s.budgetRepo.UpdateById(ctx, tx, budget.ID, model.Budget{
			Name:       demoBudgetName,
			IsSelected: true, // first budget for this user
			Metadata:   *metadata,
		})
		if err != nil {
			return fmt.Errorf("error updating budget metadata: %w", err)
		}

		now := time.Now()
		txnRows, carryover, err := buildDemoSeedRows(now, payeeIDs, accountIDs, categoryIDs)
		if err != nil {
			return err
		}
		_, err = tx.CopyFrom(ctx,
			pgx.Identifier{"transactions"},
			[]string{
				"id", "budget_id", "date", "payee_id", "category_id",
				"account_id", "amount", "note", "transfer_account_id", "transfer_transaction_id",
			},
			pgx.CopyFromSlice(len(txnRows), func(i int) ([]any, error) {
				r := txnRows[i]
				return []any{
					r.id, budget.ID, r.date, r.payeeID, r.categoryID,
					r.accountID, r.amount, "", r.transferAccountID, r.transferTransactionID,
				}, nil
			}),
		)
		if err != nil {
			return fmt.Errorf("error bulk inserting transactions: %w", err)
		}

		monthlyRows, err := buildDemoMonthlyRows(now, categoryIDs, carryover)
		if err != nil {
			return err
		}
		_, err = tx.CopyFrom(ctx,
			pgx.Identifier{"monthly_budgets"},
			[]string{"month", "budget_id", "category_id", "budgeted", "carryover_balance"},
			pgx.CopyFromSlice(len(monthlyRows), func(i int) ([]any, error) {
				r := monthlyRows[i]
				return []any{r.month, budget.ID, r.categoryID, r.budgeted, r.carryover}, nil
			}),
		)
		if err != nil {
			return fmt.Errorf("error bulk inserting monthly budgets: %w", err)
		}

		predictionRows, err := buildDemoPredictionRows(txnRows, payeeIDs, accountIDs, categoryIDs)
		if err != nil {
			return err
		}
		_, err = tx.CopyFrom(ctx,
			pgx.Identifier{"cipher_predictions"},
			[]string{
				"budget_id", "transaction_id", "email_text", "llm_reasoning", "metadata",
				"amount", "extracted_account", "extracted_payee",
				"predicted_payee_id", "predicted_category_id",
				"payee_confidence", "category_confidence", "source",
				"has_user_corrected", "actual_payee_id", "actual_category_id", "created_at",
			},
			pgx.CopyFromSlice(len(predictionRows), func(i int) ([]any, error) {
				r := predictionRows[i]
				return []any{
					budget.ID, r.txnID, r.emailText, r.reasoning, r.metadata,
					r.amount, r.extractedAccount, r.extractedMerchant,
					r.predictedPayeeID, r.predictedCategoryID,
					r.payeeConf, r.catConf, string(r.source),
					r.hasUserCorrected, r.actualPayeeID, r.actualCategoryID, r.createdAt,
				}, nil
			}),
		)
		if err != nil {
			return fmt.Errorf("error bulk inserting cipher predictions: %w", err)
		}

		userWithCreds, err = s.googleProvider.Create(
			ctx, tx, authUser.ID,
			demoGoogleID, model.GoogleOAuthClientTypeWeb, demoUserName, "", demoUserEmail,
			"", // no google refresh token; also keeps gmail watch flows away from this user
			nil,
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

// seedCategories creates the system groups (internal master + inflow, credit
// card payments) and the demo template groups, mirroring budgetService.Create.
func (s *demoService) seedCategories(
	ctx context.Context,
	tx pgx.Tx,
	budgetID uuid.UUID,
) (map[string]uuid.UUID, *model.BudgetMetadata, error) {
	masterGroup, err := s.catGroupRepo.Create(ctx, tx, model.CategoryGroup{
		Name: "Internal Master Category", BudgetID: budgetID, IsSystem: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("error creating internal master category group: %w", err)
	}
	ccGroup, err := s.catGroupRepo.Create(ctx, tx, model.CategoryGroup{
		Name: "Credit Card Payments", BudgetID: budgetID, IsSystem: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("error creating credit card category group: %w", err)
	}
	inflowCat, err := s.categoryRepo.Create(ctx, tx, model.Category{
		Name: demoInflowCategory, BudgetID: budgetID, CategoryGroupID: masterGroup.ID, IsSystem: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("error creating inflow category: %w", err)
	}

	categoryIDs := map[string]uuid.UUID{demoInflowCategory: inflowCat.ID}
	for _, templateGroup := range demoTemplateGroups() {
		group, err := s.catGroupRepo.Create(ctx, tx, model.CategoryGroup{
			Name: templateGroup.Name, BudgetID: budgetID,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("error creating category group %q: %w", templateGroup.Name, err)
		}
		for _, templateCategory := range templateGroup.Categories {
			category, err := s.categoryRepo.Create(ctx, tx, model.Category{
				Name: templateCategory.Name, BudgetID: budgetID, CategoryGroupID: group.ID,
			})
			if err != nil {
				return nil, nil, fmt.Errorf("error creating category %q: %w", templateCategory.Name, err)
			}
			categoryIDs[category.Name] = category.ID
		}
	}

	metadata := &model.BudgetMetadata{
		InflowCategoryID: inflowCat.ID,
		CCGroupID:        ccGroup.ID,
		// StartingBalPayeeID is filled in by seedAccountsAndPayees
	}
	return categoryIDs, metadata, nil
}

// seedPayeeRules maps UPI handles / bank narration strings to seeded payees
// and categories so the demo shows rule-based classification in action.
func (s *demoService) seedPayeeRules(
	ctx context.Context,
	tx pgx.Tx,
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
		err := s.payeeRuleRepo.CreatePayeeRule(ctx, tx, model.PayeeRule{
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

// seedAccountsAndPayees creates the demo accounts with their transfer payees
// (mirroring accountService.Create), the starting balance payee, and the
// regular demo payees.
func (s *demoService) seedAccountsAndPayees(
	ctx context.Context,
	tx pgx.Tx,
	budgetID uuid.UUID,
	metadata *model.BudgetMetadata,
) (payeeIDs, accountIDs map[string]uuid.UUID, err error) {
	startingPayee, err := s.payeeRepo.Create(ctx, tx, model.Payee{
		Name: demoStartingBal, BudgetID: budgetID,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("error creating starting balance payee: %w", err)
	}
	metadata.StartingBalPayeeID = startingPayee.ID
	payeeIDs = map[string]uuid.UUID{demoStartingBal: startingPayee.ID}

	accountIDs = make(map[string]uuid.UUID)
	for _, account := range demoAccounts() {
		account.BudgetID = budgetID
		created, err := s.accountRepo.Create(ctx, tx, account)
		if err != nil {
			return nil, nil, fmt.Errorf("error creating account %q: %w", account.Name, err)
		}
		transferPayee, err := s.payeeRepo.Create(ctx, tx, model.Payee{
			Name:              "Transfer : " + account.Name,
			BudgetID:          budgetID,
			TransferAccountID: &created.ID,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("error creating transfer payee for %q: %w", account.Name, err)
		}
		if err := s.accountRepo.UpdateTransferPayee(ctx, tx, created.ID, transferPayee.ID); err != nil {
			return nil, nil, fmt.Errorf("error linking transfer payee for %q: %w", account.Name, err)
		}
		accountIDs[account.Name] = created.ID
		payeeIDs[transferPayee.Name] = transferPayee.ID
	}

	for _, name := range demoPayeeNames() {
		payee, err := s.payeeRepo.Create(ctx, tx, model.Payee{Name: name, BudgetID: budgetID})
		if err != nil {
			return nil, nil, fmt.Errorf("error creating payee %q: %w", name, err)
		}
		payeeIDs[name] = payee.ID
	}
	return payeeIDs, accountIDs, nil
}
