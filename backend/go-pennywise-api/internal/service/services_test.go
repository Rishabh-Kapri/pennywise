package service

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/config"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ─────────────────────────────────────────────────────────────────────────────
// Mock repos (prefixed "svc" to avoid conflict with transaction_test.go mocks)
// ─────────────────────────────────────────────────────────────────────────────

// svcAccountRepo — full AccountRepository mock
type svcAccountRepo struct {
	mockBaseRepo
	mock.Mock
}

func (m *svcAccountRepo) GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Account, error) {
	args := m.Called(ctx, budgetId)
	if v := args.Get(0); v != nil {
		return v.([]model.Account), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcAccountRepo) GetById(ctx context.Context, tx pgx.Tx, budgetId, accountId uuid.UUID) (*model.Account, error) {
	args := m.Called(ctx, tx, budgetId, accountId)
	if v := args.Get(0); v != nil {
		return v.(*model.Account), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcAccountRepo) GetBySuffix(ctx context.Context, budgetId uuid.UUID, suffix string) (*model.Account, error) {
	args := m.Called(ctx, budgetId, suffix)
	if v := args.Get(0); v != nil {
		return v.(*model.Account), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcAccountRepo) Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.Account, error) {
	args := m.Called(ctx, budgetId, query)
	if v := args.Get(0); v != nil {
		return v.([]model.Account), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcAccountRepo) Create(ctx context.Context, tx pgx.Tx, account model.Account) (*model.Account, error) {
	args := m.Called(ctx, tx, account)
	if v := args.Get(0); v != nil {
		return v.(*model.Account), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcAccountRepo) UpdateTransferPayee(ctx context.Context, tx pgx.Tx, accountId, payeeId uuid.UUID) error {
	return m.Called(ctx, tx, accountId, payeeId).Error(0)
}
func (m *svcAccountRepo) GetAllSimplified(ctx context.Context, budgetId uuid.UUID) ([]model.AccountSimplified, error) {
	args := m.Called(ctx, budgetId)
	if v := args.Get(0); v != nil {
		return v.([]model.AccountSimplified), args.Error(1)
	}
	return nil, args.Error(1)
}

type svcTagRepo struct {
	mockBaseRepo
	mock.Mock
}

func (m *svcTagRepo) GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Tag, error) {
	args := m.Called(ctx, budgetId)
	if v := args.Get(0); v != nil {
		return v.([]model.Tag), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcTagRepo) Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.Tag, error) {
	args := m.Called(ctx, budgetId, query)
	if v := args.Get(0); v != nil {
		return v.([]model.Tag), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcTagRepo) GetById(ctx context.Context, budgetId, id uuid.UUID) (*model.Tag, error) {
	args := m.Called(ctx, budgetId, id)
	if v := args.Get(0); v != nil {
		return v.(*model.Tag), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcTagRepo) Create(ctx context.Context, tx pgx.Tx, tag model.Tag) (*model.Tag, error) {
	args := m.Called(ctx, tx, tag)
	if v := args.Get(0); v != nil {
		return v.(*model.Tag), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcTagRepo) Update(ctx context.Context, budgetId, id uuid.UUID, tag model.Tag) error {
	return m.Called(ctx, budgetId, id, tag).Error(0)
}
func (m *svcTagRepo) DeleteById(ctx context.Context, budgetId, id uuid.UUID) error {
	return m.Called(ctx, budgetId, id).Error(0)
}

// svcPayeeRepo — PayeesRepository
type svcPayeeRepo struct {
	mockBaseRepo
	mock.Mock
}

func (m *svcPayeeRepo) GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Payee, error) {
	args := m.Called(ctx, budgetId)
	if v := args.Get(0); v != nil {
		return v.([]model.Payee), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcPayeeRepo) Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.Payee, error) {
	args := m.Called(ctx, budgetId, query)
	if v := args.Get(0); v != nil {
		return v.([]model.Payee), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcPayeeRepo) GetById(ctx context.Context, budgetId, id uuid.UUID) (*model.Payee, error) {
	args := m.Called(ctx, budgetId, id)
	if v := args.Get(0); v != nil {
		return v.(*model.Payee), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcPayeeRepo) GetByIdTx(ctx context.Context, tx pgx.Tx, budgetId, id uuid.UUID) (*model.Payee, error) {
	args := m.Called(ctx, tx, budgetId, id)
	if v := args.Get(0); v != nil {
		return v.(*model.Payee), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcPayeeRepo) Create(ctx context.Context, tx pgx.Tx, payee model.Payee) (*model.Payee, error) {
	args := m.Called(ctx, tx, payee)
	if v := args.Get(0); v != nil {
		return v.(*model.Payee), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcPayeeRepo) DeleteById(ctx context.Context, budgetId, id uuid.UUID) error {
	return m.Called(ctx, budgetId, id).Error(0)
}
func (m *svcPayeeRepo) Update(ctx context.Context, budgetId, id uuid.UUID, payee model.Payee) error {
	return m.Called(ctx, budgetId, id, payee).Error(0)
}

// svcPayeeRuleRepo
type svcPayeeRuleRepo struct {
	mockBaseRepo
	mock.Mock
}

func (m *svcPayeeRuleRepo) CreatePayeeRule(ctx context.Context, tx pgx.Tx, payeeMatch model.PayeeRule) error {
	return m.Called(ctx, tx, payeeMatch).Error(0)
}
func (m *svcPayeeRuleRepo) FindByMatchString(ctx context.Context, budgetId uuid.UUID, matchString string) (*model.PayeeRule, error) {
	args := m.Called(ctx, budgetId, matchString)
	if v := args.Get(0); v != nil {
		return v.(*model.PayeeRule), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcPayeeRuleRepo) FindByPayeeID(ctx context.Context, budgetId, payeeId uuid.UUID) ([]model.PayeeRuleDetails, error) {
	args := m.Called(ctx, budgetId, payeeId)
	if v := args.Get(0); v != nil {
		return v.([]model.PayeeRuleDetails), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcPayeeRuleRepo) Update(ctx context.Context, budgetId, id uuid.UUID, payeeRule model.PayeeRule) error {
	return m.Called(ctx, budgetId, id, payeeRule).Error(0)
}
func (m *svcPayeeRuleRepo) DeleteByID(ctx context.Context, budgetId, id uuid.UUID) error {
	return m.Called(ctx, budgetId, id).Error(0)
}

// svcBudgetRepo — full BudgetRepository mock
type svcBudgetRepo struct {
	mockBaseRepo
	mock.Mock
}

func (m *svcBudgetRepo) GetAll(ctx context.Context, userID uuid.UUID) ([]model.Budget, error) {
	args := m.Called(ctx, userID)
	if v := args.Get(0); v != nil {
		return v.([]model.Budget), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcBudgetRepo) GetById(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*model.Budget, error) {
	args := m.Called(ctx, tx, id)
	if v := args.Get(0); v != nil {
		return v.(*model.Budget), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcBudgetRepo) Create(ctx context.Context, tx pgx.Tx, name string, userID uuid.UUID) (*model.Budget, error) {
	args := m.Called(ctx, tx, name, userID)
	if v := args.Get(0); v != nil {
		return v.(*model.Budget), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcBudgetRepo) UpdateById(ctx context.Context, tx pgx.Tx, id uuid.UUID, budget model.Budget) error {
	return m.Called(ctx, tx, id, budget).Error(0)
}
func (m *svcBudgetRepo) IsOwnedByUser(ctx context.Context, budgetID, userID uuid.UUID) (bool, error) {
	args := m.Called(ctx, budgetID, userID)
	return args.Bool(0), args.Error(1)
}

// svcCategoryRepo
type svcCategoryRepo struct {
	mockBaseRepo
	mock.Mock
}

func (m *svcCategoryRepo) GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Category, error) {
	args := m.Called(ctx, budgetId)
	if v := args.Get(0); v != nil {
		return v.([]model.Category), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcCategoryRepo) GetAllSimplified(ctx context.Context, budgetId uuid.UUID) ([]model.CategorySimplified, error) {
	args := m.Called(ctx, budgetId)
	if v := args.Get(0); v != nil {
		return v.([]model.CategorySimplified), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcCategoryRepo) GetInflowBalance(ctx context.Context, budgetId uuid.UUID) (float64, error) {
	args := m.Called(ctx, budgetId)
	return args.Get(0).(float64), args.Error(1)
}
func (m *svcCategoryRepo) GetByFilter(ctx context.Context, budgetId uuid.UUID, filter model.CategoryFilter) ([]model.Category, error) {
	args := m.Called(ctx, budgetId, filter)
	if v := args.Get(0); v != nil {
		return v.([]model.Category), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcCategoryRepo) Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.Category, error) {
	args := m.Called(ctx, budgetId, query)
	if v := args.Get(0); v != nil {
		return v.([]model.Category), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcCategoryRepo) GetById(ctx context.Context, budgetId, id uuid.UUID) (*model.Category, error) {
	args := m.Called(ctx, budgetId, id)
	if v := args.Get(0); v != nil {
		return v.(*model.Category), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcCategoryRepo) GetByIdSimplified(ctx context.Context, budgetId, id uuid.UUID) (*model.Category, error) {
	args := m.Called(ctx, budgetId, id)
	if v := args.Get(0); v != nil {
		return v.(*model.Category), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcCategoryRepo) GetByIdSimplifiedTx(ctx context.Context, tx pgx.Tx, budgetId, id uuid.UUID) (*model.Category, error) {
	args := m.Called(ctx, tx, budgetId, id)
	if v := args.Get(0); v != nil {
		return v.(*model.Category), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcCategoryRepo) Create(ctx context.Context, tx pgx.Tx, category model.Category) (*model.Category, error) {
	args := m.Called(ctx, tx, category)
	if v := args.Get(0); v != nil {
		return v.(*model.Category), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcCategoryRepo) DeleteById(ctx context.Context, budgetId, id uuid.UUID) error {
	return m.Called(ctx, budgetId, id).Error(0)
}
func (m *svcCategoryRepo) Update(ctx context.Context, budgetId, id uuid.UUID, category model.Category) error {
	return m.Called(ctx, budgetId, id, category).Error(0)
}

// svcCategoryGroupRepo
type svcCategoryGroupRepo struct {
	mockBaseRepo
	mock.Mock
}

func (m *svcCategoryGroupRepo) GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.CategoryGroup, error) {
	args := m.Called(ctx, budgetId)
	if v := args.Get(0); v != nil {
		return v.([]model.CategoryGroup), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcCategoryGroupRepo) Create(ctx context.Context, tx pgx.Tx, categoryGroup model.CategoryGroup) (*model.CategoryGroup, error) {
	args := m.Called(ctx, tx, categoryGroup)
	if v := args.Get(0); v != nil {
		return v.(*model.CategoryGroup), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcCategoryGroupRepo) Update(ctx context.Context, budgetId, id uuid.UUID, categoryGroup model.CategoryGroup) error {
	return m.Called(ctx, budgetId, id, categoryGroup).Error(0)
}
func (m *svcCategoryGroupRepo) DeleteById(ctx context.Context, budgetId, id uuid.UUID) error {
	return m.Called(ctx, budgetId, id).Error(0)
}

// svcAPIKeyRepo
type svcAPIKeyRepo struct {
	mockBaseRepo
	mock.Mock
}

func (m *svcAPIKeyRepo) Create(ctx context.Context, tx pgx.Tx, apiKey *model.APIKey) error {
	return m.Called(ctx, tx, apiKey).Error(0)
}
func (m *svcAPIKeyRepo) GetByKeyID(ctx context.Context, tx pgx.Tx, keyID string) (*model.APIKey, error) {
	args := m.Called(ctx, tx, keyID)
	if v := args.Get(0); v != nil {
		return v.(*model.APIKey), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcAPIKeyRepo) GetByHash(ctx context.Context, tx pgx.Tx, keyHash string) (*model.APIKey, error) {
	args := m.Called(ctx, tx, keyHash)
	if v := args.Get(0); v != nil {
		return v.(*model.APIKey), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcAPIKeyRepo) UpdateLastUsed(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	return m.Called(ctx, tx, id).Error(0)
}

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

func budgetCtxWith(budgetID uuid.UUID) context.Context {
	return utils.WithBudgetID(context.Background(), budgetID)
}

// ─────────────────────────────────────────────────────────────────────────────
// TagService tests
// ─────────────────────────────────────────────────────────────────────────────

func TestTagService_GetAll(t *testing.T) {
	budgetID := uuid.New()
	ctx := budgetCtxWith(budgetID)

	t.Run("returns_tags", func(t *testing.T) {
		repo := &svcTagRepo{}
		repo.On("GetAll", mock.Anything, budgetID).Return([]model.Tag{{ID: uuid.New(), Name: "food"}}, nil)
		tags, err := NewTagService(repo).GetAll(ctx)
		assert.NoError(t, err)
		assert.Len(t, tags, 1)
		repo.AssertExpectations(t)
	})
	t.Run("repo_error_propagates", func(t *testing.T) {
		repo := &svcTagRepo{}
		repo.On("GetAll", mock.Anything, budgetID).Return(nil, assert.AnError)
		tags, err := NewTagService(repo).GetAll(ctx)
		assert.Error(t, err)
		assert.Nil(t, tags)
		repo.AssertExpectations(t)
	})
}

func TestTagService_Search(t *testing.T) {
	budgetID := uuid.New()
	ctx := budgetCtxWith(budgetID)
	repo := &svcTagRepo{}
	repo.On("Search", mock.Anything, budgetID, "foo").Return([]model.Tag{{Name: "food"}}, nil)
	tags, err := NewTagService(repo).Search(ctx, "foo")
	assert.NoError(t, err)
	assert.Len(t, tags, 1)
	repo.AssertExpectations(t)
}

func TestTagService_GetById(t *testing.T) {
	budgetID := uuid.New()
	tagID := uuid.New()
	ctx := budgetCtxWith(budgetID)

	t.Run("returns_tag", func(t *testing.T) {
		repo := &svcTagRepo{}
		repo.On("GetById", mock.Anything, budgetID, tagID).Return(&model.Tag{ID: tagID}, nil)
		tag, err := NewTagService(repo).GetById(ctx, tagID)
		assert.NoError(t, err)
		assert.Equal(t, tagID, tag.ID)
		repo.AssertExpectations(t)
	})
	t.Run("repo_error_propagates", func(t *testing.T) {
		repo := &svcTagRepo{}
		repo.On("GetById", mock.Anything, budgetID, tagID).Return(nil, assert.AnError)
		tag, err := NewTagService(repo).GetById(ctx, tagID)
		assert.Error(t, err)
		assert.Nil(t, tag)
		repo.AssertExpectations(t)
	})
}

func TestTagService_Create(t *testing.T) {
	budgetID := uuid.New()
	ctx := budgetCtxWith(budgetID)
	created := &model.Tag{ID: uuid.New(), Name: "groceries", BudgetID: budgetID}

	t.Run("creates_tag", func(t *testing.T) {
		repo := &svcTagRepo{}
		repo.On("Create", mock.Anything, (pgx.Tx)(nil), mock.MatchedBy(func(tag model.Tag) bool {
			return tag.Name == "groceries" && tag.BudgetID == budgetID
		})).Return(created, nil)
		result, err := NewTagService(repo).Create(ctx, model.Tag{Name: "groceries"})
		assert.NoError(t, err)
		assert.Equal(t, created.ID, result.ID)
		repo.AssertExpectations(t)
	})
	t.Run("repo_error_propagates", func(t *testing.T) {
		repo := &svcTagRepo{}
		repo.On("Create", mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)
		result, err := NewTagService(repo).Create(ctx, model.Tag{Name: "groceries"})
		assert.Error(t, err)
		assert.Nil(t, result)
		repo.AssertExpectations(t)
	})
}

func TestTagService_Update(t *testing.T) {
	budgetID := uuid.New()
	tagID := uuid.New()
	ctx := budgetCtxWith(budgetID)

	t.Run("updates_tag", func(t *testing.T) {
		repo := &svcTagRepo{}
		repo.On("Update", mock.Anything, budgetID, tagID, mock.Anything).Return(nil)
		assert.NoError(t, NewTagService(repo).Update(ctx, tagID, model.Tag{Name: "updated"}))
		repo.AssertExpectations(t)
	})
	t.Run("repo_error_propagates", func(t *testing.T) {
		repo := &svcTagRepo{}
		repo.On("Update", mock.Anything, budgetID, tagID, mock.Anything).Return(assert.AnError)
		assert.Error(t, NewTagService(repo).Update(ctx, tagID, model.Tag{}))
		repo.AssertExpectations(t)
	})
}

func TestTagService_DeleteById(t *testing.T) {
	budgetID := uuid.New()
	tagID := uuid.New()
	ctx := budgetCtxWith(budgetID)

	t.Run("deletes_tag", func(t *testing.T) {
		repo := &svcTagRepo{}
		repo.On("DeleteById", mock.Anything, budgetID, tagID).Return(nil)
		assert.NoError(t, NewTagService(repo).DeleteById(ctx, tagID))
		repo.AssertExpectations(t)
	})
	t.Run("repo_error_propagates", func(t *testing.T) {
		repo := &svcTagRepo{}
		repo.On("DeleteById", mock.Anything, budgetID, tagID).Return(assert.AnError)
		assert.Error(t, NewTagService(repo).DeleteById(ctx, tagID))
		repo.AssertExpectations(t)
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// AccountService tests
// ─────────────────────────────────────────────────────────────────────────────

func TestAccountService_GetAll(t *testing.T) {
	budgetID := uuid.New()
	ctx := budgetCtxWith(budgetID)

	t.Run("returns_accounts", func(t *testing.T) {
		repo := &svcAccountRepo{}
		payeeRepo := &svcPayeeRepo{}
		repo.On("GetAll", mock.Anything, budgetID).Return([]model.Account{{ID: uuid.New()}}, nil)
		accounts, err := NewAccountService(repo, payeeRepo).GetAll(ctx)
		assert.NoError(t, err)
		assert.Len(t, accounts, 1)
		repo.AssertExpectations(t)
	})
	t.Run("repo_error_propagates", func(t *testing.T) {
		repo := &svcAccountRepo{}
		payeeRepo := &svcPayeeRepo{}
		repo.On("GetAll", mock.Anything, budgetID).Return(nil, assert.AnError)
		accounts, err := NewAccountService(repo, payeeRepo).GetAll(ctx)
		assert.Error(t, err)
		assert.Nil(t, accounts)
		repo.AssertExpectations(t)
	})
}

func TestAccountService_Search(t *testing.T) {
	budgetID := uuid.New()
	ctx := budgetCtxWith(budgetID)
	repo := &svcAccountRepo{}
	payeeRepo := &svcPayeeRepo{}
	repo.On("Search", mock.Anything, budgetID, "savings").Return([]model.Account{{Name: "Savings"}}, nil)
	accounts, err := NewAccountService(repo, payeeRepo).Search(ctx, "savings")
	assert.NoError(t, err)
	assert.Len(t, accounts, 1)
	repo.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// BudgetService tests
// ─────────────────────────────────────────────────────────────────────────────

func TestBudgetService_GetAll(t *testing.T) {
	userID := uuid.New()
	ctx := context.Background()

	t.Run("returns_budgets", func(t *testing.T) {
		repo := &svcBudgetRepo{}
		repo.On("GetAll", mock.Anything, userID).Return([]model.Budget{{ID: uuid.New(), Name: "Main"}}, nil)
		svc := NewBudgetService(repo, &svcPayeeRepo{}, &svcCategoryRepo{}, &svcCategoryGroupRepo{})
		budgets, err := svc.GetAll(ctx, userID)
		assert.NoError(t, err)
		assert.Len(t, budgets, 1)
		repo.AssertExpectations(t)
	})
	t.Run("repo_error_propagates", func(t *testing.T) {
		repo := &svcBudgetRepo{}
		repo.On("GetAll", mock.Anything, userID).Return(nil, assert.AnError)
		svc := NewBudgetService(repo, &svcPayeeRepo{}, &svcCategoryRepo{}, &svcCategoryGroupRepo{})
		budgets, err := svc.GetAll(ctx, userID)
		assert.Error(t, err)
		assert.Nil(t, budgets)
		repo.AssertExpectations(t)
	})
}

func TestBudgetService_Create_ValidationErrors(t *testing.T) {
	userID := uuid.New()
	ctx := context.Background()

	t.Run("empty_name_returns_error", func(t *testing.T) {
		repo := &svcBudgetRepo{}
		svc := NewBudgetService(repo, &svcPayeeRepo{}, &svcCategoryRepo{}, &svcCategoryGroupRepo{})
		result, err := svc.Create(ctx, model.CreateBudgetRequest{Name: "   "}, userID)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
	t.Run("get_all_error_propagates", func(t *testing.T) {
		repo := &svcBudgetRepo{}
		repo.On("GetAll", mock.Anything, userID).Return(nil, assert.AnError)
		svc := NewBudgetService(repo, &svcPayeeRepo{}, &svcCategoryRepo{}, &svcCategoryGroupRepo{})
		result, err := svc.Create(ctx, model.CreateBudgetRequest{Name: "Budget"}, userID)
		assert.Error(t, err)
		assert.Nil(t, result)
		repo.AssertExpectations(t)
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// CategoryGroupService tests
// ─────────────────────────────────────────────────────────────────────────────

func TestCategoryGroupService_GetAll(t *testing.T) {
	budgetID := uuid.New()
	ctx := budgetCtxWith(budgetID)

	t.Run("returns_groups", func(t *testing.T) {
		repo := &svcCategoryGroupRepo{}
		repo.On("GetAll", mock.Anything, budgetID).Return([]model.CategoryGroup{{ID: uuid.New(), Name: "Bills"}}, nil)
		groups, err := NewCategoryGroupService(repo).GetAll(ctx, "")
		assert.NoError(t, err)
		assert.Len(t, groups, 1)
		repo.AssertExpectations(t)
	})
	t.Run("repo_error_propagates", func(t *testing.T) {
		repo := &svcCategoryGroupRepo{}
		repo.On("GetAll", mock.Anything, budgetID).Return(nil, assert.AnError)
		groups, err := NewCategoryGroupService(repo).GetAll(ctx, "2024-01")
		assert.Error(t, err)
		assert.Nil(t, groups)
		repo.AssertExpectations(t)
	})
}

func TestCategoryGroupService_Create(t *testing.T) {
	budgetID := uuid.New()
	ctx := budgetCtxWith(budgetID)
	created := &model.CategoryGroup{ID: uuid.New(), Name: "Savings"}

	repo := &svcCategoryGroupRepo{}
	repo.On("Create", mock.Anything, (pgx.Tx)(nil), mock.MatchedBy(func(g model.CategoryGroup) bool {
		return g.Name == "Savings" && g.BudgetID == budgetID
	})).Return(created, nil)
	result, err := NewCategoryGroupService(repo).Create(ctx, model.CategoryGroup{Name: "Savings"})
	assert.NoError(t, err)
	assert.Equal(t, created.ID, result.ID)
	repo.AssertExpectations(t)
}

func TestCategoryGroupService_Update(t *testing.T) {
	budgetID := uuid.New()
	groupID := uuid.New()
	ctx := budgetCtxWith(budgetID)
	repo := &svcCategoryGroupRepo{}
	repo.On("Update", mock.Anything, budgetID, groupID, mock.Anything).Return(nil)
	assert.NoError(t, NewCategoryGroupService(repo).Update(ctx, groupID, model.CategoryGroup{Name: "Updated"}))
	repo.AssertExpectations(t)
}

func TestCategoryGroupService_DeleteById(t *testing.T) {
	budgetID := uuid.New()
	groupID := uuid.New()
	ctx := budgetCtxWith(budgetID)
	repo := &svcCategoryGroupRepo{}
	repo.On("DeleteById", mock.Anything, budgetID, groupID).Return(nil)
	assert.NoError(t, NewCategoryGroupService(repo).DeleteById(ctx, groupID))
	repo.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// CategoryService tests
// ─────────────────────────────────────────────────────────────────────────────

func TestCategoryService_GetAll(t *testing.T) {
	budgetID := uuid.New()
	ctx := budgetCtxWith(budgetID)
	repo := &svcCategoryRepo{}
	repo.On("GetAll", mock.Anything, budgetID).Return([]model.Category{{ID: uuid.New()}}, nil)
	cats, err := NewCategoryService(repo, nil, nil).GetAll(ctx)
	assert.NoError(t, err)
	assert.Len(t, cats, 1)
	repo.AssertExpectations(t)
}

func TestCategoryService_GetInflowBalance(t *testing.T) {
	budgetID := uuid.New()
	ctx := budgetCtxWith(budgetID)
	repo := &svcCategoryRepo{}
	repo.On("GetInflowBalance", mock.Anything, budgetID).Return(float64(100.5), nil)
	balance, err := NewCategoryService(repo, nil, nil).GetInflowBalance(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 100.5, balance)
	repo.AssertExpectations(t)
}

func TestCategoryService_Create(t *testing.T) {
	budgetID := uuid.New()
	ctx := budgetCtxWith(budgetID)
	created := &model.Category{ID: uuid.New(), Name: "Groceries"}

	t.Run("creates_category", func(t *testing.T) {
		repo := &svcCategoryRepo{}
		repo.On("Create", mock.Anything, (pgx.Tx)(nil), mock.MatchedBy(func(c model.Category) bool {
			return c.Name == "Groceries" && c.BudgetID == budgetID
		})).Return(created, nil)
		result, err := NewCategoryService(repo, nil, nil).Create(ctx, model.Category{Name: "Groceries"})
		assert.NoError(t, err)
		assert.Equal(t, created.ID, result.ID)
		repo.AssertExpectations(t)
	})
}

func TestCategoryService_DeleteById(t *testing.T) {
	budgetID := uuid.New()
	catID := uuid.New()
	ctx := budgetCtxWith(budgetID)
	repo := &svcCategoryRepo{}
	repo.On("DeleteById", mock.Anything, budgetID, catID).Return(nil)
	assert.NoError(t, NewCategoryService(repo, nil, nil).DeleteById(ctx, catID))
	repo.AssertExpectations(t)
}

func TestCategoryService_Update(t *testing.T) {
	budgetID := uuid.New()
	catID := uuid.New()
	ctx := budgetCtxWith(budgetID)
	repo := &svcCategoryRepo{}
	repo.On("Update", mock.Anything, budgetID, catID, mock.Anything).Return(nil)
	assert.NoError(t, NewCategoryService(repo, nil, nil).Update(ctx, catID, model.Category{Name: "Updated"}))
	repo.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// PayeeService tests
// ─────────────────────────────────────────────────────────────────────────────

func TestPayeeService_GetAll(t *testing.T) {
	budgetID := uuid.New()
	ctx := budgetCtxWith(budgetID)
	repo := &svcPayeeRepo{}
	ruleRepo := &svcPayeeRuleRepo{}
	repo.On("GetAll", mock.Anything, budgetID).Return([]model.Payee{{ID: uuid.New(), Name: "Amazon"}}, nil)
	payees, err := NewPayeeService(repo, ruleRepo).GetAll(ctx)
	assert.NoError(t, err)
	assert.Len(t, payees, 1)
	repo.AssertExpectations(t)
}

func TestPayeeService_GetById(t *testing.T) {
	budgetID := uuid.New()
	payeeID := uuid.New()
	ctx := budgetCtxWith(budgetID)
	repo := &svcPayeeRepo{}
	ruleRepo := &svcPayeeRuleRepo{}
	repo.On("GetById", mock.Anything, budgetID, payeeID).Return(&model.Payee{ID: payeeID}, nil)
	p, err := NewPayeeService(repo, ruleRepo).GetById(ctx, payeeID)
	assert.NoError(t, err)
	assert.Equal(t, payeeID, p.ID)
	repo.AssertExpectations(t)
}

func TestPayeeService_Create(t *testing.T) {
	budgetID := uuid.New()
	ctx := budgetCtxWith(budgetID)
	created := &model.Payee{ID: uuid.New(), Name: "Walmart"}

	t.Run("creates_payee", func(t *testing.T) {
		repo := &svcPayeeRepo{}
		ruleRepo := &svcPayeeRuleRepo{}
		repo.On("Create", mock.Anything, (pgx.Tx)(nil), mock.MatchedBy(func(p model.Payee) bool {
			return p.Name == "Walmart" && p.BudgetID == budgetID
		})).Return(created, nil)
		result, err := NewPayeeService(repo, ruleRepo).Create(ctx, model.Payee{Name: "Walmart"})
		assert.NoError(t, err)
		assert.Equal(t, created.ID, result.ID)
		repo.AssertExpectations(t)
	})
}

func TestPayeeService_DeleteById(t *testing.T) {
	budgetID := uuid.New()
	payeeID := uuid.New()
	ctx := budgetCtxWith(budgetID)
	repo := &svcPayeeRepo{}
	ruleRepo := &svcPayeeRuleRepo{}
	repo.On("DeleteById", mock.Anything, budgetID, payeeID).Return(nil)
	assert.NoError(t, NewPayeeService(repo, ruleRepo).DeleteById(ctx, payeeID))
	repo.AssertExpectations(t)
}

func TestPayeeService_Update(t *testing.T) {
	budgetID := uuid.New()
	payeeID := uuid.New()
	ctx := budgetCtxWith(budgetID)
	repo := &svcPayeeRepo{}
	ruleRepo := &svcPayeeRuleRepo{}
	repo.On("Update", mock.Anything, budgetID, payeeID, mock.Anything).Return(nil)
	assert.NoError(t, NewPayeeService(repo, ruleRepo).Update(ctx, payeeID, model.Payee{Name: "Updated"}))
	repo.AssertExpectations(t)
}

func TestPayeeService_GetRules(t *testing.T) {
	budgetID := uuid.New()
	payeeID := uuid.New()
	ctx := budgetCtxWith(budgetID)
	repo := &svcPayeeRepo{}
	ruleRepo := &svcPayeeRuleRepo{}
	ruleRepo.On("FindByPayeeID", mock.Anything, budgetID, payeeID).Return([]model.PayeeRuleDetails{{ID: uuid.New()}}, nil)
	rules, err := NewPayeeService(repo, ruleRepo).GetRules(ctx, payeeID)
	assert.NoError(t, err)
	assert.Len(t, rules, 1)
	ruleRepo.AssertExpectations(t)
}

func TestPayeeService_CreateRule(t *testing.T) {
	budgetID := uuid.New()
	payeeID := uuid.New()
	ctx := budgetCtxWith(budgetID)
	repo := &svcPayeeRepo{}
	ruleRepo := &svcPayeeRuleRepo{}
	ruleRepo.On("CreatePayeeRule", mock.Anything, (pgx.Tx)(nil), mock.MatchedBy(func(r model.PayeeRule) bool {
		return r.MatchType == "EXACT" && r.PayeeID == payeeID && r.BudgetID == budgetID
	})).Return(nil)
	err := NewPayeeService(repo, ruleRepo).CreateRule(ctx, payeeID, model.PayeeRule{MatchString: "test"})
	assert.NoError(t, err)
	ruleRepo.AssertExpectations(t)
}

func TestPayeeService_DeleteRule(t *testing.T) {
	budgetID := uuid.New()
	ruleID := uuid.New()
	ctx := budgetCtxWith(budgetID)
	repo := &svcPayeeRepo{}
	ruleRepo := &svcPayeeRuleRepo{}
	ruleRepo.On("DeleteByID", mock.Anything, budgetID, ruleID).Return(nil)
	assert.NoError(t, NewPayeeService(repo, ruleRepo).DeleteRule(ctx, ruleID))
	ruleRepo.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// APIKeyService tests
// ─────────────────────────────────────────────────────────────────────────────

func TestAPIKeyService_Generate(t *testing.T) {
	svc := NewApiKeyService(nil)

	t.Run("generates_valid_key", func(t *testing.T) {
		fullKey, _, err := svc.Generate()
		assert.NoError(t, err)
		assert.True(t, svc.ValidateFormat(fullKey))
	})
	t.Run("keys_are_unique", func(t *testing.T) {
		k1, _, _ := svc.Generate()
		k2, _, _ := svc.Generate()
		assert.NotEqual(t, k1, k2)
	})
}

func TestAPIKeyService_ParseKey(t *testing.T) {
	svc := NewApiKeyService(nil)

	t.Run("parses_valid_key", func(t *testing.T) {
		// Use a handcrafted key with no underscores in the base64 part
		expectedLen := base64.RawURLEncoding.EncodedLen(KeyLength)
		b64Part := strings.Repeat("A", expectedLen)
		key := "pwk_v1_" + b64Part
		prefix, version, b64, err := svc.ParseKey(key)
		assert.NoError(t, err)
		assert.Equal(t, "pwk", prefix)
		assert.Equal(t, "v1", version)
		assert.NotEmpty(t, b64)
	})
	t.Run("invalid_format_returns_error", func(t *testing.T) {
		_, _, _, err := svc.ParseKey("badkey")
		assert.Error(t, err)
	})
}

func TestAPIKeyService_ValidateFormat(t *testing.T) {
	svc := NewApiKeyService(nil)

	t.Run("valid_key_without_underscores_passes", func(t *testing.T) {
		// Construct a key that has no underscores in the base64 part
		b64 := strings.ReplaceAll("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklm", "_", "A")
		// Use exactly the expected length
		expectedLen := base64.RawURLEncoding.EncodedLen(KeyLength)
		padded := b64
		for len(padded) < expectedLen {
			padded += "A"
		}
		key := "pwk_v1_" + padded[:expectedLen]
		assert.True(t, svc.ValidateFormat(key))
	})
	t.Run("wrong_prefix_fails", func(t *testing.T) {
		assert.False(t, svc.ValidateFormat("bad_v1_abcdefg"))
	})
	t.Run("wrong_version_fails", func(t *testing.T) {
		assert.False(t, svc.ValidateFormat("pwk_v2_abcdefg"))
	})
	t.Run("short_random_part_fails", func(t *testing.T) {
		assert.False(t, svc.ValidateFormat("pwk_v1_short"))
	})
	t.Run("extra_underscores_in_b64_fail_parse", func(t *testing.T) {
		// base64 with underscore causes split to produce > 3 parts → invalid
		assert.False(t, svc.ValidateFormat("pwk_v1_abc_def"))
	})
}

func TestAPIKeyService_Create_NameRequired(t *testing.T) {
	svc := NewApiKeyService(&svcAPIKeyRepo{})
	ctx := utils.WithUserID(context.Background(), uuid.New())
	_, err := svc.Create(ctx, &model.APIKey{})
	assert.Error(t, err)
}

func TestAPIKeyService_Create_Success(t *testing.T) {
	repo := &svcAPIKeyRepo{}
	svc := NewApiKeyService(repo)
	ctx := utils.WithUserID(context.Background(), uuid.New())
	repo.On("Create", mock.Anything, (pgx.Tx)(nil), mock.Anything).Return(nil)
	fullKey, err := svc.Create(ctx, &model.APIKey{Name: "my-key"})
	assert.NoError(t, err)
	assert.NotEmpty(t, fullKey)
	assert.True(t, strings.HasPrefix(fullKey, "pwk_v1_"))
	repo.AssertExpectations(t)
}

func TestAPIKeyService_GetByKeyID(t *testing.T) {
	repo := &svcAPIKeyRepo{}
	svc := NewApiKeyService(repo)
	expected := &model.APIKey{Name: "key"}
	repo.On("GetByKeyID", mock.Anything, (pgx.Tx)(nil), "kid123").Return(expected, nil)
	result, err := svc.GetByKeyID(context.Background(), "kid123")
	assert.NoError(t, err)
	assert.Equal(t, expected, result)
	repo.AssertExpectations(t)
}

func TestAPIKeyService_UpdateLastUsed(t *testing.T) {
	repo := &svcAPIKeyRepo{}
	svc := NewApiKeyService(repo)
	id := uuid.New()
	repo.On("UpdateLastUsed", mock.Anything, (pgx.Tx)(nil), id).Return(nil)
	assert.NoError(t, svc.UpdateLastUsed(context.Background(), id))
	repo.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// Mock repos for PredictionService
// ─────────────────────────────────────────────────────────────────────────────

type svcPredictionRepo struct {
	mockBaseRepo
	mock.Mock
}

func (m *svcPredictionRepo) GetAll(ctx context.Context, budgetId uuid.UUID) ([]model.Prediction, error) {
	args := m.Called(ctx, budgetId)
	if v := args.Get(0); v != nil {
		return v.([]model.Prediction), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcPredictionRepo) GetByTxnId(ctx context.Context, budgetId uuid.UUID, txnId uuid.UUID) (*model.Prediction, error) {
	args := m.Called(ctx, budgetId, txnId)
	if v := args.Get(0); v != nil {
		return v.(*model.Prediction), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcPredictionRepo) GetByTxnIdTx(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, txnId uuid.UUID) (*model.Prediction, error) {
	args := m.Called(ctx, tx, budgetId, txnId)
	if v := args.Get(0); v != nil {
		return v.(*model.Prediction), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcPredictionRepo) Create(ctx context.Context, prediction model.Prediction) ([]model.Prediction, error) {
	args := m.Called(ctx, prediction)
	if v := args.Get(0); v != nil {
		return v.([]model.Prediction), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcPredictionRepo) Update(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, id uuid.UUID, prediction model.Prediction) error {
	return m.Called(ctx, tx, budgetId, id, prediction).Error(0)
}
func (m *svcPredictionRepo) DeleteByTxnId(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, txnId uuid.UUID) error {
	return m.Called(ctx, tx, budgetId, txnId).Error(0)
}

type svcCipherPredictionRepo struct {
	mockBaseRepo
	mock.Mock
}

func (m *svcCipherPredictionRepo) Create(ctx context.Context, tx pgx.Tx, p model.CipherPredictionRecord) (*model.CipherPredictionRecord, error) {
	args := m.Called(ctx, tx, p)
	if v := args.Get(0); v != nil {
		return v.(*model.CipherPredictionRecord), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcCipherPredictionRepo) GetByTransactionID(ctx context.Context, budgetID uuid.UUID, txnID uuid.UUID) (*model.CipherPredictionRecord, error) {
	args := m.Called(ctx, budgetID, txnID)
	if v := args.Get(0); v != nil {
		return v.(*model.CipherPredictionRecord), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcCipherPredictionRepo) MarkUserCorrected(ctx context.Context, tx pgx.Tx, budgetID uuid.UUID, txnID uuid.UUID, actualPayeeID *uuid.UUID, actualCategoryID *uuid.UUID) error {
	return m.Called(ctx, tx, budgetID, txnID, actualPayeeID, actualCategoryID).Error(0)
}

// ─────────────────────────────────────────────────────────────────────────────
// PredictionService tests
// ─────────────────────────────────────────────────────────────────────────────

func TestPredictionService_GetAll(t *testing.T) {
	budgetID := uuid.New()
	ctx := budgetCtxWith(budgetID)

	t.Run("returns_predictions", func(t *testing.T) {
		repo := &svcPredictionRepo{}
		cipherRepo := &svcCipherPredictionRepo{}
		repo.On("GetAll", mock.Anything, budgetID).Return([]model.Prediction{{ID: uuid.New()}}, nil)
		result, err := NewPredictionService(repo, cipherRepo).GetAll(ctx)
		assert.NoError(t, err)
		assert.Len(t, result, 1)
		repo.AssertExpectations(t)
	})
	t.Run("repo_error_propagates", func(t *testing.T) {
		repo := &svcPredictionRepo{}
		cipherRepo := &svcCipherPredictionRepo{}
		repo.On("GetAll", mock.Anything, budgetID).Return(nil, assert.AnError)
		result, err := NewPredictionService(repo, cipherRepo).GetAll(ctx)
		assert.Error(t, err)
		assert.Nil(t, result)
		repo.AssertExpectations(t)
	})
}

func TestPredictionService_Create(t *testing.T) {
	budgetID := uuid.New()
	ctx := budgetCtxWith(budgetID)

	t.Run("creates_prediction_with_budget_id", func(t *testing.T) {
		repo := &svcPredictionRepo{}
		cipherRepo := &svcCipherPredictionRepo{}
		created := []model.Prediction{{ID: uuid.New()}}
		repo.On("Create", mock.Anything, mock.MatchedBy(func(p model.Prediction) bool {
			return p.BudgetID == budgetID
		})).Return(created, nil)
		result, err := NewPredictionService(repo, cipherRepo).Create(ctx, model.Prediction{})
		assert.NoError(t, err)
		assert.Len(t, result, 1)
		repo.AssertExpectations(t)
	})
	t.Run("repo_error_propagates", func(t *testing.T) {
		repo := &svcPredictionRepo{}
		cipherRepo := &svcCipherPredictionRepo{}
		repo.On("Create", mock.Anything, mock.Anything).Return(nil, assert.AnError)
		result, err := NewPredictionService(repo, cipherRepo).Create(ctx, model.Prediction{})
		assert.Error(t, err)
		assert.Nil(t, result)
		repo.AssertExpectations(t)
	})
}

func TestPredictionService_Update(t *testing.T) {
	budgetID := uuid.New()
	ctx := budgetCtxWith(budgetID)
	// Update is a no-op (commented out body) — should return nil
	repo := &svcPredictionRepo{}
	cipherRepo := &svcCipherPredictionRepo{}
	err := NewPredictionService(repo, cipherRepo).Update(ctx, uuid.New(), model.Prediction{})
	assert.NoError(t, err)
}

func TestPredictionService_DeleteById(t *testing.T) {
	budgetID := uuid.New()
	txnID := uuid.New()
	ctx := budgetCtxWith(budgetID)

	t.Run("deletes_prediction", func(t *testing.T) {
		repo := &svcPredictionRepo{}
		cipherRepo := &svcCipherPredictionRepo{}
		repo.On("DeleteByTxnId", mock.Anything, (pgx.Tx)(nil), budgetID, txnID).Return(nil)
		assert.NoError(t, NewPredictionService(repo, cipherRepo).DeleteById(ctx, txnID))
		repo.AssertExpectations(t)
	})
	t.Run("repo_error_propagates", func(t *testing.T) {
		repo := &svcPredictionRepo{}
		cipherRepo := &svcCipherPredictionRepo{}
		repo.On("DeleteByTxnId", mock.Anything, (pgx.Tx)(nil), budgetID, txnID).Return(assert.AnError)
		assert.Error(t, NewPredictionService(repo, cipherRepo).DeleteById(ctx, txnID))
		repo.AssertExpectations(t)
	})
}

func TestPredictionService_CreateCipherPrediction(t *testing.T) {
	budgetID := uuid.New()
	ctx := budgetCtxWith(budgetID)
	created := &model.CipherPredictionRecord{BudgetID: budgetID}

	t.Run("creates_cipher_prediction_with_budget_id", func(t *testing.T) {
		repo := &svcPredictionRepo{}
		cipherRepo := &svcCipherPredictionRepo{}
		cipherRepo.On("Create", mock.Anything, (pgx.Tx)(nil), mock.MatchedBy(func(p model.CipherPredictionRecord) bool {
			return p.BudgetID == budgetID
		})).Return(created, nil)
		result, err := NewPredictionService(repo, cipherRepo).CreateCipherPrediction(ctx, model.CipherPredictionRecord{})
		assert.NoError(t, err)
		assert.Equal(t, budgetID, result.BudgetID)
		cipherRepo.AssertExpectations(t)
	})
	t.Run("repo_error_propagates", func(t *testing.T) {
		repo := &svcPredictionRepo{}
		cipherRepo := &svcCipherPredictionRepo{}
		cipherRepo.On("Create", mock.Anything, (pgx.Tx)(nil), mock.Anything).Return(nil, assert.AnError)
		result, err := NewPredictionService(repo, cipherRepo).CreateCipherPrediction(ctx, model.CipherPredictionRecord{})
		assert.Error(t, err)
		assert.Nil(t, result)
		cipherRepo.AssertExpectations(t)
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Mock repos for AuthService
// ─────────────────────────────────────────────────────────────────────────────

type svcAuthRepo struct {
	mockBaseRepo
	mock.Mock
}

func (m *svcAuthRepo) CreateUser(ctx context.Context, tx pgx.Tx) (*model.AuthUser, error) {
	args := m.Called(ctx, tx)
	if v := args.Get(0); v != nil {
		return v.(*model.AuthUser), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcAuthRepo) FindByID(ctx context.Context, id uuid.UUID) (*model.AuthUser, error) {
	args := m.Called(ctx, id)
	if v := args.Get(0); v != nil {
		return v.(*model.AuthUser), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcAuthRepo) GetUserWithProviders(ctx context.Context, id uuid.UUID) (*model.CurrentAuthUserResponse, error) {
	args := m.Called(ctx, id)
	if v := args.Get(0); v != nil {
		return v.(*model.CurrentAuthUserResponse), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcAuthRepo) UpdateTokenVersion(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *svcAuthRepo) GetTokenVersion(ctx context.Context, id uuid.UUID) (int, error) {
	args := m.Called(ctx, id)
	return args.Int(0), args.Error(1)
}
func (m *svcAuthRepo) SaveRefreshTokenHash(ctx context.Context, userID uuid.UUID, tokenHash string) error {
	return m.Called(ctx, userID, tokenHash).Error(0)
}
func (m *svcAuthRepo) GetRefreshTokenHash(ctx context.Context, userID uuid.UUID) (string, error) {
	args := m.Called(ctx, userID)
	return args.String(0), args.Error(1)
}
func (m *svcAuthRepo) ClearRefreshTokenHash(ctx context.Context, userID uuid.UUID) error {
	return m.Called(ctx, userID).Error(0)
}

// ─────────────────────────────────────────────────────────────────────────────
// AuthService tests (token generation/validation only — no OAuth/network)
// ─────────────────────────────────────────────────────────────────────────────

// newAuthSvcForTest builds an authService directly (same package) with a
// predictable JWT secret so we can generate and validate tokens in tests.
func newAuthSvcForTest(repo *svcAuthRepo) *authService {
	return &authService{
		config: authTestConfig(),
		repo:   repo,
	}
}

func authTestConfig() config.Config {
	return config.Config{JWTSecret: "test-jwt-secret"}
}

func TestAuthService_GenerateAccessToken(t *testing.T) {
	userID := uuid.New()
	svc := newAuthSvcForTest(&svcAuthRepo{})

	t.Run("generates_valid_token", func(t *testing.T) {
		tok, err := svc.GenerateAccessToken(context.Background(), userID, 1)
		assert.NoError(t, err)
		assert.NotEmpty(t, tok)
	})
	t.Run("token_has_correct_subject", func(t *testing.T) {
		tok, err := svc.GenerateAccessToken(context.Background(), userID, 2)
		assert.NoError(t, err)
		parsed, err := jwt.Parse(tok, func(t *jwt.Token) (any, error) {
			return []byte("test-jwt-secret"), nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		assert.NoError(t, err)
		sub, _ := parsed.Claims.GetSubject()
		assert.Equal(t, userID.String(), sub)
	})
}

func TestAuthService_GenerateRefreshToken(t *testing.T) {
	userID := uuid.New()
	svc := newAuthSvcForTest(&svcAuthRepo{})

	tok, err := svc.GenerateRefreshToken(context.Background(), userID)
	assert.NoError(t, err)
	assert.NotEmpty(t, tok)

	parsed, err := jwt.Parse(tok, func(t *jwt.Token) (any, error) {
		return []byte("test-jwt-secret"), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	assert.NoError(t, err)
	sub, _ := parsed.Claims.GetSubject()
	assert.Equal(t, userID.String(), sub)
}

func TestAuthService_ValidateToken(t *testing.T) {
	userID := uuid.New()
	svc := newAuthSvcForTest(&svcAuthRepo{})

	t.Run("valid_token_returns_token", func(t *testing.T) {
		tok, _ := svc.GenerateAccessToken(context.Background(), userID, 1)
		result, err := svc.ValidateToken(context.Background(), tok)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Valid)
	})
	t.Run("invalid_token_returns_error", func(t *testing.T) {
		result, err := svc.ValidateToken(context.Background(), "not.a.token")
		assert.Error(t, err)
		assert.Nil(t, result)
	})
	t.Run("expired_token_returns_error", func(t *testing.T) {
		// Build a token that expired in the past
		t2 := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": userID.String(),
			"exp": time.Now().Add(-time.Hour).Unix(),
			"iss": "pennywise",
			"iat": time.Now().Add(-2 * time.Hour).Unix(),
		})
		expiredTok, _ := t2.SignedString([]byte("test-jwt-secret"))
		result, err := svc.ValidateToken(context.Background(), expiredTok)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAuthService_GetUserById(t *testing.T) {
	userID := uuid.New()

	t.Run("returns_user", func(t *testing.T) {
		repo := &svcAuthRepo{}
		svc := newAuthSvcForTest(repo)
		user := &model.AuthUser{ID: userID}
		repo.On("FindByID", mock.Anything, userID).Return(user, nil)
		result, err := svc.GetUserById(context.Background(), userID)
		assert.NoError(t, err)
		assert.Equal(t, userID, result.ID)
		repo.AssertExpectations(t)
	})
	t.Run("repo_error_returns_wrapped_error", func(t *testing.T) {
		repo := &svcAuthRepo{}
		svc := newAuthSvcForTest(repo)
		repo.On("FindByID", mock.Anything, userID).Return(nil, assert.AnError)
		result, err := svc.GetUserById(context.Background(), userID)
		assert.Error(t, err)
		assert.Nil(t, result)
		repo.AssertExpectations(t)
	})
}

func TestAuthService_RefreshToken(t *testing.T) {
	userID := uuid.New()

	t.Run("success_returns_new_access_token", func(t *testing.T) {
		repo := &svcAuthRepo{}
		svc := newAuthSvcForTest(repo)
		refreshTok, _ := svc.GenerateRefreshToken(context.Background(), userID)
		user := &model.AuthUser{ID: userID, TokenVersion: 1}
		repo.On("FindByID", mock.Anything, userID).Return(user, nil)
		repo.On("GetRefreshTokenHash", mock.Anything, userID).Return(refreshTok, nil)
		result, err := svc.RefreshToken(context.Background(), refreshTok)
		assert.NoError(t, err)
		assert.NotEmpty(t, result.AccessToken)
		assert.Equal(t, 900, result.ExpiresIn)
		repo.AssertExpectations(t)
	})
	t.Run("invalid_token_returns_error", func(t *testing.T) {
		repo := &svcAuthRepo{}
		svc := newAuthSvcForTest(repo)
		result, err := svc.RefreshToken(context.Background(), "bad.token.here")
		assert.Error(t, err)
		assert.Nil(t, result)
	})
	t.Run("user_not_found_returns_error", func(t *testing.T) {
		repo := &svcAuthRepo{}
		svc := newAuthSvcForTest(repo)
		refreshTok, _ := svc.GenerateRefreshToken(context.Background(), userID)
		repo.On("FindByID", mock.Anything, userID).Return(nil, assert.AnError)
		result, err := svc.RefreshToken(context.Background(), refreshTok)
		assert.Error(t, err)
		assert.Nil(t, result)
		repo.AssertExpectations(t)
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Mock repos for MonthlyBudgetService
// ─────────────────────────────────────────────────────────────────────────────

type svcMonthlyBudgetRepo struct {
	mockBaseRepo
	mock.Mock
}

func (m *svcMonthlyBudgetRepo) GetPgxTx(ctx context.Context) (pgx.Tx, error) {
	args := m.Called(ctx)
	if v := args.Get(0); v != nil {
		return v.(pgx.Tx), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcMonthlyBudgetRepo) GetByCatIdAndMonth(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, categoryId uuid.UUID, month string) (*model.MonthlyBudget, error) {
	args := m.Called(ctx, tx, budgetId, categoryId, month)
	if v := args.Get(0); v != nil {
		return v.(*model.MonthlyBudget), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcMonthlyBudgetRepo) Create(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, monthlyBudget model.MonthlyBudget) error {
	return m.Called(ctx, tx, budgetId, monthlyBudget).Error(0)
}
func (m *svcMonthlyBudgetRepo) UpdateBudgetedByCatIdAndMonth(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, categoryId uuid.UUID, month string, newBudgeted float64) error {
	return m.Called(ctx, tx, budgetId, categoryId, month, newBudgeted).Error(0)
}
func (m *svcMonthlyBudgetRepo) UpdateCarryoverByCatIdAndMonth(ctx context.Context, tx pgx.Tx, budgetId uuid.UUID, categoryId uuid.UUID, month string, amount float64) error {
	return m.Called(ctx, tx, budgetId, categoryId, month, amount).Error(0)
}

// ─────────────────────────────────────────────────────────────────────────────
// MonthlyBudgetService.UpsertCarryover tests
// ─────────────────────────────────────────────────────────────────────────────

func TestMonthlyBudgetService_UpsertCarryover_CreatesWhenNotFound(t *testing.T) {
	budgetID := uuid.New()
	catID := uuid.New()
	repo := &svcMonthlyBudgetRepo{}
	svc := NewMonthlyBudgetService(repo)

	// Simulate: GetByCatIdAndMonth returns pgx.ErrNoRows → repo.Create is called
	repo.On("GetByCatIdAndMonth", mock.Anything, (pgx.Tx)(nil), budgetID, catID, "2025-01").Return(nil, pgx.ErrNoRows)
	repo.On("Create", mock.Anything, (pgx.Tx)(nil), budgetID, mock.MatchedBy(func(mb model.MonthlyBudget) bool {
		return mb.CategoryID == catID && mb.CarryoverBalance == 50.0
	})).Return(nil)

	err := svc.UpsertCarryover(context.Background(), nil, budgetID, catID, "2025-01", 50.0)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestMonthlyBudgetService_UpsertCarryover_UpdatesWhenExists(t *testing.T) {
	budgetID := uuid.New()
	catID := uuid.New()
	repo := &svcMonthlyBudgetRepo{}
	svc := NewMonthlyBudgetService(repo)
	existing := &model.MonthlyBudget{CategoryID: catID}

	repo.On("GetByCatIdAndMonth", mock.Anything, (pgx.Tx)(nil), budgetID, catID, "2025-01").Return(existing, nil)
	repo.On("UpdateCarryoverByCatIdAndMonth", mock.Anything, (pgx.Tx)(nil), budgetID, catID, "2025-01", -20.0).Return(nil)

	err := svc.UpsertCarryover(context.Background(), nil, budgetID, catID, "2025-01", -20.0)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestMonthlyBudgetService_UpsertCarryover_LookupErrorPropagates(t *testing.T) {
	budgetID := uuid.New()
	catID := uuid.New()
	repo := &svcMonthlyBudgetRepo{}
	svc := NewMonthlyBudgetService(repo)

	repo.On("GetByCatIdAndMonth", mock.Anything, (pgx.Tx)(nil), budgetID, catID, "2025-01").Return(nil, assert.AnError)

	err := svc.UpsertCarryover(context.Background(), nil, budgetID, catID, "2025-01", 10.0)
	assert.Error(t, err)
	repo.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// CategoryService.Search / GetById / UpdateMonthlyBudget tests
// ─────────────────────────────────────────────────────────────────────────────

func TestCategoryService_Search(t *testing.T) {
	budgetID := uuid.New()
	ctx := budgetCtxWith(budgetID)

	t.Run("returns_results", func(t *testing.T) {
		repo := &svcCategoryRepo{}
		repo.On("Search", mock.Anything, budgetID, "gro").Return([]model.Category{{Name: "Groceries"}}, nil)
		cats, err := NewCategoryService(repo, nil, nil).Search(ctx, "gro")
		assert.NoError(t, err)
		assert.Len(t, cats, 1)
		repo.AssertExpectations(t)
	})
	t.Run("repo_error_propagates", func(t *testing.T) {
		repo := &svcCategoryRepo{}
		repo.On("Search", mock.Anything, budgetID, "bad").Return(nil, assert.AnError)
		cats, err := NewCategoryService(repo, nil, nil).Search(ctx, "bad")
		assert.Error(t, err)
		assert.Nil(t, cats)
		repo.AssertExpectations(t)
	})
}

func TestCategoryService_GetById(t *testing.T) {
	budgetID := uuid.New()
	catID := uuid.New()
	ctx := budgetCtxWith(budgetID)

	t.Run("returns_category", func(t *testing.T) {
		repo := &svcCategoryRepo{}
		repo.On("GetById", mock.Anything, budgetID, catID).Return(&model.Category{ID: catID}, nil)
		cat, err := NewCategoryService(repo, nil, nil).GetById(ctx, catID)
		assert.NoError(t, err)
		assert.Equal(t, catID, cat.ID)
		repo.AssertExpectations(t)
	})
	t.Run("repo_error_propagates", func(t *testing.T) {
		repo := &svcCategoryRepo{}
		repo.On("GetById", mock.Anything, budgetID, catID).Return(nil, assert.AnError)
		cat, err := NewCategoryService(repo, nil, nil).GetById(ctx, catID)
		assert.Error(t, err)
		assert.Nil(t, cat)
		repo.AssertExpectations(t)
	})
}

func TestCategoryService_UpdateMonthlyBudget_note(t *testing.T) {
	// UpdateMonthlyBudget uses utils.WithTx which requires a real pgxpool.
	// The core sub-operation (UpsertCarryover) is already covered by its own tests.
	// Full integration test would require a live DB.
}

// ─────────────────────────────────────────────────────────────────────────────
// PayeeService.Search / UpdateRule tests
// ─────────────────────────────────────────────────────────────────────────────

func TestPayeeService_Search(t *testing.T) {
	budgetID := uuid.New()
	ctx := budgetCtxWith(budgetID)
	repo := &svcPayeeRepo{}
	ruleRepo := &svcPayeeRuleRepo{}

	t.Run("returns_payees", func(t *testing.T) {
		repo.On("Search", mock.Anything, budgetID, "ama").Return([]model.Payee{{Name: "Amazon"}}, nil)
		payees, err := NewPayeeService(repo, ruleRepo).Search(ctx, "ama")
		assert.NoError(t, err)
		assert.Len(t, payees, 1)
		repo.AssertExpectations(t)
	})
}

func TestPayeeService_UpdateRule(t *testing.T) {
	budgetID := uuid.New()
	payeeID := uuid.New()
	ruleID := uuid.New()
	ctx := budgetCtxWith(budgetID)
	repo := &svcPayeeRepo{}
	ruleRepo := &svcPayeeRuleRepo{}

	t.Run("updates_rule", func(t *testing.T) {
		ruleRepo.On("Update", mock.Anything, budgetID, ruleID, mock.MatchedBy(func(r model.PayeeRule) bool {
			return r.MatchType == "EXACT" && r.PayeeID == payeeID && r.BudgetID == budgetID
		})).Return(nil)
		err := NewPayeeService(repo, ruleRepo).UpdateRule(ctx, payeeID, ruleID, model.PayeeRule{MatchString: "test"})
		assert.NoError(t, err)
		ruleRepo.AssertExpectations(t)
	})
	t.Run("repo_error_propagates", func(t *testing.T) {
		ruleRepo2 := &svcPayeeRuleRepo{}
		ruleRepo2.On("Update", mock.Anything, budgetID, ruleID, mock.Anything).Return(assert.AnError)
		err := NewPayeeService(repo, ruleRepo2).UpdateRule(ctx, payeeID, ruleID, model.PayeeRule{MatchString: "test"})
		assert.Error(t, err)
		ruleRepo2.AssertExpectations(t)
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// BudgetService.UpdateById test
// ─────────────────────────────────────────────────────────────────────────────

func TestBudgetService_UpdateById(t *testing.T) {
	ctx := context.Background()
	budgetID := uuid.New()

	t.Run("success", func(t *testing.T) {
		repo := &svcBudgetRepo{}
		repo.On("UpdateById", mock.Anything, (pgx.Tx)(nil), budgetID, mock.Anything).Return(nil)
		svc := NewBudgetService(repo, &svcPayeeRepo{}, &svcCategoryRepo{}, &svcCategoryGroupRepo{})
		assert.NoError(t, svc.UpdateById(ctx, budgetID, model.Budget{Name: "Updated"}))
		repo.AssertExpectations(t)
	})
	t.Run("repo_error_propagates", func(t *testing.T) {
		repo := &svcBudgetRepo{}
		repo.On("UpdateById", mock.Anything, (pgx.Tx)(nil), budgetID, mock.Anything).Return(assert.AnError)
		svc := NewBudgetService(repo, &svcPayeeRepo{}, &svcCategoryRepo{}, &svcCategoryGroupRepo{})
		assert.Error(t, svc.UpdateById(ctx, budgetID, model.Budget{}))
		repo.AssertExpectations(t)
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// APIKeyService.GetByHash test
// ─────────────────────────────────────────────────────────────────────────────

func TestAPIKeyService_GetByHash(t *testing.T) {
	repo := &svcAPIKeyRepo{}
	svc := NewApiKeyService(repo)
	expected := &model.APIKey{Name: "hashed-key"}

	// Generate a real key so we can exercise the hash path
	fullKey, _, err := svc.Generate()
	assert.NoError(t, err)

	t.Run("success", func(t *testing.T) {
		repo.On("GetByHash", mock.Anything, (pgx.Tx)(nil), mock.AnythingOfType("string")).Return(expected, nil)
		result, err := svc.GetByHash(context.Background(), fullKey)
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
		repo.AssertExpectations(t)
	})
	t.Run("repo_error_propagates", func(t *testing.T) {
		repo2 := &svcAPIKeyRepo{}
		repo2.On("GetByHash", mock.Anything, (pgx.Tx)(nil), mock.AnythingOfType("string")).Return(nil, assert.AnError)
		svc2 := NewApiKeyService(repo2)
		result, err := svc2.GetByHash(context.Background(), fullKey)
		assert.Error(t, err)
		assert.Nil(t, result)
		repo2.AssertExpectations(t)
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// UserService.Search / Update tests
// ─────────────────────────────────────────────────────────────────────────────

type svcUserRepo struct {
	mockBaseRepo
	mock.Mock
}

func (m *svcUserRepo) Search(ctx context.Context, budgetId uuid.UUID, query string) ([]model.User, error) {
	args := m.Called(ctx, budgetId, query)
	if v := args.Get(0); v != nil {
		return v.([]model.User), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *svcUserRepo) Update(ctx context.Context, budgetId uuid.UUID, user model.User) (*model.User, error) {
	args := m.Called(ctx, budgetId, user)
	if v := args.Get(0); v != nil {
		return v.(*model.User), args.Error(1)
	}
	return nil, args.Error(1)
}

func TestUserService_Search(t *testing.T) {
	budgetID := uuid.New()
	ctx := budgetCtxWith(budgetID)

	t.Run("returns_users", func(t *testing.T) {
		repo := &svcUserRepo{}
		repo.On("Search", mock.Anything, budgetID, "alice").Return([]model.User{{ID: uuid.New()}}, nil)
		users, err := NewUserService(repo).Search(ctx, "alice")
		assert.NoError(t, err)
		assert.Len(t, users, 1)
		repo.AssertExpectations(t)
	})
	t.Run("repo_error_propagates", func(t *testing.T) {
		repo := &svcUserRepo{}
		repo.On("Search", mock.Anything, budgetID, "bad").Return(nil, assert.AnError)
		users, err := NewUserService(repo).Search(ctx, "bad")
		assert.Error(t, err)
		assert.Nil(t, users)
		repo.AssertExpectations(t)
	})
}

func TestUserService_Update(t *testing.T) {
	budgetID := uuid.New()
	ctx := budgetCtxWith(budgetID)
	updated := &model.User{ID: uuid.New()}

	t.Run("success", func(t *testing.T) {
		repo := &svcUserRepo{}
		repo.On("Update", mock.Anything, budgetID, mock.Anything).Return(updated, nil)
		result, err := NewUserService(repo).Update(ctx, model.User{})
		assert.NoError(t, err)
		assert.Equal(t, updated, result)
		repo.AssertExpectations(t)
	})
	t.Run("repo_error_propagates", func(t *testing.T) {
		repo := &svcUserRepo{}
		repo.On("Update", mock.Anything, budgetID, mock.Anything).Return(nil, assert.AnError)
		result, err := NewUserService(repo).Update(ctx, model.User{})
		assert.Error(t, err)
		assert.Nil(t, result)
		repo.AssertExpectations(t)
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// AuthService.GetAllGoogleUsers / GetCurrentUser / GetGoogleUserByEmail / UpdateGmailHistoryID
// ─────────────────────────────────────────────────────────────────────────────

type svcGoogleProviderRepo struct {
	mockBaseRepo
	mock.Mock
}

func (m *svcGoogleProviderRepo) GetAll(ctx context.Context, tx pgx.Tx) ([]model.GoogleProviderUser, error) {
	args := m.Called(ctx, tx)
	if v := args.Get(0); v != nil {
		return v.([]model.GoogleProviderUser), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcGoogleProviderRepo) Create(ctx context.Context, tx pgx.Tx, authUserID uuid.UUID, googleID string, oauthClientType model.GoogleOAuthClientType, name string, picture string, email string, refreshToken string, expiryAt *int64) (*model.UserWithCredentials, error) {
	args := m.Called(ctx, tx, authUserID, googleID, oauthClientType, name, picture, email, refreshToken, expiryAt)
	if v := args.Get(0); v != nil {
		return v.(*model.UserWithCredentials), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcGoogleProviderRepo) GetUserByGoogleID(ctx context.Context, googleID string) (*model.UserWithCredentials, error) {
	args := m.Called(ctx, googleID)
	if v := args.Get(0); v != nil {
		return v.(*model.UserWithCredentials), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcGoogleProviderRepo) GetUserByGoogleIDAndClientType(ctx context.Context, googleID string, oauthClientType model.GoogleOAuthClientType) (*model.UserWithCredentials, error) {
	args := m.Called(ctx, googleID, oauthClientType)
	if v := args.Get(0); v != nil {
		return v.(*model.UserWithCredentials), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcGoogleProviderRepo) GetUserByEmail(ctx context.Context, email string) (*model.GoogleUserInfo, error) {
	args := m.Called(ctx, email)
	if v := args.Get(0); v != nil {
		return v.(*model.GoogleUserInfo), args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *svcGoogleProviderRepo) UpdateUserByGoogleIDAndClientType(ctx context.Context, googleID string, oauthClientType model.GoogleOAuthClientType, data *model.GoogleProviderUser) error {
	return m.Called(ctx, googleID, oauthClientType, data).Error(0)
}
func (m *svcGoogleProviderRepo) UpdateHistoryID(ctx context.Context, googleID string, oauthClientType model.GoogleOAuthClientType, historyID uint64, expiryAt *int64) error {
	return m.Called(ctx, googleID, oauthClientType, historyID, expiryAt).Error(0)
}
func (m *svcGoogleProviderRepo) UpdateHistoryIDByEmail(ctx context.Context, email string, oauthClientType model.GoogleOAuthClientType, historyID uint64, expiryAt *int64) error {
	return m.Called(ctx, email, oauthClientType, historyID, expiryAt).Error(0)
}

func newAuthSvcWithGoogle(authRepo *svcAuthRepo, googleRepo *svcGoogleProviderRepo) *authService {
	return &authService{
		config:         authTestConfig(),
		repo:           authRepo,
		googleProvider: googleRepo,
	}
}

func TestAuthService_GetAllGoogleUsers(t *testing.T) {
	t.Run("returns_users", func(t *testing.T) {
		authRepo := &svcAuthRepo{}
		googleRepo := &svcGoogleProviderRepo{}
		googleRepo.On("GetAll", mock.Anything, (pgx.Tx)(nil)).Return([]model.GoogleProviderUser{{Email: "a@b.com"}}, nil)
		svc := newAuthSvcWithGoogle(authRepo, googleRepo)
		users, err := svc.GetAllGoogleUsers(context.Background())
		assert.NoError(t, err)
		assert.Len(t, users, 1)
		googleRepo.AssertExpectations(t)
	})
	t.Run("repo_error_propagates", func(t *testing.T) {
		authRepo := &svcAuthRepo{}
		googleRepo := &svcGoogleProviderRepo{}
		googleRepo.On("GetAll", mock.Anything, (pgx.Tx)(nil)).Return(nil, assert.AnError)
		svc := newAuthSvcWithGoogle(authRepo, googleRepo)
		users, err := svc.GetAllGoogleUsers(context.Background())
		assert.Error(t, err)
		assert.Nil(t, users)
		googleRepo.AssertExpectations(t)
	})
}

func TestAuthService_GetGoogleUserByEmail(t *testing.T) {
	t.Run("returns_user_info", func(t *testing.T) {
		authRepo := &svcAuthRepo{}
		googleRepo := &svcGoogleProviderRepo{}
		expected := &model.GoogleUserInfo{Email: "a@b.com"}
		googleRepo.On("GetUserByEmail", mock.Anything, "a@b.com").Return(expected, nil)
		svc := newAuthSvcWithGoogle(authRepo, googleRepo)
		result, err := svc.GetGoogleUserByEmail(context.Background(), "a@b.com")
		assert.NoError(t, err)
		assert.Equal(t, expected, result)
		googleRepo.AssertExpectations(t)
	})
	t.Run("repo_error_propagates", func(t *testing.T) {
		authRepo := &svcAuthRepo{}
		googleRepo := &svcGoogleProviderRepo{}
		googleRepo.On("GetUserByEmail", mock.Anything, "x@y.com").Return(nil, assert.AnError)
		svc := newAuthSvcWithGoogle(authRepo, googleRepo)
		result, err := svc.GetGoogleUserByEmail(context.Background(), "x@y.com")
		assert.Error(t, err)
		assert.Nil(t, result)
		googleRepo.AssertExpectations(t)
	})
}

func TestAuthService_UpdateGmailHistoryID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		authRepo := &svcAuthRepo{}
		googleRepo := &svcGoogleProviderRepo{}
		googleRepo.On("UpdateHistoryIDByEmail", mock.Anything, "a@b.com", model.GoogleOAuthClientTypeWeb, uint64(42), (*int64)(nil)).Return(nil)
		svc := newAuthSvcWithGoogle(authRepo, googleRepo)
		assert.NoError(t, svc.UpdateGmailHistoryID(context.Background(), "a@b.com", model.GoogleOAuthClientTypeWeb, 42, nil))
		googleRepo.AssertExpectations(t)
	})
}

func TestAuthService_GetCurrentUser(t *testing.T) {
	userID := uuid.New()
	ctx := context.Background()

	t.Run("repo_error_propagates", func(t *testing.T) {
		authRepo := &svcAuthRepo{}
		googleRepo := &svcGoogleProviderRepo{}
		authRepo.On("GetUserWithProviders", mock.Anything, userID).Return(nil, assert.AnError)
		svc := newAuthSvcWithGoogle(authRepo, googleRepo)
		result, err := svc.GetCurrentUser(ctx, userID)
		assert.Error(t, err)
		assert.Nil(t, result)
		authRepo.AssertExpectations(t)
	})
	t.Run("no_providers_returns_user", func(t *testing.T) {
		authRepo := &svcAuthRepo{}
		googleRepo := &svcGoogleProviderRepo{}
		user := &model.CurrentAuthUserResponse{ID: userID, Providers: nil}
		authRepo.On("GetUserWithProviders", mock.Anything, userID).Return(user, nil)
		svc := newAuthSvcWithGoogle(authRepo, googleRepo)
		result, err := svc.GetCurrentUser(ctx, userID)
		assert.NoError(t, err)
		assert.Equal(t, userID, result.ID)
		authRepo.AssertExpectations(t)
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// MonthlyBudgetService.ApplyCarryoverOps tests
// ─────────────────────────────────────────────────────────────────────────────

func TestMonthlyBudgetService_ApplyCarryoverOps_SameCategoryAndMonth_NoDiff(t *testing.T) {
	budgetID := uuid.New()
	catID := uuid.New()
	repo := &svcMonthlyBudgetRepo{}
	svc := NewMonthlyBudgetService(repo)

	diff := &txnDiff{
		oldCatId:    &catID,
		newCatId:    &catID,
		oldMonthKey: "2025-01",
		newMonthKey: "2025-01",
		oldAmount:   100.0,
		newAmount:   100.0, // diff == 0, no ops expected
	}
	cc := carryoverCase{sameCategory: true, sameMonth: true}
	err := svc.ApplyCarryoverOps(context.Background(), nil, budgetID, diff, cc)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// GetCurrentUser — additional branches
// ─────────────────────────────────────────────────────────────────────────────

func TestAuthService_GetCurrentUser_GoogleProviderNil(t *testing.T) {
	userID := uuid.New()
	ctx := context.Background()

	authRepo := &svcAuthRepo{}
	googleRepo := &svcGoogleProviderRepo{}
	// provider type is google but GoogleProvider is nil → skip enrichment (continue)
	user := &model.CurrentAuthUserResponse{
		ID: userID,
		Providers: []model.AuthProviderUserResponse{
			{ProviderType: model.GoogleAuthProviderType, ProviderID: "gid123"},
		},
	}
	authRepo.On("GetUserWithProviders", mock.Anything, userID).Return(user, nil)
	// GetUserByGoogleIDAndClientType returns UserWithCredentials with nil GoogleProvider
	googleRepo.On("GetUserByGoogleIDAndClientType", mock.Anything, "gid123", model.GoogleOAuthClientTypeWeb).Return(&model.UserWithCredentials{
		GoogleProvider: nil,
	}, nil)

	svc := newAuthSvcWithGoogle(authRepo, googleRepo)
	result, err := svc.GetCurrentUser(ctx, userID)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, userID, result.ID)
	authRepo.AssertExpectations(t)
	googleRepo.AssertExpectations(t)
}

func TestAuthService_GetCurrentUser_GoogleProviderEnrichesUser(t *testing.T) {
	userID := uuid.New()
	ctx := context.Background()

	authRepo := &svcAuthRepo{}
	googleRepo := &svcGoogleProviderRepo{}
	user := &model.CurrentAuthUserResponse{
		ID:      userID,
		Picture: "",
		Email:   "",
		Name:    "",
		Providers: []model.AuthProviderUserResponse{
			{ProviderType: model.GoogleAuthProviderType, ProviderID: "gid456"},
		},
	}
	authRepo.On("GetUserWithProviders", mock.Anything, userID).Return(user, nil)
	gp := &model.GoogleProviderUser{
		Name:    "Alice",
		Email:   "alice@example.com",
		Picture: "https://picture.url",
	}
	googleRepo.On("GetUserByGoogleIDAndClientType", mock.Anything, "gid456", model.GoogleOAuthClientTypeWeb).Return(&model.UserWithCredentials{
		GoogleProvider: gp,
	}, nil)

	svc := newAuthSvcWithGoogle(authRepo, googleRepo)
	result, err := svc.GetCurrentUser(ctx, userID)
	assert.NoError(t, err)
	assert.Equal(t, "alice@example.com", result.Email)
	assert.Equal(t, "Alice", result.Name)
	assert.Equal(t, "https://picture.url", result.Picture)
	authRepo.AssertExpectations(t)
	googleRepo.AssertExpectations(t)
}

func TestAuthService_GetCurrentUser_GoogleLookupError(t *testing.T) {
	userID := uuid.New()
	ctx := context.Background()

	authRepo := &svcAuthRepo{}
	googleRepo := &svcGoogleProviderRepo{}
	user := &model.CurrentAuthUserResponse{
		ID: userID,
		Providers: []model.AuthProviderUserResponse{
			{ProviderType: model.GoogleAuthProviderType, ProviderID: "gid789"},
		},
	}
	authRepo.On("GetUserWithProviders", mock.Anything, userID).Return(user, nil)
	googleRepo.On("GetUserByGoogleIDAndClientType", mock.Anything, "gid789", model.GoogleOAuthClientTypeWeb).Return(nil, assert.AnError)

	svc := newAuthSvcWithGoogle(authRepo, googleRepo)
	result, err := svc.GetCurrentUser(ctx, userID)
	assert.Error(t, err)
	assert.Nil(t, result)
	authRepo.AssertExpectations(t)
	googleRepo.AssertExpectations(t)
}

func TestAuthService_GetCurrentUser_UnknownProviderType(t *testing.T) {
	userID := uuid.New()
	ctx := context.Background()

	authRepo := &svcAuthRepo{}
	googleRepo := &svcGoogleProviderRepo{}
	// unknown provider type falls through the switch without enrichment
	user := &model.CurrentAuthUserResponse{
		ID: userID,
		Providers: []model.AuthProviderUserResponse{
			{ProviderType: "unknown", ProviderID: "xyz"},
		},
	}
	authRepo.On("GetUserWithProviders", mock.Anything, userID).Return(user, nil)

	svc := newAuthSvcWithGoogle(authRepo, googleRepo)
	result, err := svc.GetCurrentUser(ctx, userID)
	assert.NoError(t, err)
	assert.Equal(t, userID, result.ID)
	authRepo.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// RefreshToken — GetRefreshTokenHash error and nil user branches
// ─────────────────────────────────────────────────────────────────────────────

func TestAuthService_RefreshToken_GetRefreshTokenHashError(t *testing.T) {
	userID := uuid.New()
	repo := &svcAuthRepo{}
	svc := newAuthSvcForTest(repo)
	refreshTok, _ := svc.GenerateRefreshToken(context.Background(), userID)
	user := &model.AuthUser{ID: userID, TokenVersion: 1}
	repo.On("FindByID", mock.Anything, userID).Return(user, nil)
	repo.On("GetRefreshTokenHash", mock.Anything, userID).Return("", assert.AnError)

	result, err := svc.RefreshToken(context.Background(), refreshTok)
	assert.Error(t, err)
	assert.Nil(t, result)
	repo.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// CategoryGroupService.GetAll — month != "" branch
// ─────────────────────────────────────────────────────────────────────────────

func TestCategoryGroupService_GetAll_WithMonth(t *testing.T) {
	budgetID := uuid.New()
	ctx := budgetCtxWith(budgetID)

	repo := &svcCategoryGroupRepo{}
	groups := []model.CategoryGroup{
		{
			ID:   uuid.New(),
			Name: "Housing",
		},
	}
	repo.On("GetAll", mock.Anything, budgetID).Return(groups, nil)

	result, err := NewCategoryGroupService(repo).GetAll(ctx, "2025-01")
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	repo.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// transactionMappingChanged — all branches
// ─────────────────────────────────────────────────────────────────────────────

func TestTransactionMappingChanged(t *testing.T) {
	pid1 := uuid.New()
	pid2 := uuid.New()
	cid1 := uuid.New()
	cid2 := uuid.New()

	t.Run("both_nil_returns_false", func(t *testing.T) {
		assert.False(t, transactionMappingChanged(nil, nil))
	})
	t.Run("old_nil_returns_false", func(t *testing.T) {
		assert.False(t, transactionMappingChanged(nil, &model.Transaction{}))
	})
	t.Run("new_nil_returns_false", func(t *testing.T) {
		assert.False(t, transactionMappingChanged(&model.Transaction{}, nil))
	})
	t.Run("same_payee_same_category_returns_false", func(t *testing.T) {
		old := &model.Transaction{PayeeID: &pid1, CategoryID: &cid1}
		new := &model.Transaction{PayeeID: &pid1, CategoryID: &cid1}
		assert.False(t, transactionMappingChanged(old, new))
	})
	t.Run("different_payee_returns_true", func(t *testing.T) {
		old := &model.Transaction{PayeeID: &pid1, CategoryID: &cid1}
		new := &model.Transaction{PayeeID: &pid2, CategoryID: &cid1}
		assert.True(t, transactionMappingChanged(old, new))
	})
	t.Run("different_category_returns_true", func(t *testing.T) {
		old := &model.Transaction{PayeeID: &pid1, CategoryID: &cid1}
		new := &model.Transaction{PayeeID: &pid1, CategoryID: &cid2}
		assert.True(t, transactionMappingChanged(old, new))
	})
	t.Run("old_payee_nil_new_payee_set_returns_true", func(t *testing.T) {
		old := &model.Transaction{PayeeID: nil, CategoryID: &cid1}
		new := &model.Transaction{PayeeID: &pid1, CategoryID: &cid1}
		assert.True(t, transactionMappingChanged(old, new))
	})
	t.Run("both_payee_nil_same_category_returns_false", func(t *testing.T) {
		old := &model.Transaction{PayeeID: nil, CategoryID: &cid1}
		new := &model.Transaction{PayeeID: nil, CategoryID: &cid1}
		assert.False(t, transactionMappingChanged(old, new))
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// learnTransactionMappingAsync — early-return guard branches
// ─────────────────────────────────────────────────────────────────────────────

func TestLearnTransactionMappingAsync_NilCipherClient(t *testing.T) {
	svc := &transactionService{
		cipherClient:     nil,
		payeeRuleRepo:    nil,
		txnEmbeddingRepo: nil,
	}
	// Should return immediately without panic (cipherClient == nil)
	svc.learnTransactionMappingAsync(context.Background(), uuid.New(), model.Transaction{ID: uuid.New()})
}

func TestLearnTransactionMappingAsync_NilPayeeID(t *testing.T) {
	// Use a non-nil cipherClient to pass the first guard, but payeeID is nil
	mockCC := &mockCipherClient{}
	svc := &transactionService{
		cipherClient:     mockCC,
		payeeRuleRepo:    &mockPayeeRuleRepo{},
		txnEmbeddingRepo: &mockTransactionEmbeddingRepo{},
	}
	txn := model.Transaction{ID: uuid.New(), PayeeID: nil}
	svc.learnTransactionMappingAsync(context.Background(), uuid.New(), txn)
	// Should return before goroutine launch (no panic, no calls)
}

func TestLearnTransactionMappingAsync_NilRawBankText(t *testing.T) {
	mockCC2 := &mockCipherClient{}
	pid := uuid.New()
	cid := uuid.New()
	svc := &transactionService{
		cipherClient:     mockCC2,
		payeeRuleRepo:    &mockPayeeRuleRepo{},
		txnEmbeddingRepo: &mockTransactionEmbeddingRepo{},
	}
	txn := model.Transaction{ID: uuid.New(), PayeeID: &pid, CategoryID: &cid, RawBankText: nil}
	svc.learnTransactionMappingAsync(context.Background(), uuid.New(), txn)
}

func TestLearnTransactionMappingAsync_BlankRawBankText(t *testing.T) {
	mockCC3 := &mockCipherClient{}
	pid := uuid.New()
	cid := uuid.New()
	blank := "   "
	svc := &transactionService{
		cipherClient:     mockCC3,
		payeeRuleRepo:    &mockPayeeRuleRepo{},
		txnEmbeddingRepo: &mockTransactionEmbeddingRepo{},
	}
	txn := model.Transaction{ID: uuid.New(), PayeeID: &pid, CategoryID: &cid, RawBankText: &blank}
	svc.learnTransactionMappingAsync(context.Background(), uuid.New(), txn)
}

// ─────────────────────────────────────────────────────────────────────────────
// ApplyCarryoverOps — sameCategory && sameMonth but newCatId == nil
// ─────────────────────────────────────────────────────────────────────────────

func TestMonthlyBudgetService_ApplyCarryoverOps_SameCategoryAndMonth_NilNewCatId(t *testing.T) {
	budgetID := uuid.New()
	oldCatID := uuid.New()
	repo := &svcMonthlyBudgetRepo{}
	svc := NewMonthlyBudgetService(repo)

	diff := &txnDiff{
		oldCatId:    &oldCatID,
		newCatId:    nil, // triggers early return in getCarryoverOps
		oldMonthKey: "2025-01",
		newMonthKey: "2025-01",
		oldAmount:   100.0,
		newAmount:   200.0,
	}
	cc := carryoverCase{sameCategory: true, sameMonth: true}
	err := svc.ApplyCarryoverOps(context.Background(), nil, budgetID, diff, cc)
	assert.NoError(t, err)
	repo.AssertExpectations(t) // no calls expected
}

func TestMonthlyBudgetService_ApplyCarryoverOps_SameCategoryAndMonth_AmountChanged(t *testing.T) {
	budgetID := uuid.New()
	catID := uuid.New()
	repo := &svcMonthlyBudgetRepo{}
	svc := NewMonthlyBudgetService(repo)

	diff := &txnDiff{
		oldCatId:    &catID,
		newCatId:    &catID,
		oldMonthKey: "2025-01",
		newMonthKey: "2025-01",
		oldAmount:   100.0,
		newAmount:   150.0, // diff == 50
	}
	cc := carryoverCase{sameCategory: true, sameMonth: true}
	existing := &model.MonthlyBudget{CategoryID: catID}
	repo.On("GetByCatIdAndMonth", mock.Anything, (pgx.Tx)(nil), budgetID, catID, "2025-01").Return(existing, nil)
	repo.On("UpdateCarryoverByCatIdAndMonth", mock.Anything, (pgx.Tx)(nil), budgetID, catID, "2025-01", 50.0).Return(nil)

	err := svc.ApplyCarryoverOps(context.Background(), nil, budgetID, diff, cc)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestMonthlyBudgetService_ApplyCarryoverOps_DifferentCategory(t *testing.T) {
	budgetID := uuid.New()
	oldCatID := uuid.New()
	newCatID := uuid.New()
	repo := &svcMonthlyBudgetRepo{}
	svc := NewMonthlyBudgetService(repo)

	diff := &txnDiff{
		oldCatId:    &oldCatID,
		newCatId:    &newCatID,
		oldMonthKey: "2025-01",
		newMonthKey: "2025-01",
		oldAmount:   100.0,
		newAmount:   200.0,
	}
	cc := carryoverCase{sameCategory: false, sameMonth: true}

	existingOld := &model.MonthlyBudget{CategoryID: oldCatID}
	existingNew := &model.MonthlyBudget{CategoryID: newCatID}
	// op1: subtract oldAmount from old category
	repo.On("GetByCatIdAndMonth", mock.Anything, (pgx.Tx)(nil), budgetID, oldCatID, "2025-01").Return(existingOld, nil)
	repo.On("UpdateCarryoverByCatIdAndMonth", mock.Anything, (pgx.Tx)(nil), budgetID, oldCatID, "2025-01", -100.0).Return(nil)
	// op2: add newAmount to new category
	repo.On("GetByCatIdAndMonth", mock.Anything, (pgx.Tx)(nil), budgetID, newCatID, "2025-01").Return(existingNew, nil)
	repo.On("UpdateCarryoverByCatIdAndMonth", mock.Anything, (pgx.Tx)(nil), budgetID, newCatID, "2025-01", 200.0).Return(nil)

	err := svc.ApplyCarryoverOps(context.Background(), nil, budgetID, diff, cc)
	assert.NoError(t, err)
	repo.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// APIKeyService — ValidateFormat missing branches
// ─────────────────────────────────────────────────────────────────────────────

func TestAPIKeyService_ValidateFormat_WrongPrefix(t *testing.T) {
	svc := &apiKeyService{prefix: KeyPrefix, version: KeyVersion, repo: &svcAPIKeyRepo{}}
	randomPart := strings.Repeat("a", base64.RawURLEncoding.EncodedLen(KeyLength))
	key := "bad_" + KeyVersion + "_" + randomPart
	assert.False(t, svc.ValidateFormat(key))
}

func TestAPIKeyService_ValidateFormat_WrongVersion(t *testing.T) {
	svc := &apiKeyService{prefix: KeyPrefix, version: KeyVersion, repo: &svcAPIKeyRepo{}}
	randomPart := strings.Repeat("a", base64.RawURLEncoding.EncodedLen(KeyLength))
	key := KeyPrefix + "_v99_" + randomPart
	assert.False(t, svc.ValidateFormat(key))
}

func TestAPIKeyService_ValidateFormat_WrongRandomLength(t *testing.T) {
	svc := &apiKeyService{prefix: KeyPrefix, version: KeyVersion, repo: &svcAPIKeyRepo{}}
	key := KeyPrefix + "_" + KeyVersion + "_tooshort"
	assert.False(t, svc.ValidateFormat(key))
}

func TestAPIKeyService_ValidateFormat_ParseFails(t *testing.T) {
	svc := &apiKeyService{prefix: KeyPrefix, version: KeyVersion, repo: &svcAPIKeyRepo{}}
	key := "onlyone_segment"
	assert.False(t, svc.ValidateFormat(key))
}

// ─────────────────────────────────────────────────────────────────────────────
// APIKeyService — Create missing branches
// ─────────────────────────────────────────────────────────────────────────────

func TestAPIKeyService_Create_EmptyName(t *testing.T) {
	repo := &svcAPIKeyRepo{}
	svc := NewApiKeyService(repo)
	ctx := utils.WithUserID(context.Background(), uuid.New())
	_, err := svc.Create(ctx, &model.APIKey{Name: ""})
	assert.Error(t, err)
}

func TestAPIKeyService_Create_RepoError(t *testing.T) {
	repo := &svcAPIKeyRepo{}
	repo.On("Create", mock.Anything, (pgx.Tx)(nil), mock.Anything).Return(assert.AnError)
	svc := NewApiKeyService(repo)
	ctx := utils.WithUserID(context.Background(), uuid.New())
	_, err := svc.Create(ctx, &model.APIKey{Name: "mykey"})
	assert.Error(t, err)
	repo.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// MonthlyBudgetService — UpsertCarryover: UpdateCarryover returns error
// ─────────────────────────────────────────────────────────────────────────────

func TestMonthlyBudgetService_UpsertCarryover_UpdateError(t *testing.T) {
	budgetID := uuid.New()
	catID := uuid.New()
	repo := &svcMonthlyBudgetRepo{}
	svc := NewMonthlyBudgetService(repo)
	existing := &model.MonthlyBudget{CategoryID: catID}
	repo.On("GetByCatIdAndMonth", mock.Anything, (pgx.Tx)(nil), budgetID, catID, "2025-01").Return(existing, nil)
	repo.On("UpdateCarryoverByCatIdAndMonth", mock.Anything, (pgx.Tx)(nil), budgetID, catID, "2025-01", 50.0).Return(assert.AnError)
	err := svc.UpsertCarryover(context.Background(), nil, budgetID, catID, "2025-01", 50.0)
	assert.Error(t, err)
	repo.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// MonthlyBudgetService — ApplyCarryoverOps: UpsertCarryover propagates error
// ─────────────────────────────────────────────────────────────────────────────

func TestMonthlyBudgetService_ApplyCarryoverOps_UpsertError(t *testing.T) {
	budgetID := uuid.New()
	catID := uuid.New()
	repo := &svcMonthlyBudgetRepo{}
	svc := NewMonthlyBudgetService(repo)
	diff := &txnDiff{
		oldCatId:    &catID,
		newCatId:    &catID,
		oldMonthKey: "2025-01",
		newMonthKey: "2025-01",
		oldAmount:   100.0,
		newAmount:   150.0,
	}
	cc := carryoverCase{sameCategory: true, sameMonth: true}
	repo.On("GetByCatIdAndMonth", mock.Anything, (pgx.Tx)(nil), budgetID, catID, "2025-01").Return((*model.MonthlyBudget)(nil), assert.AnError)
	err := svc.ApplyCarryoverOps(context.Background(), nil, budgetID, diff, cc)
	assert.Error(t, err)
	repo.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// AuthService — RefreshToken: nil user branch
// ─────────────────────────────────────────────────────────────────────────────

func TestAuthService_RefreshToken_NilUser(t *testing.T) {
	repo := &svcAuthRepo{}
	svc := newAuthSvcForTest(repo)
	userID := uuid.New()
	refreshTok, err := svc.GenerateRefreshToken(context.Background(), userID)
	assert.NoError(t, err)

	repo.On("FindByID", mock.Anything, userID).Return((*model.AuthUser)(nil), nil)

	result, err := svc.RefreshToken(context.Background(), refreshTok)
	assert.Error(t, err)
	assert.Nil(t, result)
	repo.AssertExpectations(t)
}

// ─────────────────────────────────────────────────────────────────────────────
// EmbeddingService — constructor + Get + Create (all stubs)
// ─────────────────────────────────────────────────────────────────────────────

func TestEmbeddingService_GetAndCreate(t *testing.T) {
	svc := NewEmbeddingService(nil)
	assert.NotNil(t, svc)
	docs, err := svc.Get(context.Background(), "journal_bullet", "query", 5)
	assert.NoError(t, err)
	assert.Nil(t, docs)
	err = svc.Create(context.Background(), model.Embedding{})
	assert.NoError(t, err)
}

// ─────────────────────────────────────────────────────────────────────────────
// CipherClient — NewCipherClient
// ─────────────────────────────────────────────────────────────────────────────

func TestNewCipherClient(t *testing.T) {
	cc := NewCipherClient(nil)
	assert.NotNil(t, cc)
}

// ─────────────────────────────────────────────────────────────────────────────
// transactionService — validateTransactionPayload branches
// ─────────────────────────────────────────────────────────────────────────────

func TestValidateTransactionPayload_BudgetMismatch(t *testing.T) {
	svc := &transactionService{}
	txn := model.Transaction{
		BudgetID:  uuid.New(),
		AccountID: uuidPtr(uuid.New()),
		PayeeID:   uuidPtr(uuid.New()),
		Date:      "2025-01-15",
	}
	err := svc.validateTransactionPayload(txn, uuid.New())
	assert.Error(t, err)
}

func TestValidateTransactionPayload_AccountIDNil(t *testing.T) {
	svc := &transactionService{}
	budgetID := uuid.New()
	txn := model.Transaction{
		BudgetID:  budgetID,
		AccountID: nil,
		PayeeID:   uuidPtr(uuid.New()),
		Date:      "2025-01-15",
	}
	err := svc.validateTransactionPayload(txn, budgetID)
	assert.Error(t, err)
}

func TestValidateTransactionPayload_PayeeIDNil(t *testing.T) {
	svc := &transactionService{}
	budgetID := uuid.New()
	txn := model.Transaction{
		BudgetID:  budgetID,
		AccountID: uuidPtr(uuid.New()),
		PayeeID:   nil,
		Date:      "2025-01-15",
	}
	err := svc.validateTransactionPayload(txn, budgetID)
	assert.Error(t, err)
}

// uuidPtr is a small helper to get a *uuid.UUID pointer
func uuidPtr(id uuid.UUID) *uuid.UUID { return &id }

// ─────────────────────────────────────────────────────────────────────────────
// transactionService — loadDependencies: transfer account error
// ─────────────────────────────────────────────────────────────────────────────

func TestLoadDependencies_TransferAccountError(t *testing.T) {
	budgetID := uuid.New()
	accountID := uuid.New()
	payeeID := uuid.New()
	transferAccountID := uuid.New()

	txnRepo := &mockTransactionRepo{}
	budgetRepo := &mockBudgetRepo{}
	accountRepo := &mockAccountRepo{}
	payeeRepo := &mockPayeesRepo{}

	ctx := utils.WithBudgetID(context.Background(), budgetID)

	budget := &model.Budget{ID: budgetID}
	account := &model.Account{ID: accountID}
	payee := &model.Payee{ID: payeeID, TransferAccountID: &transferAccountID}

	budgetRepo.On("GetById", mock.Anything, (pgx.Tx)(nil), budgetID).Return(budget, nil)
	accountRepo.On("GetById", mock.Anything, (pgx.Tx)(nil), budgetID, accountID).Return(account, nil)
	payeeRepo.On("GetByIdTx", mock.Anything, (pgx.Tx)(nil), budgetID, payeeID).Return(payee, nil)
	accountRepo.On("GetById", mock.Anything, (pgx.Tx)(nil), budgetID, transferAccountID).Return((*model.Account)(nil), assert.AnError)

	svc := &transactionService{
		repo:        txnRepo,
		budgetRepo:  budgetRepo,
		accountRepo: accountRepo,
		payeeRepo:   payeeRepo,
	}
	txn := model.Transaction{
		BudgetID:  budgetID,
		AccountID: &accountID,
		PayeeID:   &payeeID,
	}
	_, _, _, _, err := svc.loadDependencies(ctx, nil, budgetID, txn)
	assert.Error(t, err)
}

// ─────────────────────────────────────────────────────────────────────────────
// transactionService — learnTransactionMappingAsync: payeeRuleRepo nil guard
// ─────────────────────────────────────────────────────────────────────────────

func TestLearnTransactionMappingAsync_PayeeRuleRepoNil(t *testing.T) {
	svc := &transactionService{
		cipherClient:     &mockCipherClient{},
		payeeRuleRepo:    nil,
		txnEmbeddingRepo: &mockTransactionEmbeddingRepo{},
	}
	svc.learnTransactionMappingAsync(context.Background(), uuid.New(), model.Transaction{})
}

// ─────────────────────────────────────────────────────────────────────────────
// transactionService — UpdateStatus branches (no real DB needed)
// ─────────────────────────────────────────────────────────────────────────────

func TestUpdateStatus_TxnNotFound(t *testing.T) {
	budgetID := uuid.New()
	txnID := uuid.New()
	txnRepo := &mockTransactionRepo{}
	cipherPredRepo := &mockCipherPredictionRepo{}
	txnRepo.On("GetById", mock.Anything, budgetID, txnID).Return((*model.Transaction)(nil), nil)
	svc := &transactionService{
		repo:                 txnRepo,
		cipherPredictionRepo: cipherPredRepo,
	}
	ctx := utils.WithBudgetID(context.Background(), budgetID)
	err := svc.UpdateStatus(ctx, txnID, model.TransactionStatusApproved)
	assert.Error(t, err)
}

func TestUpdateStatus_NilCipherPredRepo(t *testing.T) {
	budgetID := uuid.New()
	txnID := uuid.New()
	txnRepo := &mockTransactionRepo{}
	foundTxn := &model.Transaction{ID: txnID}
	txnRepo.On("GetById", mock.Anything, budgetID, txnID).Return(foundTxn, nil)
	svc := &transactionService{
		repo:                 txnRepo,
		cipherPredictionRepo: nil,
	}
	ctx := utils.WithBudgetID(context.Background(), budgetID)
	err := svc.UpdateStatus(ctx, txnID, model.TransactionStatusApproved)
	assert.Error(t, err)
}

func TestUpdateStatus_CipherPredRepoError(t *testing.T) {
	budgetID := uuid.New()
	txnID := uuid.New()
	txnRepo := &mockTransactionRepo{}
	cipherPredRepo := &mockCipherPredictionRepo{}
	foundTxn := &model.Transaction{ID: txnID}
	txnRepo.On("GetById", mock.Anything, budgetID, txnID).Return(foundTxn, nil)
	cipherPredRepo.On("GetByTransactionID", mock.Anything, budgetID, txnID).Return((*model.CipherPredictionRecord)(nil), assert.AnError)
	svc := &transactionService{
		repo:                 txnRepo,
		cipherPredictionRepo: cipherPredRepo,
	}
	ctx := utils.WithBudgetID(context.Background(), budgetID)
	err := svc.UpdateStatus(ctx, txnID, model.TransactionStatusApproved)
	assert.Error(t, err)
}

func TestUpdateStatus_PayeeIDNil(t *testing.T) {
	budgetID := uuid.New()
	txnID := uuid.New()
	txnRepo := &mockTransactionRepo{}
	cipherPredRepo := &mockCipherPredictionRepo{}
	catID := uuid.New()
	rawText := "BANK TEXT"
	foundTxn := &model.Transaction{ID: txnID, PayeeID: nil, CategoryID: &catID, RawBankText: &rawText}
	cipherPred := &model.CipherPredictionRecord{}
	txnRepo.On("GetById", mock.Anything, budgetID, txnID).Return(foundTxn, nil)
	cipherPredRepo.On("GetByTransactionID", mock.Anything, budgetID, txnID).Return(cipherPred, nil)
	svc := &transactionService{
		repo:                 txnRepo,
		cipherPredictionRepo: cipherPredRepo,
	}
	ctx := utils.WithBudgetID(context.Background(), budgetID)
	err := svc.UpdateStatus(ctx, txnID, model.TransactionStatusApproved)
	assert.Error(t, err)
}

func TestUpdateStatus_CategoryIDNil(t *testing.T) {
	budgetID := uuid.New()
	txnID := uuid.New()
	txnRepo := &mockTransactionRepo{}
	cipherPredRepo := &mockCipherPredictionRepo{}
	payeeID := uuid.New()
	rawText := "BANK TEXT"
	foundTxn := &model.Transaction{ID: txnID, PayeeID: &payeeID, CategoryID: nil, RawBankText: &rawText}
	cipherPred := &model.CipherPredictionRecord{}
	txnRepo.On("GetById", mock.Anything, budgetID, txnID).Return(foundTxn, nil)
	cipherPredRepo.On("GetByTransactionID", mock.Anything, budgetID, txnID).Return(cipherPred, nil)
	svc := &transactionService{
		repo:                 txnRepo,
		cipherPredictionRepo: cipherPredRepo,
	}
	ctx := utils.WithBudgetID(context.Background(), budgetID)
	err := svc.UpdateStatus(ctx, txnID, model.TransactionStatusApproved)
	assert.Error(t, err)
}

func TestUpdateStatus_RawBankTextNil(t *testing.T) {
	budgetID := uuid.New()
	txnID := uuid.New()
	txnRepo := &mockTransactionRepo{}
	cipherPredRepo := &mockCipherPredictionRepo{}
	payeeID := uuid.New()
	catID := uuid.New()
	foundTxn := &model.Transaction{ID: txnID, PayeeID: &payeeID, CategoryID: &catID, RawBankText: nil}
	cipherPred := &model.CipherPredictionRecord{}
	txnRepo.On("GetById", mock.Anything, budgetID, txnID).Return(foundTxn, nil)
	cipherPredRepo.On("GetByTransactionID", mock.Anything, budgetID, txnID).Return(cipherPred, nil)
	svc := &transactionService{
		repo:                 txnRepo,
		cipherPredictionRepo: cipherPredRepo,
	}
	ctx := utils.WithBudgetID(context.Background(), budgetID)
	err := svc.UpdateStatus(ctx, txnID, model.TransactionStatusApproved)
	assert.Error(t, err)
}

// ─────────────────────────────────────────────────────────────────────────────
// transactionService — Update: validateTransactionPayload fails
// ─────────────────────────────────────────────────────────────────────────────

func TestTransactionService_Update_ValidationFails(t *testing.T) {
	budgetID := uuid.New()
	txnID := uuid.New()
	txnRepo := &mockTransactionRepo{}
	svc := &transactionService{repo: txnRepo}
	wrongBudgetID := uuid.New()
	txn := model.Transaction{BudgetID: wrongBudgetID, Date: "2025-01-15"}
	ctx := utils.WithBudgetID(context.Background(), budgetID)
	err := svc.Update(ctx, txnID, txn)
	assert.Error(t, err)
}
