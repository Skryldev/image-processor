// Package storage provides StorageAdapter implementations.
package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Skryldev/image-processor/core"
	apperrors "github.com/Skryldev/image-processor/errors"
)

// Local stores images on the local filesystem.
type Local struct {
	rootDir     string
	permissions os.FileMode
}

// NewLocal creates a Local storage adapter rooted at dir.
func NewLocal(dir string, perm os.FileMode) (*Local, error) {
	if perm == 0 {
		perm = 0o644
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("local storage: mkdir %s: %w", dir, err)
	}
	return &Local{rootDir: dir, permissions: perm}, nil
}

func (l *Local) absPath(key core.StorageKey) string {
	// Bucket maps to a subdirectory; Path is the filename.
	return filepath.Join(l.rootDir, filepath.Clean(key.Bucket), filepath.Clean(key.Path))
}

func (l *Local) Put(ctx context.Context, key core.StorageKey, r io.Reader, meta map[string]string) error {
	if err := ctx.Err(); err != nil {
		return apperrors.Wrap(apperrors.CategoryStorage, "local.put", err)
	}

	path := l.absPath(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return apperrors.Wrap(apperrors.CategoryStorage, "local.put.mkdir", err)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, l.permissions)
	if err != nil {
		return apperrors.Wrap(apperrors.CategoryStorage, "local.put.open", err)
	}
	defer f.Close()

	if _, err = io.Copy(f, r); err != nil {
		return apperrors.Wrap(apperrors.CategoryStorage, "local.put.copy", err)
	}

	// Persist metadata as a side-car JSON file.
	if len(meta) > 0 {
		metaPath := path + ".meta.json"
		mf, err := os.OpenFile(metaPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, l.permissions)
		if err == nil {
			_ = json.NewEncoder(mf).Encode(meta)
			mf.Close()
		}
	}
	return nil
}

func (l *Local) Get(ctx context.Context, key core.StorageKey) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, apperrors.Wrap(apperrors.CategoryStorage, "local.get", err)
	}
	f, err := os.Open(l.absPath(key))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, apperrors.New(apperrors.CategoryStorage, "local.get", fmt.Errorf("key not found: %v", key))
		}
		return nil, apperrors.Wrap(apperrors.CategoryStorage, "local.get.open", err)
	}
	return f, nil
}

func (l *Local) Delete(ctx context.Context, key core.StorageKey) error {
	if err := ctx.Err(); err != nil {
		return apperrors.Wrap(apperrors.CategoryStorage, "local.delete", err)
	}
	path := l.absPath(key)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return apperrors.Wrap(apperrors.CategoryStorage, "local.delete", err)
	}
	_ = os.Remove(path + ".meta.json")
	return nil
}

func (l *Local) Exists(ctx context.Context, key core.StorageKey) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, apperrors.Wrap(apperrors.CategoryStorage, "local.exists", err)
	}
	_, err := os.Stat(l.absPath(key))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, apperrors.Wrap(apperrors.CategoryStorage, "local.exists.stat", err)
}