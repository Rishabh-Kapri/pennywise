package service

import (
	"context"
	"errors"
	"testing"

	repository "github.com/Rishabh-Kapri/pennywise/backend/shared/db"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pageCipher fails any request spanning more than one page, the way a model
// that runs out of output budget on a dense chunk does.
type pageCipher struct {
	CipherClient
	calls []model.StatementExtractRequest
}

func (c *pageCipher) ExtractStatement(_ context.Context, req model.StatementExtractRequest) (*model.StatementExtractResponse, error) {
	c.calls = append(c.calls, req)
	if req.LastPage > req.FirstPage {
		return nil, errors.New("output truncated at the token limit")
	}
	section := model.StatementSection{
		Rows: []model.StatementRow{{Date: "2026-09-0" + string(rune('0'+req.FirstPage)), Description: "ROW", Amount: 10, Direction: model.StatementDebit}},
	}
	if req.FirstPage == 1 {
		section.AccountLabel, section.AccountNumber = "HDFC", "XX4321"
	}
	return &model.StatementExtractResponse{Sections: []model.StatementSection{section}}, nil
}

type recordingStatementRepo struct {
	repository.StatementImportRepository
	completed []model.StatementSection
	progress  []int
}

func (r *recordingStatementRepo) UpdateProgress(_ context.Context, _ uuid.UUID, pages int) error {
	r.progress = append(r.progress, pages)
	return nil
}

func (r *recordingStatementRepo) Complete(_ context.Context, _ uuid.UUID, sections []model.StatementSection, _ int) error {
	r.completed = sections
	return nil
}

func TestRunExtractionFallsBackToSinglePages(t *testing.T) {
	cipher := &pageCipher{}
	repo := &recordingStatementRepo{}
	svc := &statementService{repo: repo, cipherClient: cipher}

	defaultAccount := uuid.New()
	err := svc.runExtraction(context.Background(), model.StatementImport{
		ID:               uuid.New(),
		SourceText:       "page one\fpage two\fpage three",
		DefaultAccountID: &defaultAccount,
	})
	require.NoError(t, err)

	// one failed 3-page call, then one call per page
	require.Len(t, cipher.calls, 4)
	assert.Equal(t, 3, cipher.calls[0].LastPage)
	assert.Equal(t, "HDFC XX4321", cipher.calls[2].PreviousSection, "account carries across single pages")
	assert.NotContains(t, cipher.calls[2].Text, "page one")

	// header-less continuation pages merge into the first section
	require.Len(t, repo.completed, 1)
	assert.Len(t, repo.completed[0].Rows, 3)
	assert.Equal(t, &defaultAccount, repo.completed[0].AccountID)
	assert.Equal(t, []int{3}, repo.progress)
}
