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
			name: "an alert email has only its kind",
			raw:  "Dear Customer,Greetings from HDFC Bank!Rs.750.00 is debited from your account ending 1111 towards VPA 9000000001-3@ybl (SAMPLE PERSON NAME) on 07-10-26.",
			kind: KindEmail,
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

func TestLookupMerchant(t *testing.T) {
	cases := []struct{ channel, merchant, vpa, want string }{
		{ChannelUPI, "Sample Person", "shop@oksbi", "SHOP@OKSBI"},
		{ChannelUPI, "  sample   person. ", "", "SAMPLE PERSON"},
		{ChannelCard, "Redbus India", "shop@oksbi", "REDBUS INDIA"},
		{"", "", "", ""},
	}
	for _, c := range cases {
		if got := LookupMerchant(c.channel, c.merchant, c.vpa); got != c.want {
			t.Errorf("LookupMerchant(%q, %q, %q) = %q, want %q", c.channel, c.merchant, c.vpa, got, c.want)
		}
	}
}
