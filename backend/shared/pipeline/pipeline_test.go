package pipeline

import (
	"strings"
	"testing"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/google/uuid"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func ptr[T any](v T) *T { return &v }

func TestEmbeddedPipelinesLoad(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	msg, ok := c.Pipeline(MessagePipeline)
	if !ok {
		t.Fatal("no message pipeline")
	}
	for _, id := range []StepID{StepCheck, StepExtract, StepVerify, StepLookup, StepPredict, StepCreate} {
		if _, ok := msg.Step(id); !ok {
			t.Errorf("message pipeline has no %q step", id)
		}
	}
	stmt, ok := c.Pipeline(StatementPipeline)
	if !ok {
		t.Fatal("no statement pipeline")
	}
	for _, id := range []StepID{StepStatementExtract, StepStatementFallback, StepStatementSave} {
		if _, ok := stmt.Step(id); !ok {
			t.Errorf("statement pipeline has no %q step", id)
		}
	}
}

func mustSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	s, err := compileSchema()
	if err != nil {
		t.Fatalf("compileSchema: %v", err)
	}
	return s
}

const validPipeline = `{
  "id": "demo", "label": "Demo", "description": "", "version": 1,
  "steps": [{
    "id": "one", "label": "One", "description": "", "defaultAdapter": "a",
    "adapters": [{"id": "a", "label": "A", "description": ""}]
  }]
}`

func TestParse(t *testing.T) {
	schema := mustSchema(t)
	tests := []struct {
		name string
		json string
		want string // part of the error; "" = valid
	}{
		{name: "valid", json: validPipeline},
		{
			name: "unknown key",
			json: strings.Replace(validPipeline, `"version": 1`, `"version": 1, "stepz": []`, 1),
			want: "stepz",
		},
		{name: "bad id", json: strings.Replace(validPipeline, `"id": "one"`, `"id": "One Step"`, 1), want: "pattern"},
		{
			name: "missing default",
			json: strings.Replace(validPipeline, `"defaultAdapter": "a"`, `"defaultAdapter": "b"`, 1),
			want: "isn't in its list",
		},
		{
			name: "maxTokens without usesModel",
			json: strings.Replace(validPipeline, `"label": "A",`, `"label": "A", "maxTokens": 10,`, 1),
			want: "usesModel",
		},
		{
			name: "preferredModels without usesModel",
			json: strings.Replace(
				validPipeline,
				`"label": "A",`,
				`"label": "A", "preferredModels": [{"provider": "ollama", "slug": "glm-ocr"}],`,
				1,
			),
			want: "usesModel",
		},
		{
			name: "rerunOf a later step",
			json: strings.Replace(validPipeline, `"defaultAdapter": "a",`, `"defaultAdapter": "a", "rerunOf": "two",`, 1),
			want: "earlier step",
		},
		{
			name: "duplicate step",
			json: strings.Replace(
				validPipeline,
				`"steps": [{`,
				`"steps": [{"id": "one", "label": "Dup", "description": "", "defaultAdapter": "a", "adapters": [{"id": "a", "label": "A", "description": ""}]}, {`,
				1,
			),
			want: "twice",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parse(schema, []byte(tt.json))
			if tt.want == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want an error containing %q, got %v", tt.want, err)
			}
			if !errs.HasCode(err, errs.CodePipelineInvalid) {
				t.Fatalf("want code %s, got %v", errs.CodePipelineInvalid, err)
			}
		})
	}
}

func TestBuildViewKeepsPipelinesApart(t *testing.T) {
	p := &Pipeline{ID: "message", Steps: []Step{check}}
	overrides := []model.PipelineStepOverride{
		{Pipeline: "statement", Step: "check", Adapter: ptr("llm")},
	}
	view := BuildView(p, overrides, models)
	if view.Steps[0].Override != nil {
		t.Fatal("another pipeline's override was applied")
	}
}

