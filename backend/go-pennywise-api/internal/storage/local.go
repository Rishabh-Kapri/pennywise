package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Store persists uploaded document bodies. Paths handed out and accepted are
// relative to the store root so DB rows stay valid if the root moves.
type Store interface {
	// Save streams r to a new file under dir (relative), returning the relative path.
	Save(dir string, fileName string, r io.Reader) (string, error)
	// Open returns a reader for a previously saved relative path.
	Open(relPath string) (io.ReadSeekCloser, error)
	Delete(relPath string) error
}

type localStore struct {
	root string
}

func NewLocalStore(root string) (Store, error) {
	if root == "" {
		return nil, fmt.Errorf("uploads root is required")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("error creating uploads root %q: %w", root, err)
	}
	return &localStore{root: root}, nil
}

// resolve joins relPath to the root and rejects anything escaping it.
func (s *localStore) resolve(relPath string) (string, error) {
	full := filepath.Join(s.root, filepath.Clean("/"+relPath))
	rootAbs, err := filepath.Abs(s.root)
	if err != nil {
		return "", err
	}
	fullAbs, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	if fullAbs != rootAbs && !strings.HasPrefix(fullAbs, rootAbs+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid storage path %q", relPath)
	}
	return full, nil
}

func (s *localStore) Save(dir string, fileName string, r io.Reader) (string, error) {
	relPath := filepath.Join(dir, fileName)
	full, err := s.resolve(relPath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	f, err := os.OpenFile(full, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		os.Remove(full)
		return "", err
	}
	return relPath, nil
}

func (s *localStore) Open(relPath string) (io.ReadSeekCloser, error) {
	full, err := s.resolve(relPath)
	if err != nil {
		return nil, err
	}
	return os.Open(full)
}

func (s *localStore) Delete(relPath string) error {
	full, err := s.resolve(relPath)
	if err != nil {
		return err
	}
	err = os.Remove(full)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
