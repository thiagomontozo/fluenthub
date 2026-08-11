package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type AsaasProvider struct {
	baseURL, apiKey string
	client          *http.Client
}

func (*AsaasProvider) Name() string { return "asaas" }

func NewAsaasProvider(baseURL, apiKey string, client *http.Client) (*AsaasProvider, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" || apiKey == "" {
		return nil, errors.New("invalid Asaas configuration")
	}
	if parsed.Scheme != "https" && !(parsed.Scheme == "http" && (parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1")) {
		return nil, errors.New("Asaas API must use HTTPS outside localhost")
	}
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &AsaasProvider{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, client: client}, nil
}

func (p *AsaasProvider) EnsureCustomer(ctx context.Context, customer Customer) (string, error) {
	var result struct {
		ID string `json:"id"`
	}
	err := p.request(ctx, http.MethodPost, "/customers", map[string]string{"name": customer.Name, "email": customer.Email, "externalReference": customer.ExternalID}, &result)
	if err != nil {
		return "", err
	}
	if result.ID == "" {
		return "", errors.New("Asaas returned no customer ID")
	}
	return result.ID, nil
}

func (p *AsaasProvider) CreateInvoice(ctx context.Context, invoice Invoice) (Invoice, error) {
	if invoice.CustomerReference == "" || invoice.AmountCents <= 0 {
		return Invoice{}, errors.New("Asaas customer and positive amount are required")
	}
	var result asaasPayment
	input := struct {
		Customer          string     `json:"customer"`
		BillingType       string     `json:"billingType"`
		Description       string     `json:"description"`
		DueDate           string     `json:"dueDate"`
		ExternalReference string     `json:"externalReference"`
		Value             centsValue `json:"value"`
	}{invoice.CustomerReference, "BOLETO", invoice.Description, invoice.DueDate.Format("2006-01-02"), invoice.ID, centsValue(invoice.AmountCents)}
	if err := p.request(ctx, http.MethodPost, "/payments", input, &result); err != nil {
		return Invoice{}, err
	}
	if result.ID == "" {
		return Invoice{}, errors.New("Asaas returned no payment ID")
	}
	invoice.Provider, invoice.ProviderReference = "asaas", result.ID
	invoice.Status = mapAsaasStatus(result.Status)
	invoice.BoletoURL = firstString(result.BankSlipURL, result.InvoiceURL)
	now := time.Now().UTC()
	invoice.IssuedAt = &now
	var barcode struct {
		IdentificationField string `json:"identificationField"`
	}
	if err := p.request(ctx, http.MethodGet, "/payments/"+url.PathEscape(result.ID)+"/identificationField", nil, &barcode); err == nil && barcode.IdentificationField != "" {
		invoice.BoletoBarcode = &barcode.IdentificationField
	}
	return invoice, nil
}

func (p *AsaasProvider) GetInvoice(ctx context.Context, reference string) (Invoice, error) {
	var result asaasPayment
	if err := p.request(ctx, http.MethodGet, "/payments/"+url.PathEscape(reference), nil, &result); err != nil {
		return Invoice{}, err
	}
	return Invoice{Provider: "asaas", ProviderReference: result.ID, Status: mapAsaasStatus(result.Status), BoletoURL: firstString(result.BankSlipURL, result.InvoiceURL)}, nil
}

func (p *AsaasProvider) CancelInvoice(ctx context.Context, reference string) (Invoice, error) {
	if err := p.request(ctx, http.MethodDelete, "/payments/"+url.PathEscape(reference), nil, nil); err != nil {
		return Invoice{}, err
	}
	now := time.Now().UTC()
	return Invoice{Provider: "asaas", ProviderReference: reference, Status: "cancelled", CancelledAt: &now}, nil
}

type asaasPayment struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	InvoiceURL  string `json:"invoiceUrl"`
	BankSlipURL string `json:"bankSlipUrl"`
}

type centsValue int64

func (value centsValue) MarshalJSON() ([]byte, error) {
	if value < 0 {
		return nil, errors.New("negative monetary value")
	}
	return []byte(fmt.Sprintf("%d.%02d", int64(value)/100, int64(value)%100)), nil
}

func (p *AsaasProvider) request(ctx context.Context, method, path string, input, output any) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, body)
	if err != nil {
		return err
	}
	request.Header.Set("access_token", p.apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "FluentHub/0.1")
	response, err := p.client.Do(request)
	if err != nil {
		return fmt.Errorf("Asaas request failed: %w", err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Asaas returned HTTP %d", response.StatusCode)
	}
	if output != nil && len(payload) > 0 {
		if err := json.Unmarshal(payload, output); err != nil {
			return fmt.Errorf("decode Asaas response: %w", err)
		}
	}
	return nil
}

func mapAsaasStatus(status string) string {
	switch status {
	case "RECEIVED", "CONFIRMED", "RECEIVED_IN_CASH":
		return "paid"
	case "OVERDUE":
		return "overdue"
	case "REFUNDED", "REFUND_REQUESTED", "CHARGEBACK_REQUESTED", "CHARGEBACK_DISPUTE", "AWAITING_CHARGEBACK_REVERSAL":
		return "cancelled"
	default:
		return "issued"
	}
}

func firstString(values ...string) *string {
	for _, value := range values {
		if value != "" {
			copy := value
			return &copy
		}
	}
	return nil
}
