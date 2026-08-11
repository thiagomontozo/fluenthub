package storage

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type BackupManager struct {
	source, destination string
	retention           time.Duration
}

func NewBackupManager(source, destination string, retention time.Duration) (*BackupManager, error) {
	sourceAbs, err := filepath.Abs(source)
	if err != nil {
		return nil, err
	}
	destinationAbs, err := filepath.Abs(destination)
	if err != nil {
		return nil, err
	}
	if sourceAbs == destinationAbs || strings.HasPrefix(destinationAbs+string(filepath.Separator), sourceAbs+string(filepath.Separator)) {
		return nil, errors.New("backup destination must be outside the storage root")
	}
	if err := os.MkdirAll(destinationAbs, 0o750); err != nil {
		return nil, err
	}
	return &BackupManager{source: sourceAbs, destination: destinationAbs, retention: retention}, nil
}

func (manager *BackupManager) Run(ctx context.Context) error {
	timestamp := time.Now().UTC().Format("20060102T150405Z")
	finalPath := filepath.Join(manager.destination, "fluenthub-storage-"+timestamp+".tar.gz")
	temporary, err := os.CreateTemp(manager.destination, ".backup-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	hash := sha256.New()
	gzipWriter := gzip.NewWriter(io.MultiWriter(temporary, hash))
	tarWriter := tar.NewWriter(gzipWriter)
	err = filepath.Walk(manager.source, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("storage backup refuses symbolic links")
		}
		if info.IsDir() {
			return nil
		}
		if strings.HasPrefix(info.Name(), ".upload-") && strings.HasSuffix(info.Name(), ".tmp") {
			return nil
		}
		relative, err := filepath.Rel(manager.source, path)
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relative)
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(tarWriter, file)
		closeErr := file.Close()
		return errors.Join(copyErr, closeErr)
	})
	err = errors.Join(err, tarWriter.Close(), gzipWriter.Close(), temporary.Sync(), temporary.Close())
	if err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return err
	}
	checksum := hex.EncodeToString(hash.Sum(nil)) + "  " + filepath.Base(finalPath) + "\n"
	if err := os.WriteFile(finalPath+".sha256", []byte(checksum), 0o640); err != nil {
		return err
	}
	return manager.prune(time.Now().UTC())
}

func VerifyBackup(archivePath string) error {
	archive, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, archive); err != nil {
		archive.Close()
		return err
	}
	if err := archive.Close(); err != nil {
		return err
	}
	checksum, err := os.ReadFile(archivePath + ".sha256")
	if err != nil {
		return err
	}
	expected := strings.Fields(string(checksum))
	if len(expected) < 1 || !strings.EqualFold(expected[0], hex.EncodeToString(hash.Sum(nil))) {
		return errors.New("backup checksum mismatch")
	}
	archive, err = os.Open(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()
	gzipReader, err := gzip.NewReader(archive)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	for {
		_, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("invalid backup archive: %w", err)
		}
		if _, err := io.Copy(io.Discard, tarReader); err != nil {
			return err
		}
	}
}

func (manager *BackupManager) prune(now time.Time) error {
	if manager.retention <= 0 {
		return nil
	}
	entries, err := os.ReadDir(manager.destination)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "fluenthub-storage-") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if now.Sub(info.ModTime()) > manager.retention {
			path := filepath.Join(manager.destination, entry.Name())
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("prune backup: %w", err)
			}
		}
	}
	return nil
}
