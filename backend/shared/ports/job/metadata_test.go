package job

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

func TestMetadataRoundTrip(t *testing.T) {
	// The enqueuing request: ingest, called by the api, as a verified
	// internal request for some user.
	enqueueCtx := utils.WithRequestMetadata(context.Background(), utils.RequestMetadata{
		CorrelationID:    "corr-1",
		LocalService:     "ingest",
		CallerService:    "api",
		OriginService:    "api",
		InternalRequest:  true,
		VerifiedInternal: true,
	})
	enqueueCtx = utils.WithUserID(enqueueCtx, uuid.New())

	m := MetadataFromContext(enqueueCtx)
	if m.CorrelationID != "corr-1" || m.OriginService != "api" {
		t.Fatalf("captured %+v", m)
	}

	// Through the database as JSON and back.
	raw, _ := json.Marshal(m)
	var stored Metadata
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}

	workerCtx := stored.Context(context.Background())
	got := utils.RequestMetadataFromContext(workerCtx)
	if got.CorrelationID != "corr-1" || got.OriginService != "api" {
		t.Fatalf("restored %+v", got)
	}
	// Trust and identity must not ride along with the job.
	if got.VerifiedInternal || got.InternalRequest {
		t.Fatal("job carried the internal-verified flag")
	}
	if _, err := utils.UserIDFromContext(workerCtx); err == nil {
		t.Fatal("job carried the user")
	}
}

func TestMetadataWithoutRequestGetsCorrelationID(t *testing.T) {
	ctx := utils.WithLocalService(context.Background(), "cipher")
	m := MetadataFromContext(ctx)
	if m.CorrelationID == "" {
		t.Fatal("want a new correlation ID")
	}
	if m.OriginService != "cipher" {
		t.Fatalf("origin = %q, want the enqueuing service", m.OriginService)
	}
}
