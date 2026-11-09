package factory

import (
	"fmt"

	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/pipeline"
)

// The describe helpers turn rows into short strings for logs. Model rows carry
// the encrypted API key, so never pass a row to a logger.

func adapterIDs(step pipeline.Step) []string {
	ids := make([]string, 0, len(step.Adapters))
	for _, adapter := range step.Adapters {
		ids = append(ids, adapter.ID)
	}
	return ids
}

func describeOverride(o *sharedModel.PipelineStepOverride) string {
	if o == nil {
		return "none"
	}
	adapter, modelID, fallback := "default", "default", "default"
	if o.Adapter != nil {
		adapter = *o.Adapter
	}
	if o.ModelID != nil {
		modelID = o.ModelID.String()
	}
	if o.Fallback != nil {
		fallback = fmt.Sprint(*o.Fallback)
	}
	return fmt.Sprintf("adapter=%s model=%s fallback=%s", adapter, modelID, fallback)
}

func describeModels(models []sharedModel.UserLLMModel) []string {
	out := make([]string, 0, len(models))
	for _, m := range models {
		out = append(out, fmt.Sprintf(
			"%s (%s, %s key, default=%t, priority=%d)", m.Slug, m.Kind, m.Provider, m.IsDefault, m.Priority,
		))
	}
	return out
}

func describeAttempt(attempt pipeline.Attempt) string {
	if attempt.Model == nil {
		return attempt.Adapter
	}
	return attempt.Adapter + ":" + attempt.Model.Slug
}

func describeAttempts(attempts []pipeline.Attempt) []string {
	out := make([]string, 0, len(attempts))
	for _, attempt := range attempts {
		out = append(out, describeAttempt(attempt))
	}
	return out
}

func linkNames[P any](links []chainLink[P]) []string {
	names := make([]string, 0, len(links))
	for _, link := range links {
		names = append(names, link.name)
	}
	return names
}
