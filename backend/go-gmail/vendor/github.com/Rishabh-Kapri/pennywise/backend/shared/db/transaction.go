package db

import (
	"context"
	"fmt"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TransactionRepository interface {
	BaseRepositoryInterface
	GetAll(ctx context.Context, budgetId uuid.UUID, filter *model.TransactionFilter) ([]model.Transaction, error)
	GetById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.Transaction, error)
	GetByIdTx(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, id uuid.UUID) (*model.Transaction, error)
	GetAllNormalized(
		ctx context.Context,
		budgetId uuid.UUID,
		filter *model.TransactionFilter,
	) (model.PaginatedResponse[model.Transaction], error)
	Update(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, id uuid.UUID, txn model.Transaction) error
	UpdateLocation(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, id uuid.UUID, lat *float64, lng *float64, name *string, source *model.LocationSource) error
	UpdateStatus(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, id uuid.UUID, status model.TransactionStatus) error
	Create(ctx context.Context, tx pgx.Tx, txn model.Transaction) ([]model.Transaction, error)
	CreateDeduped(ctx context.Context, tx pgx.Tx, txn model.Transaction) (*model.Transaction, bool, error)
	DeleteById(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, id uuid.UUID) error
}

type transactionRepo struct {
	BaseRepository
}

func NewTransactionRepository(pool *pgxpool.Pool) TransactionRepository {
	return &transactionRepo{BaseRepository: NewBaseRepository(pool)}
}

func (r *transactionRepo) GetAll(
	ctx context.Context,
	budgetId uuid.UUID,
	filter *model.TransactionFilter,
) ([]model.Transaction, error) {
	sql := `SELECT
			id,
			budget_id,
			date,
			payee_id,
			category_id,
			account_id,
			note,
			amount,
			status,
			raw_bank_text,
			summary,
			transfer_account_id,
			transfer_transaction_id,
			tag_ids,
			location_lat,
			location_lng,
			location_name,
			location_source,
			created_at,
			updated_at
		FROM transactions
		WHERE deleted = FALSE AND budget_id = $1`
	args := []any{budgetId}
	if filter != nil {
		argsIndex := 2 // $1 is budgetId
		if len(filter.AccountIDs) > 0 {
			sql += fmt.Sprintf(" AND account_id = ANY($%d)", argsIndex)
			args = append(args, filter.AccountIDs)
			argsIndex++
		}
	}
	// add the order part at last
	sql += "\nORDER BY date DESC, updated_at DESC"
	rows, err := r.Executor(nil).Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []model.Transaction
	for rows.Next() {
		var txn model.Transaction
		err := rows.Scan(
			&txn.ID,
			&txn.BudgetID,
			&txn.Date,
			&txn.PayeeID,
			&txn.CategoryID,
			&txn.AccountID,
			&txn.Note,
			&txn.Amount,
			&txn.Status,
			&txn.RawBankText,
			&txn.Summary,
			&txn.TransferAccountID,
			&txn.TransferTransactionID,
			&txn.TagIDs,
			&txn.LocationLat,
			&txn.LocationLng,
			&txn.LocationName,
			&txn.LocationSource,
			&txn.CreatedAt,
			&txn.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, txn)
	}
	return transactions, nil
}

func (r *transactionRepo) GetById(ctx context.Context, budgetId uuid.UUID, id uuid.UUID) (*model.Transaction, error) {
	var txn model.Transaction
	err := r.Executor(nil).QueryRow(
		ctx, `
		  SELECT
		    transactions.id,
				transactions.budget_id,
				transactions.date,
				transactions.payee_id,
				transactions.category_id,
				transactions.account_id,
				transactions.note,
				transactions.amount,
				transactions.status,
		    transactions.raw_bank_text,
				transactions.summary,
				transactions.transfer_account_id,
				transactions.transfer_transaction_id,
				transactions.tag_ids,
				transactions.location_lat,
				transactions.location_lng,
				transactions.location_name,
				transactions.location_source,
				transactions.created_at,
				transactions.updated_at,
				accounts.name AS account_name,
				payees.name AS payee_name,
				categories.name AS category_name
		  FROM transactions
		  LEFT JOIN accounts    ON transactions.account_id = accounts.id
		  LEFT JOIN payees     ON transactions.payee_id = payees.id
		  LEFT JOIN categories ON transactions.category_id = categories.id
		  WHERE transactions.budget_id = $1 AND transactions.id = $2 AND transactions.deleted = FALSE
		`, budgetId, id,
	).Scan(
		&txn.ID,
		&txn.BudgetID,
		&txn.Date,
		&txn.PayeeID,
		&txn.CategoryID,
		&txn.AccountID,
		&txn.Note,
		&txn.Amount,
		&txn.Status,
		&txn.RawBankText,
		&txn.Summary,
		&txn.TransferAccountID,
		&txn.TransferTransactionID,
		&txn.TagIDs,
		&txn.LocationLat,
		&txn.LocationLng,
		&txn.LocationName,
		&txn.LocationSource,
		&txn.CreatedAt,
		&txn.UpdatedAt,
		&txn.AccountName,
		&txn.PayeeName,
		&txn.CategoryName,
	)
	if err != nil {
		return nil, err
	}
	return &txn, nil
}

