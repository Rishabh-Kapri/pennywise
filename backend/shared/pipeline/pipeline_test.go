package pipeline

import (
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"strings"
	"testing"

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
		{name: "unknown key", json: strings.Replace(validPipeline, `"version": 1`, `"version": 1, "stepz": []`, 1), want: "stepz"},
		{name: "bad id", json: strings.Replace(validPipeline, `"id": "one"`, `"id": "One Step"`, 1), want: "pattern"},
		{name: "missing default", json: strings.Replace(validPipeline, `"defaultAdapter": "a"`, `"defaultAdapter": "b"`, 1), want: "isn't in its list"},
		{
			name: "maxTokens without usesModel",
			json: strings.Replace(validPipeline, `"label": "A",`, `"label": "A", "maxTokens": 10,`, 1),
			want: "usesModel",
		},
		{
			name: "duplicate step",
			json: strings.Replace(validPipeline, `"steps": [{`, `"steps": [{"id": "one", "label": "Dup", "description": "", "defaultAdapter": "a", "adapters": [{"id": "a", "label": "A", "description": ""}]}, {`, 1),
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
	sonnet   = model.UserLLMModel{ID: uuid.New(), Kind: model.LLMModelChat, Provider: "anthropic", Slug: "claude-sonnet", IsDefault: true}
	qwen     = model.UserLLMModel{ID: uuid.New(), Kind: model.LLMModelChat, Provider: "ollama", Slug: "qwen3:8b"}
	decision = model.UserLLMModel{ID: uuid.New(), Kind: model.LLMModelDecision, Provider: "openrouter", Slug: "classify-v1", IsDefault: true}
	models   = []model.UserLLMModel{sonnet, decision, qwen}
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
		{name: "fallback off runs the default alone", models: usable, adapter: "decisions", want: []string{"cloudflare/clef"}},
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
	locked := Step{ID: StepCreate, Label: "Create", DefaultAdapter: "api", Adapters: []Adapter{{ID: "api", Label: "API"}}}
	tests := []struct {
		name string
		step Step
		o    model.PipelineStepOverride
		ok   bool
	}{
		{name: "switch adapter", step: check, o: model.PipelineStepOverride{Adapter: ptr("llm")}, ok: true},
		{name: "chat model for llm", step: check, o: model.PipelineStepOverride{Adapter: ptr("llm"), ModelID: &qwen.ID}, ok: true},
		{name: "decision model for the default adapter", step: check, o: model.PipelineStepOverride{ModelID: &decision.ID}, ok: true},
		{name: "unknown adapter", step: check, o: model.PipelineStepOverride{Adapter: ptr("nope")}},
		{name: "unavailable adapter", step: check, o: model.PipelineStepOverride{Adapter: ptr("future")}},
		{name: "model on a non-model adapter", step: check, o: model.PipelineStepOverride{Adapter: ptr("regex"), ModelID: &qwen.ID}},
		{name: "chat model for decisions", step: check, o: model.PipelineStepOverride{ModelID: &sonnet.ID}},
		{name: "unknown model", step: check, o: model.PipelineStepOverride{Adapter: ptr("llm"), ModelID: ptr(uuid.New())}},
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
