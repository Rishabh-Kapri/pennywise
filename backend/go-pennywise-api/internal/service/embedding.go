package service

import (
	"context"

	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/model"
	"github.com/Rishabh-Kapri/pennywise/backend/go-pennywise-api/internal/repository"
)

type EmbeddingService interface {
	Get(ctx context.Context, docType string, queryStr string, limit int64) ([]model.Embedding, error)
	Create(ctx context.Context, data model.Embedding) error
}

type embeddingService struct {
	repo repository.EmbeddingRepository
}

func NewEmbeddingService(r repository.EmbeddingRepository) EmbeddingService {
	return &embeddingService{repo: r}
}

func (s *embeddingService) Get(ctx context.Context, docType string, queryStr string, limit int64) ([]model.Embedding, error) {
	// url := "http://192.168.1.24:8000/embeddings"
	// postData := map[string]string{
	// 	"content": queryStr,
	// }
	// embedding, err := httpclient.Post[[]float64](ctx, url, postData)
	// if err != nil {
	// 	return nil, err
	// }
	// embeddingStr := utils.Float64SliceToVectorString(embedding)
	// return s.repo.Get(ctx, docType, embeddingStr, limit)
	return nil, nil
}

func (s *embeddingService) Create(ctx context.Context, data model.Embedding) error {
	// url := "http://192.168.1.24:8000/embeddings"
	// postData := map[string]string{
	// 	"content": data.Content,
	// }
	// embedding, err := httpclient.Post[[]float64](ctx, url, postData)
	// if err != nil {
	// 	return err
	// }
	// embeddingStr := utils.Float64SliceToVectorString(embedding)
	// return s.repo.Create(ctx, data, embeddingStr)
	return nil
}