func TestParseRequiresModelKind(t *testing.T) {
	schema := mustSchema(t)
	noKind := strings.Replace(validPipeline, `"label": "A",`, `"label": "A", "usesModel": true,`, 1)
	if _, err := parse(schema, []byte(noKind)); err == nil || !strings.Contains(err.Error(), "modelKind") {
		t.Fatalf("want a modelKind error, got %v", err)
	}
	kindWithoutModel := strings.Replace(validPipeline, `"label": "A",`, `"label": "A", "modelKind": "chat",`, 1)
	if _, err := parse(schema, []byte(kindWithoutModel)); err == nil {
		t.Fatal("want an error for modelKind without usesModel")
	}
}

var check = Step{
	ID:             StepCheck,
	Label:          "Check",
	DefaultAdapter: "decisions",
	Adapters: []Adapter{
		{ID: "decisions", Label: "Decisions", UsesModel: true, ModelKind: model.LLMModelDecision},
		{ID: "llm", Label: "Your AI", UsesModel: true, ModelKind: model.LLMModelChat},
		{ID: "regex", Label: "Keywords"},
		{ID: "future", Label: "Later", Unavailable: true},
	},
}

var (
	sonnet = model.UserLLMModel{
		ID:        uuid.New(),
		Kind:      model.LLMModelChat,
		Provider:  "anthropic",
		Slug:      "claude-sonnet",
		IsDefault: true,
	}
	qwen     = model.UserLLMModel{ID: uuid.New(), Kind: model.LLMModelChat, Provider: "ollama", Slug: "qwen3:8b"}
	decision = model.UserLLMModel{
		ID:        uuid.New(),
		Kind:      model.LLMModelDecision,
		Provider:  "openrouter",
		Slug:      "classify-v1",
		IsDefault: true,
	}
	models = []model.UserLLMModel{sonnet, decision, qwen}
)

func TestResolve(t *testing.T) {
	tests := []struct {
		name     string
		override *model.PipelineStepOverride
		models   []model.UserLLMModel
		want     Choice
		warn     bool
	}{
		{name: "default adapter, default decision model", want: Choice{Adapter: "decisions", Model: refOf(decision)}},
		{
			name:     "llm uses the default chat model",
			override: &model.PipelineStepOverride{Adapter: ptr("llm")},
			want:     Choice{Adapter: "llm", Model: refOf(sonnet)},
		},
		{
			name:     "chosen chat model",
			override: &model.PipelineStepOverride{Adapter: ptr("llm"), ModelID: &qwen.ID},
			want:     Choice{Adapter: "llm", Model: refOf(qwen)},
		},
		{
			name:     "no model for a non-model adapter",
			override: &model.PipelineStepOverride{Adapter: ptr("regex")},
			want:     Choice{Adapter: "regex"},
		},
		{
			name:     "retired adapter falls back",
			override: &model.PipelineStepOverride{Adapter: ptr("gone")},
			want:     Choice{Adapter: "decisions", Model: refOf(decision)},
			warn:     true,
		},
		{
			name:     "deleted model falls back to the default",
			override: &model.PipelineStepOverride{Adapter: ptr("llm"), ModelID: ptr(uuid.New())},
			want:     Choice{Adapter: "llm", Model: refOf(sonnet)},
			warn:     true,
		},
		{
			name:     "model of the wrong kind falls back",
			override: &model.PipelineStepOverride{Adapter: ptr("llm"), ModelID: &decision.ID},
			want:     Choice{Adapter: "llm", Model: refOf(sonnet)},
			warn:     true,
		},
		{
			name:   "no decision model set up",
			models: []model.UserLLMModel{sonnet},
			want:   Choice{Adapter: "decisions"},
			warn:   true,
		},
		{
			name:     "non-default stands in when the default's key is disabled",
			models:   []model.UserLLMModel{qwen},
			override: &model.PipelineStepOverride{Adapter: ptr("llm")},
			want:     Choice{Adapter: "llm", Model: refOf(qwen)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := tt.models
			if ms == nil {
				ms = models
			}
			got := Resolve(check, tt.override, ms)
			if got.Effective.Adapter != tt.want.Adapter || !sameRef(got.Effective.Model, tt.want.Model) {
				t.Errorf("effective = %+v, want %+v", got.Effective, tt.want)
			}
			if (got.Warning != "") != tt.warn {
				t.Errorf("warning = %q, want one: %v", got.Warning, tt.warn)
			}
		})
	}
}

