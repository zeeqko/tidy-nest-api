// Package storage abstracts where user-uploaded item photos live: a Cloudflare
// R2 bucket when the app is configured for it, the local filesystem otherwise
// so the backend still runs with no cloud credentials.
package storage

import (
	"context"
	"errors"
	"io"
	"os"
)

// ErrNotFound is returned by Get when the key has no object behind it.
var ErrNotFound = errors.New("storage: object not found")

// Object is a stored photo opened for reading. Callers must close Body.
type Object struct {
	Body        io.ReadCloser
	ContentType string
	Size        int64
}

// Store reads and writes item photos by key (e.g. "ab12cd34.jpg"). Keys are
// opaque to callers other than the upload controller, which mints them.
type Store interface {
	Put(ctx context.Context, key, contentType string, body io.Reader, size int64) error
	Get(ctx context.Context, key string) (*Object, error)
	// Describe names the backing store for startup logging.
	Describe() string
}

// FromEnv builds the store described by the environment: R2 when its four
// settings are present, otherwise a local directory. Partial R2 configuration
// is an error rather than a silent fall back to disk, which would look like
// uploads working while nothing reaches the bucket.
func FromEnv(ctx context.Context) (Store, error) {
	cfg := R2Config{
		AccountID:       os.Getenv("R2_ACCOUNT_ID"),
		Bucket:          os.Getenv("R2_BUCKET"),
		AccessKeyID:     os.Getenv("R2_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"),
	}

	set := 0
	for _, v := range []string{cfg.AccountID, cfg.Bucket, cfg.AccessKeyID, cfg.SecretAccessKey} {
		if v != "" {
			set++
		}
	}
	switch set {
	case 4:
		return NewR2(ctx, cfg)
	case 0:
		dir := os.Getenv("UPLOAD_DIR")
		if dir == "" {
			dir = "uploads"
		}
		return NewLocal(dir), nil
	default:
		return nil, errors.New("incomplete R2 configuration: set R2_ACCOUNT_ID, R2_BUCKET, R2_ACCESS_KEY_ID and R2_SECRET_ACCESS_KEY together, or none of them")
	}
}