func (r *transactionRepo) GetByIdTx(
	ctx context.Context,
	tx pgx.Tx,
	budgetId uuid.UUID,
	id uuid.UUID,
) (*model.Transaction, error) {
	var txn model.Transaction
	err := r.Executor(tx).QueryRow(
		ctx, `
		  SELECT
		    transactions.id,
				transactions.budget_id,
				transactions.date,
				transactions.payee_id,
				transactions.category_id,
				transactions.account_id,
				transactions.note,
				transactions.amount,
				transactions.status,
				transactions.raw_bank_text,
				transactions.summary,
				transactions.transfer_account_id,
				transactions.transfer_transaction_id,
				transactions.tag_ids,
				transactions.location_lat,
				transactions.location_lng,
				transactions.location_name,
				transactions.location_source,
				transactions.created_at,
				transactions.updated_at,
				accounts.name AS account_name,
				payees.name AS payee_name,
				categories.name AS category_name
		  FROM transactions
		  LEFT JOIN accounts    ON transactions.account_id = accounts.id
		  LEFT JOIN payees     ON transactions.payee_id = payees.id
		  LEFT JOIN categories ON transactions.category_id = categories.id
		  WHERE transactions.budget_id = $1 AND transactions.id = $2 AND transactions.deleted = FALSE
		`, budgetId, id,
	).Scan(
		&txn.ID,
		&txn.BudgetID,
		&txn.Date,
		&txn.PayeeID,
		&txn.CategoryID,
		&txn.AccountID,
		&txn.Note,
		&txn.Amount,
		&txn.Status,
		&txn.RawBankText,
		&txn.Summary,
		&txn.TransferAccountID,
		&txn.TransferTransactionID,
		&txn.TagIDs,
		&txn.LocationLat,
		&txn.LocationLng,
		&txn.LocationName,
		&txn.LocationSource,
		&txn.CreatedAt,
		&txn.UpdatedAt,
		&txn.AccountName,
		&txn.PayeeName,
		&txn.CategoryName,
	)
	if err != nil {
		return nil, err
	}
	return &txn, nil
}

