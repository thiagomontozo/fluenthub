package storage

import (
	"bufio"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"os"
)

const encryptedMagic = "FHENC01\n"
const encryptionChunk = 1024 * 1024

type SecureObjectStorage struct {
	inner                ObjectStorage
	scanner              MalwareScanner
	key                  []byte
	maxBytes             int64
	allowLegacyPlaintext bool
}

func NewSecure(inner ObjectStorage, scanner MalwareScanner, key []byte, maxBytes int64, allowLegacy bool) (*SecureObjectStorage, error) {
	if inner == nil || maxBytes < 1 {
		return nil, errors.New("invalid secure storage configuration")
	}
	if len(key) != 0 && len(key) != 32 {
		return nil, errors.New("storage encryption key must contain 32 bytes")
	}
	return &SecureObjectStorage{inner: inner, scanner: scanner, key: append([]byte(nil), key...), maxBytes: maxBytes, allowLegacyPlaintext: allowLegacy}, nil
}

func (storage *SecureObjectStorage) Put(ctx context.Context, namespace string, source io.Reader) (string, error) {
	plain, err := os.CreateTemp("", "fluenthub-quarantine-*")
	if err != nil {
		return "", err
	}
	plainName := plain.Name()
	defer os.Remove(plainName)
	defer plain.Close()
	written, err := io.Copy(plain, io.LimitReader(source, storage.maxBytes+1))
	if err != nil || written > storage.maxBytes {
		if written > storage.maxBytes {
			return "", errors.New("upload exceeds configured limit")
		}
		return "", err
	}
	if storage.scanner != nil {
		if _, err := plain.Seek(0, io.SeekStart); err != nil {
			return "", err
		}
		if err := storage.scanner.Scan(ctx, plain); err != nil {
			return "", err
		}
	}
	if _, err := plain.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	if len(storage.key) == 0 {
		return storage.inner.Put(ctx, namespace, plain)
	}
	encrypted, err := os.CreateTemp("", "fluenthub-encrypted-*")
	if err != nil {
		return "", err
	}
	encryptedName := encrypted.Name()
	defer os.Remove(encryptedName)
	defer encrypted.Close()
	if err := encryptStream(encrypted, plain, storage.key); err != nil {
		return "", err
	}
	if _, err := encrypted.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	return storage.inner.Put(ctx, namespace, encrypted)
}

func (storage *SecureObjectStorage) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	raw, err := storage.inner.Open(ctx, key)
	if err != nil {
		return nil, err
	}
	if len(storage.key) == 0 {
		return raw, nil
	}
	reader := bufio.NewReader(raw)
	header, err := reader.Peek(len(encryptedMagic))
	if err != nil || string(header) != encryptedMagic {
		if storage.allowLegacyPlaintext {
			return &combinedReadCloser{Reader: reader, Closer: raw}, nil
		}
		raw.Close()
		return nil, errors.New("object is not encrypted with the FluentHub envelope")
	}
	pipeReader, pipeWriter := io.Pipe()
	go func() {
		err := decryptStream(pipeWriter, reader, storage.key)
		_ = raw.Close()
		_ = pipeWriter.CloseWithError(err)
	}()
	return pipeReader, nil
}

func (storage *SecureObjectStorage) Delete(ctx context.Context, key string) error {
	return storage.inner.Delete(ctx, key)
}
func (storage *SecureObjectStorage) Exists(ctx context.Context, key string) (bool, error) {
	return storage.inner.Exists(ctx, key)
}
func (storage *SecureObjectStorage) Close() error {
	for index := range storage.key {
		storage.key[index] = 0
	}
	return storage.inner.Close()
}

type combinedReadCloser struct {
	io.Reader
	io.Closer
}

func encryptStream(destination io.Writer, source io.Reader, key []byte) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(destination, encryptedMagic); err != nil {
		return err
	}
	buffer := make([]byte, encryptionChunk)
	for {
		count, readErr := source.Read(buffer)
		if count > 0 {
			nonce := make([]byte, aead.NonceSize())
			if _, err := rand.Read(nonce); err != nil {
				return err
			}
			var size [4]byte
			binary.BigEndian.PutUint32(size[:], uint32(count))
			if _, err := destination.Write(size[:]); err != nil {
				return err
			}
			if _, err := destination.Write(nonce); err != nil {
				return err
			}
			if _, err := destination.Write(aead.Seal(nil, nonce, buffer[:count], size[:])); err != nil {
				return err
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	_, err = destination.Write([]byte{0, 0, 0, 0})
	return err
}

func decryptStream(destination io.Writer, source io.Reader, key []byte) error {
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	header := make([]byte, len(encryptedMagic))
	if _, err := io.ReadFull(source, header); err != nil || string(header) != encryptedMagic {
		return errors.New("invalid encrypted object header")
	}
	for {
		var size [4]byte
		if _, err := io.ReadFull(source, size[:]); err != nil {
			return err
		}
		plainSize := binary.BigEndian.Uint32(size[:])
		if plainSize == 0 {
			return nil
		}
		if plainSize > encryptionChunk {
			return errors.New("invalid encrypted object chunk")
		}
		nonce := make([]byte, aead.NonceSize())
		if _, err := io.ReadFull(source, nonce); err != nil {
			return err
		}
		sealed := make([]byte, int(plainSize)+aead.Overhead())
		if _, err := io.ReadFull(source, sealed); err != nil {
			return err
		}
		plain, err := aead.Open(nil, nonce, sealed, size[:])
		if err != nil {
			return errors.New("encrypted object authentication failed")
		}
		if _, err := destination.Write(plain); err != nil {
			return err
		}
	}
}
