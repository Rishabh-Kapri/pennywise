package service

import (
	"context"
	"time"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	"github.com/google/uuid"
)

const ingestConnectionsPath = "/api/ingest/connections"

// MailConnection is a mailbox feeding a budget, without its credentials.
type MailConnection struct {
	ID         uuid.UUID  `json:"id"`
	Provider   string     `json:"provider"`
	Account    string     `json:"account"`
	Status     string     `json:"status"`
	LastSyncAt *time.Time `json:"lastSyncAt,omitempty"`
	LastError  string     `json:"lastError,omitempty"`
	PausedAt   *time.Time `json:"pausedAt,omitempty"`
}

// BackfillCandidate is a recent email a user can choose to import.
type BackfillCandidate struct {
	ExternalID string    `json:"externalId"`
	Sender     string    `json:"sender"`
	Subject    string    `json:"subject"`
	ReceivedAt time.Time `json:"receivedAt"`
}

// BackfillResult names the job importing the chosen emails.
type BackfillResult struct {
	JobID uuid.UUID `json:"jobId"`
}

// MailConnectionService forwards to ingest, which owns the Gmail adapter and
// the key that seals credentials. The budget comes from ctx and travels in the
// internal headers. Failures carry ingest's status (errs.StatusOf); with no
// ingest configured they return CodeMailNotConfigured.
type MailConnectionService interface {
	List(ctx context.Context) ([]MailConnection, error)
	// Connect turns a Google consent code into a connected mailbox.
	Connect(ctx context.Context, code string) (MailConnection, error)
	// SetPaused pauses or resumes a mailbox's sync.
	SetPaused(ctx context.Context, connectionID uuid.UUID, paused bool) (MailConnection, error)
	// Disconnect removes a mailbox from the budget.
	Disconnect(ctx context.Context, connectionID uuid.UUID) error
	// BackfillCandidates lists a mailbox's recent emails.
	BackfillCandidates(ctx context.Context, connectionID uuid.UUID) ([]BackfillCandidate, error)
	// Backfill imports the chosen candidates.
	Backfill(ctx context.Context, connectionID uuid.UUID, externalIDs []string) (BackfillResult, error)
}

type mailConnectionService struct {
	ingest *transport.Client
}

// NewMailConnectionService forwards to ingest. With no ingest client
// (INGEST_SERVICE_URL unset) email sync is off and every call says so.
func NewMailConnectionService(ingest *transport.Client) MailConnectionService {
	if ingest == nil {
		return mailSyncOff{}
	}
	return &mailConnectionService{ingest: ingest}
}

// mailSyncOff answers every call with CodeMailNotConfigured.
type mailSyncOff struct{}

var errMailSyncOff = errs.New(errs.CodeMailNotConfigured, "INGEST_SERVICE_URL not set")

func (mailSyncOff) List(context.Context) ([]MailConnection, error) { return nil, errMailSyncOff }
func (mailSyncOff) Connect(context.Context, string) (MailConnection, error) {
	return MailConnection{}, errMailSyncOff
}
func (mailSyncOff) SetPaused(context.Context, uuid.UUID, bool) (MailConnection, error) {
	return MailConnection{}, errMailSyncOff
}
func (mailSyncOff) Disconnect(context.Context, uuid.UUID) error { return errMailSyncOff }
func (mailSyncOff) BackfillCandidates(context.Context, uuid.UUID) ([]BackfillCandidate, error) {
	return nil, errMailSyncOff
}
func (mailSyncOff) Backfill(context.Context, uuid.UUID, []string) (BackfillResult, error) {
	return BackfillResult{}, errMailSyncOff
}

func (s *mailConnectionService) List(ctx context.Context) ([]MailConnection, error) {
	return transport.Get[[]MailConnection](ctx, s.ingest, ingestConnectionsPath)
}

func (s *mailConnectionService) Connect(ctx context.Context, code string) (MailConnection, error) {
	return transport.Post[MailConnection](ctx, s.ingest, ingestConnectionsPath, nil, map[string]string{"code": code})
}

func connectionPath(connectionID uuid.UUID) string {
	return ingestConnectionsPath + "/" + connectionID.String()
}

func backfillPath(connectionID uuid.UUID) string {
	return connectionPath(connectionID) + "/backfill"
}

func (s *mailConnectionService) SetPaused(
	ctx context.Context,
	connectionID uuid.UUID,
	paused bool,
) (MailConnection, error) {
	return transport.Patch[MailConnection](ctx, s.ingest, connectionPath(connectionID), nil,
		map[string]bool{"paused": paused})
}

func (s *mailConnectionService) Disconnect(ctx context.Context, connectionID uuid.UUID) error {
	return transport.Delete(ctx, s.ingest, connectionPath(connectionID))
}

func (s *mailConnectionService) BackfillCandidates(
	ctx context.Context,
	connectionID uuid.UUID,
) ([]BackfillCandidate, error) {
	return transport.Get[[]BackfillCandidate](ctx, s.ingest, backfillPath(connectionID))
}

func (s *mailConnectionService) Backfill(
	ctx context.Context,
	connectionID uuid.UUID,
	externalIDs []string,
) (BackfillResult, error) {
	return transport.Post[BackfillResult](ctx, s.ingest, backfillPath(connectionID), nil,
		map[string][]string{"externalIds": externalIDs})
}
