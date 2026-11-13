package factory

import (
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/pipeline"
)

func TestCipherRunsEveryAdapterInTheMessagePipeline(t *testing.T) {
	catalog, err := pipeline.Load()
	if err != nil {
		t.Fatal(err)
	}
	p, ok := catalog.Pipeline(pipeline.MessagePipeline)
	if !ok {
		t.Fatal("no message pipeline")
	}
	if err := checkKnown(p); err != nil {
		t.Fatal(err)
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
