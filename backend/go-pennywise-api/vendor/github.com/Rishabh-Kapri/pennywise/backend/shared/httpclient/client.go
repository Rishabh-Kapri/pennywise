package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

type Client interface {
	Get(ctx context.Context, url string, dest any) error
	Post(ctx context.Context, url string, data any, dest any) error
	Do(ctx context.Context, req *http.Request, dest any) error
}

type defaultClient struct {
	httpClient *http.Client
}

func NewClient() Client {
	return &defaultClient{
		httpClient: &http.Client{},
	}
}

func (c *defaultClient) Get(ctx context.Context, url string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		logger.Logger(ctx).Error("error creating GET request", "url", url, "error", err)
		return err
	}
	return c.Do(ctx, req, dest)
}

func (c *defaultClient) Post(ctx context.Context, url string, data any, dest any) error {
	var body io.Reader
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			logger.Logger(ctx).Error("error marshaling POST request body", "url", url, "error", err)
			return err
		}
		body = bytes.NewBuffer(b)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		logger.Logger(ctx).Error("error creating POST request", "url", url, "error", err)
		return errs.Wrap(errs.CodeHTTPClientError, "error creating POST request", err)
	}
	return c.Do(ctx, req, dest)
}

func (c *defaultClient) Do(ctx context.Context, req *http.Request, dest any) error {
	log := logger.Logger(ctx)
	req.Header.Set("Content-Type", "application/json")

	// Inject correlation ID if available
	if cid := logger.CorrelationIDFromContext(ctx); cid != "" {
		req.Header.Set("X-Correlation-ID", cid)
	}

	// Inject budget ID if available
	if bid, err := utils.BudgetIDFromContext(ctx); err == nil {
		req.Header.Set("X-Budget-ID", bid.String())
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		log.Error("error executing request", "method", req.Method, "url", req.URL.String(), "error", err)
		return errs.Wrap(errs.CodeHTTPClientError, "error executing request", err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(res.Body)
		log.Error("request failed", "method", req.Method, "url", req.URL.String(), "status", res.StatusCode, "body", string(body))
		return errs.New(errs.CodeHTTPClientError, "%s request for %s failed with status code: %d", req.Method, req.URL.String(), res.StatusCode)
	}

	if dest != nil {
		body, err := io.ReadAll(res.Body)
		if err != nil {
			log.Error("error reading response", "method", req.Method, "url", req.URL.String(), "error", err)
			return errs.Wrap(errs.CodeHTTPClientError, "error reading response", err)
		}

		if err := json.Unmarshal(body, dest); err != nil {
			log.Error("error unmarshalling response", "method", req.Method, "url", req.URL.String(), "error", err)
			return errs.Wrap(errs.CodeHTTPClientError, "error unmarshalling response", err)
		}
	}
	return nil
}

// Helper generic functions matching the original signature
func Get[T any](ctx context.Context, url string) (T, error) {
	var zero T
	err := NewClient().Get(ctx, url, &zero)
	return zero, err
}

func Post[T any](ctx context.Context, url string, data any) (T, error) {
	var zero T
	err := NewClient().Post(ctx, url, data, &zero)
	return zero, err
}
