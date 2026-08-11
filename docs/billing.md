# Billing

`Invoice` represents an operational charge and stores currency in `amount_cents` (`int64`). `Payment` records a settlement without overwriting issuance history. Invoice states are draft, issued, paid, overdue and cancelled.

`BillingProvider` provisions provider customers and creates, fetches and cancels provider-side invoices. The Asaas adapter authenticates with `access_token`, supports sandbox and production base URLs, creates BOLETO charges, retrieves the identification field and maps provider states into FluentHub states. Values are serialized directly from integer cents to exact two-decimal JSON numbers. Provider customer IDs are persisted in `billing_accounts`.

The public `POST /api/v1/webhooks/asaas` endpoint requires the configured 32–255 character token in `asaas-access-token`. Payloads are bounded and validated. Asaas uses at-least-once delivery, so `asaas_webhook_events.event_id` is the idempotency key. Event insertion, invoice transition, payment creation and audit entry share one transaction; an unknown invoice rolls back and returns a retryable error. Paid and cancelled states cannot be regressed by delayed events.

Automatic reconciliation runs at startup and every `BILLING_RECONCILE_MINUTES`. It queries a bounded, rotating set of Asaas invoices, compares authoritative provider state, applies the same monotonic transition rules and records counts in `billing_reconciliation_runs`. Provider failures are counted and retried on later runs. The mock adapter remains explicit and non-payable. Asaas API error bodies and credentials are not logged. Refund detail workflows, PIX and generalized outbound idempotency keys remain future work.
