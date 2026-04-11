package parser

import (
	"log/slog"
	"os"
	"testing"
)

func readTestEmail(t *testing.T, filename string) string {
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read file %s: %v", filename, err)
	}
	return string(data)
}

func TestExtractDate(t *testing.T) {
	parser := NewEmailParser()
	tests := []struct {
		name    string
		text    string
		want    string
		wantErr bool
	}{
		{
			"Valid Date YYYY",
			"Dear Card Member, <br> <br>Thank you for using your HDFC Bank Credit Card ending 9876 for Rs 5065.68 at NAKODA DAIRY PRIVATE L on 04-08-2025.",
			"2025-08-04",
			false,
		},
		{
			"Valid Date Slash Separator",
			"Dear Customer, Greetings from HDFC Bank! Your Spotify bill, set up through E-mandate (Auto payment), has been successfully paid using your HDFC Bank Credit Card ending 9876. Transaction Details: Amount: INR 149.00 Date: 12/08/2025.",
			"2025-08-12",
			false,
		},
		{
			"Valid Date Month Name",
			"Dear Customer, Greetings from HDFC Bank!  Rs.438.87 is debited from your HDFC Bank Credit Card ending 9876 towards OPENAI on 11 Aug, 2025.",
			"2025-08-11",
			false,
		},
		{
			"Valid Date YY",
			"Dear Customer,</r> Rs. 3000.00 is successfully credited to your account **4567 by VPA janedoe42@okicici JANE DOE on 09-08-25.",
			"2025-08-09",
			false,
		},
		{
			"NoDate",
			"",
			"",
			true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			email := &EmailDetails{Text: testCase.text}
			err := parser.extractDate(email)
			// t.Logf("%v %v", err, email)
			// log.Printf("%v %v", err, email)
			if testCase.wantErr && err == nil {
				t.Errorf("Expected error, got nil")
			}
			if !testCase.wantErr && (err != nil || email.Date != testCase.want) {
				t.Errorf("Expected %v, got %v (err %v)", testCase.want, email.Date, err)
			}
		})
	}
}

func TestExtractType(t *testing.T) {
	parser := NewEmailParser()
	tests := []struct {
		name string
		text string
		want string
	}{
		{"DebitCC", "Dear Card Member, <br> <br>Thank you for using your HDFC Bank Credit Card ending 9876 for Rs 5065.68 at NAKODA DAIRY PRIVATE L on 04-08-2025.", "debited"},
		{"DebitUPI", "Dear Customer,</r> Rs.500.00 has been debited from account 4567 to VPA janedoe42@okicici JANE DOE on 09-08-25.", "debited"},
		{"CreditUPI", "Dear Customer,</r> Rs. 3000.00 is successfully credited to your account **4567 by VPA janedoe42@okicici JANE DOE on 09-08-25.", "credited"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			email := &EmailDetails{Text: testCase.text}
			err := parser.extractType(email)
			slog.Info("transaction type", "type", email.TransactionType)

			if email.TransactionType != testCase.want {
				t.Errorf("Expected %v, got %v (err %v)", testCase.want, email.TransactionType, err)
			}
		})
	}
}

func TestExtractAmount(t *testing.T) {
	parser := NewEmailParser()
	tests := []struct {
		name    string
		text    string
		txnType string
		want    float64
		wantErr bool
	}{
		{
			"CC Debit",
			"Dear Card Member, <br> <br>Thank you for using your HDFC Bank Credit Card ending 9876 for Rs 5065.68 at NAKODA DAIRY PRIVATE L on 04-08-2025.",
			"debited",
			-5065.68,
			false,
		},
		{
			"CC Debit New",
			"Dear Customer, Greetings from HDFC Bank!  Rs.438.87 is debited from your HDFC Bank Credit Card ending 9876 towards OPENAI on 11 Aug, 2025.",
			"debited",
			-438.87,
			false,
		},
		{
			"CC Mandate",
			"Dear Customer, Greetings from HDFC Bank! Your Spotify bill, set up through E-mandate (Auto payment), has been successfully paid using your HDFC Bank Credit Card ending 9876. Transaction Details: Amount: INR 149.00 Date: 12/08/2025.",
			"debited",
			-149,
			false,
		},
		{
			"UPI Debit",
			"Dear Customer,</r> Rs.500.00 has been debited from account 4567 to VPA janedoe42@okicici JANE DOE on 09-08-25.",
			"debited",
			-500,
			false,
		},
		{
			"UPI Credit",
			"Dear Customer,</r> Rs. 3000.00 is successfully credited to your account **4567 by VPA janedoe42@okicici JANE DOE on 09-08-25.",
			"credited",
			3000,
			false,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			email := &EmailDetails{Text: testCase.text, TransactionType: testCase.txnType}
			err := parser.extractAmount(email)

			if testCase.wantErr && err == nil {
				t.Errorf("Expected error, got nil")
			}
			if !testCase.wantErr && (err != nil || email.Amount != testCase.want) {
				t.Errorf("Expected %v, got %v (err %v)", testCase.want, email.Amount, err)
			}
		})
	}
}

