package repository

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository interface {
	db.BaseRepositoryInterface
	// Create(ctx context.Context, user model.User) (*model.User, error)
	Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.User, error)
	Update(ctx context.Context, budgetId uuid.UUID, user model.User) (*model.User, error)
}

type userRepo struct {
	db.BaseRepository
}

func NewUserRepository(pool *pgxpool.Pool) UserRepository {
	return &userRepo{BaseRepository: db.NewBaseRepository(pool)}
}

func (r *userRepo) Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.User, error) {
	rows, err := r.Executor(nil).Query(
		ctx,
		`SELECT id, budget_id, email, history_id, gmail_refresh_token, created_at, updated_at FROM users WHERE budget_id = $1 AND email LIKE $2`,
		budgetId,
		"%"+query+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		var user model.User
		err := rows.Scan(&user.ID, &user.BudgetID, &user.Email, &user.HistoryID, &user.GmailRefreshToken, &user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}

func (r *userRepo) Update(ctx context.Context, budgetId uuid.UUID, user model.User) (*model.User, error) {
	var updatedUser model.User
	err := r.Executor(nil).QueryRow(
		ctx,
		`UPDATE users SET 
		  history_id=$1,
		  updated_at=NOW()
		WHERE budget_id = $2 AND email = $3
		RETURNING id, budget_id, email, history_id, gmail_refresh_token, created_at, updated_at`,
		user.HistoryID,
		budgetId,
		user.Email,
	).Scan(&updatedUser.ID, &updatedUser.BudgetID, &updatedUser.Email, &updatedUser.HistoryID, &updatedUser.GmailRefreshToken, &updatedUser.CreatedAt, &updatedUser.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return &updatedUser, nil
}
