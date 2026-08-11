package billing

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type AsaasWebhookEvent struct {
	ID          string `json:"id"`
	Event       string `json:"event"`
	DateCreated string `json:"dateCreated"`
	Payment     struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	} `json:"payment"`
}

type AsaasWebhookProcessor interface {
	ProcessAsaasWebhook(context.Context, AsaasWebhookEvent) error
}

func AsaasWebhookHandler(authToken string, processor AsaasWebhookProcessor) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		provided := request.Header.Get("asaas-access-token")
		providedDigest, expectedDigest := sha256.Sum256([]byte(provided)), sha256.Sum256([]byte(authToken))
		if subtle.ConstantTimeCompare(providedDigest[:], expectedDigest[:]) != 1 {
			http.Error(w, `{"error":{"code":"UNAUTHENTICATED","message":"Invalid webhook authentication"}}`, http.StatusUnauthorized)
			return
		}
		request.Body = http.MaxBytesReader(w, request.Body, 256<<10)
		var event AsaasWebhookEvent
		if err := json.NewDecoder(request.Body).Decode(&event); err != nil || !validAsaasEvent(event) {
			http.Error(w, `{"error":{"code":"VALIDATION_ERROR","message":"Invalid webhook payload"}}`, http.StatusBadRequest)
			return
		}
		if err := processor.ProcessAsaasWebhook(request.Context(), event); err != nil {
			http.Error(w, `{"error":{"code":"WEBHOOK_PROCESSING_FAILED","message":"Webhook could not be processed"}}`, http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"received":true}`))
	})
}

func validAsaasEvent(event AsaasWebhookEvent) bool {
	return len(event.ID) >= 5 && len(event.ID) <= 200 && strings.HasPrefix(event.Event, "PAYMENT_") && len(event.Event) <= 80 && len(event.Payment.ID) >= 5 && len(event.Payment.ID) <= 200
}

func paymentStatus(current, eventType, providerStatus string) (string, bool) {
	desired := providerStatus
	if desired != "issued" && desired != "overdue" && desired != "paid" && desired != "cancelled" {
		desired = mapAsaasStatus(providerStatus)
	}
	switch eventType {
	case "PAYMENT_RECEIVED", "PAYMENT_CONFIRMED", "PAYMENT_RECEIVED_IN_CASH":
		desired = "paid"
	case "PAYMENT_OVERDUE":
		desired = "overdue"
	case "PAYMENT_DELETED", "PAYMENT_REFUNDED", "PAYMENT_REFUND_IN_PROGRESS", "PAYMENT_CHARGEBACK_REQUESTED", "PAYMENT_CHARGEBACK_DISPUTE":
		desired = "cancelled"
	}
	if current == "cancelled" || current == desired {
		return current, false
	}
	if current == "paid" && desired != "cancelled" {
		return current, false
	}
	if desired == "issued" && current != "draft" {
		return current, false
	}
	if desired == "overdue" && current != "issued" && current != "draft" {
		return current, false
	}
	if desired != "issued" && desired != "overdue" && desired != "paid" && desired != "cancelled" {
		return current, false
	}
	return desired, true
}

var errInvoiceNotFound = errors.New("provider invoice not found")
