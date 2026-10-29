package regex

import (
	"context"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
)

const hdfcCreditCardEmail = "Inserted suggestion this opportunity\nNone selected\n\nSkip to content\nUsing Gmail with screen readers\n2 of 23\nA payment was made using your Credit Card\nInbox\nHDFC\nHDFC Bank InstaAlerts <alerts@hdfcbank.bank.in>\n\t\n10:30 AM (6 minutes ago)\n\t\n\t\nto customer+hdfc\nDear Customer,\n\nGreetings from HDFC Bank.\n\nWe would like to inform you that Rs. 2440.95 has been debited from your HDFC Bank Credit Card ending 2222 towards Redbus India Private L on 01 Oct, 2026 at 10:30:05.\nTo check your available balance, outstanding amount, or view recent transactions, you may use:\n\n    Mycards:https://mycards.hdfc.bank.in\n    WhatsApp Banking:https://hdfcbk.io/HDFCBK/K/DUvfZ20acT6\n\n\nImportant Note:\nIf you did not authorise this transaction, please act immediately. Your safety is our top priority, and we are here to help.\n\nWhat You Can Do:\n\n    Report Unauthorised Transaction\n        Call HDFC Bank Helpline at 1800 258 6161\n        Submit the Online Dispute Form: Click Here\n        Email us at: cc.disputes@hdfc.bank.in\n    Report Cyber Fraud\n        Visit: https://cybercrime.gov.in\n        Call the National Cyber Crime Helpline at 1930\n        File a complaint: https://cybercrime.gov.in/Webform/Crime_AuthoLogin.aspx\n    Block Your Credit Card:\n    Via NetBanking\n        Log in with your Customer ID and Password\n        Go to the 'Cards' tab\n        Select 'Request' > 'Credit Card Hotlisting'\n    Via SMS\n        From your registered mobile number, type BLOCK CC 2222\n        Send it to 7308080808\n\n\nNeed Help?\nCall us at 1800 1600 or 1800 2600 (Toll-free, across India)\n\nThank you for banking with us.\n\nWarm Regards,\nHDFC Bank\n\n(This is a system-generated email.Please do not reply)\nFor more details on Service charges and Fees, click here.\n© HDFC Bank\n"

const hdfcCreditCardSMS = "Spent Rs.3782 On HDFC Bank Card 2222 At CONFIRM12344135 On 2026-10-28:21:19:35.Not You? To Block+Reissue Call 18002586161/SMS BLOCK CC 2222 to 7308080808"

func TestCheckTransaction(t *testing.T) {
	tests := []struct {
		name string
		msg  ports.Message
		want ports.TxnVerdict
	}{
		{"hdfc credit card email", ports.Message{Body: hdfcCreditCardEmail, Source: ports.EmailSource}, ports.TxnVerdict{IsTransaction: true, Confidence: 1}},
		{"hdfc credit card sms", ports.Message{Body: hdfcCreditCardSMS, Source: ports.SMSSource}, ports.TxnVerdict{IsTransaction: true, Confidence: 1}},
		{"unknown format", ports.Message{Body: "Your OTP is 123456", Source: ports.SMSSource}, ports.TxnVerdict{IsTransaction: false, Confidence: 0}},
	}

	adapter := NewCheckTransaction()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := adapter.IsTransaction(context.Background(), tt.msg)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
