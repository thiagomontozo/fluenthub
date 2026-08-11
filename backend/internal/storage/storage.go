package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ObjectStorage interface {
	Put(context.Context, string, io.Reader) (string, error)
	Open(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
	Exists(context.Context, string) (bool, error)
	Close() error
}

type LocalObjectStorage struct { root string; maxBytes int64 }

func NewLocal(root string, maxBytes int64) (*LocalObjectStorage, error) {
	abs, err := filepath.Abs(root); if err != nil { return nil, err }
	if err := os.MkdirAll(abs, 0o750); err != nil { return nil, err }
	return &LocalObjectStorage{root: abs, maxBytes: maxBytes}, nil
}

func (s *LocalObjectStorage) Put(_ context.Context, namespace string, source io.Reader) (string, error) {
	if !validNamespace(namespace) { return "", errors.New("invalid storage namespace") }
	buf := make([]byte, 24); if _, err := rand.Read(buf); err != nil { return "", err }
	key := namespace + "/" + hex.EncodeToString(buf)
	path, err := s.safePath(key); if err != nil { return "", err }
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil { return "", err }
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640); if err != nil { return "", err }
	written, copyErr := io.Copy(f, io.LimitReader(source, s.maxBytes+1)); closeErr := f.Close()
	if copyErr != nil || closeErr != nil || written > s.maxBytes { _ = os.Remove(path); if written > s.maxBytes { return "", errors.New("upload exceeds configured limit") }; return "", errors.Join(copyErr, closeErr) }
	return key, nil
}

func (s *LocalObjectStorage) Open(_ context.Context, key string) (io.ReadCloser, error) { path, err := s.safePath(key); if err != nil { return nil, err }; return os.Open(path) }
func (s *LocalObjectStorage) Delete(_ context.Context, key string) error { path, err := s.safePath(key); if err != nil { return err }; return os.Remove(path) }
func (s *LocalObjectStorage) Exists(_ context.Context, key string) (bool, error) { path, err := s.safePath(key); if err != nil { return false, err }; _, err = os.Stat(path); if errors.Is(err, os.ErrNotExist) { return false, nil }; return err == nil, err }
func (s *LocalObjectStorage) Close() error { return nil }
func (s *LocalObjectStorage) safePath(key string) (string, error) { if key == "" || filepath.IsAbs(key) || strings.Contains(key, "..") { return "", errors.New("unsafe storage key") }; path := filepath.Clean(filepath.Join(s.root, filepath.FromSlash(key))); rel, err := filepath.Rel(s.root, path); if err != nil || strings.HasPrefix(rel, "..") { return "", fmt.Errorf("storage path escapes root") }; return path, nil }
func validNamespace(v string) bool { if v == "" || len(v) > 40 { return false }; for _, r := range v { if !(r >= 'a' && r <= 'z') && r != '-' { return false } }; return true }
