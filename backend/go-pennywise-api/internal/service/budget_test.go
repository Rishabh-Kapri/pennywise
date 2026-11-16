package service

import (
	"context"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memBudgets struct {
	repository.BudgetRepository
	existing []model.Budget
	saved    model.Budget
}

func (m *memBudgets) GetAll(context.Context, uuid.UUID) ([]model.Budget, error) {
	return m.existing, nil
}
func (m *memBudgets) Create(_ context.Context, name string, _ uuid.UUID) (*model.Budget, error) {
	return &model.Budget{ID: uuid.New(), Name: name}, nil
}
func (m *memBudgets) UpdateById(_ context.Context, _ uuid.UUID, b model.Budget) error {
	m.saved = b
	return nil
}

type memGroups struct {
	repository.CategoryGroupRepository
	made []model.CategoryGroup
}

func (m *memGroups) Create(_ context.Context, g model.CategoryGroup) (*model.CategoryGroup, error) {
	g.ID = uuid.New()
	m.made = append(m.made, g)
	return &g, nil
}

type memCategories struct {
	repository.CategoryRepository
	made []model.Category
}

func (m *memCategories) Create(_ context.Context, c model.Category) (*model.Category, error) {
	c.ID = uuid.New()
	m.made = append(m.made, c)
	return &c, nil
}

type memPayees struct {
	repository.PayeesRepository
	made []model.Payee
}

func (m *memPayees) Create(_ context.Context, p model.Payee) (*model.Payee, error) {
	p.ID = uuid.New()
	m.made = append(m.made, p)
	return &p, nil
}

// A new budget comes with its system groups, the inflow category and the
// starting balance payee, and records them in its metadata. Demo seeding
// builds its budget the same way.
func TestBudgetService_CreateMakesWhatEveryBudgetNeeds(t *testing.T) {
	budgets, groups, categories, payees := &memBudgets{}, &memGroups{}, &memCategories{}, &memPayees{}
	svc := NewBudgetService(budgets, payees, categories, groups, fakeDBTxn{})

	created, err := svc.Create(context.Background(), model.CreateBudgetRequest{
		Name: " Home ",
		TemplateGroups: []model.BudgetTemplateGroup{
			{Name: "Bills", Categories: []model.BudgetTemplateCategory{{Name: "Rent"}, {Name: " "}}},
			{Name: ""},
		},
	}, uuid.New())
	require.NoError(t, err)

	assert.Equal(t, "Home", created.Name)
	assert.True(t, created.IsSelected, "a user's first budget is selected")
	assert.Equal(t, created.Metadata, budgets.saved.Metadata)

	var groupNames []string
	for _, g := range groups.made {
		groupNames = append(groupNames, g.Name)
	}
	assert.Equal(t, []string{"Internal Master Category", "Credit Card Payments", "Bills"}, groupNames)
	assert.Equal(t, groups.made[1].ID, created.Metadata.CCGroupID)

	require.Len(t, categories.made, 2)
	assert.Equal(t, inflowCategoryName, categories.made[0].Name)
	assert.Equal(t, categories.made[0].ID, created.Metadata.InflowCategoryID)
	assert.Equal(t, "Rent", categories.made[1].Name)

	require.Len(t, payees.made, 1)
	assert.Equal(t, payees.made[0].ID, created.Metadata.StartingBalPayeeID)

	budgets.existing = []model.Budget{*created}
	second, err := svc.Create(context.Background(), model.CreateBudgetRequest{Name: "Work"}, uuid.New())
	require.NoError(t, err)
	assert.False(t, second.IsSelected)
}
