package handler

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

// A malformed :id used to fall through with uuid.Nil (no return after the
// 500), so DELETE /transactions/not-a-uuid reached the service. It must stop
// at a 400 now.
func TestTransactionHandler_InvalidIDNeverReachesService(t *testing.T) {
	routes := map[string]func(TransactionHandler, *gin.Context){
		"Update":         TransactionHandler.Update,
		"UpdateLocation": TransactionHandler.UpdateLocation,
		"UpdateStatus":   TransactionHandler.UpdateStatus,
		"DeleteById":     TransactionHandler.DeleteById,
	}
	for name, call := range routes {
		t.Run(name, func(t *testing.T) {
			svc := &mockTransactionService{}
			w, c := makeReq("PATCH", "/transactions/not-a-uuid", model.Transaction{})
			c.Params = gin.Params{{Key: "id", Value: "not-a-uuid"}}
			call(NewTransactionHandler(svc), c)
			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.JSONEq(t, `{"error":"invalid transaction id"}`, w.Body.String())
			svc.AssertNotCalled(t, "Update", mock.Anything, mock.Anything, mock.Anything)
			svc.AssertNotCalled(t, "DeleteById", mock.Anything, mock.Anything)
		})
	}
}

func TestTransactionHandler_ListNormalizedQuery(t *testing.T) {
	a, b, cat := uuid.New(), uuid.New(), uuid.New()

	t.Run("defaults and both id styles", func(t *testing.T) {
		svc := &mockTransactionService{}
		var got *model.TransactionFilter
		svc.On("GetAllNormalized", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) { got = args.Get(1).(*model.TransactionFilter) }).
			Return(model.PaginatedResponse[model.Transaction]{}, nil)

		path := "/transactions?accountId=" + a.String() + "," + b.String() +
			"&categoryId[]=" + cat.String() + "&startDate=2026-11-01&note=%20coffee%20"
		w, c := makeReq("GET", path, nil)
		NewTransactionHandler(svc).ListNormalized(c)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, uint64(30), got.Limit)
		assert.Equal(t, "DESC", got.SortOrder)
		assert.Equal(t, "month", *got.GroupBy)
		assert.Equal(t, []uuid.UUID{a, b}, got.AccountIDs)
		assert.Equal(t, []uuid.UUID{cat}, got.CategoryIDs)
		assert.Equal(t, "2026-11-01", *got.StartDate)
		assert.Nil(t, got.EndDate)
		assert.Equal(t, "coffee", *got.Note)
	})

	t.Run("status filter", func(t *testing.T) {
		svc := &mockTransactionService{}
		var got *model.TransactionFilter
		svc.On("GetAllNormalized", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) { got = args.Get(1).(*model.TransactionFilter) }).
			Return(model.PaginatedResponse[model.Transaction]{}, nil)

		w, c := makeReq("GET", "/transactions?status=UNAPPROVED", nil)
		NewTransactionHandler(svc).ListNormalized(c)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, model.TransactionStatusUnapproved, *got.Status)
	})

	bad := map[string]string{
		"unknown status":      "/transactions?status=PENDING",
		"date not YYYY-MM-DD": "/transactions?startDate=01-11-2026",
		"unknown sort order":  "/transactions?sortOrder=sideways",
		"negative limit":      "/transactions?limit=-1",
	}
	for name, path := range bad {
		t.Run(name, func(t *testing.T) {
			svc := &mockTransactionService{}
			w, c := makeReq("GET", path, nil)
			NewTransactionHandler(svc).ListNormalized(c)
			assert.Equal(t, http.StatusBadRequest, w.Code)
			svc.AssertNotCalled(t, "GetAllNormalized", mock.Anything, mock.Anything)
		})
	}

	t.Run("error names the field", func(t *testing.T) {
		w, c := makeReq("GET", "/transactions?startDate=yesterday", nil)
		NewTransactionHandler(&mockTransactionService{}).ListNormalized(c)
		assert.JSONEq(t, `{"error":"invalid startDate"}`, w.Body.String())
	})
}

func TestTransactionHandler_BulkDelete(t *testing.T) {
	id := uuid.New()

	t.Run("deletes the ids", func(t *testing.T) {
		svc := &mockTransactionService{}
		svc.On("BulkDelete", mock.Anything, []uuid.UUID{id}).Return(1, nil)
		w, c := makeReq("POST", "/transactions/bulk-delete", map[string]any{"ids": []uuid.UUID{id}})
		NewTransactionHandler(svc).BulkDelete(c)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.JSONEq(t, `{"deleted":1}`, w.Body.String())
	})

	tooMany := make([]uuid.UUID, maxBulkDelete+1)
	for i := range tooMany {
		tooMany[i] = uuid.New()
	}
	bad := map[string]any{
		"no ids":   map[string]any{"ids": []uuid.UUID{}},
		"not ids":  map[string]any{"ids": []string{"nope"}},
		"too many": map[string]any{"ids": tooMany},
	}
	for name, body := range bad {
		t.Run(name, func(t *testing.T) {
			svc := &mockTransactionService{}
			w, c := makeReq("POST", "/transactions/bulk-delete", body)
			NewTransactionHandler(svc).BulkDelete(c)
			assert.Equal(t, http.StatusBadRequest, w.Code)
			svc.AssertNotCalled(t, "BulkDelete", mock.Anything, mock.Anything)
		})
	}
}
