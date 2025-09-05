package repository

import (
	"context"

	"pennywise-api/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type EmbeddingRepository interface {
	Get(ctx context.Context, docType string, embeddingStr string, limit int64) ([]model.Embedding, error)
	Create(ctx context.Context, data model.Embedding, embeddingStr string) error
}

type embeddingRepository struct {
	db *pgxpool.Pool
}

func NewEmbeddingRepository(db *pgxpool.Pool) EmbeddingRepository {
	return &embeddingRepository{db}
}

func (er *embeddingRepository) Get(ctx context.Context, docType string, embeddingStr string, limit int64) ([]model.Embedding, error) {
	// rows, err := er.db.Query(
	// 	ctx, `
	// 		SELECT
	// 			content,
	// 			doc_type,
	// 			source_id,
	// 			sequence_index,
	// 			email,
	// 			created_at,
	// 			updated_at,
	// 	    1 - (embedding <=> $2) AS similarity_score
	// 		FROM
	// 			embeddings
	// 		WHERE
	// 			doc_type = $1
	// 		ORDER BY
	// 	    embedding <=> $2 ASC
	// 		LIMIT $3;
	//   `, docType, embeddingStr, limit,
	// )
	rows, err := er.db.Query(
		ctx, `
			SELECT
				content,
				doc_type,
				source_id,
				sequence_index,
				email,
				created_at,
				updated_at,
		    1 - (embedding <=> $2) AS similarity_score
			FROM embeddings
			WHERE doc_type = $1
			ORDER BY embedding <=> $2 ASC
			LIMIT $3;
		`, docType, embeddingStr, limit,
		)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var embeddings []model.Embedding
	for rows.Next() {
		var e model.Embedding
		if err = rows.Scan(
			&e.Content,
			&e.DocType,
			&e.SourceID,
			&e.SequenceIndex,
			&e.Email,
			&e.CreatedAt,
			&e.UpdatedAt,
			&e.SimilarityScore,
		); err != nil {
			return nil, err
		}
		embeddings = append(embeddings, e)
	}

	return embeddings, nil
}

func (er *embeddingRepository) Create(ctx context.Context, data model.Embedding, embeddingStr string) error {
	_, err := er.db.Exec(
		ctx, `
	  INSERT INTO embeddings (
		  content,
		  doc_type,
		  embedding,
		  source_id,
		  sequence_index,
		  email,
		  created_at,
		  updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
	`, data.Content, data.DocType, embeddingStr, data.SourceID, data.SequenceIndex, data.Email,
	)
	return err
}
