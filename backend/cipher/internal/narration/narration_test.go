package narration

import (
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name                         string
		raw                          string
		kind, key, vpa, ref, account string
		direction, amount, currency  string
		on, at                       string // "2006-01-02", "15:04:05"
		hint                         Hint
	}{
		{
			name: "upi email",
			raw:  "Dear Customer,Greetings from HDFC Bank!Rs.750.00 is debited from your account ending 1111 towards VPA 9000000001-3@ybl (SAMPLE PERSON NAME) on 07-10-26.UPI transaction reference no.: 900000000001.If you did not authorize this transaction, please report it immediately at:a. When in India (Toll free): 1800 258 6161",
			kind: KindEmail, key: "UPI | SAMPLE PERSON NAME", vpa: "9000000001-3@ybl", ref: "900000000001", account: "1111",
			direction: Debit, amount: "750.00", currency: "INR", on: "2026-10-07",
		},
		{
			name: "upi email, name with parentheses-free hyphen",
			raw:  "Dear Customer,Greetings from HDFC Bank!Rs.1790.00 is debited from your account ending 1111 towards VPA paytm.d10000000001@pty (Sample Coffee Roasters-Central Market H Block) on 02-10-26.UPI transaction reference no.: 900000000002.",
			kind: KindEmail, key: "UPI | SAMPLE COFFEE ROASTERS-CENTRAL MARKET H BLOCK", vpa: "paytm.d10000000001@pty", ref: "900000000002", account: "1111",
			direction: Debit, amount: "1790.00", currency: "INR", on: "2026-10-02",
		},
		{
			name: "upi email, older format",
			raw:  "Dear Customer, Rs.250.00 has been debited from account 1111 to VPA 9000000002-1@okbizaxis SAMPLE Clinic and Wellness Center on 06-05-26.",
			kind: KindEmail, key: "UPI | SAMPLE CLINIC AND WELLNESS CENTER", vpa: "9000000002-1@okbizaxis", account: "1111",
			direction: Debit, amount: "250.00", currency: "INR", on: "2026-05-06",
		},
		{
			name: "credit card email with time",
			raw:  "Dear Customer,Greetings from HDFC Bank.We would like to inform you that Rs. 330.00. has been debited from your HDFC Bank Credit Card ending 2222. towards TODOIST. on 08 Oct, 2026 at 16:37:04..To check your available balance",
			kind: KindEmail, key: "CARD | TODOIST", account: "2222",
			direction: Debit, amount: "330.00", currency: "INR", on: "2026-10-08", at: "16:37:04",
		},
		{
			name: "credit card email, html",
			raw:  "Dear Customer,<br><br>Greetings from HDFC Bank.<br><br>We would like to inform you that <b>Rs. 500.00</b> has been debited from your HDFC Bank Credit Card ending <b>2222</b> towards <b>CANVA* I00000-00000001</b> on <b>10 May, 2026.</b>",
			kind: KindEmail, key: "CARD | CANVA* I00000-00000001", account: "2222",
			direction: Debit, amount: "500.00", currency: "INR", on: "2026-05-10",
		},
		{
			name: "card 'thank you for using' email",
			raw:  "Dear Customer,Thank you for using your HDFC Bank Card XX2222. for Rs. 149.0. at SPOTIFY SI. on 12-06-2026 13:16:17..Authorization code:. 000001Please note",
			kind: KindEmail, key: "CARD | SPOTIFY SI", account: "2222",
			direction: Debit, amount: "149.0", currency: "INR", on: "2026-06-12", at: "13:16:17",
		},
		{
			name: "forwarded card email: header 'at' must not start the merchant",
			raw:  "From: HDFC Bank InstaAlerts. < alerts@hdfcbank.bank.in >Date: Sat, Jul 4, 2026 at 10:31 AMSubject: We noticed a transaction on your Credit CardTo: < x@gmail.com >Dear Customer,Greetings from HDFC Bank.Thank you for using your HDFC Bank Credit Card ending in 3333..You made a transaction of Rs. 222.00. at RAZ*Swiggy. on 04-07-2026 10:31:17..Authorization code:. 000002Important Note",
			kind: KindEmail, key: "CARD | RAZ*SWIGGY", account: "3333",
			direction: Debit, amount: "222.00", currency: "INR", on: "2026-07-04", at: "10:31:17",
		},
		{
			name: "e-mandate paid",
			raw:  "Dear Customer, Greetings from HDFC Bank! Your Canva Pty Ltd bill, set up through E-mandate (Auto payment), has been successfully paid using your HDFC Bank Credit Card ending 2222. Transaction Details: Amount: INR 500.00 Date: 10/06/2026 SI Hub ID: TESTSIHUB1",
			kind: KindEmail, key: "CARD | CANVA PTY LTD", account: "2222",
			direction: Debit, amount: "500.00", currency: "INR", on: "2026-06-10",
		},
		{
			name: "e-mandate upcoming, foreign currency",
			raw:  "Dear Customer, Greetings from HDFC Bank! There is an upcoming E-mandate (Auto payment) of USD 6.58 for Railway. Amount will be debited from your HDFC Bank Credit Card ending 2222 on 09/09/2026. SI Hub",
			kind: KindEmail, key: "CARD | RAILWAY", account: "2222",
			direction: Debit, amount: "6.58", currency: "USD", on: "2026-09-09",
		},
		{
			name: "nach email",
			raw:  "Dear Customer, Rs.12345.00 has been debited from HDFC Bank Account Number XXXXXXXXXX1111 towards HDFC Bank Ltd/700000001 with UMRN HDFC4600000000000001 on 05-Aug-2026. Assuring you",
			kind: KindEmail, key: "NACH | HDFC BANK LTD", account: "1111",
			direction: Debit, amount: "12345.00", currency: "INR", on: "2026-08-05",
		},
		{
			name: "credit alert has no counterparty",
			raw:  "Dear Customer,<br><br>Greetings from HDFC Bank!<br><br>We're writing to inform you that Rs.900.00 has been successfully credited to your HDFC Bank account ending in 1111.<br><br>Transaction Details: <br>a. Date: 26-05-26.",
			kind: KindEmail, key: "", account: "1111",
			direction: Credit, amount: "900.00", currency: "INR", on: "2026-05-26",
		},
		{
			name: "non-bank receipt yields nothing",
			raw:  "Payment receivedYour payment of ₹12.74 (reference CLOUD AbCdEf on Visa • • • • 2222 ) was applied to Google Cloud Platform & APIs on Oct 3, 2026.",
			kind: KindEmail,
		},
		{
			name: "card reversal email",
			raw:  "Dear Customer,Greetings from HDFC Bank!A transaction reversal of Rs. 2.00. has been initiated to your HDFC Bank Credit Card ending 2222.From Merchant:. UBER INDIA SYSTEMS PRIDate Time:. 07 Sep, 2026 at 19:15:52Please allow up to 48 hours.",
			kind: KindEmail, key: "CARD | UBER INDIA SYSTEMS PRI", account: "2222",
			direction: Credit, amount: "2.00", currency: "INR", on: "2026-09-07", at: "19:15:52",
		},
		{
			name: "card descriptor without prefix",
			raw:  "PNB*IRCTC TICKETING    GURGAON     IN",
			kind: KindStatementLine, key: "CARD | PNB*IRCTC TICKETING GURGAON IN",
		},
		{
			name: "card account: any statement description is a card merchant",
			raw:  "TODOISTTODOIST.CO",
			hint: Hint{CardAccount: true},
			kind: KindStatementLine, key: "CARD | TODOISTTODOIST.CO",
		},
		{
			name: "bank's own statement line has no key",
			raw:  "INTEREST PAID TILL 30-SEP-2026",
			kind: KindStatementLine,
		},
		{
			name: "upi statement narration, hyphen inside the vpa",
			raw:  "UPI-SAMPLE PERSON-9000000001-3@YBL-KKBK0000001-900000000001-PAYMENT FROM PHONE",
			kind: KindStatementLine, key: "UPI | SAMPLE PERSON", vpa: "9000000001-3@ybl", ref: "900000000001",
		},
		{
			name: "upi statement narration, masked vpa",
			raw:  "UPI-SAMPLE SWEETS TOWN O5-***********9999@PTY-YESB0MCHUPI-900000000003-TRUE",
			kind: KindStatementLine, key: "UPI | SAMPLE SWEETS TOWN O5", vpa: "***********9999@pty", ref: "900000000003",
		},
		{
			name: "neft credit narration",
			raw:  "NEFT CR-ICIC0099999-CENTRAL DEPOSITORY SERVICES INDIA LIMITED G-SAMPLE US ER-IN22000000000001",
			kind: KindStatementLine, key: "NEFT | CENTRAL DEPOSITORY SERVICES INDIA LIMITED G", ref: "IN22000000000001", direction: Credit,
		},
		{
			name: "imps narration",
			raw:  "IMPS-900000000004-NSDLPB SERVICE-NSPB-XXXXXXX0000-BENEVAL-HDFC0000001-XX1111",
			kind: KindStatementLine, key: "IMPS | NSDLPB SERVICE", ref: "900000000004",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := ParseWith(c.raw, c.hint)
			check := func(field, got, want string) {
				t.Helper()
				if got != want {
					t.Errorf("%s = %q, want %q", field, got, want)
				}
			}
			check("kind", p.Kind, c.kind)
			check("key", p.LookupKey(), c.key)
			check("vpa", p.VPA, c.vpa)
			check("ref", p.Ref, c.ref)
			check("account", p.AccountHint, c.account)
			check("direction", p.Direction, c.direction)
			check("amount", p.Amount, c.amount)
			check("currency", p.Currency, c.currency)
			check("on", formatTime(p.OccurredOn, "2006-01-02"), c.on)
			check("at", formatTime(p.OccurredAt, "15:04:05"), c.at)
		})
	}
}

func formatTime(t *time.Time, layout string) string {
	if t == nil {
		return ""
	}
	return t.Format(layout)
}
