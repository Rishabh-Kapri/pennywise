package model

import "testing"

func TestInboundMessageStatusCanMoveTo(t *testing.T) {
	all := []InboundMessageStatus{
		InboundMessageProcessing, InboundMessageNeedsReview, InboundMessageDone, InboundMessageDismissed,
	}
	allowed := map[[2]InboundMessageStatus]bool{
		{InboundMessageProcessing, InboundMessageNeedsReview}: true,
		{InboundMessageProcessing, InboundMessageDone}:        true,
		{InboundMessageProcessing, InboundMessageDismissed}:   true,
		{InboundMessageNeedsReview, InboundMessageProcessing}: true,
		{InboundMessageNeedsReview, InboundMessageDismissed}:  true,
		{InboundMessageDone, InboundMessageProcessing}:        true,
		{InboundMessageDone, InboundMessageDismissed}:         true,
	}
	for _, from := range all {
		for _, to := range all {
			want := allowed[[2]InboundMessageStatus{from, to}]
			if got := from.CanMoveTo(to); got != want {
				t.Errorf("%s -> %s: CanMoveTo = %v, want %v", from, to, got, want)
			}
		}
	}
}

func TestMessageOutcomeStatusAfter(t *testing.T) {
	if got := MessageOutcomeNeedsReview.StatusAfter(); got != InboundMessageNeedsReview {
		t.Errorf("needs_review -> %s", got)
	}
	for _, o := range []MessageOutcome{
		MessageOutcomeNotTransaction, MessageOutcomeUnverified, MessageOutcomeNoPrediction,
		MessageOutcomeTransactionCreated, MessageOutcomeTransactionDuplicate,
	} {
		if got := o.StatusAfter(); got != InboundMessageDone {
			t.Errorf("%s -> %s, want done", o, got)
		}
	}
}

func TestReceivedMessageKeyIgnoresRunID(t *testing.T) {
	m := ReceivedMessage{Source: "sms", Provider: "android", ExternalID: "x"}
	again := m
	again.RunID = "review:123"
	if m.Key([16]byte{1}) != again.Key([16]byte{1}) {
		t.Fatal("a continued run must keep the message's key")
	}
}
