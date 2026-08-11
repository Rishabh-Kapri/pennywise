package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// postgresStore keeps document bodies in the database instead of on a volume or
// in object storage.
//
// The trade is deliberate at this scale: receipts are capped at 10MB each and
// arrive a handful at a time, so the database stays small, while backups become
// a single pg_dump and a document body can no longer outlive -- or go missing
// from under -- its transaction_documents row.
type postgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) (Store, error) {
	if pool == nil {
		return nil, fmt.Errorf("postgres storage requires a database pool")
	}
	return &postgresStore{pool: pool}, nil
}

// key normalises a relative path into the primary key. Kept identical to the
// other implementations so storage_path values are portable between them.
func (s *postgresStore) key(relPath string) (string, error) {
	cleaned := path.Clean("/" + strings.ReplaceAll(relPath, `\`, "/"))
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "" || cleaned == "." {
		return "", fmt.Errorf("invalid storage path %q", relPath)
	}
	return cleaned, nil
}

func (s *postgresStore) Save(dir string, fileName string, r io.Reader) (string, error) {
	relPath := path.Join(dir, fileName)
	blobKey, err := s.key(relPath)
	if err != nil {
		return "", err
	}

	body, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("error reading upload body: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Upsert rather than plain insert: documents are soft-deleted, so a key can
	// legitimately be reused after a delete-and-reupload, and failing there would
	// be surprising.
	_, err = s.pool.Exec(ctx, `
		INSERT INTO document_blobs (key, body)
		VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET body = EXCLUDED.body
	`, blobKey, body)
	if err != nil {
		return "", fmt.Errorf("error storing %q: %w", blobKey, err)
	}
	return relPath, nil
}

// byteReadSeekCloser adapts bytes.Reader, which already seeks, to the Closer the
// interface needs. No temp file is involved -- unlike an object store, the body
// arrives as a single value.
type byteReadSeekCloser struct {
	*bytes.Reader
}

func (byteReadSeekCloser) Close() error { return nil }

func (s *postgresStore) Open(relPath string) (io.ReadSeekCloser, error) {
	blobKey, err := s.key(relPath)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var body []byte
	err = s.pool.QueryRow(ctx, `SELECT body FROM document_blobs WHERE key = $1`, blobKey).Scan(&body)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("error fetching %q: %w", blobKey, err)
	}
	return byteReadSeekCloser{bytes.NewReader(body)}, nil
}

func (s *postgresStore) Delete(relPath string) error {
	blobKey, err := s.key(relPath)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Deleting an absent key is not an error: Delete is also the cleanup path
	// when a document row fails to save, which can run before anything is stored.
	if _, err := s.pool.Exec(ctx, `DELETE FROM document_blobs WHERE key = $1`, blobKey); err != nil {
		return fmt.Errorf("error deleting %q: %w", blobKey, err)
	}
	return nil
}
