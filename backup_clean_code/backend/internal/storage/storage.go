package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

var ErrStorageDisabled = errors.New("artifact storage disabled")

type ArtifactStorage interface {
	Upload(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	Health(ctx context.Context) error
}

type NoopStorage struct{}

func (NoopStorage) Upload(context.Context, string, io.Reader, int64, string) error {
	return ErrStorageDisabled
}

func (NoopStorage) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "", ErrStorageDisabled
}

func (NoopStorage) Health(context.Context) error {
	return ErrStorageDisabled
}
