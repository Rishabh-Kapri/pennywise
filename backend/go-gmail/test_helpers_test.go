package main

import (
	"testing"
)

// ---------------------------------------------------------------------------
// normalizePayee
// ---------------------------------------------------------------------------

func TestNormalizePayee(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Exact match lowercase", "unexpected", "unexpected"},
		{"Google cloud exact", "google cloud", "google cloud"},
		{"Gym maps to fitness", "gym", "fitness"},
		{"Nakpro maps to fitness", "nakpro", "fitness"},
		{"Protein maps to fitness", "protein", "fitness"},
		{"Muscleblaze maps to fitness", "muscleblaze", "fitness"},
		{"Myprotein maps to fitness", "myprotein", "fitness"},
		{"Ola maps to taxi", "ola", "taxi"},
		{"Uber maps to taxi", "uber", "taxi"},
		{"Netflix maps to streaming", "netflix", "streaming"},
		{"Amazon maps to shopping", "amazon", "shopping"},
		{"Spotify maps to subscription", "spotify", "subscription"},
		{"Adobe maps to subscription", "adobe", "subscription"},
		{"Airtel maps to bills", "airtel", "bills"},
		{"Steam maps to gaming", "steam", "gaming"},
		{"Substring match hdfc", "hdfc bank", "bank"},
		{"Unknown payee returns itself", "some-totally-unknown-vendor-xyz", "some-totally-unknown-vendor-xyz"},
		{"Cred maps to transfer", "cred", "transfer"},
		{"Indmoney maps to investment", "indmoney", "investment"},
		// Fuzzy match: "spotif" is within distance 1 of "spotify" -> "subscription"
		{"Fuzzy match spotify typo", "spotif", "subscription"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePayee(tt.input)
			if got != tt.expected {
				t.Errorf("normalizePayee(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// resolvePayee
// ---------------------------------------------------------------------------

func TestResolvePayee(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"Spotify pattern", "spotify", "Spotify"},
		{"Google Cloud cybs pattern", "google cloud cybs si on 03-07-2025", "Google Cloud"},
		{"Adobe pattern", "adobe systems software", "Adobe"},
		{"OpenAI pattern", "openai", "OpenAI"},
		{"Transfer: cred.club pattern", "cred.club", "Transfer"},
		{"Airtel pattern", "airtel payments bank", "Airtel"},
		{"Unknown falls back to Unexpected", "xyzunknownmerchant999", "Unexpected"},
		{"At pattern extraction", "rs 500 debited at some shop on 14-07-25", "Unexpected"},
		// "to vpa" branch: possiblePayee becomes "Spotif" which fuzzy-matches Spotify (distance 1)
		{"To VPA extraction with fuzzy match", "rs 500 debited to vpa Spotif", "Spotify"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePayee(tt.input)
			if got != tt.want {
				t.Errorf("resolvePayee(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// resolveAccount
// ---------------------------------------------------------------------------

func TestResolveAccount(t *testing.T) {
	tests := []struct {
		name        string
		emailBody   string
		payee       string
		wantAccount string
		wantType    string
	}{
		{
			name:        "HDFC Credit Card by suffix and pattern",
			emailBody:   "thank you for using credit card ending 4432",
			payee:       "Google Cloud",
			wantAccount: "HDFC Credit Card",
			wantType:    "debit",
		},
		{
			name:        "Transfer payee overrides account",
			emailBody:   "amount debited to cred.club",
			payee:       "Transfer",
			wantAccount: "HDFC Credit Card",
			wantType:    "transfer",
		},
		{
			name:        "Credit transaction type",
			emailBody:   "rs 3000 credited to account 1234",
			payee:       "Some Person",
			wantAccount: "",
			wantType:    "credit",
		},
		{
			name:        "Debit transaction with debit keyword",
			emailBody:   "rs 500 debited from account 9876",
			payee:       "Shop",
			wantAccount: "",
			wantType:    "debit",
		},
		{
			name:        "Default type is debit when no keywords match",
			emailBody:   "transaction at merchant",
			payee:       "Merchant",
			wantAccount: "",
			wantType:    "debit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveAccount(tt.emailBody, tt.payee)
			if got.Account != tt.wantAccount {
				t.Errorf("resolveAccount().Account = %q, want %q", got.Account, tt.wantAccount)
			}
			if got.Type != tt.wantType {
				t.Errorf("resolveAccount().Type = %q, want %q", got.Type, tt.wantType)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// test()
// ---------------------------------------------------------------------------

func TestTestFunction(t *testing.T) {
	// test() is an empty stub; calling it simply ensures the line is covered.
	test()
}

