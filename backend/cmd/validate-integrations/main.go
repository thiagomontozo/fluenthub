package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thiagomontozo/fluenthub/backend/internal/billing"
	"github.com/thiagomontozo/fluenthub/backend/internal/certificates"
	"github.com/thiagomontozo/fluenthub/backend/internal/liveclasses"
	"github.com/thiagomontozo/fluenthub/backend/internal/storage"
)

func main() {
	certificatePath := flag.String("certificate", "", "optional output path for a fictitious certificate preview")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	must(validateLiveKit(ctx))
	must(validateAsaas(ctx))
	must(validateStorage(ctx))
	if *certificatePath != "" {
		must(renderCertificate(*certificatePath))
	}
	fmt.Println("validation completed: LiveKit, Asaas, ClamAV protocol, encrypted storage, backup integrity and certificate rendering")
}

func validateLiveKit(ctx context.Context) error {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if !strings.HasPrefix(request.Header.Get("Authorization"), "Bearer ") {
			http.Error(w, "missing bearer", http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/twirp/livekit.RoomService/CreateRoom":
			_, _ = io.WriteString(w, `{"name":"fluenthub-lesson-demo"}`)
		case "/twirp/livekit.RoomService/DeleteRoom", "/twirp/livekit.Egress/StopEgress":
			_, _ = io.WriteString(w, `{}`)
		case "/twirp/livekit.Egress/StartRoomCompositeEgress":
			_, _ = io.WriteString(w, `{"egressId":"EG_demo"}`)
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()
	provider, err := liveclasses.NewLiveKitProvider(server.URL, "demo-key", "demo-secret-at-least-16-bytes", server.Client())
	if err != nil {
		return err
	}
	session, err := provider.CreateSession(ctx, "lesson-demo")
	if err != nil {
		return err
	}
	join, err := provider.GetStudentJoinInfo(ctx, session, "student-demo")
	if err != nil || len(strings.Split(join.Token, ".")) != 3 {
		return errors.New("invalid LiveKit participant token")
	}
	if err := provider.StartRecording(ctx, session); err != nil {
		return err
	}
	if err := provider.StopRecording(ctx, session); err != nil {
		return err
	}
	_, err = provider.EndSession(ctx, session)
	return err
}

func validateAsaas(ctx context.Context) error {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("access_token") != "sandbox-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/customers":
			_, _ = io.WriteString(w, `{"id":"cus_demo"}`)
		case request.Method == http.MethodPost && request.URL.Path == "/payments":
			var payload map[string]any
			_ = json.NewDecoder(request.Body).Decode(&payload)
			if payload["value"] != 123.45 || payload["billingType"] != "BOLETO" {
				http.Error(w, "bad payment", http.StatusBadRequest)
				return
			}
			_, _ = io.WriteString(w, `{"id":"pay_demo","status":"PENDING","bankSlipUrl":"https://sandbox.invalid/pay_demo"}`)
		case request.Method == http.MethodGet && strings.HasSuffix(request.URL.Path, "/identificationField"):
			_, _ = io.WriteString(w, `{"identificationField":"DEMO-BARCODE"}`)
		case request.Method == http.MethodDelete && request.URL.Path == "/payments/pay_demo":
			_, _ = io.WriteString(w, `{"deleted":true}`)
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()
	provider, err := billing.NewAsaasProvider(server.URL, "sandbox-key", server.Client())
	if err != nil {
		return err
	}
	customer, err := provider.EnsureCustomer(ctx, billing.Customer{ExternalID: "student-demo", Name: "Student Example", Email: "student@example.invalid"})
	if err != nil {
		return err
	}
	invoice, err := provider.CreateInvoice(ctx, billing.Invoice{ID: "invoice-demo", CustomerReference: customer, Description: "Fictitious tuition", AmountCents: 12345, DueDate: time.Now().AddDate(0, 0, 7)})
	if err != nil {
		return err
	}
	if invoice.ProviderReference != "pay_demo" || invoice.BoletoBarcode == nil {
		return errors.New("Asaas payment mapping failed")
	}
	_, err = provider.CancelInvoice(ctx, invoice.ProviderReference)
	return err
}

func validateStorage(ctx context.Context) error {
	root, err := os.MkdirTemp("", "fluenthub-storage-validation-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	backupPath := filepath.Join(filepath.Dir(root), filepath.Base(root)+"-backups")
	defer os.RemoveAll(backupPath)
	address, closeScanner, err := fakeClamAV()
	if err != nil {
		return err
	}
	defer closeScanner()
	scanner := storage.ClamAVScanner{Address: address, Timeout: 5 * time.Second}
	if err := scanner.Ping(ctx); err != nil {
		return err
	}
	local, err := storage.NewLocal(root, 4<<20)
	if err != nil {
		return err
	}
	key, _ := base64.StdEncoding.DecodeString("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	secure, err := storage.NewSecure(local, scanner, key, 2<<20, false)
	if err != nil {
		return err
	}
	objectKey, err := secure.Put(ctx, "materials", strings.NewReader("fictitious FluentHub storage payload"))
	if err != nil {
		return err
	}
	reader, err := secure.Open(ctx, objectKey)
	if err != nil {
		return err
	}
	plain, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return err
	}
	if string(plain) != "fictitious FluentHub storage payload" {
		return errors.New("encrypted storage round trip failed")
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(objectKey)))
	if err != nil {
		return err
	}
	if strings.Contains(string(raw), "fictitious FluentHub") {
		return errors.New("plaintext leaked into storage")
	}
	backup, err := storage.NewBackupManager(root, backupPath, 24*time.Hour)
	if err != nil {
		return err
	}
	if err := backup.Run(ctx); err != nil {
		return err
	}
	archives, _ := filepath.Glob(filepath.Join(backupPath, "*.tar.gz"))
	checksums, _ := filepath.Glob(filepath.Join(backupPath, "*.sha256"))
	if len(archives) != 1 || len(checksums) != 1 {
		return errors.New("backup archive or checksum missing")
	}
	if err := storage.VerifyBackup(archives[0]); err != nil {
		return err
	}
	return secure.Close()
}

func fakeClamAV() (string, func(), error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			go handleFakeClamAV(connection)
		}
	}()
	return listener.Addr().String(), func() { _ = listener.Close() }, nil
}

