package utils

import (
	"sort"
	"strings"
)

// GetMonthKey returns the YYYY-MM key of a YYYY-MM-DD date.
func GetMonthKey(date string) string {
	key := strings.Split(date, "-")
	return key[0] + "-" + key[1]
}

func getSortedMonths(values []string) []string {
	sort.Strings(values)
	return values
}

func FillCarryForward(values map[string]float64, month string) map[string]float64 {
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