func TestExtractText(t *testing.T) {
	parser := NewEmailParser()
	tests := []struct {
		name    string
		html    string
		want    string
		wantErr bool
	}{
		{
			"CC Debit",
			readTestEmail(t, "testdata/valid_cc.txt"),
			"Dear Customer, Thank you for using HDFC Bank Credit Card 9876 for Rs. 22.63 at GOOGLE CLOUD CYBS SI on 03-07-2025.",
			false,
		},
		{
			"CC Debit New",
			readTestEmail(t, "testdata/valid_cc_new.txt"),
			"Dear Customer, Greetings from HDFC Bank!  Rs.438.87 is debited from your HDFC Bank Credit Card ending 9876 towards OPENAI on 11 Aug, 2025.",
			false,
		},
		{
			"CC Mandate",
			readTestEmail(t, "testdata/valid_cc_mandate.txt"),
			"",
			true,
		},
		{
			"CC Mandate Upcoming",
			readTestEmail(t, "testdata/valid_cc_mandate_upcoming.txt"),
			"",
			true,
		},
		{
			"UPI Debit",
			readTestEmail(t, "testdata/valid_upi_debit.txt"),
			"Dear Customer, Rs.500.00 has been debited from account 4567 to VPA 9876543210@ybl JOHN DOE S O JAMES DOE on 14-07-25.",
			false,
		},
		{
			"UPI Credit",
			readTestEmail(t, "testdata/valid_upi_credit.txt"),
			"Dear Customer, Rs. 3000.00 is successfully credited to your account **4567 by VPA 9876543210@ybl JOHN DOE S O JAMES DOE on 12-07-25.",
			false,
		},
		{
			"CC Mandate Combined",
			readTestEmail(t, "testdata/valid_cc_mandate_combined.txt"),
			"Dear Customer, Thank you for using HDFC Bank Card XX9876 for Rs. 16.56 at GOOGLE CLOUD CYBS SI on 04-04-2026.",
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			email := &EmailDetails{}
			err := parser.extractText(email, tt.html)


			if tt.wantErr && err == nil {
				t.Errorf("Expected error, got nil")
			}
			if !tt.wantErr && (err != nil || email.Text != tt.want) {
				t.Errorf("Expected: %v; got: %v (err %v)", tt.want, email.Text, err)
			}
		})
	}
}

func TestParseEmail(t *testing.T) {
	parser := NewEmailParser()
	tests := []struct {
		name    string
		html    string
		want    string
		wantErr bool
	}{
		{
			"DebitCC",
			readTestEmail(t, "testdata/valid_cc.txt"),
			"Dear Customer, Thank you for using HDFC Bank Credit Card 9876 for Rs. 22.63 at GOOGLE CLOUD CYBS SI on 03-07-2025.",
			false,
		},
		{
			"DebitCCNew",
			readTestEmail(t, "testdata/valid_cc_new.txt"),
			"Dear Customer, Greetings from HDFC Bank!  Rs.438.87 is debited from your HDFC Bank Credit Card ending 9876 towards OPENAI on 11 Aug, 2025.",
			false,
		},
		{
			"CCMandate",
			readTestEmail(t, "testdata/valid_cc_mandate.txt"),
			"",
			true,
		},
		{
			"CCMandateUpcoming",
			readTestEmail(t, "testdata/valid_cc_mandate_upcoming.txt"),
			"",
			true,
		},
		{
			"DebitUPI",
			readTestEmail(t, "testdata/valid_upi_debit.txt"),
			"Dear Customer, Rs.500.00 has been debited from account 4567 to VPA 9876543210@ybl JOHN DOE S O JAMES DOE on 14-07-25.",
			false,
		},
		{
			"CreditUPI",
			readTestEmail(t, "testdata/valid_upi_credit.txt"),
			"Dear Customer, Rs. 3000.00 is successfully credited to your account **4567 by VPA 9876543210@ybl JOHN DOE S O JAMES DOE on 12-07-25.",
			false,
		},
		{
			"CCMandateCombined",
			readTestEmail(t, "testdata/valid_cc_mandate_combined.txt"),
			"Dear Customer, Thank you for using HDFC Bank Card XX9876 for Rs. 16.56 at GOOGLE CLOUD CYBS SI on 04-04-2026.",
			false,
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			// email := &EmailDetails{}
			emailData, err := parser.ParseEmail(testCase.html)

			if testCase.wantErr && err == nil {
				t.Errorf("Expected error, got nil")
			}
			if !testCase.wantErr && (err != nil || emailData.Text != testCase.want) {
				t.Errorf("Expected %v, got %v (err %v)", testCase.want, emailData.Text, err)
			}
		})
	}
}
