package service

import (
	"strings"
	"testing"
	"unicode/utf8"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/stretchr/testify/assert"
)

func TestCleanDescription(t *testing.T) {
	assert.Equal(t, "Groceries and household staples.", cleanDescription("  \"Groceries and\n household staples.\" "))
	assert.Equal(t, "", cleanDescription("   "))

	long := strings.Repeat("word ", 100)
	got := cleanDescription(long)
	assert.LessOrEqual(t, utf8.RuneCountInString(got), maxDescriptionRunes+1)
	assert.True(t, strings.HasSuffix(got, "…"))
	assert.False(t, strings.Contains(got, "wor…"), "cut on a word boundary")
}

func TestCategoryDescriptionInput(t *testing.T) {
	in := categoryDescriptionInput(sharedModel.CategoryDescriptionContext{
		CategoryName:      "Groceries",
		GroupName:         "Food",
		SiblingCategories: []string{"Dining Out", "Coffee"},
	})
	assert.Contains(t, in, "Category: Groceries")
	assert.Contains(t, in, "Group: Food")
	assert.Contains(t, in, "Other categories in this group: Dining Out, Coffee")
	assert.Contains(t, in, "Most frequent payees: none yet")
	assert.NotContains(t, in, "Current description")
}
