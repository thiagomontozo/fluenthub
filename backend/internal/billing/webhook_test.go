package billing

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type webhookProcessorStub struct {
	events []AsaasWebhookEvent
	err    error
}

func (stub *webhookProcessorStub) ProcessAsaasWebhook(_ context.Context, event AsaasWebhookEvent) error {
	stub.events = append(stub.events, event)
	return stub.err
}

func TestAsaasWebhookHandlerAuthenticatesAndProcesses(t *testing.T) {
	token := "test-webhook-token-with-at-least-32-characters"
	processor := &webhookProcessorStub{}
	handler := AsaasWebhookHandler(token, processor)
	body := `{"id":"evt_12345","event":"PAYMENT_RECEIVED","payment":{"id":"pay_12345","status":"RECEIVED"}}`

	unauthorized := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/asaas", strings.NewReader(body))
	unauthorized.Header.Set("asaas-access-token", "wrong")
	unauthorizedResponse := httptest.NewRecorder()
	handler.ServeHTTP(unauthorizedResponse, unauthorized)
	if unauthorizedResponse.Code != http.StatusUnauthorized || len(processor.events) != 0 {
		t.Fatalf("unauthorized request was not rejected: status=%d events=%d", unauthorizedResponse.Code, len(processor.events))
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/asaas", strings.NewReader(body))
	request.Header.Set("asaas-access-token", token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("valid webhook returned %d: %s", response.Code, response.Body.String())
	}
	if len(processor.events) != 1 || processor.events[0].Payment.ID != "pay_12345" {
		t.Fatalf("webhook was not decoded correctly: %#v", processor.events)
	}
}

func TestAsaasWebhookHandlerRejectsInvalidPayloadAndRetriesFailures(t *testing.T) {
	token := "test-webhook-token-with-at-least-32-characters"
	processor := &webhookProcessorStub{}
	handler := AsaasWebhookHandler(token, processor)

	invalid := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"event":"PAYMENT_RECEIVED"}`))
	invalid.Header.Set("asaas-access-token", token)
	invalidResponse := httptest.NewRecorder()
	handler.ServeHTTP(invalidResponse, invalid)
	if invalidResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalid payload returned %d", invalidResponse.Code)
	}

	processor.err = errors.New("database unavailable")
	failing := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"id":"evt_12345","event":"PAYMENT_RECEIVED","payment":{"id":"pay_12345"}}`))
	failing.Header.Set("asaas-access-token", token)
	failingResponse := httptest.NewRecorder()
	handler.ServeHTTP(failingResponse, failing)
	if failingResponse.Code != http.StatusServiceUnavailable {
		t.Fatalf("processing failure returned %d", failingResponse.Code)
	}
}

func TestPaymentStatusTransitionsAreMonotonic(t *testing.T) {
	tests := []struct {
		name, current, event, provider, expected string
		changed                                  bool
	}{
		{"receive payment", "overdue", "PAYMENT_RECEIVED", "RECEIVED", "paid", true},
		{"duplicate payment", "paid", "PAYMENT_RECEIVED", "RECEIVED", "paid", false},
		{"late overdue event", "paid", "PAYMENT_OVERDUE", "OVERDUE", "paid", false},
		{"refund paid invoice", "paid", "PAYMENT_REFUNDED", "REFUNDED", "cancelled", true},
		{"cancelled is terminal", "cancelled", "PAYMENT_RECEIVED", "RECEIVED", "cancelled", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, changed := paymentStatus(test.current, test.event, test.provider)
			if status != test.expected || changed != test.changed {
				t.Fatalf("got (%s,%t), want (%s,%t)", status, changed, test.expected, test.changed)
			}
		})
	}
}
