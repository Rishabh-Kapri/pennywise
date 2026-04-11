package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

type MLPClient struct {
	baseURL string
	client  *http.Client
}

type PredictRequest struct {
	EmailText string  `json:"email_text"`
	Amount    float64 `json:"amount"`
	Type      string  `json:"type"`
	Account   string  `json:"account,omitempty"`
	Payee     string  `json:"payee,omitempty"`
}

type PredictResponse struct {
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
}

func NewMLPClient(baseURL string) *MLPClient {
	return &MLPClient{
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

// PredictAll calls the MLP predict endpoint sequentially for account, payee, category
// (matching the current go-gmail flow) and returns all three results.
func (c *MLPClient) PredictAll(ctx context.Context, emailText string, amount float64) (account, payee, category *PredictResponse, err error) {
	account, err = c.predict(ctx, PredictRequest{
		EmailText: emailText,
		Amount:    amount,
		Type:      "account",
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("predict account: %w", err)
	}

	payee, err = c.predict(ctx, PredictRequest{
		EmailText: emailText,
		Amount:    amount,
		Type:      "payee",
		Account:   account.Label,
	})
	if err != nil {
		return account, nil, nil, fmt.Errorf("predict payee: %w", err)
	}

	category, err = c.predict(ctx, PredictRequest{
		EmailText: emailText,
		Amount:    amount,
		Type:      "category",
		Account:   account.Label,
		Payee:     payee.Label,
	})
	if err != nil {
		return account, payee, nil, fmt.Errorf("predict category: %w", err)
	}

	return account, payee, category, nil
}

func (c *MLPClient) predict(ctx context.Context, req PredictRequest) (*PredictResponse, error) {
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	url := c.baseURL + "/predict"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request to %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Error("MLP predict failed", "url", url, "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("predict request failed with status %d", resp.StatusCode)
	}

	var result PredictResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	return &result, nil
}
