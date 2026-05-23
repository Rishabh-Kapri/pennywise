package httpclient

import (
	"bufio"
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

type doResult struct {
	StatusCode  int
	OriginalReq *http.Request
	Body        io.ReadCloser
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

func (h *httpTransport) get(
	ctx context.Context,
	path string,
	headers map[string][]string,
) (doResult, error) {
	var resp doResult
	url := h.baseUrl + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		logger.Logger(ctx).Error("error creating GET request", "url", url, "error", err)
		resp.StatusCode = http.StatusInternalServerError
		return resp, err
	}
	return h.do(ctx, req, headers)
}

func (h *httpTransport) requestWithBody(
	ctx context.Context,
	method string,
	path string,
	headers map[string][]string,
	data any,
) (doResult, error) {
	// logger.Logger(ctx).Debug("httpTransport."+method, "url", h.baseUrl, "path", path, "data", data)
	var url string
	if strings.HasPrefix(path, "https://") {
		url = path
	} else {
		url = h.baseUrl + path
	}

	var resp doResult
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

func (h *httpTransport) handleBodyParse(ctx context.Context, req doResult) (transport.Response, error) {
	log := logger.Logger(ctx)
	var resp transport.Response

	originalReq := req.OriginalReq
	body := req.Body
	if body == nil {
		return resp, errs.New(errs.CodeHTTPClientError, "empty body to read from")
	}
	defer body.Close()

	parsedBody, err := io.ReadAll(body)
	if err != nil {
		log.Error(
			"error reading body",
			"method",
			originalReq.Method,
			"url",
			originalReq.URL.String(),
			"status",
			req.StatusCode,
		)
		req.StatusCode = http.StatusInternalServerError
		return resp, errs.New(
			errs.CodeHTTPClientError,
			"%s request for %s failed with status code: %d",
			originalReq.Method,
			originalReq.URL.String(),
			req.StatusCode,
		)
	}
	log.Info(
		"response",
		"method",
		originalReq.Method,
		"url",
		originalReq.URL.String(),
		"status",
		req.StatusCode,
		"body",
		string(parsedBody),
	)

	resp.StatusCode = req.StatusCode
	resp.Body = parsedBody

	return resp, nil
}

func (h *httpTransport) handleSSE(ctx context.Context, doReq doResult) <-chan transport.SSEEvent {
	out := make(chan transport.SSEEvent)

	if doReq.Body == nil {
		return nil
	}

	go func() {
		defer func() {
			close(out)
			doReq.Body.Close()
		}()

		scanner := bufio.NewScanner(doReq.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		var eventName string
		var dataLines []string

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				if eventName != "" || len(dataLines) > 0 {
					out <- transport.SSEEvent{
						Event: eventName,
						Data:  []byte(strings.Join(dataLines, "\n")),
					}
					eventName = ""
					dataLines = make([]string, 0)
				}
			}

			if strings.HasPrefix(line, "event: ") {
				eventName = strings.TrimPrefix(line, "event: ")
				continue
			}

			if strings.HasPrefix(line, "data: ") {
				dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
				continue
			}
		}
	}()

	return out
}

func (h *httpTransport) do(
	ctx context.Context,
	req *http.Request,
	headers map[string][]string,
) (doResult, error) {
	log := logger.Logger(ctx)
	var result doResult

	applyHeaders(ctx, req, headers)

	log.Info("httpTransport.do", "method", req.Method, "url", req.URL.String(), "headers", headers)

	res, err := h.client.Do(req)
	if err != nil {
		log.Error("error executing request", "method", req.Method, "url", req.URL.String(), "error", err)
		result.StatusCode = http.StatusInternalServerError
		return result, errs.Wrap(errs.CodeHTTPClientError, "error executing request", err)
	}

	result.StatusCode = res.StatusCode
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
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
		return result, errs.New(
			errs.CodeHTTPClientError,
			"%s request for %s failed with status code: %d",
			req.Method,
			req.URL.String(),
			res.StatusCode,
		)
	}
	result.Body = res.Body
	result.OriginalReq = req
	return result, nil
}

// Send satisfies the transport interface
// Go considers httpTransport to be a valid transport.Transport because it has all the methods
func (h *httpTransport) Send(ctx context.Context, req *transport.Request) (transport.Response, error) {
	var doReq doResult
	var err error
	switch req.Method {
	case http.MethodGet:
		doReq, err = h.get(ctx, req.Path, req.MergedHeaders)
		break
	case http.MethodPost, http.MethodPatch, http.MethodPut:
		doReq, err = h.requestWithBody(ctx, req.Method, req.Path, req.MergedHeaders, req.Payload)
		break
	default:
		return transport.Response{}, errs.New(errs.CodeInvalidArgument, "unsupported request method")
	}
	if err != nil {
		return transport.Response{}, err
	}
	return h.handleBodyParse(ctx, doReq)
}

func (h *httpTransport) Stream(ctx context.Context, req *transport.Request) (transport.StreamResponse, error) {
	var doReq doResult
	var err error
	switch req.Method {
	case http.MethodPost:
		doReq, err = h.requestWithBody(ctx, req.Method, req.Path, req.MergedHeaders, req.Payload)
	default:
		return transport.StreamResponse{}, errs.New(errs.CodeInvalidArgument, "unsupported stream request method")
	}
	if err != nil {
		return transport.StreamResponse{}, err
	}
	sseChan := h.handleSSE(ctx, doReq)
	return transport.StreamResponse{
		StatusCode: doReq.StatusCode,
		Events:     sseChan,
	}, nil
}
