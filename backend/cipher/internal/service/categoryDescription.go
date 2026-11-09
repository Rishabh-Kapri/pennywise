package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

// maxDescriptionRunes caps a suggested description. The prompt asks for well
// under this; the cap only guards against a model that ignores it.
const maxDescriptionRunes = 280

const categoryDescriptionPrompt = `You write short descriptions for budget categories in a personal finance app.

You are given a category, the group it belongs to, the other categories in that group, and the payees its transactions most often go to. Write one or two plain sentences (at most 35 words) that tell the user what spending belongs in this category.

Rules:
- Describe the kind of spending, not the data you were given. Mention a payee only as an example ("e.g. BigBasket").
- When a sibling category could be confused with this one, say what belongs there instead.
- If there are no payees, describe the category from its name and group alone.
- No markdown, no emoji, no quotation marks, no preamble.

Respond with JSON only: {"description": "..."}`

type categoryDescriptionResult struct {
	Description string `json:"description"`
}

// SuggestCategoryDescription drafts a description for a budget category with
// the same provider chain as the email pipeline, so an Ollama outage falls
// through to the hosted provider here too.
func (s *predictionService) SuggestCategoryDescription(
	ctx context.Context,
	req sharedModel.CategoryDescriptionContext,
) (string, error) {
	if strings.TrimSpace(req.CategoryName) == "" {
		return "", errs.New(errs.CodeInvalidArgument, "category name is required")
	}

	res, err := s.chatWithFallback(ctx, "category:describe", func(model string) sharedModel.ChatRequest {
		return sharedModel.ChatRequest{
			Model: model,
			Messages: []sharedModel.AgentMessage{
				{
					Role:    sharedModel.RoleSystem,
					Content: []sharedModel.ContentBlock{{Type: "text", Text: categoryDescriptionPrompt}},
				},
				{
					Role:    sharedModel.RoleUser,
					Content: []sharedModel.ContentBlock{{Type: "text", Text: categoryDescriptionInput(req)}},
				},
			},
			Temperature: 0.4,
			MaxTokens:   400,
			Stream:      false,
			Format:      "json",
		}
	})
	if err != nil {
		return "", err
	}
	if res == nil || len(res.Message.Content) == 0 {
		return "", errs.New(errs.CodeInternalError, "category description: no content returned")
	}

	parsed, err := utils.UnmarshalResponse[categoryDescriptionResult]([]byte(res.Message.Content[0].Text))
	if err != nil {
		return "", err
	}
	description := cleanDescription(parsed.Description)
	if description == "" {
		return "", errs.New(errs.CodeInternalError, "category description: empty suggestion")
	}
	return description, nil
}

func categoryDescriptionInput(req sharedModel.CategoryDescriptionContext) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Category: %s\n", req.CategoryName)
	if req.GroupName != "" {
		fmt.Fprintf(&b, "Group: %s\n", req.GroupName)
	}
	if len(req.SiblingCategories) > 0 {
		fmt.Fprintf(&b, "Other categories in this group: %s\n", strings.Join(req.SiblingCategories, ", "))
	}
	if len(req.TopPayees) > 0 {
		fmt.Fprintf(&b, "Most frequent payees: %s\n", strings.Join(req.TopPayees, ", "))
	} else {
		b.WriteString("Most frequent payees: none yet\n")
	}
	if req.CurrentDescription != "" {
		fmt.Fprintf(&b, "Current description (write a better one): %s\n", req.CurrentDescription)
	}
	return b.String()
}

// cleanDescription normalises a model's description: one line, no wrapping
// quotes, and capped at maxDescriptionRunes on a word boundary.
func cleanDescription(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	text = strings.Trim(text, "\"'`“”‘’ ")
	if utf8.RuneCountInString(text) <= maxDescriptionRunes {
		return text
	}
	runes := []rune(text)[:maxDescriptionRunes]
	cut := string(runes)
	if i := strings.LastIndex(cut, " "); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, ",;:- ") + "…"
}
