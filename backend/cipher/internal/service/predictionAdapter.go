package service

import "context"

// A common interface for adapters that will implement this
// Currently we have llm and jev as providers for email prediction
type PredictionPort interface {
	Predict(ctx context.Context, req PredictRequest) (*PredictResponse, error)
	ExtractEmailData(ctx context.Context, req ExtractEmailDataRequest) (*ExtractedInputs, error)
}

type jevPredictionAdapter struct{
	url string
	apiKey string
}

type jevClient struct {}

func NewJevPredictionAdapter() PredictionPort {
	return &jevPredictionAdapter{}
}

func (a *jevPredictionAdapter) Predict(ctx context.Context, req PredictRequest) (*PredictResponse, error) {
	return nil, nil
}

func (a *jevPredictionAdapter) ExtractEmailData(
	ctx context.Context,
	req ExtractEmailDataRequest,
) (*ExtractedInputs, error) {
	return nil, nil
}
