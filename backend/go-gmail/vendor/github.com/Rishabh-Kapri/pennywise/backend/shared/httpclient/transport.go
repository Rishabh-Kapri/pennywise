package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
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

func applyHeaders(ctx context.Context, req *http.Request, headers map[string][]string) {
	for k, v := range headers {
		if req.Header.Get(k) == "" {
			req.Header.Set(k, v[0])
		}
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
}

func (h *httpTransport) get(ctx context.Context, path string, headers map[string][]string) (transport.Response, error) {
	var resp transport.Response
	url := h.baseUrl + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		logger.Logger(ctx).Error("error creating GET request", "url", url, "error", err)
		resp.StatusCode = http.StatusInternalServerError
		return resp, err
	}
	return h.do(ctx, req, nil)
}

func (h *httpTransport) requestWithBody(
	ctx context.Context,
	method string,
	path string,
	headers map[string][]string,
	data any,
) (transport.Response, error) {
	// logger.Logger(ctx).Debug("httpTransport."+method, "url", h.baseUrl, "path", path, "data", data)
	var url string
	if strings.HasPrefix(path, "https://") {
		url = path
	} else {
		url = h.baseUrl + path
	}

	var resp transport.Response
	var reqBody io.Reader

	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			logger.Logger(ctx).Error("error marshaling request body", "method", method, "url", url, "error", err)
			resp.StatusCode = http.StatusInternalServerError
			return resp, err
		}
		reqBody = bytes.NewBuffer(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		logger.Logger(ctx).Error("error creating request", "method", method, "url", url, "error", err)
		resp.StatusCode = http.StatusInternalServerError
		return resp, errs.Wrap(errs.CodeHTTPClientError, "error creating request", err)
	}

	return h.do(ctx, req, headers)
}

func (h *httpTransport) do(
	ctx context.Context,
	req *http.Request,
	headers map[string][]string,
) (transport.Response, error) {
	log := logger.Logger(ctx)
	var resp transport.Response

	applyHeaders(ctx, req, headers)

	res, err := h.client.Do(req)
	if err != nil {
		log.Error("error executing request", "method", req.Method, "url", req.URL.String(), "error", err)
		resp.StatusCode = http.StatusInternalServerError
		return resp, errs.Wrap(errs.CodeHTTPClientError, "error executing request", err)
	}
	defer res.Body.Close()

	resp.StatusCode = res.StatusCode
	body, err := io.ReadAll(res.Body)
	// log.Debug("response", "method", req.Method, "url", req.URL.String(), "status", res.StatusCode, "body", string(body))
	if err != nil {
		log.Error("error reading body", "method", req.Method, "url", req.URL.String(), "status", res.StatusCode)
		resp.StatusCode = http.StatusInternalServerError
		return resp, errs.New(
			errs.CodeHTTPClientError,
			"%s request for %s failed with status code: %d",
			req.Method,
			req.URL.String(),
			res.StatusCode,
		)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		log.Error(
			"request failed",
			"method",
			req.Method,
			"url",
			req.URL.String(),
			"status",
			res.StatusCode,
			"body",
			string(body),
		)
		return resp, errs.New(
			errs.CodeHTTPClientError,
			"%s request for %s failed with status code: %d",
			req.Method,
			req.URL.String(),
			res.StatusCode,
		)
	}
	resp.Body = body

	return resp, nil
}

// Send satisfies the transport interface
// Go considers httpTransport to be a valid transport.Transport because it has all the methods
func (h *httpTransport) Send(ctx context.Context, req *transport.Request) (transport.Response, error) {
	switch req.Method {
	case http.MethodGet:
		return h.get(ctx, req.Path, req.MergedHeaders)
	case http.MethodPost, http.MethodPatch, http.MethodPut:
		return h.requestWithBody(ctx, req.Method, req.Path, req.MergedHeaders, req.Payload)
	default:
		return transport.Response{}, errs.New(errs.CodeInvalidArgument, "unsupported request method")
	}
}