func (r *transactionRepo) GetAllNormalized(
	ctx context.Context,
	budgetId uuid.UUID,
	filter *model.TransactionFilter,
) (model.PaginatedResponse[model.Transaction], error) {
	balanceExpr := "0 AS balance"

	if filter != nil && len(filter.AccountIDs) > 0 {
		balanceExpr = `SUM(transactions.amount) OVER (
			ORDER BY transactions.date ASC, transactions.updated_at ASC
			ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
		) AS balance`
	}

	limit := 30
	if filter != nil && filter.Limit > 0 {
		limit = int(filter.Limit)
	}

	isFirstPage := filter == nil || filter.CursorString == ""
	pointsNext := false

	var cursorDate model.Date
	var cursorUpdatedAt time.Time
	var cursorID uuid.UUID

	logger.Logger(ctx).Info("filter", "filter", filter)

	if filter != nil && filter.CursorString != "" {
		decodedCursor, err := utils.DecodeCursor(filter.CursorString)
		if err != nil {
			return model.PaginatedResponse[model.Transaction]{}, errs.Wrap(
				errs.CodeInternalError,
				"error decoding cursor",
				err,
			)
		}

		pointsNext = decodedCursor.PointsNext

		cursorDate, err = utils.CursorDateValue(decodedCursor)
		if err != nil {
			return model.PaginatedResponse[model.Transaction]{}, err
		}

		cursorUpdatedAt, err = utils.CursorTime(decodedCursor)
		if err != nil {
			return model.PaginatedResponse[model.Transaction]{}, err
		}

		cursorID, err = utils.CursorUUID(decodedCursor)
		if err != nil {
			return model.PaginatedResponse[model.Transaction]{}, err
		}
	}

	query := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).
		Select(
			"transactions.id",
			"transactions.budget_id",
			"transactions.date",
			"transactions.payee_id",
			"transactions.category_id",
			"transactions.account_id",
			"transactions.note",
			"transactions.amount",
			"transactions.dedupe_hash",
			"transactions.status",
			"transactions.raw_bank_text",
			"transactions.summary",
			"transactions.transfer_account_id",
			"transactions.transfer_transaction_id",
			"transactions.tag_ids",
			"transactions.location_lat",
			"transactions.location_lng",
			"transactions.location_name",
			"transactions.location_source",
			"transactions.created_at",
			"transactions.updated_at",
			"accounts.name AS account_name",
			"payees.name AS payee_name",
			"categories.name AS category_name",
			"CASE WHEN transactions.amount >= 0 THEN transactions.amount ELSE 0 END AS inflow",
			"CASE WHEN transactions.amount < 0 THEN ABS(transactions.amount) ELSE 0 END AS outflow",
			balanceExpr,
		).
		From("transactions").
		LeftJoin("accounts ON transactions.account_id = accounts.id").
		LeftJoin("payees ON transactions.payee_id = payees.id").
		LeftJoin("categories ON transactions.category_id = categories.id").
		Where(sq.Eq{"transactions.budget_id": budgetId}).
		Where(sq.Eq{"transactions.deleted": false})

	countQuery := sq.StatementBuilder.PlaceholderFormat(sq.Dollar).
		Select("COUNT(*)").
		From("transactions").
		Where(sq.Eq{"transactions.budget_id": budgetId}).
		Where(sq.Eq{"transactions.deleted": false})

	query = applyTransactionFilters(query, filter)
	countQuery = applyTransactionFilters(countQuery, filter)

	countSQL, countArgs, err := countQuery.ToSql()
	if err != nil {
		return model.PaginatedResponse[model.Transaction]{}, err
	}

	var total int
	if err := r.Executor(nil).QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return model.PaginatedResponse[model.Transaction]{}, err
	}

	queryOrder := filter.SortOrder
	if !isFirstPage && !pointsNext {
		queryOrder = utils.ReverseSortOrder(filter.SortOrder)
	}

	if !isFirstPage {
		query = query.Where(
			sq.Expr(
				fmt.Sprintf(
					"(transactions.date, transactions.updated_at, transactions.id) %s (?, ?, ?)",
					utils.CursorOperator(filter.SortOrder, pointsNext),
				),
				cursorDate,
				cursorUpdatedAt,
				cursorID,
			),
		)
	}
	query = query.OrderBy(
		"transactions.date "+queryOrder,
		"transactions.updated_at "+queryOrder,
	)

	query = query.Limit(uint64(limit + 1))

	sql, args, err := query.ToSql()
	if err != nil {
		return model.PaginatedResponse[model.Transaction]{}, err
	}

	rows, err := r.Executor(nil).Query(ctx, sql, args...)
	if err != nil {
		return model.PaginatedResponse[model.Transaction]{}, err
	}
	defer rows.Close()

	var txns []model.Transaction
	for rows.Next() {
		var txn model.Transaction
		var status *model.TransactionStatus
		err := rows.Scan(
			&txn.ID,
			&txn.BudgetID,
			&txn.Date,
			&txn.PayeeID,
			&txn.CategoryID,
			&txn.AccountID,
			&txn.Note,
			&txn.Amount,
			&txn.DedupeHash,
			&status,
			&txn.RawBankText,
			&txn.Summary,
			&txn.TransferAccountID,
			&txn.TransferTransactionID,
			&txn.TagIDs,
			&txn.LocationLat,
			&txn.LocationLng,
			&txn.LocationName,
			&txn.LocationSource,
			&txn.CreatedAt,
			&txn.UpdatedAt,
			&txn.AccountName,
			&txn.PayeeName,
			&txn.CategoryName,
			&txn.Inflow,
			&txn.Outflow,
			&txn.Balance,
		)
		if err != nil {
			return model.PaginatedResponse[model.Transaction]{}, err
		}
		if status != nil {
			txn.Status = *status
		} else {
			txn.Status = model.TransactionStatusManual
		}
		txns = append(txns, txn)
	}
	if err := rows.Err(); err != nil {
		return model.PaginatedResponse[model.Transaction]{}, err
	}

	hasMore := len(txns) > limit
	if hasMore {
		txns = txns[:limit]
	}
	if !isFirstPage && !pointsNext {
		utils.ReversePage(txns)
	}

	return model.PaginatedResponse[model.Transaction]{
		Data:       txns,
		Total:      total,
		Pagination: utils.GenerateCursorPager(txns, isFirstPage, pointsNext, hasMore, transactionCursor),
	}, nil
}

