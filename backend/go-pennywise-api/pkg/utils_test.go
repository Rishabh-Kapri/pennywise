package utils

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestBudgetIDFromContext_Missing(t *testing.T) {
	ctx := context.Background()
	_, err := BudgetIDFromContext(ctx)
	if err == nil {
		t.Error("Expected error for missing budget ID, got nil")
	}
}

func TestBudgetIDFromContext_Valid(t *testing.T) {
	expected := uuid.MustParse("aab2f424-cb49-40f2-aa52-c07fb625961d")
	ctx := WithBudgetID(context.Background(), expected)

	got, err := BudgetIDFromContext(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %s", err)
	}
	if got != expected {
		t.Errorf("Expected %v, got %v", expected, got)
	}
}

func TestMustBudgetID_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for missing budget ID, but did not panic")
		}
	}()
	MustBudgetID(context.Background())
}

func TestMustBudgetID_Valid(t *testing.T) {
	expected := uuid.MustParse("aab2f424-cb49-40f2-aa52-c07fb625961d")
	ctx := WithBudgetID(context.Background(), expected)

	got := MustBudgetID(ctx)
	if got != expected {
		t.Errorf("Expected %v, got %v", expected, got)
	}
}
