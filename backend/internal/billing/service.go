package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ReconciliationResult struct {
	Inspected, Updated, Failures int
}

type CreateInvoiceInput struct {
	StudentID, Description string
	EnrollmentID           *string
	AmountCents            int64
	DueDate                time.Time
}

type Service struct {
	db       *pgxpool.Pool
	provider BillingProvider
}

func NewService(db *pgxpool.Pool, provider BillingProvider) *Service {
	return &Service{db: db, provider: provider}
}

func (s *Service) Create(ctx context.Context, schoolID, actorID string, input CreateInvoiceInput) (Invoice, error) {
	if input.StudentID == "" || len(input.Description) < 3 || input.AmountCents < 1 || input.DueDate.IsZero() {
		return Invoice{}, errors.New("student, description, positive amountCents and dueDate are required")
	}
	var studentName, studentEmail string
	if err := s.db.QueryRow(ctx, `SELECT name,email FROM users WHERE id=$1 AND school_id=$2 AND active=true`, input.StudentID, schoolID).Scan(&studentName, &studentEmail); err != nil {
		return Invoice{}, errors.New("student not found in this school")
	}
	var customerReference string
	err := s.db.QueryRow(ctx, `SELECT COALESCE(provider_customer_reference,'') FROM billing_accounts WHERE school_id=$1 AND student_id=$2 AND active=true`, schoolID, input.StudentID).Scan(&customerReference)
	if err != nil || customerReference == "" {
		customerReference, err = s.provider.EnsureCustomer(ctx, Customer{ExternalID: input.StudentID, Name: studentName, Email: studentEmail})
		if err != nil {
			return Invoice{}, err
		}
		_, err = s.db.Exec(ctx, `INSERT INTO billing_accounts(school_id,student_id,provider_customer_reference) VALUES($1,$2,$3) ON CONFLICT(school_id,student_id) DO UPDATE SET provider_customer_reference=EXCLUDED.provider_customer_reference,active=true`, schoolID, input.StudentID, customerReference)
		if err != nil {
			return Invoice{}, err
		}
	}
	invoice, err := s.provider.CreateInvoice(ctx, Invoice{ID: uuid.NewString(), SchoolID: schoolID, StudentID: input.StudentID, CustomerReference: customerReference, EnrollmentID: input.EnrollmentID, Description: input.Description, AmountCents: input.AmountCents, DueDate: input.DueDate})
	if err != nil {
		return Invoice{}, err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Invoice{}, err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `INSERT INTO invoices(id,school_id,student_id,enrollment_id,description,amount_cents,due_date,status,provider,provider_reference,boleto_url,boleto_barcode,issued_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, invoice.ID, schoolID, invoice.StudentID, invoice.EnrollmentID, invoice.Description, invoice.AmountCents, invoice.DueDate, invoice.Status, invoice.Provider, invoice.ProviderReference, invoice.BoletoURL, invoice.BoletoBarcode, invoice.IssuedAt)
	if err != nil {
		return Invoice{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_events(school_id,user_id,action,resource_type,resource_id,metadata) VALUES($1,$2,'invoice.created','invoice',$3,jsonb_build_object('amountCents',$4,'provider',$5))`, schoolID, actorID, invoice.ID, invoice.AmountCents, invoice.Provider)
	if err != nil {
		return Invoice{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Invoice{}, err
	}
	return invoice, nil
}

func (s *Service) ProcessAsaasWebhook(ctx context.Context, event AsaasWebhookEvent) error {
	if s.provider.Name() != "asaas" {
		return errors.New("Asaas webhook received while provider is disabled")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	result, err := tx.Exec(ctx, `INSERT INTO asaas_webhook_events(event_id,event_type,provider_reference) VALUES($1,$2,$3) ON CONFLICT(event_id) DO NOTHING`, event.ID, event.Event, event.Payment.ID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return tx.Commit(ctx)
	}
	var invoiceID, schoolID, currentStatus string
	var amountCents int64
	err = tx.QueryRow(ctx, `SELECT id,school_id,status,amount_cents FROM invoices WHERE provider='asaas' AND provider_reference=$1 FOR UPDATE`, event.Payment.ID).Scan(&invoiceID, &schoolID, &currentStatus, &amountCents)
	if err != nil {
		return errInvoiceNotFound
	}
	nextStatus, changed := paymentStatus(currentStatus, event.Event, event.Payment.Status)
	if changed {
		if err := applyInvoiceStatus(ctx, tx, invoiceID, event.Payment.ID, currentStatus, nextStatus, amountCents); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_events(school_id,action,resource_type,resource_id,metadata) VALUES($1,'invoice.webhook_updated','invoice',$2,jsonb_build_object('eventId',$3,'eventType',$4,'previousStatus',$5,'newStatus',$6))`, schoolID, invoiceID, event.ID, event.Event, currentStatus, nextStatus)
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE asaas_webhook_events SET processed_at=now() WHERE event_id=$1`, event.ID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) Reconcile(ctx context.Context, limit int) (ReconciliationResult, error) {
	var outcome ReconciliationResult
	if s.provider.Name() != "asaas" {
		return outcome, nil
	}
	if limit < 1 || limit > 500 {
		limit = 100
	}
	var runID string
	if err := s.db.QueryRow(ctx, `INSERT INTO billing_reconciliation_runs(provider) VALUES('asaas') RETURNING id`).Scan(&runID); err != nil {
		return outcome, err
	}
	rows, err := s.db.Query(ctx, `SELECT id,school_id,provider_reference,status,amount_cents FROM invoices WHERE provider='asaas' AND provider_reference IS NOT NULL AND status IN ('issued','overdue','paid') ORDER BY provider_synced_at NULLS FIRST,created_at LIMIT $1`, limit)
	if err != nil {
		return outcome, err
	}
	type candidate struct {
		id, schoolID, reference, status string
		amount                          int64
	}
	items := make([]candidate, 0, limit)
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.id, &item.schoolID, &item.reference, &item.status, &item.amount); err != nil {
			rows.Close()
			return outcome, err
		}
		items = append(items, item)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return outcome, err
	}
	for _, item := range items {
		outcome.Inspected++
		remote, providerErr := s.provider.GetInvoice(ctx, item.reference)
		if providerErr != nil {
			outcome.Failures++
			_, _ = s.db.Exec(ctx, `UPDATE invoices SET provider_synced_at=now() WHERE id=$1`, item.id)
			continue
		}
		nextStatus, changed := paymentStatus(item.status, "PAYMENT_RECONCILED", remote.Status)
		tx, err := s.db.Begin(ctx)
		if err != nil {
			outcome.Failures++
			continue
		}
		if changed {
			err = applyInvoiceStatus(ctx, tx, item.id, item.reference, item.status, nextStatus, item.amount)
			if err == nil {
				_, err = tx.Exec(ctx, `INSERT INTO audit_events(school_id,action,resource_type,resource_id,metadata) VALUES($1,'invoice.reconciled','invoice',$2,jsonb_build_object('previousStatus',$3,'newStatus',$4))`, item.schoolID, item.id, item.status, nextStatus)
			}
			if err == nil {
				outcome.Updated++
			}
		}
		if err == nil {
			_, err = tx.Exec(ctx, `UPDATE invoices SET provider_synced_at=now() WHERE id=$1`, item.id)
		}
		if err == nil {
			err = tx.Commit(ctx)
		} else {
			_ = tx.Rollback(ctx)
		}
		if err != nil {
			outcome.Failures++
		}
	}
	_, finishErr := s.db.Exec(ctx, `UPDATE billing_reconciliation_runs SET completed_at=now(),inspected_count=$2,updated_count=$3,failure_count=$4 WHERE id=$1`, runID, outcome.Inspected, outcome.Updated, outcome.Failures)
	if finishErr != nil {
		return outcome, finishErr
	}
	if outcome.Failures > 0 {
		return outcome, fmt.Errorf("Asaas reconciliation completed with %d failures", outcome.Failures)
	}
	return outcome, nil
}

type invoiceTx interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func applyInvoiceStatus(ctx context.Context, tx invoiceTx, invoiceID, providerReference, currentStatus, nextStatus string, amountCents int64) error {
	_, err := tx.Exec(ctx, `UPDATE invoices SET status=$2,paid_at=CASE WHEN $2='paid' THEN COALESCE(paid_at,now()) ELSE paid_at END,cancelled_at=CASE WHEN $2='cancelled' THEN COALESCE(cancelled_at,now()) ELSE cancelled_at END,provider_synced_at=now() WHERE id=$1`, invoiceID, nextStatus)
	if err != nil {
		return err
	}
	if nextStatus == "paid" && currentStatus != "paid" {
		_, err = tx.Exec(ctx, `INSERT INTO payments(invoice_id,amount_cents,provider_reference,paid_at) VALUES($1,$2,$3,now()) ON CONFLICT(invoice_id,provider_reference) WHERE provider_reference IS NOT NULL DO NOTHING`, invoiceID, amountCents, providerReference)
	}
	return err
}