func applyTransactionFilters(query sq.SelectBuilder, filter *model.TransactionFilter) sq.SelectBuilder {
	if filter == nil {
		return query
	}

	if len(filter.AccountIDs) > 0 {
		query = query.Where(sq.Eq{"transactions.account_id": filter.AccountIDs})
	}

	if len(filter.CategoryIDs) > 0 {
		query = query.Where(sq.Eq{"transactions.category_id": filter.CategoryIDs})
	}

	if len(filter.PayeeIDs) > 0 {
		query = query.Where(sq.Eq{"transactions.payee_id": filter.PayeeIDs})
	}

	if len(filter.TagIDs) > 0 {
		// uuid[] overlap: matches transactions carrying any of the tags
		query = query.Where(sq.Expr("transactions.tag_ids && ?", filter.TagIDs))
	}

	if filter.StartDate != nil {
		query = query.Where(sq.GtOrEq{"transactions.date": *filter.StartDate})
	}

	if filter.EndDate != nil {
		query = query.Where(sq.LtOrEq{"transactions.date": *filter.EndDate})
	}

	if filter.Note != nil {
		query = query.Where(sq.Expr("transactions.note ILIKE ?", "%"+*filter.Note+"%"))
	}

	return query
}

func transactionCursor(txn model.Transaction, pointsNext bool) model.Cursor {
	return model.Cursor{
		ID:         txn.ID,
		Date:       txn.Date.String(),
		UpdatedAt:  txn.UpdatedAt,
		PointsNext: pointsNext,
	}
}

func (r *transactionRepo) Create(ctx context.Context, tx pgx.Tx, txn model.Transaction) ([]model.Transaction, error) {
	var createdTxn model.Transaction
	err := r.Executor(tx).QueryRow(
		ctx,
		`INSERT INTO transactions (
		  budget_id,
		  date,
		  payee_id,
		  category_id,
		  account_id,
		  note,
		  amount,
		  dedupe_hash,
		  status,
			raw_bank_text,
			summary,
		  transfer_account_id,
		  transfer_transaction_id,
		  tag_ids,
		  location_lat,
		  location_lng,
		  location_name,
		  location_source
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		RETURNING id, amount, budget_id, status, summary`,
		txn.BudgetID,
		txn.Date,
		txn.PayeeID,
		txn.CategoryID,
		txn.AccountID,
		txn.Note,
		txn.Amount,
		txn.DedupeHash,
		txn.Status,
		txn.RawBankText,
		txn.Summary,
		txn.TransferAccountID,
		txn.TransferTransactionID,
		txn.TagIDs,
		txn.LocationLat,
		txn.LocationLng,
		txn.LocationName,
		txn.LocationSource,
	).Scan(&createdTxn.ID, &createdTxn.Amount, &createdTxn.BudgetID, &createdTxn.Status, &createdTxn.Summary)
	if err != nil {
		return nil, err
	}
	txns := make([]model.Transaction, 0)
	return append(txns, createdTxn), nil
}

