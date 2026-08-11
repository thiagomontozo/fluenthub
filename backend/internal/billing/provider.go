package billing

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"time"
)

type Invoice struct {
	ID, SchoolID, StudentID, Description, Status, Provider, ProviderReference string
	EnrollmentID                                                              *string
	AmountCents                                                               int64
	DueDate                                                                   time.Time
	BoletoURL, BoletoBarcode                                                  *string
	IssuedAt, PaidAt, CancelledAt                                             *time.Time
}
type BillingProvider interface {
	CreateInvoice(context.Context, Invoice) (Invoice, error)
	GetInvoice(context.Context, string) (Invoice, error)
	CancelInvoice(context.Context, string) (Invoice, error)
}
type MockProvider struct{}

func (MockProvider) CreateInvoice(_ context.Context, invoice Invoice) (Invoice, error) {
	if invoice.AmountCents <= 0 {
		return Invoice{}, errors.New("amountCents must be positive")
	}
	now := time.Now().UTC()
	ref := "DEMO-" + uuid.NewString()
	url := "https://example.invalid/fluenthub/demo-invoice/" + ref
	barcode := fmt.Sprintf("DEMONSTRATION-NOT-PAYABLE-%d", invoice.AmountCents)
	invoice.Provider = "mock"
	invoice.ProviderReference = ref
	invoice.Status = "issued"
	invoice.IssuedAt = &now
	invoice.BoletoURL = &url
	invoice.BoletoBarcode = &barcode
	return invoice, nil
}
func (MockProvider) GetInvoice(context.Context, string) (Invoice, error) {
	return Invoice{}, errors.New("mock provider is stateless")
}
func (MockProvider) CancelInvoice(_ context.Context, _ string) (Invoice, error) {
	now := time.Now().UTC()
	return Invoice{Status: "cancelled", CancelledAt: &now}, nil
}
