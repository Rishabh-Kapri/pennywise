package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/pipeline"
	"github.com/google/uuid"
)

type fakeReader struct {
	res   model.StatementExtractResponse
	err   error
	calls *int
}

func (f fakeReader) ExtractStatement(context.Context, []byte, []string) (model.StatementExtractResponse, error) {
	*f.calls++
	return f.res, f.err
}

type fakeStatementPipeline struct {
	extract, fallback ports.StatementImport
	fallbackErr       error
	fallbackOff       bool
	emailOff          bool
}

func (f fakeStatementPipeline) Enabled(_ context.Context, _ uuid.UUID, step pipeline.StepID) (bool, error) {
	off := (f.fallbackOff && step == pipeline.StepStatementFallback) ||
		(f.emailOff && step == pipeline.StepStatementEmail)
	return !off, nil
}

func (f fakeStatementPipeline) Extract(context.Context, uuid.UUID) (ports.StatementImport, error) {
	return f.extract, nil
}

func (f fakeStatementPipeline) Fallback(context.Context, uuid.UUID) (ports.StatementImport, error) {
	return f.fallback, f.fallbackErr
}

func amount(v float64) *float64 { return &v }

// adds up: 1000 - 200 = 800.
var checksOut = model.StatementExtractResponse{Sections: []model.StatementSection{{
	AccountNumber: "XX1", OpeningBalance: amount(1000), ClosingBalance: amount(800),
	Rows: []model.StatementRow{{Amount: 200, Direction: model.StatementDebit}},
}}}

// nothing printed to check against.
var unchecked = model.StatementExtractResponse{Sections: []model.StatementSection{{
	AccountNumber: "XX2",
	Rows:          []model.StatementRow{{Amount: 200, Direction: model.StatementDebit}},
}}}

func TestReadStatement(t *testing.T) {
	tests := []struct {
		name          string
		first, second model.StatementExtractResponse
		secondErr     error
		fallbackErr   error
		fallbackOff   bool
		file          string // default a PDF
		wantAccount   string
		wantFallback  int // times the fallback reader ran
	}{
		{name: "first read checks out", first: checksOut, second: unchecked, wantAccount: "XX1"},
		{name: "fallback checks out", first: unchecked, second: checksOut, wantAccount: "XX1", wantFallback: 1},
		{
			name: "fallback no better", first: unchecked, second: model.StatementExtractResponse{},
			wantAccount: "XX2", wantFallback: 1,
		},
		{
			name: "fallback fails", first: unchecked, secondErr: errors.New("model down"),
			wantAccount: "XX2", wantFallback: 1,
		},
		{name: "fallback can't be built", first: unchecked, fallbackErr: errors.New("no model"), wantAccount: "XX2"},
		{name: "fallback turned off", first: unchecked, second: checksOut, fallbackOff: true, wantAccount: "XX2"},
		{name: "a CSV gets no fallback", first: unchecked, second: checksOut, file: "Date,Narration\n", wantAccount: "XX2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var firstCalls, secondCalls int
			deps := WorkerDeps{StatementImport: fakeStatementPipeline{
				extract:     fakeReader{res: tt.first, calls: &firstCalls},
				fallback:    fakeReader{res: tt.second, err: tt.secondErr, calls: &secondCalls},
				fallbackErr: tt.fallbackErr,
				fallbackOff: tt.fallbackOff,
			}}
			file := "%PDF-1.7"
			if tt.file != "" {
				file = tt.file
			}
			res, err := readStatement(context.Background(), deps, uuid.New(), []byte(file), nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := res.Sections[0].AccountNumber; got != tt.wantAccount {
				t.Errorf("kept %s, want %s", got, tt.wantAccount)
			}
			if firstCalls != 1 || secondCalls != tt.wantFallback {
				t.Errorf("extract ran %d, fallback %d; want 1, %d", firstCalls, secondCalls, tt.wantFallback)
			}
		})
	}
}
