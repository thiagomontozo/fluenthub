package billing

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type integrationProvider struct{ statuses map[string]string }

func (*integrationProvider) Name() string { return "asaas" }
func (*integrationProvider) EnsureCustomer(context.Context, Customer) (string, error) {
	return "cus_test", nil
}
func (*integrationProvider) CreateInvoice(_ context.Context, invoice Invoice) (Invoice, error) {
	return invoice, nil
}
func (provider *integrationProvider) GetInvoice(_ context.Context, reference string) (Invoice, error) {
	return Invoice{ProviderReference: reference, Status: provider.statuses[reference]}, nil
}
func (*integrationProvider) CancelInvoice(_ context.Context, reference string) (Invoice, error) {
	return Invoice{ProviderReference: reference, Status: "cancelled"}, nil
}

func TestWebhookIdempotencyAndReconciliationWithPostgreSQL(t *testing.T) {
	databaseURL := os.Getenv("FLUENTHUB_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("FLUENTHUB_TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	applyTestMigrations(t, ctx, db)

	schoolID, studentID := uuid.NewString(), uuid.NewString()
	_, err = db.Exec(ctx, `INSERT INTO schools(id,legal_name,display_name,slug,email,timezone,locale) VALUES($1,'Test School','Test School',$2,'school@example.invalid','UTC','en')`, schoolID, "test-"+uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, `INSERT INTO users(id,school_id,name,email,password_hash) VALUES($1,$2,'Student Test',$3,'not-used')`, studentID, schoolID, uuid.NewString()+"@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	paidInvoice, overdueInvoice := uuid.NewString(), uuid.NewString()
	_, err = db.Exec(ctx, `INSERT INTO invoices(id,school_id,student_id,description,amount_cents,due_date,status,provider,provider_reference) VALUES($1,$3,$4,'Webhook invoice',12345,current_date,'issued','asaas','pay_webhook'),($2,$3,$4,'Reconcile invoice',5000,current_date,'overdue','asaas','pay_reconcile')`, paidInvoice, overdueInvoice, schoolID, studentID)
	if err != nil {
		t.Fatal(err)
	}
	provider := &integrationProvider{statuses: map[string]string{"pay_reconcile": "paid", "pay_webhook": "paid"}}
	service := NewService(db, provider)
	event := AsaasWebhookEvent{ID: "evt_integration_123", Event: "PAYMENT_RECEIVED"}
	event.Payment.ID, event.Payment.Status = "pay_webhook", "RECEIVED"
	if err := service.ProcessAsaasWebhook(ctx, event); err != nil {
		t.Fatal(err)
	}
	if err := service.ProcessAsaasWebhook(ctx, event); err != nil {
		t.Fatal(err)
	}
	var status string
	var payments, events int
	if err := db.QueryRow(ctx, `SELECT status FROM invoices WHERE id=$1`, paidInvoice).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM payments WHERE invoice_id=$1`, paidInvoice).Scan(&payments); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(ctx, `SELECT count(*) FROM asaas_webhook_events WHERE event_id=$1`, event.ID).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if status != "paid" || payments != 1 || events != 1 {
		t.Fatalf("idempotency failed: status=%s payments=%d events=%d", status, payments, events)
	}

	result, err := service.Reconcile(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	if result.Updated < 1 {
		t.Fatalf("expected reconciliation update, got %#v", result)
	}
	if err := db.QueryRow(ctx, `SELECT status FROM invoices WHERE id=$1`, overdueInvoice).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "paid" {
		t.Fatalf("reconciliation left status %s", status)
	}
}

func applyTestMigrations(t *testing.T, ctx context.Context, db *pgxpool.Pool) {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve integration test path")
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(currentFile), "..", "..", "migrations", "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(ctx, string(content)); err != nil {
			t.Fatalf("apply %s: %v", filepath.Base(file), err)
		}
	}
}
