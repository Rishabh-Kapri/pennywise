package utils

import (
	"regexp"
	"strings"
)

var (
	upiRegex = regexp.MustCompile(`(?i)[a-zA-Z0-9.\-_]+@[a-zA-Z0-9]+`)

	// Remove invoice numbers like "I04143-17316417"
	invoiceNoiseRegex = regexp.MustCompile(`(?i)\b([A-Z0-9]+-[A-Z0-9]{4,}|[A-Z]+[0-9]{5,}|[0-9]{6,})\b`)

	digitsRegex = regexp.MustCompile(`\d+`)
)

func CleanUPIText(merchantString string) (upiText string, merchantName string) {
	upiText = upiRegex.FindString(merchantString)
	merchantName = upiRegex.ReplaceAllString(merchantString, "")

	// Remove double spaces, trailing spaces
	merchantName = strings.Join(strings.Fields(merchantName), " ")
	merchantName = invoiceNoiseRegex.ReplaceAllString(merchantName, "")
	merchantName = strings.TrimSpace(merchantName)

	return upiText, merchantName
}

func CleanMerchantString(merchantString string) (merchantName string) {
	merchantName = invoiceNoiseRegex.ReplaceAllString(merchantString, "")
	merchantName = strings.TrimSpace(merchantName)
	return merchantName
}

func CleanAccountString(raw string) string {
	if raw == "" {
		return ""
	}

	cleaned := digitsRegex.FindString(raw)

	return cleaned
}
