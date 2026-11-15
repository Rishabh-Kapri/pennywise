package postgres

import (
	"context"
	"errors"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type budgetMemberRepo struct {
	BaseRepository
}

func NewBudgetMemberRepository(pool *pgxpool.Pool) repository.BudgetMemberRepository {
	return &budgetMemberRepo{BaseRepository: NewBaseRepository(pool)}
}

func (r *budgetMemberRepo) Role(ctx context.Context, budgetID, userID uuid.UUID) (model.BudgetRole, error) {
	var role model.BudgetRole
	err := r.Executor(ctx).QueryRow(
		ctx, `
			SELECT m.role
			FROM budget_members m
			JOIN budgets b ON b.id = m.budget_id AND b.deleted = FALSE
			WHERE m.budget_id = $1 AND m.user_id = $2
		`, budgetID, userID,
	).Scan(&role)
	if err != nil {
		return "", err
	}
	return role, nil
}

// googleIdentity is the newest live Google login of the user in the outer
// query's m.user_id, for showing who a member is.
const googleIdentity = `
	LEFT JOIN LATERAL (
		SELECT gpu.name, gpu.email, gpu.picture
		FROM auth_providers ap
		JOIN google_provider_users gpu
		  ON gpu.id = ap.provider_id AND gpu.oauth_client_type = ap.oauth_client_type
		 AND gpu.deleted IS NOT TRUE
		WHERE ap.auth_user_id = m.user_id AND ap.provider_type = 'google' AND ap.deleted IS NOT TRUE
		ORDER BY gpu.updated_at DESC
		LIMIT 1
	) g ON TRUE`

func (r *budgetMemberRepo) List(ctx context.Context, budgetID uuid.UUID) ([]model.BudgetMember, error) {
	rows, err := r.Executor(ctx).Query(
		ctx, `
			SELECT m.user_id, m.role, COALESCE(g.name, ''), COALESCE(lower(g.email), ''),
			       COALESCE(g.picture, ''), m.created_at
			FROM budget_members m`+googleIdentity+`
			WHERE m.budget_id = $1
			ORDER BY (m.role = 'owner') DESC, m.created_at
		`, budgetID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	members := []model.BudgetMember{}
	for rows.Next() {
		var m model.BudgetMember
		if err := rows.Scan(&m.UserID, &m.Role, &m.Name, &m.Email, &m.Picture, &m.CreatedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (r *budgetMemberRepo) Add(
	ctx context.Context,
	budgetID, userID uuid.UUID,
	role model.BudgetRole,
	invitedBy *uuid.UUID,
) (bool, error) {
	tag, err := r.Executor(ctx).Exec(
		ctx, `
			INSERT INTO budget_members (budget_id, user_id, role, invited_by)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (budget_id, user_id) DO NOTHING
		`, budgetID, userID, role, invitedBy,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *budgetMemberRepo) UpdateRole(ctx context.Context, budgetID, userID uuid.UUID, role model.BudgetRole) error {
	tag, err := r.Executor(ctx).Exec(
		ctx, `
			UPDATE budget_members SET role = $3, updated_at = NOW()
			WHERE budget_id = $1 AND user_id = $2 AND role <> 'owner'
		`, budgetID, userID, role,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *budgetMemberRepo) Remove(ctx context.Context, budgetID, userID uuid.UUID) error {
	tag, err := r.Executor(ctx).Exec(
		ctx,
		`DELETE FROM budget_members WHERE budget_id = $1 AND user_id = $2 AND role <> 'owner'`,
		budgetID, userID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *budgetMemberRepo) SetSelected(ctx context.Context, userID, budgetID uuid.UUID) error {
	var found bool
	err := r.Executor(ctx).QueryRow(
		ctx, `
			WITH updated AS (
				UPDATE budget_members
				SET is_selected = (budget_id = $2), updated_at = NOW()
				WHERE user_id = $1 AND (is_selected OR budget_id = $2)
				  AND EXISTS (SELECT 1 FROM budget_members WHERE user_id = $1 AND budget_id = $2)
				RETURNING budget_id
			)
			SELECT EXISTS (SELECT 1 FROM updated WHERE budget_id = $2)
		`, userID, budgetID,
	).Scan(&found)
	if err != nil {
		return err
	}
	if !found {
		return repository.ErrNotFound
	}
	return nil
}

func (r *budgetMemberRepo) EmailsOfUser(ctx context.Context, userID uuid.UUID) ([]string, error) {
	rows, err := r.Executor(ctx).Query(
		ctx, `
			SELECT DISTINCT lower(gpu.email)
			FROM auth_providers ap
			JOIN google_provider_users gpu
			  ON gpu.id = ap.provider_id AND gpu.oauth_client_type = ap.oauth_client_type
			 AND gpu.deleted IS NOT TRUE
			WHERE ap.auth_user_id = $1 AND ap.provider_type = 'google' AND ap.deleted IS NOT TRUE
			  AND gpu.email <> ''
		`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var emails []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, err
		}
		emails = append(emails, email)
	}
	return emails, rows.Err()
}

type budgetInviteRepo struct {
	BaseRepository
}

func NewBudgetInviteRepository(pool *pgxpool.Pool) repository.BudgetInviteRepository {
	return &budgetInviteRepo{BaseRepository: NewBaseRepository(pool)}
}

// openInvite is the WHERE clause for invites still open, on alias i.
const openInvite = `i.accepted_at IS NULL AND i.declined_at IS NULL AND i.revoked_at IS NULL AND i.expires_at > NOW()`

const inviteColumns = `i.id, i.budget_id, i.email, i.role, i.invited_by, i.created_at, i.expires_at`

func scanInvite(row interface{ Scan(...any) error }, extra ...any) (model.BudgetInvite, error) {
	var inv model.BudgetInvite
	dest := append([]any{
		&inv.ID, &inv.BudgetID, &inv.Email, &inv.Role, &inv.InvitedBy, &inv.CreatedAt, &inv.ExpiresAt,
	}, extra...)
	err := row.Scan(dest...)
	return inv, err
}

func (r *budgetInviteRepo) Create(ctx context.Context, invite model.BudgetInvite) (*model.BudgetInvite, error) {
	err := r.InTx(ctx, func(ctx context.Context) error {
		// An expired invite still holds the unique index; close it first so
		// the same email can be invited again.
		if _, err := r.Executor(ctx).Exec(
			ctx, `
				UPDATE budget_invites i SET revoked_at = NOW()
				WHERE i.budget_id = $1 AND i.email = $2
				  AND i.accepted_at IS NULL AND i.declined_at IS NULL AND i.revoked_at IS NULL
				  AND i.expires_at <= NOW()
			`, invite.BudgetID, invite.Email,
		); err != nil {
			return err
		}
		created, err := scanInvite(r.Executor(ctx).QueryRow(
			ctx, `
				INSERT INTO budget_invites AS i (budget_id, email, role, invited_by, expires_at)
				VALUES ($1, $2, $3, $4, $5)
				RETURNING `+inviteColumns,
			invite.BudgetID, invite.Email, invite.Role, invite.InvitedBy, invite.ExpiresAt,
		))
		if err != nil {
			return err
		}
		invite = created
		return nil
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, repository.ErrInviteExists
		}
		return nil, err
	}
	return &invite, nil
}

func (r *budgetInviteRepo) ListByBudget(ctx context.Context, budgetID uuid.UUID) ([]model.BudgetInvite, error) {
	rows, err := r.Executor(ctx).Query(
		ctx, `
			SELECT `+inviteColumns+`
			FROM budget_invites i
			WHERE i.budget_id = $1 AND `+openInvite+`
			ORDER BY i.created_at DESC
		`, budgetID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	invites := []model.BudgetInvite{}
	for rows.Next() {
		inv, err := scanInvite(rows)
		if err != nil {
			return nil, err
		}
		invites = append(invites, inv)
	}
	return invites, rows.Err()
}

func (r *budgetInviteRepo) ListForEmails(ctx context.Context, emails []string) ([]model.BudgetInvite, error) {
	invites := []model.BudgetInvite{}
	if len(emails) == 0 {
		return invites, nil
	}
	rows, err := r.Executor(ctx).Query(
		ctx, `
			SELECT `+inviteColumns+`, b.name, COALESCE(g.name, '')
			FROM budget_invites i
			JOIN budgets b ON b.id = i.budget_id AND b.deleted = FALSE
			-- googleIdentity reads m.user_id: here, the inviter.
			CROSS JOIN LATERAL (SELECT i.invited_by AS user_id) m`+googleIdentity+`
			WHERE i.email = ANY($1) AND `+openInvite+`
			ORDER BY i.created_at DESC
		`, emails,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var budgetName, inviterName string
		inv, err := scanInvite(rows, &budgetName, &inviterName)
		if err != nil {
			return nil, err
		}
		inv.BudgetName, inv.InviterName = budgetName, inviterName
		invites = append(invites, inv)
	}
	return invites, rows.Err()
}

func (r *budgetInviteRepo) GetOpen(ctx context.Context, id uuid.UUID) (*model.BudgetInvite, error) {
	inv, err := scanInvite(r.Executor(ctx).QueryRow(
		ctx, `
			SELECT `+inviteColumns+`
			FROM budget_invites i
			JOIN budgets b ON b.id = i.budget_id AND b.deleted = FALSE
			WHERE i.id = $1 AND `+openInvite,
		id,
	))
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

// closeOpen runs an UPDATE on one open invite and maps "no row" to ErrNotFound.
func (r *budgetInviteRepo) closeOpen(ctx context.Context, sql string, args ...any) error {
	tag, err := r.Executor(ctx).Exec(ctx, sql, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *budgetInviteRepo) MarkAccepted(ctx context.Context, id, userID uuid.UUID) error {
	return r.closeOpen(ctx, `
		UPDATE budget_invites i SET accepted_at = NOW(), accepted_by = $2
		WHERE i.id = $1 AND `+openInvite, id, userID)
}

func (r *budgetInviteRepo) MarkDeclined(ctx context.Context, id uuid.UUID) error {
	return r.closeOpen(ctx, `
		UPDATE budget_invites i SET declined_at = NOW()
		WHERE i.id = $1 AND `+openInvite, id)
}

func (r *budgetInviteRepo) Revoke(ctx context.Context, budgetID, id uuid.UUID) error {
	return r.closeOpen(ctx, `
		UPDATE budget_invites i SET revoked_at = NOW()
		WHERE i.id = $1 AND i.budget_id = $2 AND `+openInvite, id, budgetID)
}

func (r *budgetInviteRepo) RevokeForEmails(ctx context.Context, budgetID uuid.UUID, emails []string) error {
	if len(emails) == 0 {
		return nil
	}
	_, err := r.Executor(ctx).Exec(
		ctx, `
			UPDATE budget_invites i SET revoked_at = NOW()
			WHERE i.budget_id = $1 AND i.email = ANY($2)
			  AND i.accepted_at IS NULL AND i.declined_at IS NULL AND i.revoked_at IS NULL
		`, budgetID, emails,
	)
	return err
}
