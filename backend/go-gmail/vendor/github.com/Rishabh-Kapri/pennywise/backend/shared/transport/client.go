package transport

import (
	"context"
	"encoding/json"

	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
)

// Request is protocol agnostic request object
type Request struct {
	Method  string
	Path    string
	Headers map[string][]string
	Payload any
}

// Response is protocol agnostic response object
type Response struct {
	StatusCode int
	Body       []byte
}

// Defines the strategy pattern for inter service communication
// All the actual transports like HTTP will have to satisfy this interface
type Transport interface {
	Send(ctx context.Context, req *Request) (Response, error)
}

type Client struct {
	serviceName string
	transport   Transport
}

func NewClient(serviceName string, transport Transport) *Client {
	return &Client{
		serviceName: serviceName,
		transport:   transport,
	}
}

// Helper methods to accept generics, go doesn't allow generics on struct methods

func Get[T any](ctx context.Context, c *Client, path string) (T, error) {
	var result T

	req := &Request{Method: "GET", Path: path}

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
	logger := logger.Logger(ctx)
	logger.Debug("transport.Post", "service", c.serviceName, "path", path, "data", data)
	var result T

	req := &Request{Method: "POST", Path: path, Headers: headers, Payload: data}

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
	logger := logger.Logger(ctx)
	logger.Debug("transport.Patch", "service", c.serviceName, "path", path, "data", data)
	var result T

	req := &Request{Method: "PATCH", Path: path, Headers: headers, Payload: data}

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
