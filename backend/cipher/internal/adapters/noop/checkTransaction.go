package noop

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
)

type checkTransaction struct{}

func NewCheckTransaction() *checkTransaction {
	return &checkTransaction{}
}

func (c *checkTransaction) Check(ctx context.Context, msg ports.Message) (ports.TxnVerdict, error) {
	return ports.TxnVerdict{
		IsTransaction: false,
		Confidence:    new(0.0),
	}, nil
}
