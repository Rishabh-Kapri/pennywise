package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/repository"
	utils "github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// roleRepo answers Role from a map; everything else is unused here.
type roleRepo struct {
	repository.BudgetMemberRepository
	roles map[uuid.UUID]model.BudgetRole
}

func (r roleRepo) Role(_ context.Context, _ uuid.UUID, userID uuid.UUID) (model.BudgetRole, error) {
	role, ok := r.roles[userID]
	if !ok {
		return "", repository.ErrNotFound
	}
	return role, nil
}

func TestBudgetIdMiddlewareRoles(t *testing.T) {
	gin.SetMode(gin.TestMode)
	budgetID := uuid.New()
	owner, viewer, stranger := uuid.New(), uuid.New(), uuid.New()
	repo := roleRepo{roles: map[uuid.UUID]model.BudgetRole{owner: model.BudgetRoleOwner, viewer: model.BudgetRoleViewer}}

	router := gin.New()
	router.Use(func(c *gin.Context) {
		if id, err := uuid.Parse(c.GetHeader("X-Test-User")); err == nil {
			c.Request = c.Request.WithContext(utils.WithUserID(c.Request.Context(), id))
		}
	}, BudgetIdMiddleware(repo))
	var seenRole model.BudgetRole
	handler := func(c *gin.Context) {
		seenRole = utils.BudgetRoleFromContext(c.Request.Context())
		c.Status(http.StatusNoContent)
	}
	router.GET("/thing", handler)
	router.POST("/thing", handler)
	router.POST("/owner-only", RequireBudgetRole(model.BudgetRoleOwner), handler)

	do := func(method, path string, user uuid.UUID) int {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set(utils.HeaderBudgetID, budgetID.String())
		req.Header.Set("X-Test-User", user.String())
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Code
	}

	require.Equal(t, http.StatusNoContent, do(http.MethodGet, "/thing", viewer))
	require.Equal(t, model.BudgetRoleViewer, seenRole)
	require.Equal(t, http.StatusForbidden, do(http.MethodPost, "/thing", viewer), "a viewer can't write")
	require.Equal(t, http.StatusNoContent, do(http.MethodPost, "/thing", owner))
	require.Equal(t, http.StatusForbidden, do(http.MethodGet, "/thing", stranger), "a non-member can't read")
	require.Equal(t, http.StatusNoContent, do(http.MethodPost, "/owner-only", owner))

	editor := uuid.New()
	repo.roles[editor] = model.BudgetRoleEditor
	require.Equal(t, http.StatusForbidden, do(http.MethodPost, "/owner-only", editor))
}
