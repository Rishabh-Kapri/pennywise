package transport

import (
	"context"
	"encoding/json"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/utils"
)

// Request is protocol agnostic request object
type Request struct {
	Method        string
	Path          string
	MergedHeaders map[string][]string
	Payload       any
}

// Response is protocol agnostic response object
type Response struct {
	StatusCode int
	Body       []byte
}

type SSEEvent struct {
	Event string
	Data  []byte
}

type StreamResponse struct {
	StatusCode int
	Events     <-chan SSEEvent // receive only channel, prevents consumers sending events
}

// Defines the strategy pattern for inter service communication
// All the actual transports like HTTP will have to satisfy this interface
type Transport interface {
	Send(ctx context.Context, req *Request) (Response, error)
	Stream(ctx context.Context, req *Request) (StreamResponse, error)
}

type Client struct {
	serviceName              string
	transport                Transport
	defaultHeaders           map[string][]string
	propagateInternalHeaders bool
}

type Options struct {
	Headers                  map[string][]string
	PropagateInternalHeaders bool
}

type Option func(*Options)

func WithDefaultHeaders(headers map[string][]string) Option {
	return func(o *Options) {
		o.Headers = headers
	}
}

func WithPropagateInternalHeaders(value bool) Option {
	return func(o *Options) {
		o.PropagateInternalHeaders = value
	}
}

func NewClient(serviceName string, transport Transport, opts ...Option) *Client {
	options := Options{
		PropagateInternalHeaders: true,
	}
	for _, opt := range opts {
		opt(&options)
	}
	return &Client{
		serviceName:              serviceName,
		transport:                transport,
		defaultHeaders:           options.Headers,
		propagateInternalHeaders: options.PropagateInternalHeaders,
	}
}

func (c *Client) getMergedHeader(ctx context.Context, headers map[string][]string) map[string][]string {
	mergedHeaders := make(map[string][]string)

	if c.defaultHeaders != nil {
		for key, value := range c.defaultHeaders {
			mergedHeaders[key] = value
		}
	}

	if headers != nil {
		for key, value := range headers {
			mergedHeaders[key] = value
		}
	}

	if c.propagateInternalHeaders == true {
		for key, value := range utils.GetHeaders(ctx) {
			if mergedHeaders[key] == nil {
				mergedHeaders[key] = value
			}
		}
	}
	return mergedHeaders
}

// Helper methods to accept generics, go doesn't allow generics on struct methods

func Get[T any](ctx context.Context, c *Client, path string) (T, error) {
	var result T

	req := &Request{Method: "GET", Path: path, MergedHeaders: c.getMergedHeader(ctx, nil)}

	res, err := c.transport.Send(ctx, req)
	if err != nil {
		return result, err
	}

	if len(res.Body) > 0 {
		if err := json.Unmarshal(res.Body, &result); err != nil {
			return result, err
		}
	}
	return result, nil
}

func Post[T any](ctx context.Context, c *Client, path string, headers map[string][]string, data any) (T, error) {
	// log := logger.Logger(ctx)
	// log.Info("transport.Post", "service", c.serviceName, "path", path, "data", data)
	var result T

	req := &Request{Method: "POST", Path: path, MergedHeaders: c.getMergedHeader(ctx, headers), Payload: data}

	// Engine gives us raw bytes
	res, err := c.transport.Send(ctx, req)
	if err != nil {
		return result, err
	}

	// Unmarshal raw bytes to generic type T
	if len(res.Body) > 0 {
		if err := json.Unmarshal(res.Body, &result); err != nil {
			return result, err
		}
	}

	return result, nil
}

func Patch[T any](ctx context.Context, c *Client, path string, headers map[string][]string, data any) (T, error) {
	// logger := logger.Logger(ctx)
	// logger.Debug("transport.Patch", "service", c.serviceName, "path", path, "data", data)
	var result T

	req := &Request{Method: "PATCH", Path: path, MergedHeaders: c.getMergedHeader(ctx, headers), Payload: data}

	res, err := c.transport.Send(ctx, req)
	if err != nil {
		return result, err
	}

	if len(res.Body) > 0 {
		if err := json.Unmarshal(res.Body, &result); err != nil {
			return result, err
		}
	}

	return result, nil
}

func StreamPost(ctx context.Context, c *Client, path string, headers map[string][]string, data any) (StreamResponse, error) {
	req := &Request{Method: "POST", Path: path, MergedHeaders: c.getMergedHeader(ctx, headers), Payload: data}

	return c.transport.Stream(ctx, req)
}
