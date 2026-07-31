package client

import "context"

// Embedder produces a dense vector for text using the named model. It is
// deliberately narrow so the email pipeline can hold an ordered chain of
// interchangeable embedding backends.
//
// Every backend in a chain must serve the *same* model: stored pgvector rows
// are only comparable to query vectors from the same embedding space, so
// swapping in a different model would silently invalidate the whole corpus.
type Embedder interface {
	Embed(ctx context.Context, model string, text string) ([]float64, error)
}
