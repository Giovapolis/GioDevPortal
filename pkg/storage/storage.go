package storage

import (
	"context"
	"io"
)

type Store interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	URL(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
	Backend() string
}
