package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

// Private struct that satisfies the transport interface
type httpTransport struct {
	baseUrl string
	client  *http.Client
}

// Factory function returns the actual transport interface, hiding the struct
func NewHttpTransport(baseUrl string) transport.Transport {
	return &httpTransport{
		baseUrl: baseUrl,
		client:  &http.Client{},
	}
}

func (h *httpTransport) get(ctx context.Context, path string) (transport.Response, error) {
	var resp transport.Response
	url := h.baseUrl + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		logger.Logger(ctx).Error("error creating GET request", "url", url, "error", err)
		resp.StatusCode = http.StatusInternalServerError
		return resp, err
	}
	return h.do(ctx, req)
}

func (h *httpTransport) post(ctx context.Context, path string, data any) (transport.Response, error) {
	logger.Logger(ctx).Debug("httpTransport.post", "url", h.baseUrl, "path", path, "data", data)
	url := h.baseUrl + path
	var resp transport.Response
	var reqBody io.Reader

	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			logger.Logger(ctx).Error("error marshaling POST request body", "url", url, "error", err)
			resp.StatusCode = http.StatusInternalServerError
			return resp, err
		}
		reqBody = bytes.NewBuffer(b)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, reqBody)
	if err != nil {
		logger.Logger(ctx).Error("error creating POST request", "url", url, "error", err)
		resp.StatusCode = http.StatusInternalServerError
		return resp, errs.Wrap(errs.CodeHTTPClientError, "error creating POST request", err)
	}

	return h.do(ctx, req)
}

func (h *httpTransport) do(ctx context.Context, req *http.Request) (transport.Response, error) {
	log := logger.Logger(ctx)
	var resp transport.Response
	req.Header.Set("Content-Type", "application/json")

	for _, header := range utils.GetHeaders(ctx) {
		for k, v := range header {
			req.Header[k] = v
		}
	}

	res, err := h.client.Do(req)
	if err != nil {
		log.Error("error executing request", "method", req.Method, "url", req.URL.String(), "error", err)
		resp.StatusCode = http.StatusInternalServerError
		return resp, errs.Wrap(errs.CodeHTTPClientError, "error executing request", err)
	}
	defer res.Body.Close()

	resp.StatusCode = res.StatusCode
	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Error("error reading body", "method", req.Method, "url", req.URL.String(), "status", res.StatusCode)
		resp.StatusCode = http.StatusInternalServerError
		return resp, errs.New(errs.CodeHTTPClientError, "%s request for %s failed with status code: %d", req.Method, req.URL.String(), res.StatusCode)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		log.Error("request failed", "method", req.Method, "url", req.URL.String(), "status", res.StatusCode, "body", string(body))
		return resp, errs.New(errs.CodeHTTPClientError, "%s request for %s failed with status code: %d", req.Method, req.URL.String(), res.StatusCode)
	}
	resp.Body = body

	return resp, nil
}

// Send satisfies the transport interface
// Go considers httpTransport to be a valid transport.Transport because it has all the methods
func (h *httpTransport) Send(ctx context.Context, req *transport.Request) (transport.Response, error) {
	switch req.Method {
	case http.MethodGet:
		return h.get(ctx, req.Path)
	case http.MethodPost:
		return h.post(ctx, req.Path, req.Payload)
	default:
		return transport.Response{}, errs.New(errs.CodeInvalidArgument, "unsupported request method")
	}
}
