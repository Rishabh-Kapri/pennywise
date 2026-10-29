package noop

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/cipher/internal/ports"
)

type checkTransaction struct{}

func NewCheckTransaction() *checkTransaction {
	return &checkTransaction{}
}

func (c *checkTransaction) IsTransaction(ctx context.Context, msg ports.Message) (ports.TxnVerdict, error) {
	return ports.TxnVerdict{
		IsTransaction: false,
		Confidence:    0.0,
	}, nil
}