func sameRef(a, b *ModelRef) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func TestResolveFallback(t *testing.T) {
	on, off := true, false
	withDefault := check
	withDefault.Fallback = true
	regexOnly := &model.PipelineStepOverride{Adapter: ptr("regex"), Fallback: &on}

	tests := []struct {
		name     string
		step     Step
		override *model.PipelineStepOverride
		want     bool
	}{
		{name: "off by default", step: check},
		{name: "step default applies", step: withDefault, want: true},
		{name: "budget turns it on", step: check, override: &model.PipelineStepOverride{Fallback: &on}, want: true},
		{name: "budget turns it off", step: withDefault, override: &model.PipelineStepOverride{Fallback: &off}},
		{name: "never for an adapter without a model", step: withDefault, override: regexOnly},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Resolve(tt.step, tt.override, models).Effective.Fallback; got != tt.want {
				t.Fatalf("Fallback = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAttempts(t *testing.T) {
	on := true
	clef := model.UserLLMModel{ID: uuid.New(), Kind: model.LLMModelDecision, Slug: "cloudflare/clef", IsDefault: true}
	jev := model.UserLLMModel{ID: uuid.New(), Kind: model.LLMModelDecision, Slug: "typesafe/jev-latest"}
	// As ListUsableForBudget returns them: default first, then by priority.
	usable := []model.UserLLMModel{clef, jev, sonnet}

	withFallback := func(o model.PipelineStepOverride) *model.PipelineStepOverride {
		o.Fallback = &on
		return &o
	}
	removed := uuid.New()

	tests := []struct {
		name     string
		override *model.PipelineStepOverride
		models   []model.UserLLMModel
		adapter  string
		want     []string // model slugs in order
	}{
		{
			name:    "fallback off runs the default alone",
			models:  usable,
			adapter: "decisions",
			want:    []string{"cloudflare/clef"},
		},
		{
			name:     "fallback on adds the other models of the kind",
			override: withFallback(model.PipelineStepOverride{}),
			models:   usable, adapter: "decisions",
			want: []string{"cloudflare/clef", "typesafe/jev-latest"},
		},
		{
			name:     "a picked model goes ahead of the default",
			override: withFallback(model.PipelineStepOverride{ModelID: &jev.ID}),
			models:   usable, adapter: "decisions",
			want: []string{"typesafe/jev-latest", "cloudflare/clef"},
		},
		{
			name:     "a picked model runs alone with fallback off",
			override: &model.PipelineStepOverride{ModelID: &jev.ID},
			models:   usable, adapter: "decisions",
			want: []string{"typesafe/jev-latest"},
		},
		{
			name:     "a removed pick starts from the default",
			override: withFallback(model.PipelineStepOverride{ModelID: &removed}),
			models:   usable, adapter: "decisions",
			want: []string{"cloudflare/clef", "typesafe/jev-latest"},
		},
		{
			name:     "only the adapter's kind is used",
			override: withFallback(model.PipelineStepOverride{Adapter: ptr("llm")}),
			models:   usable, adapter: "llm",
			want: []string{"claude-sonnet"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Attempts(check, tt.override, tt.models)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d attempts, want %d: %+v", len(got), len(tt.want), got)
			}
			for i, attempt := range got {
				if attempt.Adapter != tt.adapter || attempt.Model == nil || attempt.Model.Slug != tt.want[i] {
					t.Fatalf("attempt %d = %+v, want %s with %s", i, attempt, tt.adapter, tt.want[i])
				}
			}
		})
	}

	t.Run("an adapter without a model is one attempt", func(t *testing.T) {
		got := Attempts(check, &model.PipelineStepOverride{Adapter: ptr("regex")}, usable)
		if len(got) != 1 || got[0].Adapter != "regex" || got[0].Model != nil {
			t.Fatalf("got %+v", got)
		}
	})
	t.Run("no model of the kind is no attempts", func(t *testing.T) {
		if got := Attempts(check, nil, []model.UserLLMModel{sonnet}); got != nil {
			t.Fatalf("got %+v", got)
		}
	})
}

func TestValidateOverrideRejectsFallbackWithoutModel(t *testing.T) {
	on := true
	o := model.PipelineStepOverride{Adapter: ptr("regex"), Fallback: &on}
	if err := ValidateOverride(check, o, models); !errs.HasCode(err, errs.CodePipelineInvalidOverride) {
		t.Fatalf("want %s, got %v", errs.CodePipelineInvalidOverride, err)
	}
}

func TestValidateOverride(t *testing.T) {
	locked := Step{
		ID:             StepCreate,
		Label:          "Create",
		DefaultAdapter: "api",
		Adapters:       []Adapter{{ID: "api", Label: "API"}},
	}
	tests := []struct {
		name string
		step Step
		o    model.PipelineStepOverride
		ok   bool
	}{
		{name: "switch adapter", step: check, o: model.PipelineStepOverride{Adapter: ptr("llm")}, ok: true},
		{
			name: "chat model for llm",
			step: check,
			o:    model.PipelineStepOverride{Adapter: ptr("llm"), ModelID: &qwen.ID},
			ok:   true,
		},
		{
			name: "decision model for the default adapter",
			step: check,
			o:    model.PipelineStepOverride{ModelID: &decision.ID},
			ok:   true,
		},
		{name: "unknown adapter", step: check, o: model.PipelineStepOverride{Adapter: ptr("nope")}},
		{name: "unavailable adapter", step: check, o: model.PipelineStepOverride{Adapter: ptr("future")}},
		{
			name: "model on a non-model adapter",
			step: check,
			o:    model.PipelineStepOverride{Adapter: ptr("regex"), ModelID: &qwen.ID},
		},
		{name: "chat model for decisions", step: check, o: model.PipelineStepOverride{ModelID: &sonnet.ID}},
		{
			name: "unknown model",
			step: check,
			o:    model.PipelineStepOverride{Adapter: ptr("llm"), ModelID: ptr(uuid.New())},
		},
		{name: "locked step", step: locked, o: model.PipelineStepOverride{Adapter: ptr("api")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateOverride(tt.step, tt.o, models)
			if tt.ok && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.ok && !errs.HasCode(err, errs.CodePipelineInvalidOverride) {
				t.Fatalf("want %s, got %v", errs.CodePipelineInvalidOverride, err)
			}
		})
	}
}

// read is a step whose adapters are each written for one model, like the
// statement pipeline's extract.
var read = Step{
	ID: "read", Label: "Read", DefaultAdapter: "ocr",
	Adapters: []Adapter{
		{
			ID: "ocr", Label: "OCR", UsesModel: true, ModelKind: model.LLMModelChat,
			PreferredModels: []PreferredModel{{Provider: "ollama", Slug: "glm-ocr"}, {Provider: "ollama", Slug: "qwen3:8b"}},
		},
		{ID: "llm", Label: "LLM", UsesModel: true, ModelKind: model.LLMModelChat},
	},
}

func TestResolvePrefersTheAdaptersModel(t *testing.T) {
	// glm-ocr isn't set up, so the second preferred model runs, not the
	// owner's default (sonnet).
	view := Resolve(read, nil, models)
	if view.Effective.Model == nil || view.Effective.Model.ID != qwen.ID {
		t.Fatalf("model = %+v, want qwen", view.Effective.Model)
	}
	if view.Warning != "" {
		t.Fatalf("warning = %q, want none", view.Warning)
	}

	// A chosen model still wins.
	view = Resolve(read, &model.PipelineStepOverride{ModelID: &sonnet.ID}, models)
	if view.Effective.Model == nil || view.Effective.Model.ID != sonnet.ID {
		t.Fatalf("model = %+v, want the chosen sonnet", view.Effective.Model)
	}

	// None of the preferred ones: the default runs, with a warning.
	view = Resolve(read, nil, []model.UserLLMModel{sonnet})
	if view.Effective.Model == nil || view.Effective.Model.ID != sonnet.ID || view.Warning == "" {
		t.Fatalf("model = %+v warning = %q, want sonnet and a warning", view.Effective.Model, view.Warning)
	}
}

func TestValidateRerun(t *testing.T) {
	again := read
	again.ID, again.Label, again.DefaultAdapter, again.RerunOf = "again", "Again", "llm", "read"
	p := &Pipeline{ID: "demo", Steps: []Step{read, again}}

	// Defaults differ: ocr+qwen, then llm+sonnet.
	if err := ValidateRerun(p, "again", nil, nil, models); err != nil {
		t.Fatalf("defaults: %v", err)
	}
	// Rerun switched to the first step's adapter: same adapter, same model.
	same := &model.PipelineStepOverride{Pipeline: "demo", Step: "again", Adapter: ptr("ocr")}
	if err := ValidateRerun(p, "again", same, nil, models); !errs.HasCode(err, errs.CodePipelineInvalidOverride) {
		t.Fatalf("same reader: err = %v", err)
	}
	// Same adapter, another model is fine.
	other := &model.PipelineStepOverride{Pipeline: "demo", Step: "again", Adapter: ptr("ocr"), ModelID: &sonnet.ID}
	if err := ValidateRerun(p, "again", other, nil, models); err != nil {
		t.Fatalf("other model: %v", err)
	}
	// Changing the first step to match the stored rerun is caught too.
	stored := []model.PipelineStepOverride{{Pipeline: "demo", Step: "again", Adapter: ptr("ocr")}}
	if err := ValidateRerun(p, "read", nil, stored, models); err == nil {
		t.Fatal("first step matching the stored rerun: want an error")
	}
}

func TestOptionalSteps(t *testing.T) {
	opt := Step{
		ID: "verify", Label: "Verify", DefaultAdapter: "a", Optional: true,
		Adapters: []Adapter{{ID: "a", Label: "A"}},
	}
	off := model.PipelineStepOverride{Enabled: ptr(false)}

	if !Enabled(opt, nil) || Enabled(opt, &off) {
		t.Fatal("an optional step is on by default and off when turned off")
	}
	if !Resolve(opt, nil, models).Effective.Enabled || Resolve(opt, &off, models).Effective.Enabled {
		t.Fatal("Resolve must carry Enabled")
	}
	if !opt.Editable() {
		t.Fatal("an optional step with one plain adapter is still editable: it can be turned off")
	}
	if err := ValidateOverride(opt, off, models); err != nil {
		t.Fatalf("turning off an optional step: %v", err)
	}

	required := opt
	required.Optional = false
	required.Adapters = []Adapter{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}}
	if Enabled(required, &off) == false {
		t.Fatal("a required step ignores a stored off")
	}
	if err := ValidateOverride(required, off, models); !errs.HasCode(err, errs.CodePipelineInvalidOverride) {
		t.Fatalf("turning off a required step: err = %v", err)
	}
}

func TestValidateRerunIgnoresARerunThatsOff(t *testing.T) {
	again := read
	again.ID, again.Label, again.DefaultAdapter, again.RerunOf, again.Optional = "again", "Again", "ocr", "read", true
	p := &Pipeline{ID: "demo", Steps: []Step{read, again}}
	if err := ValidateRerun(p, "again", nil, nil, models); err == nil {
		t.Fatal("same reader in both: want an error")
	}
	off := &model.PipelineStepOverride{Pipeline: "demo", Step: "again", Enabled: ptr(false)}
	if err := ValidateRerun(p, "again", off, nil, models); err != nil {
		t.Fatalf("rerun turned off: %v", err)
	}
}
