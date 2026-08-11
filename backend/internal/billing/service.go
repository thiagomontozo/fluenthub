package billing

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

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
