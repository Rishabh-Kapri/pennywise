package factory

import (
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/pipeline"
)

func TestCipherRunsEveryAdapterInItsPipelines(t *testing.T) {
	catalog, err := pipeline.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{pipeline.MessagePipeline, pipeline.StatementPipeline} {
		p, ok := catalog.Pipeline(id)
		if !ok {
			t.Fatalf("no %s pipeline", id)
		}
		if err := checkKnown(p); err != nil {
			t.Errorf("%s: %v", id, err)
		}
	}
}

func TestAnAdapterCipherLacksFailsTheCheck(t *testing.T) {
	p := &pipeline.Pipeline{ID: "message", Steps: []pipeline.Step{
		{ID: pipeline.StepCheck, Adapters: []pipeline.Adapter{{ID: "regex"}, {ID: "crystal-ball"}}},
	}}
	if err := checkKnown(p); err == nil {
		t.Fatal("want an error for an unknown adapter")
	}

	p = &pipeline.Pipeline{ID: "message", Steps: []pipeline.Step{{ID: "categorise"}}}
	if err := checkKnown(p); err == nil {
		t.Fatal("want an error for an unknown step")
	}
}
