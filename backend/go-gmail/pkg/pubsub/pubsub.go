package pubsub

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/auth"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/config"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/gmail"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/parser"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/pennywise-api"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/prediction"
	"github.com/Rishabh-Kapri/pennywise/backend/go-gmail/pkg/runner"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"

	"cloud.google.com/go/pubsub"
	"google.golang.org/api/option"
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
	runner          *runner.Runner
}

func NewEventProcessor(runner *runner.Runner) *EventProcessor {
	return &EventProcessor{
		processingQueue: make(map[uint64]bool),
		pendingEvents:   make(chan *pubsub.Message, 1), // buffered channel for pending historyIds
		processed:       make(map[string]historyIDMap),
		lastProcessed:   make(map[string]uint64),
		runner:          runner,
	}
}

func (p *EventProcessor) startProcessing(ctx context.Context) {
	for {
		select {
		// when receiving event from queue, process it
		case event := <-p.pendingEvents:
			p.processMessage(event)
		case <-ctx.Done():
			slog.Info("channel done")
			return
		}
	}
}

func (p *EventProcessor) processMessage(event *pubsub.Message) {
	// lock the mutex
	p.mu.Lock()

	var m runner.EventData
	err := json.Unmarshal(event.Data, &m)
	if err != nil {
		slog.Error("failed to unmarshal pubsub msg data", "error", err)
		p.mu.Unlock()
		event.Nack()
		return
	}

	// Create a context with a correlation ID for this message
	ctx := utils.WithCorrelationID(context.Background(), utils.NewCorrelationID())
	log := logger.Logger(ctx)

	email := m.Email
	historyID := m.HistoryId
	historyIDMap := p.processed[email]
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
		slog.Debug("defer func", "lastProcessed", p.lastProcessed)
		delete(p.processingQueue, m.HistoryId)
		if m.HistoryId > p.lastProcessed[email] {
			p.lastProcessed[email] = m.HistoryId
		}
		slog.Debug("defer func after", "lastProcessed", p.lastProcessed)
		p.mu.Unlock()

		event.Ack()
	}()

	err = p.runner.ProcessGmailHistoryId(ctx, m)
	if err != nil {
		log.Error("error processing gmail historyId", "historyId", m.HistoryId, "error", err)
		event.Nack()
		return
	}
	// FakeProcessHistoryId(m)
}

// adds the event to the pendingEvents channel
func (p *EventProcessor) addEventDataToQueue(event *pubsub.Message) {
	p.pendingEvents <- event
}

func FakeProcessHistoryId(event runner.EventData) {
	slog.Info("fake processing history ID", "event", event)
	time.Sleep(3 * time.Second)
}

func TestMessages() {
	// ctx := context.Background()
	ctx, cancel := context.WithCancel(context.Background())

	processor := NewEventProcessor(nil)
	go processor.startProcessing(ctx)

	eventData1 := gmail.EventData{
		HistoryId: 1,
		Email:     "rishabhkapri@gmail.com",
	}
	data, _ := json.Marshal(eventData1)
	msg := pubsub.Message{
		Data: []byte(data),
	}
	eventData2 := gmail.EventData{
		HistoryId: 2,
		Email:     "rishabhkapri@gmail.com",
	}
	data2, _ := json.Marshal(eventData2)
	msg2 := pubsub.Message{
		Data: []byte(data2),
	}
	eventData3 := gmail.EventData{
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

func PullMessages(ctx context.Context) {
	config := config.LoadConfig()

	pennywiseTransport := httpclient.NewHttpTransport(config.PennywiseServiceURL)
	pennywiseClient := transport.NewClient("pennywise-api", pennywiseTransport)

	runnerInstance := runner.NewRunner(
		auth.NewService(config),
		gmail.NewService(),
		parser.NewEmailParser(),
		prediction.NewService(config),
		pennywise.NewService(pennywiseClient),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	projectId := config.ProjectID
	subName := config.SubscriptionName
	client, err := pubsub.NewClient(ctx, projectId, option.WithCredentialsJSON([]byte(config.GoogleApplicationCredentialsJson)))
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

	processor := NewEventProcessor(runnerInstance)
	// start a goroutine to process events
	go processor.startProcessing(ctx)

	err = sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		slog.Info("received event data", "msg", msg)
		processor.addEventDataToQueue(msg)
	})
}
