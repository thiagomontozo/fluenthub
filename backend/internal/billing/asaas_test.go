package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAsaasProviderUsesExactMoneyAndAuthentication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("access_token") != "test-api-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch request.URL.Path {
		case "/payments":
			var payload struct {
				Value       json.Number `json:"value"`
				BillingType string      `json:"billingType"`
			}
			decoder := json.NewDecoder(request.Body)
			decoder.UseNumber()
			if err := decoder.Decode(&payload); err != nil {
				t.Errorf("decode request: %v", err)
				http.Error(w, "bad", http.StatusBadRequest)
				return
			}
			if payload.Value.String() != "123.45" || payload.BillingType != "BOLETO" {
				t.Errorf("unexpected payment payload: %#v", payload)
			}
			_, _ = w.Write([]byte(`{"id":"pay_test","status":"PENDING","bankSlipUrl":"https://sandbox.invalid/pay_test"}`))
		case "/payments/pay_test/identificationField":
			_, _ = w.Write([]byte(`{"identificationField":"TEST-BARCODE"}`))
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()
	provider, err := NewAsaasProvider(server.URL, "test-api-key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	invoice, err := provider.CreateInvoice(context.Background(), Invoice{ID: "invoice-test", CustomerReference: "cus_test", Description: "Test tuition", AmountCents: 12345, DueDate: time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	if invoice.ProviderReference != "pay_test" || invoice.BoletoBarcode == nil || *invoice.BoletoBarcode != "TEST-BARCODE" {
		t.Fatalf("unexpected provider invoice: %#v", invoice)
	}
}