func handleFakeClamAV(connection net.Conn) {
	defer connection.Close()
	reader := bufio.NewReader(connection)
	command, err := reader.ReadString(0)
	if err != nil {
		return
	}
	if command == "zPING\x00" {
		_, _ = connection.Write([]byte("PONG\x00"))
		return
	}
	if command != "zINSTREAM\x00" {
		return
	}
	for {
		var size [4]byte
		if _, err := io.ReadFull(reader, size[:]); err != nil {
			return
		}
		count := binary.BigEndian.Uint32(size[:])
		if count == 0 {
			break
		}
		if _, err := io.CopyN(io.Discard, reader, int64(count)); err != nil {
			return
		}
	}
	_, _ = connection.Write([]byte("stream: OK\x00"))
}

func renderCertificate(path string) error {
	score := 9250
	input := certificates.PDFInput{SchoolName: "English Way School", Title: "Certificate of Completion", Footer: "Issued electronically. Validate authenticity using the QR code.", StudentName: "Student Example", CourseName: "General English", LevelName: "B1", CertificateNumber: "FH-2026-000012", VerificationCode: "DEMO-VALIDATION-2026", VerificationBaseURL: "https://example.invalid", Template: "modern", PrimaryColor: "#1D4ED8", AccentColor: "#F59E0B", CompletionDate: time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC), WorkloadHours: 120, FinalScoreScaled: &score}
	payload, err := (certificates.PDFRenderer{}).Render(input)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o640)
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "validation failed:", err)
		os.Exit(1)
	}
}
