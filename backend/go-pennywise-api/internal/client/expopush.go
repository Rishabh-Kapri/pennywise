package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ExpoPushClient sends push notifications through Expo's push service
// (https://docs.expo.dev/push-notifications/sending-notifications/).
// No SDK needed — it is a single JSON POST.
type ExpoPushClient interface {
	Send(ctx context.Context, messages []ExpoPushMessage) error
}

type ExpoPushMessage struct {
	To       string         `json:"to"`
	Title    string         `json:"title,omitempty"`
	Body     string         `json:"body,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
	Priority string         `json:"priority,omitempty"`
}

type expoPushClient struct {
	url        string
	httpClient *http.Client
}

func NewExpoPushClient(url string) ExpoPushClient {
	if url == "" {
		url = "https://exp.host/--/api/v2/push/send"
	}
	return &expoPushClient{
		url:        url,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *expoPushClient) Send(ctx context.Context, messages []ExpoPushMessage) error {
	if len(messages) == 0 {
		return nil
	}
	payload, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("error marshaling push messages: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("error creating push request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error calling expo push service: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("expo push service returned status %d", resp.StatusCode)
	}
	return nil
}
