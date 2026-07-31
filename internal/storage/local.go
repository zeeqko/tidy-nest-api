package storage

import (
	"context"
	"io"
	"mime"
	"os"
	"path/filepath"
)

// localStore keeps photos in a directory on the machine running the backend.
// It is the development default and the fallback when R2 is not configured.
type localStore struct {
	dir string
}

func NewLocal(dir string) Store {
	return &localStore{dir: dir}
}

func (s *localStore) Describe() string {
	return "local directory " + s.dir
}

func (s *localStore) Put(_ context.Context, key, _ string, body io.Reader, _ int64) error {
	path := filepath.Join(s.dir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	dst, err := os.Create(path)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, body); err != nil {
		return err
	}
	return dst.Close()
}

func (s *localStore) Delete(_ context.Context, key string) error {
	path := filepath.Join(s.dir, filepath.FromSlash(key))
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *localStore) Get(_ context.Context, key string) (*Object, error) {
	path := filepath.Join(s.dir, filepath.FromSlash(key))
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	// The extension is ours (the upload controller picked it from the sniffed
	// type), so deriving the content type back from it is safe.
	contentType := mime.TypeByExtension(filepath.Ext(path))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return &Object{Body: file, ContentType: contentType, Size: info.Size()}, nil
}
