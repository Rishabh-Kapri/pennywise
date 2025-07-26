package pubsub

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"gmail-transactions/pkg/gmail"

	"cloud.google.com/go/pubsub"
)

type GmailPushPayload struct {
	EmailAddress string `json:"emailAddress"`
	HistoryID    uint64 `json:"historyId"`
}

type EventProcessor struct {
	mu              sync.Mutex
	processingQueue map[uint64]bool
	pendingEvents   chan *pubsub.Message
	processed       map[uint64]bool
	lastProcessed   uint64
}

func NewEventProcessor() *EventProcessor {
	return &EventProcessor{
		processingQueue: make(map[uint64]bool),
		pendingEvents:   make(chan *pubsub.Message, 1), // buffered channel for pending historyIds
		processed:       make(map[uint64]bool),
		lastProcessed:   0,
	}
}

func (p *EventProcessor) startProcessing(ctx context.Context) {
	for {
		select {
		// when receiving event from queue, process it
		case event := <-p.pendingEvents:
			p.processMessage(event)
		case <-ctx.Done():
			log.Printf("channel done!")
			return
		}
	}
}

func (p *EventProcessor) processMessage(event *pubsub.Message) {
	// lock the mutex
	p.mu.Lock()

	var m gmail.EventData
	err := json.Unmarshal(event.Data, &m)
	if err != nil {
		log.Printf("Failed to unmarshal pubsub msg data :%v", err.Error())
		event.Nack()
		return
	}

	if p.processed[m.HistoryId] {
		p.mu.Unlock()
		log.Printf("Duplicate historyId %v detected, skipping!\n", m.HistoryId)
		event.Ack()
		return
	}
	if m.HistoryId < p.lastProcessed {
		p.mu.Unlock()
		log.Printf("Outdated historyId %v detected, skipping! \n", m.HistoryId)
		event.Ack()
		return
	}
	p.processingQueue[m.HistoryId] = true
	p.processed[m.HistoryId] = true
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		log.Printf("Inside defer func %v", p.lastProcessed)
		delete(p.processingQueue, m.HistoryId)
		if m.HistoryId > p.lastProcessed {
			p.lastProcessed = m.HistoryId
		}
		log.Printf("Inside defer func after %v", p.lastProcessed)
		p.mu.Unlock()

		event.Ack()
	}()

	_, err = gmail.ProcessGmailHistoryId(m)
	if err != nil {
		log.Printf("Error while processing gmail historyId %v: %v", m.HistoryId, err.Error())
		event.Nack()
		return
	}
}

// adds the event to the pendingEvents channel
func (p *EventProcessor) addEventDataToQueue(event *pubsub.Message) {
	p.pendingEvents <- event
}

func FakeProcessHistoryId(event gmail.EventData) {
	log.Printf("Fake Processing History ID: %v", event)
	time.Sleep(3 * time.Second)
}

func TestMessages() {
	ctx := context.Background()

	processor := NewEventProcessor()
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
}

func PullMessages() {
	projectId := os.Getenv("PROJECT_ID")
	subName := os.Getenv("SUB_NAME")

	ctx := context.Background()

	client, err := pubsub.NewClient(ctx, projectId)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	sub := client.Subscription(subName)
	ok, err := sub.Exists(ctx)
	if err != nil {
		log.Fatalf("Failed to check if sub exists: %v", err)
	}
	if !ok {
		log.Fatalf("Sub %s does not exists", subName)
	}

	processor := NewEventProcessor()
	// start a goroutine to process events
	go processor.startProcessing(ctx)

	err = sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		log.Printf("Received event data: %v", msg)
		processor.addEventDataToQueue(msg)
	})
}
