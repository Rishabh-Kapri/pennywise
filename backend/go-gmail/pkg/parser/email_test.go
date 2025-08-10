package parser

import (
	"log"
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
		{"ValidDateYYYY", "Dear Card Member, <br> <br>Thank you for using your HDFC Bank Credit Card ending 4432 for Rs 5065.68 at NAKODA DAIRY PRIVATE L on 04-08-2025.", "2025-08-04", false},
		{"ValidDateYY", "Dear Customer,</r> Rs. 3000.00 is successfully credited to your account **8936 by VPA vishakhamarkun17@okicici VISHAKHA MARKUN on 09-08-25.", "2025-08-09", false},
		{"NoDate", "", "", true},
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
		{"DebitCC", "Dear Card Member, <br> <br>Thank you for using your HDFC Bank Credit Card ending 4432 for Rs 5065.68 at NAKODA DAIRY PRIVATE L on 04-08-2025.", "debited"},
		{"DebitUPI", "Dear Customer,</r> Rs.500.00 has been debited from account 8936 to VPA vishakhamarkun17@okicici VISHAKHA MARKUN on 09-08-25.", "debited"},
		{"CreditUPI", "Dear Customer,</r> Rs. 3000.00 is successfully credited to your account **8936 by VPA vishakhamarkun17@okicici VISHAKHA MARKUN on 09-08-25.", "credited"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			email := &EmailDetails{Text: testCase.text}
			err := parser.extractType(email)
			log.Printf("%v", email.TransactionType)

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
		{"DebitCC", "Dear Card Member, <br> <br>Thank you for using your HDFC Bank Credit Card ending 4432 for Rs 5065.68 at NAKODA DAIRY PRIVATE L on 04-08-2025.", "debited", -5065.68, false},
		{"DebitUPI", "Dear Customer,</r> Rs.500.00 has been debited from account 8936 to VPA vishakhamarkun17@okicici VISHAKHA MARKUN on 09-08-25.", "debited", -500, false},
		{"CreditUPI", "Dear Customer,</r> Rs. 3000.00 is successfully credited to your account **8936 by VPA vishakhamarkun17@okicici VISHAKHA MARKUN on 09-08-25.", "credited", 3000, false},
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

func TestParseEmail(t *testing.T) {
	parser := NewEmailParser()
	tests := []struct {
		name    string
		html    string
		want    string
		wantErr bool
	}{
		{"DebitCC", readTestEmail(t, "testdata/valid_cc.txt"), "Dear Customer, Thank you for using HDFC Bank Credit Card 4432 for Rs. 22.63 at GOOGLE CLOUD CYBS SI on 03-07-2025.", false},
		{"DebitUPI", readTestEmail(t, "testdata/valid_upi_debit.txt"), "Dear Customer, Rs.500.00 has been debited from account 8936 to VPA 9997167687@ybl RISHABH KAPRI S O GOKUL CHANDRA KAP on 14-07-25.", false},
		{"CreditUPI", readTestEmail(t, "testdata/valid_upi_credit.txt"), "Dear Customer, Rs. 3000.00 is successfully credited to your account **8936 by VPA 9997167687@ybl RISHABH KAPRI S O GOKUL CHANDRA KAP on 12-07-25.", false},
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
