package utils

import (
	"fmt"
	"sort"
	"strings"
)

// @TODO: add more validations
// Returns the month key from date (YYYY-MM-DD) in the format YYYY-MM.
func GetMonthKey(date string) string {
	key := strings.Split(date, "-")
	monthKey := key[0] + "-" + key[1]
	return monthKey
}

func getSortedMonths(values []string) []string {
	sort.Strings(values)
	return values
}

func FillCarryForward(values map[string]float32, month string) map[string]float32 {
	_, exists := values[month]
	if exists {
		return values
	}

	var months []string
	for k := range values {
		months = append(months, k)
	}
	if len(months) == 0 {
		return values
	}
	sortedMonths := getSortedMonths(months)

	values[month] = values[sortedMonths[len(sortedMonths)-1]]

	return values
}

func Float64SliceToVectorString(vec []float64) string {
	parts := make([]string, len(vec))
	for i, v := range vec {
		parts[i] = fmt.Sprintf("%.8f", v)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
