package utils

import (
	"testing"
)

func TestCleanEmailText(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		txnType string
		want    string
	}{
		{
			"CC debit old format",
			"Dear Customer, Thank you for using HDFC Bank Credit Card 9876 for Rs. 22.63 at GOOGLE CLOUD CYBS SI on 03-07-2025.",
			"debited",
			"debit 9876 GOOGLE CLOUD CYBS SI",
		},
		{
			"CC debit new format",
			"Dear Customer, Greetings from HDFC Bank!  Rs.438.87 is debited from your HDFC Bank Credit Card ending 9876 towards OPENAI on 11 Aug, 2025.",
			"debited",
			"debit 9876 OPENAI",
		},
		{
			"CC mandate",
			"Dear Customer, Greetings from HDFC Bank! Your Spotify bill, set up through E-mandate (Auto payment), has been successfully paid using your HDFC Bank Credit Card ending 9876. Transaction Details: Amount: INR 149.00 Date: 12/08/2025.",
			"debited",
			"debit 9876 Spotify",
		},
		{
			"UPI debit",
			"Dear Customer, Rs.500.00 has been debited from account 4567 to VPA 9876543210@ybl JOHN DOE S O JAMES DOE on 14-07-25.",
			"debited",
			"debit 4567 9876543210@ybl JOHN DOE S O JAMES DOE",
		},
		{
			"UPI credit",
			"Dear Customer, Rs. 3000.00 is successfully credited to your account **4567 by VPA 9876543210@ybl JOHN DOE S O JAMES DOE on 12-07-25.",
			"credited",
			"credit 4567 9876543210@ybl JOHN DOE S O JAMES DOE",
		},
		{
			"RAZ prefix merchant",
			"Dear Customer, Greetings from HDFC Bank!  Rs.399.50 is debited from your HDFC Bank Credit Card ending 9876 towards RAZ*Quickocom on 15 Aug, 2025.",
			"debited",
			"debit 9876 RAZ*Quickocom",
		},
		{
			"empty text",
			"",
			"debited",
			"debit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanEmailText(tt.text, tt.txnType)
			if got != tt.want {
				t.Errorf("\ngot:  %q\nwant: %q", got, tt.want)
			}
		})
	}
}
