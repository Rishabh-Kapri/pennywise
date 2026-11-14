// Package pipeline holds the system pipelines: what each one does, its steps in
// run order, and the adapters each step may run. Each pipeline is one file in
// pipelines/, checked against pipeline.schema.json when loaded. The API serves
// them (with a budget's overrides applied) to the Pipeline screen; the
// services that run a pipeline build their adapters from the same files.
//
// Budgets can change a step's adapter, AI model and fallback, never the steps
// themselves. Pipeline, step and adapter IDs are stored in
// budget_pipeline_steps rows: never rename one. Bump a pipeline's version
// whenever its steps change.
package pipeline

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"sync"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed pipeline.schema.json
var schemaJSON []byte

//go:embed pipelines/*.json
var pipelineFiles embed.FS

const schemaURL = "https://pennywise.cloud/schemas/pipeline.schema.json"

// IDs of the pipelines and steps that code refers to. A pipeline file may
// define others; the service that runs a pipeline checks at startup that it
// knows every step.
const (
	MessagePipeline   = "message"
	StatementPipeline = "statement"
)

type StepID string

// Steps of the message pipeline.
const (
	StepCheck   StepID = "check"
	StepExtract StepID = "extract"
	StepVerify  StepID = "verify"
	StepLookup  StepID = "lookup"
	StepPredict StepID = "predict"
	StepCreate  StepID = "create"
)

// Steps of the statement pipeline. It shares StepExtract with the message
// pipeline.
const (
	StepOCR      StepID = "ocr"
	StepFallback StepID = "fallback"
	StepSave     StepID = "save"
)

// Adapter is one way of running a step.
type Adapter struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	// UsesModel: calls one of the budget owner's models, of kind ModelKind,
	// so a model can be picked for it.
	UsesModel bool               `json:"usesModel,omitempty"`
	ModelKind model.LLMModelKind `json:"modelKind,omitempty"`
	// Unavailable: listed but not built yet, so it can't be chosen.
	Unavailable bool `json:"unavailable,omitempty"`
	// MaxTokens caps the model's output for this adapter. Model adapters only.
	MaxTokens int `json:"maxTokens,omitempty"`
}

type Step struct {
	ID             StepID `json:"id"`
	Label          string `json:"label"`
	Description    string `json:"description"`
	DefaultAdapter string `json:"defaultAdapter"`
	// Fallback is the default for trying the owner's other models of the same
	// kind when the step's model fails. Model adapters only.
	Fallback bool      `json:"fallback,omitempty"`
	Adapters []Adapter `json:"adapters"`
}

// Adapter returns the step's adapter with id.
func (s Step) Adapter(id string) (Adapter, bool) {
	for _, a := range s.Adapters {
		if a.ID == id {
			return a, true
		}
	}
	return Adapter{}, false
}

// Editable reports whether a budget has anything to choose for this step:
// more than one usable adapter, or one that runs on a model.
func (s Step) Editable() bool {
	usable := 0
	for _, a := range s.Adapters {
		if a.Unavailable {
			continue
		}
		if a.UsesModel {
			return true
		}
		usable++
	}
	return usable > 1
}

// Pipeline is one file in pipelines/.
type Pipeline struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Version     int    `json:"version"`
	Steps       []Step `json:"steps"`
}

// Step returns the step with id.
func (p *Pipeline) Step(id StepID) (Step, bool) {
	for _, s := range p.Steps {
		if s.ID == id {
			return s, true
		}
	}
	return Step{}, false
}

// Catalog is every system pipeline, sorted by file name.
type Catalog struct {
	Pipelines []*Pipeline
}

// Pipeline returns the pipeline with id.
func (c *Catalog) Pipeline(id string) (*Pipeline, bool) {
	for _, p := range c.Pipelines {
		if p.ID == id {
			return p, true
		}
	}
	return nil, false
}

var (
	loadOnce sync.Once
	loaded   *Catalog
	loadErr  error
)

// Load returns the embedded pipelines. They're parsed once; later calls return
// the same value, which callers must not modify.
func Load() (*Catalog, error) {
	loadOnce.Do(func() {
		loaded, loadErr = loadEmbedded()
	})
	return loaded, loadErr
}

func loadEmbedded() (*Catalog, error) {
	schema, err := compileSchema()
	if err != nil {
		return nil, err
	}
	entries, err := pipelineFiles.ReadDir("pipelines")
	if err != nil {
		return nil, err
	}
	catalog := &Catalog{}
	for _, entry := range entries {
		name := entry.Name()
		data, err := pipelineFiles.ReadFile(path.Join("pipelines", name))
		if err != nil {
			return nil, err
		}
		p, err := parse(schema, data)
		if err != nil {
			return nil, errs.Wrap(errs.CodePipelineInvalid, "pipelines/"+name, err, errs.WithDetail("file", name))
		}
		if want := strings.TrimSuffix(name, ".json"); p.ID != want {
			return nil, errs.NewWith(
				errs.CodePipelineInvalid,
				fmt.Sprintf("pipelines/%s: id is %q, want %q (the file name)", name, p.ID, want),
				errs.WithDetail("file", name),
			)
		}
		catalog.Pipelines = append(catalog.Pipelines, p)
	}
	if len(catalog.Pipelines) == 0 {
		return nil, errs.New(errs.CodePipelineInvalid, "no pipelines in pipelines/")
	}
	return catalog, nil
}

func compileSchema() (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaJSON))
	if err != nil {
		return nil, errs.Wrap(errs.CodePipelineInvalid, "pipeline.schema.json", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(schemaURL, doc); err != nil {
		return nil, errs.Wrap(errs.CodePipelineInvalid, "pipeline.schema.json", err)
	}
	schema, err := c.Compile(schemaURL)
	if err != nil {
		return nil, errs.Wrap(errs.CodePipelineInvalid, "pipeline.schema.json", err)
	}
	return schema, nil
}

// parse checks one pipeline file against the schema, then for what the
// schema can't express: unique IDs and a default adapter that exists.
func parse(schema *jsonschema.Schema, data []byte) (*Pipeline, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, errs.Wrap(errs.CodePipelineInvalid, "not valid JSON", err)
	}
	if err := schema.Validate(doc); err != nil {
		return nil, errs.Wrap(errs.CodePipelineInvalid, "doesn't match pipeline.schema.json", err)
	}
	var p Pipeline
	// The schema already rejected unknown keys; $schema is the one key the
	// struct doesn't carry.
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, errs.Wrap(errs.CodePipelineInvalid, "decode pipeline", err)
	}
	return &p, p.validate()
}

func (p *Pipeline) validate() error {
	invalid := func(format string, args ...any) error {
		return errs.New(errs.CodePipelineInvalid, format, args...)
	}
	steps := map[StepID]bool{}
	for _, s := range p.Steps {
		if steps[s.ID] {
			return invalid("step %q is listed twice", s.ID)
		}
		steps[s.ID] = true

		adapters := map[string]bool{}
		for _, a := range s.Adapters {
			if adapters[a.ID] {
				return invalid("step %q lists adapter %q twice", s.ID, a.ID)
			}
			adapters[a.ID] = true
			if a.UsesModel && !a.ModelKind.Valid() {
				return invalid("step %q adapter %q: usesModel needs a modelKind", s.ID, a.ID)
			}
		}
		def, ok := s.Adapter(s.DefaultAdapter)
		if !ok {
			return invalid("step %q: default adapter %q isn't in its list", s.ID, s.DefaultAdapter)
		}
		if def.Unavailable {
			return invalid("step %q: default adapter %q is marked unavailable", s.ID, s.DefaultAdapter)
		}
	}
	return nil
}
