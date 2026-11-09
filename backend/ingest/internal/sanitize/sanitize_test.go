package sanitize

import (
	"strings"
	"testing"
)

func TestText(t *testing.T) {
	got := Text(`<html><head><title>T</title><style>.x{}</style></head><body><!-- note -->
<p>Rs.500&nbsp;debited</p>
<table><tr><td>Amount</td><td>500</td></tr></table>
<div style="display: none">hidden inline</div><div hidden>hidden attribute</div>
<span style="font-size:0px">hidden zero</span><span style="font-size:0.9em">small</span>
<script>alert(1)</script>
<a href="https://evil.example/x">click</a> see https://visible.example/y now</body></html>`)

	for _, want := range []string{"Rs.500 debited", "Amount 500", "small", "click"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	for _, gone := range []string{"hidden", "alert", "note", "evil.example", "visible.example", ".x{}"} {
		if strings.Contains(got, gone) {
			t.Errorf("kept %q in %q", gone, got)
		}
	}
}

func TestTextKeepsPlainText(t *testing.T) {
	if got := Text("Rs 500 debited\nfrom a/c 1234"); got != "Rs 500 debited\nfrom a/c 1234" {
		t.Fatalf("got %q", got)
	}
}

func TestTextCapsLengthOnACharacter(t *testing.T) {
	if got := Text(strings.Repeat("₹", maxChars+10)); len([]rune(got)) != maxChars {
		t.Fatalf("got %d characters", len([]rune(got)))
	}
}
