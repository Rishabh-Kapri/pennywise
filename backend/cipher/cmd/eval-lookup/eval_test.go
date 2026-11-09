package main

import "testing"

func fixture() ([]alias, scoreFn) {
	aliases := []alias{
		{Budget: "b", Merchant: "SWIGGY", PayeeID: "swiggy"},
		{Budget: "b", Merchant: "SWIGGY LIMITED", PayeeID: "swiggy"},
		{Budget: "b", Merchant: "ZOMATO", PayeeID: "zomato"},
		{Budget: "b", Merchant: "RELIANCE FRESH", PayeeID: "reliance"},
	}
	score := func(i, j int) float64 {
		if i > j {
			i, j = j, i
		}
		switch {
		case i == 0 && j == 1:
			return 0.9
		case i == 0 && j == 2:
			return 0.5
		case i == 2 && j == 3:
			return 0.6
		}
		return 0.1
	}
	return aliases, score
}

func TestEvaluate(t *testing.T) {
	aliases, score := fixture()
	results := evaluate(aliases, bestMatches(aliases, score), []float64{0.55, 0.95})

	loose := results[0]
	if loose.Correct != 2 || loose.Wrong != 2 || loose.Missed != 0 || loose.Rejected != 0 {
		t.Fatalf("0.55: %+v", loose)
	}
	strict := results[1]
	if strict.Correct != 0 || strict.Wrong != 0 || strict.Missed != 2 || strict.Rejected != 2 {
		t.Fatalf("0.95: %+v", strict)
	}
}

func TestSameMerchantOnAnotherChannelIsNotANearMiss(t *testing.T) {
	aliases := []alias{
		{Budget: "b", Merchant: "SWIGGY", PayeeID: "swiggy"},
		{Budget: "b", Merchant: "SWIGGY", PayeeID: "swiggy"},
	}
	matches := bestMatches(aliases, func(int, int) float64 { return 1 })
	if matches[0].Candidate != -1 {
		t.Fatalf("matched %+v", matches[0])
	}
	if findable(aliases)[0] {
		t.Fatal("nothing else to find")
	}
}

func TestBudgetsStaySeparate(t *testing.T) {
	aliases := []alias{
		{Budget: "a", Merchant: "SWIGGY", PayeeID: "p1"},
		{Budget: "b", Merchant: "SWIGGY LTD", PayeeID: "p1"},
	}
	if bestMatches(aliases, func(int, int) float64 { return 1 })[0].Candidate != -1 {
		t.Fatal("matched across budgets")
	}
}

func TestBestSafePicksMostFindsAboveThePrecisionBar(t *testing.T) {
	results := []result{
		{Threshold: 0.3, Correct: 10, Wrong: 10},
		{Threshold: 0.6, Correct: 8, Wrong: 0},
		{Threshold: 0.9, Correct: 3, Wrong: 0},
	}
	best, ok := bestSafe(results, 0.95)
	if !ok || best.Threshold != 0.6 {
		t.Fatalf("best = %+v, %v", best, ok)
	}
	if _, ok := bestSafe([]result{{Correct: 1, Wrong: 1}}, 0.95); ok {
		t.Fatal("nothing should pass")
	}
}
