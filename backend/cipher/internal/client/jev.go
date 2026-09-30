package client

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/config"
	errs "github.com/Rishabh-Kapri/pennywise/backend/shared/errors"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/httpclient"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/logger"
	"github.com/Rishabh-Kapri/pennywise/backend/shared/transport"
)

type jevClient struct {
	name   string
	client *transport.Client
}

type ClientOpts struct {
	transport transport.Transport
}

type ClientOpt func(*ClientOpts)

func WithTransport(transport transport.Transport) ClientOpt {
	return func(o *ClientOpts) {
		o.transport = transport
	}
}

func NewJevClient(config *config.Config, name string, opts ...ClientOpt) error {
	if config.TypesafeAPIKey == "" {
		return errs.New(errs.CodeInternalError, "no api key found for jev")
	}
	headers := map[string][]string{
		"content-type":  {"application/json"},
		"authorization": {config.TypesafeAPIKey},
	}
	headerOpts := transport.WithDefaultHeaders(headers)

	options := ClientOpts{}
	for _, opt := range opts {
		opt(&options)
	}
	if options.transport == nil {
		// default to http transport
		options.transport = httpclient.NewHttpTransport("https://api.typesafe.ai")
	}
	transportClient := transport.NewClient(
		"cipher."+name,
		options.transport,
		headerOpts,
		transport.WithPropagateInternalHeaders(false),
	)
	logger.Logger(context.Background()).Info("jev client created")

	c := &jevClient{
		name:   name,
		client: transportClient,
	}

	return nil
}

func (c *jevClient) Test(ctx context.Context) error {
	transport.Post[any](ctx, c.client, "")
}
