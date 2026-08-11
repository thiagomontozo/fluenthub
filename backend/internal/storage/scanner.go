package storage

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

var ErrMalwareDetected = errors.New("upload rejected: malware detected")

type MalwareScanner interface {
	Scan(context.Context, io.Reader) error
}

type ClamAVScanner struct {
	Address string
	Timeout time.Duration
}

func (scanner ClamAVScanner) Ping(ctx context.Context) error {
	timeout := scanner.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	dialer := net.Dialer{Timeout: timeout}
	connection, err := dialer.DialContext(ctx, "tcp", scanner.Address)
	if err != nil {
		return fmt.Errorf("connect to malware scanner: %w", err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(timeout))
	if _, err := connection.Write([]byte("zPING\x00")); err != nil {
		return err
	}
	response, err := bufio.NewReader(connection).ReadString(0)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	if strings.TrimSpace(strings.TrimSuffix(response, "\x00")) != "PONG" {
		return errors.New("malware scanner health check failed")
	}
	return nil
}

func (scanner ClamAVScanner) Scan(ctx context.Context, source io.Reader) error {
	timeout := scanner.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	dialer := net.Dialer{Timeout: timeout}
	connection, err := dialer.DialContext(ctx, "tcp", scanner.Address)
	if err != nil {
		return fmt.Errorf("connect to malware scanner: %w", err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(timeout))
	if _, err := connection.Write([]byte("zINSTREAM\x00")); err != nil {
		return fmt.Errorf("start malware scan: %w", err)
	}
	buffer := make([]byte, 32*1024)
	for {
		count, readErr := source.Read(buffer)
		if count > 0 {
			var size [4]byte
			binary.BigEndian.PutUint32(size[:], uint32(count))
			if _, err := connection.Write(size[:]); err != nil {
				return fmt.Errorf("stream malware scan: %w", err)
			}
			if _, err := connection.Write(buffer[:count]); err != nil {
				return fmt.Errorf("stream malware scan: %w", err)
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if _, err := connection.Write([]byte{0, 0, 0, 0}); err != nil {
		return fmt.Errorf("finish malware scan: %w", err)
	}
	response, err := bufio.NewReader(connection).ReadString(0)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("read malware scan result: %w", err)
	}
	response = strings.TrimSpace(strings.TrimSuffix(response, "\x00"))
	if strings.HasSuffix(response, " FOUND") {
		return ErrMalwareDetected
	}
	if !strings.HasSuffix(response, " OK") {
		return fmt.Errorf("malware scanner returned an indeterminate result")
	}
	return nil
}
