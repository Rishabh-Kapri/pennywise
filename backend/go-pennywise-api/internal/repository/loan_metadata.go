package repository

import (
	"context"
	"errors"

	"pennywise-api/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LoanMetadataRepository interface {
	GetAllByBudgetId(ctx context.Context, budgetId uuid.UUID) ([]model.LoanMetadata, error)
	GetByAccountId(ctx context.Context, accountId uuid.UUID) (*model.LoanMetadata, error)
	Create(ctx context.Context, loan model.LoanMetadata) (*model.LoanMetadata, error)
	Update(ctx context.Context, accountId uuid.UUID, loan model.LoanMetadata) (*model.LoanMetadata, error)
	Delete(ctx context.Context, accountId uuid.UUID) error
}

type loanMetadataRepo struct {
	baseRepository
}

func NewLoanMetadataRepository(db *pgxpool.Pool) LoanMetadataRepository {
	return &loanMetadataRepo{baseRepository: NewBaseRepository(db)}
}

func (r *loanMetadataRepo) GetAllByBudgetId(ctx context.Context, budgetId uuid.UUID) ([]model.LoanMetadata, error) {
	rows, err := r.Executor(nil).Query(
		ctx, `
		SELECT
			lm.id,
			lm.account_id,
			lm.interest_rate,
			lm.original_balance,
			lm.monthly_payment,
			lm.loan_start_date,
			lm.category_id,
			lm.created_at,
			lm.updated_at
		FROM loan_metadata lm
		INNER JOIN accounts a ON a.id = lm.account_id
		WHERE a.budget_id = $1 AND a.deleted = FALSE
		`, budgetId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var loans []model.LoanMetadata
	for rows.Next() {
		var l model.LoanMetadata
		err := rows.Scan(
			&l.ID,
			&l.AccountID,
			&l.InterestRate,
			&l.OriginalBalance,
			&l.MonthlyPayment,
			&l.LoanStartDate,
			&l.CategoryID,
			&l.CreatedAt,
			&l.UpdatedAt,
		)
		if err != nil {
			errorMsg := errors.New("Error while parsing loan_metadata rows: ")
			return nil, errors.Join(errorMsg, err)
		}
		loans = append(loans, l)
	}
	return loans, nil
}

func (r *loanMetadataRepo) GetByAccountId(ctx context.Context, accountId uuid.UUID) (*model.LoanMetadata, error) {
	var l model.LoanMetadata
	err := r.Executor(nil).QueryRow(
		ctx, `
		SELECT id, account_id, interest_rate, original_balance, monthly_payment,
		       loan_start_date, category_id, created_at, updated_at
		FROM loan_metadata
		WHERE account_id = $1
		`, accountId,
	).Scan(
		&l.ID,
		&l.AccountID,
		&l.InterestRate,
		&l.OriginalBalance,
		&l.MonthlyPayment,
		&l.LoanStartDate,
		&l.CategoryID,
		&l.CreatedAt,
		&l.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

func (r *loanMetadataRepo) Create(ctx context.Context, loan model.LoanMetadata) (*model.LoanMetadata, error) {
	var created model.LoanMetadata
	err := r.Executor(nil).QueryRow(
		ctx, `
		INSERT INTO loan_metadata (
			account_id, interest_rate, original_balance, monthly_payment,
			loan_start_date, category_id, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
		RETURNING id, account_id, interest_rate, original_balance, monthly_payment,
		          loan_start_date, category_id, created_at, updated_at
		`,
		loan.AccountID, loan.InterestRate, loan.OriginalBalance, loan.MonthlyPayment,
		loan.LoanStartDate, loan.CategoryID,
	).Scan(
		&created.ID,
		&created.AccountID,
		&created.InterestRate,
		&created.OriginalBalance,
		&created.MonthlyPayment,
		&created.LoanStartDate,
		&created.CategoryID,
		&created.CreatedAt,
		&created.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func (r *loanMetadataRepo) Update(ctx context.Context, accountId uuid.UUID, loan model.LoanMetadata) (*model.LoanMetadata, error) {
	var updated model.LoanMetadata
	err := r.Executor(nil).QueryRow(
		ctx, `
		UPDATE loan_metadata SET
			interest_rate = $1,
			original_balance = $2,
			monthly_payment = $3,
			loan_start_date = $4,
			category_id = $5,
			updated_at = NOW()
		WHERE account_id = $6
		RETURNING id, account_id, interest_rate, original_balance, monthly_payment,
		          loan_start_date, category_id, created_at, updated_at
		`,
		loan.InterestRate, loan.OriginalBalance, loan.MonthlyPayment,
		loan.LoanStartDate, loan.CategoryID, accountId,
	).Scan(
		&updated.ID,
		&updated.AccountID,
		&updated.InterestRate,
		&updated.OriginalBalance,
		&updated.MonthlyPayment,
		&updated.LoanStartDate,
		&updated.CategoryID,
		&updated.CreatedAt,
		&updated.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (r *loanMetadataRepo) Delete(ctx context.Context, accountId uuid.UUID) error {
	_, err := r.Executor(nil).Exec(
		ctx, `DELETE FROM loan_metadata WHERE account_id = $1`, accountId,
	)
	return err
}
