package pubsub

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/auth"
	pc "github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/client"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/config"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/gmail"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/parser"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/temporal"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	sharedModel "github.com/Rishabh-Kapri/pennywise/backend/shared/model"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/option"

	tc "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

type GmailPushPayload struct {
	EmailAddress string `json:"emailAddress"`
	HistoryID    uint64 `json:"historyId"`
}

type historyIDMap map[uint64]bool

type EventProcessor struct {
	mu              sync.Mutex
	processingQueue map[uint64]bool
	pendingEvents   chan *pubsub.Message
	processed       map[string]historyIDMap
	lastProcessed   map[string]uint64
	temporalClient  tc.Client
}

func NewEventProcessor(tc *tc.Client) *EventProcessor {
	return &EventProcessor{
		processingQueue: make(map[uint64]bool),
		pendingEvents:   make(chan *pubsub.Message, 1), // buffered channel for pending historyIds
		processed:       make(map[string]historyIDMap),
		lastProcessed:   make(map[string]uint64),
		temporalClient:  *tc,
	}
}

type EventData struct {
	Email     string `json:"emailAddress"`
	HistoryId uint64 `json:"historyId"`
}

func (p *EventProcessor) startProcessing(ctx context.Context) {
	for {
		select {
		// when receiving event from queue, process it
		case event := <-p.pendingEvents:
			p.processMessage(event)
		case <-ctx.Done():
			logger.Logger(ctx).Info("channel done")
			return
		}
	}
}

func (p *EventProcessor) processMessage(event *pubsub.Message) {
	// lock the mutex
	p.mu.Lock()
	// Create a context with a correlation ID for this message
	ctx := utils.WithCorrelationID(context.Background(), utils.NewCorrelationID())
	log := logger.Logger(ctx)

	var m EventData
	err := json.Unmarshal(event.Data, &m)
	if err != nil {
		log.Error("failed to unmarshal pubsub msg data", "error", err)
		p.mu.Unlock()
		event.Nack()
		return
	}

	email := m.Email
	historyID := m.HistoryId
	historyIDMap := p.processed[email]
	if historyIDMap == nil {
		historyIDMap = make(map[uint64]bool)
		p.processed[email] = historyIDMap
	}
	if historyIDMap[historyID] {
		p.mu.Unlock()
		log.Info("duplicate historyId detected, skipping", "historyId", m.HistoryId)
		event.Ack()
		return
	}
	if m.HistoryId < p.lastProcessed[email] {
		p.mu.Unlock()
		log.Info("outdated historyId detected, skipping", "historyId", m.HistoryId)
		event.Ack()
		return
	}
	p.processingQueue[m.HistoryId] = true
	historyIDMap[historyID] = true
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		logger.Logger(ctx).Debug("defer func", "lastProcessed", p.lastProcessed)
		delete(p.processingQueue, m.HistoryId)
		if m.HistoryId > p.lastProcessed[email] {
			p.lastProcessed[email] = m.HistoryId
		}
		logger.Logger(ctx).Debug("defer func after", "lastProcessed", p.lastProcessed)
		p.mu.Unlock()

		event.Ack()
	}()

	we, err := p.temporalClient.ExecuteWorkflow(
		ctx,
		tc.StartWorkflowOptions{
			TaskQueue: sharedModel.PennywiseTaskQueue,
		},
		sharedModel.EmailToTransactionWorkflowName,
		sharedModel.EmailWorflowInput{
			Email:     email,
			HistoryId: historyID,
		},
	)
	if err != nil {
		log.Error("error starting workflow", "error", err)
		event.Nack()
		return
	}
	log.Info("workflow started", "workflowId", we.GetID(), "runId", we.GetRunID())
	// FakeProcessHistoryId(m)
}

// adds the event to the pendingEvents channel
func (p *EventProcessor) addEventDataToQueue(event *pubsub.Message) {
	p.pendingEvents <- event
}

func FakeProcessHistoryId(event EventData) {
	logger.Logger(context.Background()).Info("fake processing history ID", "event", event)
	time.Sleep(3 * time.Second)
}

func TestMessages() {
	// ctx := context.Background()
	ctx, cancel := context.WithCancel(context.Background())

	processor := NewEventProcessor(nil)
	go processor.startProcessing(ctx)

	eventData1 := EventData{
		HistoryId: 1,
		Email:     "rishabhkapri@gmail.com",
	}
	data, _ := json.Marshal(eventData1)
	msg := pubsub.Message{
		Data: []byte(data),
	}
	eventData2 := EventData{
		HistoryId: 2,
		Email:     "rishabhkapri@gmail.com",
	}
	data2, _ := json.Marshal(eventData2)
	msg2 := pubsub.Message{
		Data: []byte(data2),
	}
	eventData3 := EventData{
		HistoryId: 3,
		Email:     "rishabhkapri@gmail.com",
	}
	data3, _ := json.Marshal(eventData3)
	msg3 := pubsub.Message{
		Data: []byte(data3),
	}
	processor.addEventDataToQueue(&msg)
	processor.addEventDataToQueue(&msg2)
	processor.addEventDataToQueue(&msg2)
	processor.addEventDataToQueue(&msg3)

	cancel()
}

func connectToTemporal(ctx context.Context, cfg config.Config) (tc.Client, error) {
	logger.Logger(ctx).Info("temporal", "host", cfg.TemporalServerHost, "port", cfg.TemporalServerPort)
	c, err := tc.Dial(tc.Options{
		HostPort: cfg.TemporalServerHost + ":" + cfg.TemporalServerPort,
		Logger:   logger.Logger(ctx),
	})
	if err != nil {
		return nil, err
	}
	logger.Logger(ctx).Info("connected to temporal")
	return c, nil
}

func PullMessages(ctx context.Context) {
	config := config.LoadConfig()

	projectId := config.ProjectID
	subName := config.SubscriptionName
	client, err := pubsub.NewClient(
		ctx,
		projectId,
		option.WithCredentialsJSON([]byte(config.GoogleApplicationCredentialsJson)),
	)
	if err != nil {
		logger.Fatal("failed to create pubsub client", "error", err)
	}
	defer client.Close()

	sub := client.Subscription(subName)
	ok, err := sub.Exists(ctx)
	if err != nil {
		logger.Fatal("failed to check if sub exists", "error", err)
	}
	if !ok {
		logger.Fatal("sub does not exist", "sub", subName)
	}

	tc, err := connectToTemporal(ctx, *config)
	if err != nil {
		logger.Fatal("Unable to connect to Temporal", "error", err)
	}

	pennywiseTransport := httpclient.NewHttpTransport(config.PennywiseServiceURL)
	pennywiseClient := transport.NewClient("pennywise-api", pennywiseTransport)
	w := worker.New(tc, sharedModel.GmailActivitiesTaskQueue, worker.Options{
		UseBuildIDForVersioning: false,
	})
	w.RegisterActivity(&temporal.GmailActivities{
		Auth:      auth.NewService(config),
		Gmail:     gmail.NewService(),
		Parser:    parser.NewEmailParser(),
		Pennywise: pc.NewPennywiseClient(pennywiseClient),
	})
	go func() {
		if err := w.Run(worker.InterruptCh()); err != nil {
			logger.Fatal("Temporal activity worker failed", "error", err)
		}
	}()

	processor := NewEventProcessor(&tc)
	// start a goroutine to process events
	go processor.startProcessing(ctx)

	err = sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		logger.Logger(ctx).Info("received event data", "msg", msg)
		processor.addEventDataToQueue(msg)
	})
}
