package postgres

import (
	"context"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/stretchr/testify/require"
)

func TestTransactionDocumentSearch(t *testing.T) {
	pool, txns, s := setup(t)
	docs := NewTransactionDocumentRepository(pool)
	ctx := context.Background()

	oct := mustCreate(t, txns, newTxn(s, "2026-10-10", -100)) // payee "Blue Tokai"
	nov := newTxn(s, "2026-11-10", -200)
	nov.PayeeID = &s.payee2ID // "Swiggy"
	novTxn := mustCreate(t, txns, nov)

	add := func(txnID, name, mime string) model.TransactionDocument {
		t.Helper()
		var tid = oct.ID
		if txnID == "nov" {
			tid = novTxn.ID
		}
		doc, err := docs.Create(ctx, model.TransactionDocument{
			BudgetID: s.budgetID, TransactionID: tid, FileName: name, MimeType: mime, SizeBytes: 1, StoragePath: "p/" + name,
		})
		require.NoError(t, err)
		return *doc
	}
	receipt := add("oct", "coffee-receipt.jpg", "image/jpeg")
	invoice := add("nov", "invoice.pdf", "application/pdf")
	gone := add("nov", "deleted.pdf", "application/pdf")
	require.NoError(t, docs.DeleteById(ctx, s.budgetID, gone.ID))

	names := func(f model.DocumentFilter) []string {
		t.Helper()
		res, err := docs.Search(ctx, s.budgetID, f)
		require.NoError(t, err)
		out := []string{}
		for _, item := range res.Data {
			out = append(out, item.FileName)
		}
		require.Equal(t, len(res.Data), res.Total, "total counts the same rows the page lists")
		return out
	}

	t.Run("newest transaction first, soft-deleted documents left out", func(t *testing.T) {
		require.Equal(t, []string{invoice.FileName, receipt.FileName}, names(model.DocumentFilter{}))
	})
	t.Run("search matches file name and payee name, trimmed and case-insensitive", func(t *testing.T) {
		require.Equal(t, []string{receipt.FileName}, names(model.DocumentFilter{Search: " COFFEE "}))
		require.Equal(t, []string{invoice.FileName}, names(model.DocumentFilter{Search: "swiggy"}))
	})
	t.Run("blank search is no filter", func(t *testing.T) {
		require.Len(t, names(model.DocumentFilter{Search: "   "}), 2)
	})
	t.Run("kind", func(t *testing.T) {
		require.Equal(t, []string{receipt.FileName}, names(model.DocumentFilter{Kind: model.DocumentKindImage}))
		require.Equal(t, []string{invoice.FileName}, names(model.DocumentFilter{Kind: model.DocumentKindPDF}))
	})
	t.Run("dates filter on the transaction date", func(t *testing.T) {
		require.Equal(t, []string{invoice.FileName}, names(model.DocumentFilter{StartDate: "2026-11-01"}))
		require.Equal(t, []string{receipt.FileName}, names(model.DocumentFilter{EndDate: "2026-10-31"}))
		require.Equal(t, []string{invoice.FileName}, names(model.DocumentFilter{Search: "invoice", StartDate: "2026-10-01", EndDate: "2026-11-30"}))
	})
	t.Run("another budget sees nothing", func(t *testing.T) {
		other := newSeed(t, pool)
		res, err := docs.Search(ctx, other.budgetID, model.DocumentFilter{})
		require.NoError(t, err)
		require.Empty(t, res.Data)
	})
	t.Run("pages report HasMore and the full total", func(t *testing.T) {
		res, err := docs.Search(ctx, s.budgetID, model.DocumentFilter{Limit: 1})
		require.NoError(t, err)
		require.Len(t, res.Data, 1)
		require.Equal(t, 2, res.Total)
		require.True(t, res.HasMore)
		last, err := docs.Search(ctx, s.budgetID, model.DocumentFilter{Limit: 1, Offset: 1})
		require.NoError(t, err)
		require.False(t, last.HasMore)
	})
}
