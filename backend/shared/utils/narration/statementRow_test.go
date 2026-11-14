package narration

import "testing"

// Made-up narrations in the shapes banks print.

func TestParseStatementRowUPITail(t *testing.T) {
	line := ParseStatementRow(
		"UPI-Example Coffee Ro-paytm.d12345678901@pty-YESB0MCHUPI-111122223333-Payment from Phone Value Dt 02/10/2026 Ref 111122223333",
		Hint{},
	)
	if line.Clean != "UPI-Example Coffee Ro-paytm.d12345678901@pty-YESB0MCHUPI-111122223333-Payment from Phone" {
		t.Errorf("clean = %q", line.Clean)
	}
	if line.ValueDate != "2026-10-02" {
		t.Errorf("value date = %q", line.ValueDate)
	}
	if line.Channel != ChannelUPI || line.Ref != "111122223333" || line.VPA != "paytm.d12345678901@pty" {
		t.Errorf("parsed = %+v", line.Parsed)
	}
	if line.Merchant != "EXAMPLE COFFEE RO" {
		t.Errorf("merchant = %q", line.Merchant)
	}
}

func TestParseStatementRowWrapSpaces(t *testing.T) {
	// The OCR joined wrapped lines with spaces inside the IFSC and address.
	line := ParseStatementRow(
		"UPI-EXAMPLE CAFE-Getepay.abc 123@icici-ICIC0 DC0099-444455556666-Payment from Phone Value Dt 01/10/2026 Ref 444455556666",
		Hint{},
	)
	if line.Channel != ChannelUPI || line.Ref != "444455556666" || line.VPA != "getepay.abc123@icici" {
		t.Errorf("parsed = %+v", line.Parsed)
	}
	if line.Merchant != "EXAMPLE CAFE" {
		t.Errorf("merchant = %q", line.Merchant)
	}
}

func TestParseStatementRowGenericRef(t *testing.T) {
	line := ParseStatementRow("FUNDS TRANSFER TO SAVINGS RRN: AB12CD345678", Hint{})
	if line.Ref != "AB12CD345678" || line.Clean != "FUNDS TRANSFER TO SAVINGS" {
		t.Errorf("ref/clean = %q/%q", line.Ref, line.Clean)
	}
}

func TestParseStatementRowMidRefKept(t *testing.T) {
	// A "ref" in the middle is part of the narration, not its tail.
	line := ParseStatementRow("REFUND ref 123456 FOR ORDER EXAMPLE SHOP", Hint{})
	if line.Clean != "REFUND ref 123456 FOR ORDER EXAMPLE SHOP" || line.Ref != "" {
		t.Errorf("clean/ref = %q/%q", line.Clean, line.Ref)
	}
}

func TestParseStatementRowCard(t *testing.T) {
	line := ParseStatementRow("EXAMPLE STORE MUMBAI", Hint{CardAccount: true})
	if line.Channel != ChannelCard || line.Merchant != "EXAMPLE STORE MUMBAI" || line.Clean != "EXAMPLE STORE MUMBAI" {
		t.Errorf("line = %+v", line)
	}
}
