package jobrunner

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/ports/job"
)

type progressBroker struct {
	job.Broker
	sent []string
}

func (b *progressBroker) UpdateProgress(_ context.Context, _ job.Claim, progress json.RawMessage) error {
	b.sent = append(b.sent, string(progress))
	return nil
}

func TestReportProgress(t *testing.T) {
	ReportProgress(context.Background(), job.Progress{Message: "outside a job"}) // must not panic

	broker := &progressBroker{}
	ctx := withProgress(context.Background(), broker, job.Claim{})
	ReportProgress(ctx, job.Progress{Message: "Queuing", Current: 1, Total: 3})
	ReportProgress(ctx, job.Progress{Message: "Queuing", Current: 2, Total: 3}) // too soon, dropped
	ReportProgress(ctx, job.Progress{Message: "Queuing", Current: 3, Total: 3}) // the last always goes
	ReportProgress(ctx, job.Progress{Message: "Done"})                          // a new message always goes

	if len(broker.sent) != 3 {
		t.Fatalf("sent %v", broker.sent)
	}
	if broker.sent[1] != `{"message":"Queuing","current":3,"total":3}` {
		t.Fatalf("second = %s", broker.sent[1])
	}
}
