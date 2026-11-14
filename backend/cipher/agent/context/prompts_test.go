package context

import (
	"testing"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
)

func TestFormatCategories(t *testing.T) {
	got := FormatCategories([]sharedModel.CategorySimplified{
		{Name: "Groceries", Description: "supermarket runs,\n  not restaurants "},
		{Name: "Fuel"},
	})
	want := "- Groceries: supermarket runs, not restaurants\n- Fuel"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if FormatCategories(nil) != "" {
		t.Fatal("no categories should format to an empty string")
	}
}
