package service

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils/narration"

	"github.com/google/uuid"
)

// Card statements mix purchases with the bank's own lines: GST, fees,
// interest, and the instalments of purchases converted to EMI. These rules
// tell them apart by their text, so the screen can total the charges, suggest
// a category for them, and keep EMI instalments out of "missing".

var (
	taxRe        = regexp.MustCompile(`(?i)\b(?:igst|cgst|sgst|utgst|gst|service\s+tax)\b`)
	interestRe   = regexp.MustCompile(`(?i)\b(?:interest|finance\s+charges?|fin\.?\s*chg)\b`)
	feeRe        = regexp.MustCompile(`(?i)\b(?:fees?|charges?|chg|chgs|mark-?up|surcharge|penalty|late\s+payment|over\s*limit|membership|renewal|joining)\b`)
	emiRe        = regexp.MustCompile(`(?i)\b(?:emi|e\.m\.i\.?|instal+ments?|amorti[sz]ation|loan\s+on\s+card)\b`)
	emiNumRe     = regexp.MustCompile(`(?i)\b(\d{1,3})\s*(?:/|of|out\s+of)\s*(\d{1,3})\b`)
	principalRe  = regexp.MustCompile(`(?i)\bprin(?:cipal|\.)?\b`)
	conversionRe = regexp.MustCompile(`(?i)\bconver(?:t|ted|sion)\b`)
)

// EMI parts.
const (
	emiPartInstalment = "instalment"
	emiPartPrincipal  = "principal"
	emiPartInterest   = "interest"
	// emiPartConversion is the credit that cancels a purchase when it is
	// turned into an EMI; the instalments then charge it back.
	emiPartConversion = "conversion"
)

// classifyStatementRow sets ChargeKind and EMI from the row's text. Payments
// between accounts (UPI, NEFT, IMPS, NACH) are never charges. EMI lines are
// only read on card statements: a bank account's loan EMI is a real payment.
func classifyStatementRow(r *model.StatementRow, card bool) {
	r.ChargeKind, r.EMI = "", nil
	switch r.Channel {
	case narration.ChannelUPI, narration.ChannelNEFT, narration.ChannelIMPS, narration.ChannelNACH:
		return
	}
	text := r.Description
	if r.RawDescription != "" {
		text = r.RawDescription
	}

	// Tax first: "IGST ON EMI INTEREST" is a tax, not an instalment.
	if taxRe.MatchString(text) {
		r.ChargeKind = model.StatementChargeTax
		return
	}
	if card && emiRe.MatchString(text) {
		line := &model.StatementEMILine{Part: emiPartInstalment}
		switch {
		case interestRe.MatchString(text):
			line.Part = emiPartInterest
			r.ChargeKind = model.StatementChargeInterest
		case conversionRe.MatchString(text):
			line.Part = emiPartConversion
		case principalRe.MatchString(text):
			line.Part = emiPartPrincipal
		}
		if m := emiNumRe.FindStringSubmatch(text); m != nil {
			n, _ := strconv.Atoi(m[1])
			of, _ := strconv.Atoi(m[2])
			if n > 0 && of >= n {
				line.Number, line.Of = n, of
			}
		}
		r.EMI = line
		return
	}
	// Interest paid to a savings account is income, not a charge.
	if interestRe.MatchString(text) && (card || r.Direction == model.StatementDebit) {
		r.ChargeKind = model.StatementChargeInterest
		return
	}
	if feeRe.MatchString(text) {
		r.ChargeKind = model.StatementChargeFee
	}
}

// coveredByEMI reports whether a row is part of an EMI the budget already
// paid in full when it was bought: its instalments, their principal, and the
// conversion credit. EMI interest is a cost, so it isn't covered.
func coveredByEMI(r model.StatementRow) bool {
	return r.EMI != nil && r.EMI.Part != emiPartInterest
}

// statementCharges totals each kind of charge; a reversal (credit) counts
// against its kind.
func statementCharges(rows []model.StatementRow) []model.StatementChargeTotal {
	byKind := map[string]*model.StatementChargeTotal{}
	for _, r := range rows {
		if r.ChargeKind == "" {
			continue
		}
		t := byKind[r.ChargeKind]
		if t == nil {
			t = &model.StatementChargeTotal{Kind: r.ChargeKind}
			byKind[r.ChargeKind] = t
		}
		t.Amount = roundCents(t.Amount - r.SignedAmount())
		t.Count++
	}
	out := make([]model.StatementChargeTotal, 0, len(byKind))
	for _, kind := range []string{model.StatementChargeInterest, model.StatementChargeFee, model.StatementChargeTax} {
		if t := byKind[kind]; t != nil {
			out = append(out, *t)
		}
	}
	return out
}

// rowRewardPoints adds up the points printed against rows, nil when none.
func rowRewardPoints(rows []model.StatementRow) *int {
	total, any := 0, false
	for _, r := range rows {
		if r.RewardPoints != nil {
			total += *r.RewardPoints
			any = true
		}
	}
	if !any {
		return nil
	}
	return &total
}

// chargeCategoryWords are the category name fragments, in order of
// preference, that suit each kind of charge.
var chargeCategoryWords = map[string][]string{
	model.StatementChargeTax:      {"gst", "tax", "bank charge", "charges", "fees", "fee"},
	model.StatementChargeInterest: {"interest", "bank charge", "charges", "fees", "fee"},
	model.StatementChargeFee:      {"bank charge", "card fee", "charges", "fees", "fee"},
}

// chargeCategory picks the budget's category for a kind of charge by name,
// or nil when none fits. Categories are tried in name order, so the pick is
// stable.
func chargeCategory(kind string, categories map[uuid.UUID]string) *uuid.UUID {
	type named struct {
		id   uuid.UUID
		name string
	}
	list := make([]named, 0, len(categories))
	for id, name := range categories {
		list = append(list, named{id, strings.ToLower(name)})
	}
	sort.Slice(list, func(a, b int) bool { return list[a].name < list[b].name })
	for _, word := range chargeCategoryWords[kind] {
		for _, c := range list {
			if strings.Contains(c.name, word) {
				id := c.id
				return &id
			}
		}
	}
	return nil
}
