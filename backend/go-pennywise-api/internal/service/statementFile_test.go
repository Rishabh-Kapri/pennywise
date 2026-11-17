package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatementFileExt(t *testing.T) {
	for _, tt := range []struct {
		name, file, data, want string
	}{
		{"pdf", "statement.pdf", "%PDF-1.7\n", ".pdf"},
		{"pdf with any name", "statement", "\n%PDF-1.4", ".pdf"},
		{"csv", "Acct_Statement.CSV", "Date,Narration\n", ".csv"},
		{"text not named csv", "statement.txt", "Date,Narration\n", ""},
		{"binary named csv", "x.csv", "PK\x03\x04\x00", ""},
	} {
		got, ok := statementFileExt(tt.file, []byte(tt.data))
		assert.Equal(t, tt.want, got, tt.name)
		assert.Equal(t, tt.want != "", ok, tt.name)
	}
}
