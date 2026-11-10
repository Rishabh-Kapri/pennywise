package decisionsAdapter

import (
	"regexp"
	"slices"
	"strings"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils/narration"
)

const (
	noneChoice    = "none"
	maxCandidates = 12

	// A wrong ref can merge two payments and a wrong account posts to the wrong
	// place, so those need a real score. The others only affect lookup.
	minRefConfidence   = 0.8
	minFieldConfidence = 0.5
)

// These only find strings that look like the value, in any wording. The model
// decides which one is right, so a new bank template doesn't break them.
var (
	refPattern     = regexp.MustCompile(`[A-Z]{0,11}\d{6,}`)
	vpaPattern     = regexp.MustCompile(`[\w.-]+@[\w.-]+`)
	accountPattern = regexp.MustCompile(`(?:^|\D)(\d{4})(?:\D|$)`)
)

// refCandidates skips VPAs, whose digits are often a phone number.
func refCandidates(text string) []string {
	return unique(refPattern.FindAllString(vpaPattern.ReplaceAllString(text, " "), -1))
}

func vpaCandidates(text string) []string {
	found := vpaPattern.FindAllString(text, -1)
	for i := range found {
		found[i] = strings.TrimRight(found[i], ".-")
	}
	return unique(found)
}

func accountCandidates(text string) []string {
	var found []string
	for _, match := range accountPattern.FindAllStringSubmatch(text, -1) {
		found = append(found, match[1])
	}
	return unique(found)
}

func unique(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" && !slices.Contains(out, value) {
			out = append(out, value)
		}
		if len(out) == maxCandidates {
			break
		}
	}
	return out
}

// answerConfidence is the score of a choice answer. known is false when the
// model sent none.
func answerConfidence(ans sharedModel.Answer) (score float64, known bool) {
	if ans.Confidence != nil {
		return *ans.Confidence, true
	}
	if ans.Choice != nil {
		score, known = ans.Probabilities[*ans.Choice]
	}
	return score, known
}

// pickCandidate returns the chosen candidate, or "" when the model chose none,
// refused, chose something that wasn't offered or wasn't sure enough. A blank
// is always safe: the field just stays unknown.
func pickCandidate(ans sharedModel.Answer, candidates []string, minConfidence float64, needScore bool) string {
	if ans.Choice == nil || !slices.Contains(candidates, *ans.Choice) {
		return ""
	}
	score, known := answerConfidence(ans)
	if (needScore && !known) || (known && score < minConfidence) {
		return ""
	}
	return *ans.Choice
}

func pickChannel(ans sharedModel.Answer) string {
	channels := []string{
		narration.ChannelUPI, narration.ChannelCard, narration.ChannelNEFT, narration.ChannelIMPS, narration.ChannelNACH,
	}
	return pickCandidate(ans, channels, minFieldConfidence, false)
}
