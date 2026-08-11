# Billing

`Invoice` represents an operational charge and stores currency in `amount_cents` (`int64`). `Payment` records a settlement without overwriting issuance history. Invoice states are draft, issued, paid, overdue and cancelled.

`BillingProvider` provisions provider customers and creates, fetches and cancels provider-side invoices. The Asaas adapter authenticates with `access_token`, supports sandbox and production base URLs, creates BOLETO charges, retrieves the identification field and maps provider states into FluentHub states. Values are serialized directly from integer cents to exact two-decimal JSON numbers. Provider customer IDs are persisted in `billing_accounts`.

The mock adapter remains explicit and non-payable. Asaas API error bodies and credentials are not logged. URLs and barcodes should be shown only to authorized school staff and the owning student. Authenticated webhooks, automatic payment reconciliation, idempotent retry keys, refunds and PIX remain future work; enabling `BILLING_PROVIDER=asaas` therefore requires an operational reconciliation process.