// CreateDeduped inserts a transaction but treats a (budget_id, dedupe_hash)
// conflict as "already created": it returns the existing row and created=false
// instead of a unique-violation error, so pipeline activity retries are
// idempotent. Transactions without a dedupe hash always insert.
func (r *transactionRepo) CreateDeduped(
	ctx context.Context,
	tx pgx.Tx,
	txn model.Transaction,
) (*model.Transaction, bool, error) {
	if txn.DedupeHash == nil || *txn.DedupeHash == "" {
		created, err := r.Create(ctx, tx, txn)
		if err != nil {
			return nil, false, err
		}
		return &created[0], true, nil
	}

	var createdTxn model.Transaction
	err := r.Executor(tx).QueryRow(
		ctx,
		`INSERT INTO transactions (
		  budget_id,
		  date,
		  payee_id,
		  category_id,
		  account_id,
		  note,
		  amount,
		  dedupe_hash,
		  status,
			raw_bank_text,
			summary,
		  transfer_account_id,
		  transfer_transaction_id,
		  tag_ids,
		  location_lat,
		  location_lng,
		  location_name,
		  location_source
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		ON CONFLICT (budget_id, dedupe_hash) WHERE dedupe_hash IS NOT NULL and deleted = FALSE
		DO NOTHING
		RETURNING id, amount, budget_id, status, summary`,
		txn.BudgetID,
		txn.Date,
		txn.PayeeID,
		txn.CategoryID,
		txn.AccountID,
		txn.Note,
		txn.Amount,
		txn.DedupeHash,
		txn.Status,
		txn.RawBankText,
		txn.Summary,
		txn.TransferAccountID,
		txn.TransferTransactionID,
		txn.TagIDs,
		txn.LocationLat,
		txn.LocationLng,
		txn.LocationName,
		txn.LocationSource,
	).Scan(&createdTxn.ID, &createdTxn.Amount, &createdTxn.BudgetID, &createdTxn.Status, &createdTxn.Summary)
	if err == nil {
		return &createdTxn, true, nil
	}
	if err != pgx.ErrNoRows {
		return nil, false, err
	}

	// Conflict: fetch the already-committed row.
	var existing model.Transaction
	err = r.Executor(tx).QueryRow(
		ctx,
		`SELECT id, amount, budget_id, status, summary
		 FROM transactions
		 WHERE budget_id = $1 AND dedupe_hash = $2 AND deleted = FALSE`,
		txn.BudgetID,
		txn.DedupeHash,
	).Scan(&existing.ID, &existing.Amount, &existing.BudgetID, &existing.Status, &existing.Summary)
	if err != nil {
		return nil, false, err
	}
	return &existing, false, nil
}

func (r *transactionRepo) Update(
	ctx context.Context,
	tx pgx.Tx,
	budgetId uuid.UUID,
	id uuid.UUID,
	txn model.Transaction,
) error {
	cmdTag, err := r.Executor(tx).Exec(
		ctx, `
		  UPDATE transactions SET
				date = $1,
				payee_id = $2,
				category_id = $3,
				account_id = $4,
				note = $5,
				amount = $6,
				transfer_account_id = $7,
				transfer_transaction_id = $8,
				tag_ids = $9,
				status = $10,
				location_lat = $11,
				location_lng = $12,
				location_name = $13,
				location_source = $14,
				updated_at = NOW()
		  WHERE budget_id = $15 AND id = $16
		`, txn.Date,
		txn.PayeeID,
		txn.CategoryID,
		txn.AccountID,
		txn.Note,
		txn.Amount,
		txn.TransferAccountID,
		txn.TransferTransactionID,
		txn.TagIDs,
		txn.Status,
		txn.LocationLat,
		txn.LocationLng,
		txn.LocationName,
		txn.LocationSource,
		budgetId,
		id,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("Transaction not found for id: %v", id)
	}
	return nil
}

// UpdateLocation sets or clears (all-nil) the location fields without touching
// the rest of the transaction.
func (r *transactionRepo) UpdateLocation(
	ctx context.Context,
	tx pgx.Tx,
	budgetId uuid.UUID,
	id uuid.UUID,
	lat *float64,
	lng *float64,
	name *string,
	source *model.LocationSource,
) error {
	cmdTag, err := r.Executor(tx).Exec(
		ctx, `
			UPDATE transactions SET
				location_lat = $1,
				location_lng = $2,
				location_name = $3,
				location_source = $4,
				updated_at = NOW()
			WHERE budget_id = $5 AND id = $6 AND deleted = FALSE
		`, lat, lng, name, source, budgetId, id,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("Transaction not found for id: %v", id)
	}
	return nil
}

func (r *transactionRepo) UpdateStatus(
	ctx context.Context,
	tx pgx.Tx,
	budgetId uuid.UUID,
	id uuid.UUID,
	status model.TransactionStatus,
) error {
	cmdTag, err := r.Executor(tx).Exec(
		ctx, `
			UPDATE transactions 
			SET status = $1 WHERE budget_id = $2 AND id = $3 
			`, status, budgetId, id,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("Transaction not found for id: %v", id)
	}
	return nil
}

func (r *transactionRepo) DeleteById(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, id uuid.UUID) error {
	cmdTag, err := r.Executor(tx).Exec(
		ctx, `
			UPDATE transactions 
			SET deleted = TRUE WHERE budget_id = $1 AND id = $2 
			`, budgetId, id,
	)
	if err != nil {
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		return fmt.Errorf("No active transactions found with the given id and budgetId")
	}

	return nil
}
