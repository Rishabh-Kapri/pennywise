package main

import (
	"fmt"
	"math"
)

// alias is one known merchant of a budget.
type alias struct {
	Budget    string
	Merchant  string
	PayeeID   string
	PayeeName string
}

// scoreFn says how alike aliases i and j are. Higher is closer.
type scoreFn func(i, j int) float64

// result is how one method did at one threshold.
//
// Each alias is treated as a new merchant and matched against the budget's
// others. If another alias shares its payee, the right answer is one of those;
// if none does, the right answer is no match.
type result struct {
	Threshold float64
	Correct   int // matched an alias with the right payee
	Wrong     int // matched an alias with another payee
	Missed    int // had a right answer, but nothing was close enough
	Rejected  int // had no right answer, and correctly matched nothing
}

// Precision is how often a match was right. 1 when nothing matched.
func (r result) Precision() float64 {
	if r.Correct+r.Wrong == 0 {
		return 1
	}
	return float64(r.Correct) / float64(r.Correct+r.Wrong)
}

// Coverage is how many of the findable merchants were found.
func (r result) Coverage(findable int) float64 {
	if findable == 0 {
		return 0
	}
	return float64(r.Correct) / float64(findable)
}

// match is the closest other alias to one alias.
type match struct {
	Query, Candidate int // Candidate is -1 when there's nothing to compare with
	Score            float64
}

// bestMatches finds each alias's closest other alias. Aliases with the same
// merchant text (the same name on another channel) are skipped: an exact match
// would find those, and this measures near-misses.
func bestMatches(aliases []alias, score scoreFn) []match {
	matches := make([]match, len(aliases))
	for i := range aliases {
		best := match{Query: i, Candidate: -1, Score: math.Inf(-1)}
		for j := range aliases {
			if i == j || aliases[i].Budget != aliases[j].Budget || aliases[i].Merchant == aliases[j].Merchant {
				continue
			}
			if s := score(i, j); s > best.Score {
				best.Candidate, best.Score = j, s
			}
		}
		matches[i] = best
	}
	return matches
}

// findable marks the aliases that have a right answer to find.
func findable(aliases []alias) []bool {
	out := make([]bool, len(aliases))
	for i := range aliases {
		for j := range aliases {
			if i != j && aliases[i].Budget == aliases[j].Budget &&
				aliases[i].Merchant != aliases[j].Merchant && aliases[i].PayeeID == aliases[j].PayeeID {
				out[i] = true
				break
			}
		}
	}
	return out
}

func evaluate(aliases []alias, matches []match, thresholds []float64) []result {
	canFind := findable(aliases)
	results := make([]result, 0, len(thresholds))
	for _, threshold := range thresholds {
		r := result{Threshold: threshold}
		for _, m := range matches {
			switch {
			case m.Candidate >= 0 && m.Score >= threshold:
				if aliases[m.Candidate].PayeeID == aliases[m.Query].PayeeID {
					r.Correct++
				} else {
					r.Wrong++
				}
			case canFind[m.Query]:
				r.Missed++
			default:
				r.Rejected++
			}
		}
		results = append(results, r)
	}
	return results
}

// bestSafe is the threshold finding the most merchants at or above
// minPrecision. A wrong match files a transaction under the wrong payee, so
// precision is the constraint and coverage the goal. ok is false when no
// threshold that matches anything is precise enough.
func bestSafe(results []result, minPrecision float64) (result, bool) {
	var best result
	found := false
	for _, r := range results {
		if r.Correct > 0 && r.Precision() >= minPrecision && (!found || r.Correct > best.Correct) {
			best, found = r, true
		}
	}
	return best, found
}

func cosine(a, b []float64) float64 {
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func printResults(name string, results []result, total int) {
	fmt.Printf("\n%s\n", name)
	fmt.Printf("%9s %8s %6s %7s %9s %10s %9s\n", "threshold", "correct", "wrong", "missed", "rejected", "precision", "coverage")
	for _, r := range results {
		fmt.Printf("%9.2f %8d %6d %7d %9d %9.0f%% %8.0f%%\n",
			r.Threshold, r.Correct, r.Wrong, r.Missed, r.Rejected, r.Precision()*100, r.Coverage(total)*100)
	}
}
