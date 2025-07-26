package mlp

import (
	"context"
	"encoding/json"
	"log"

	"gmail-transactions/pkg/gmail"

	"cloud.google.com/go/pubsub"
)

type GmailPushPayload struct {
	EmailAddress string `json:"emailAddress"`
	HistoryID    uint64 `json:"historyId"`
}

func PullMessages() {
	projectId := "pennywise-39654"
	topicId := "process_transactions"
	topicId = "gmail_watch"
	subName := "process_transactions-sub"
	subName = "gmail_watch"

	ctx := context.Background()

	client, err := pubsub.NewClient(ctx, projectId)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	topic := client.Topic(topicId)
	ok, err := topic.Exists(ctx)
	if err != nil {
		log.Fatalf("Failed to check if topic exists: %v", err)
	}
	if !ok {
		log.Fatalf("Topic %s does not exists", topicId)
	}

	sub := client.Subscription(subName)
	ok, err = sub.Exists(ctx)
	if err != nil {
		log.Fatalf("Failed to check if sub exists: %v", err)
	}
	if !ok {
		log.Fatalf("Sub %s does not exists", subName)
	}

	err = sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		var m gmail.EventData
		err := json.Unmarshal(msg.Data, &m)
		if err != nil {
			log.Printf("Failed to unmarshal pubsub msg data :%v", err.Error())
			// msg.Nack()
			return
		}
		log.Printf("Processing event data: %v", m)
		_, err = gmail.ProcessGmailHistoryId(m)
		if err != nil {
			log.Printf("Error while processing gmail historyId: %v", err.Error())
			return
			// msg.Nack()
		} else {
			msg.Ack()
		}
	})
}
