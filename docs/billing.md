# Billing

`Invoice` represents an operational charge and stores currency in `amount_cents` (`int64`). `Payment` records a settlement without overwriting issuance history. Invoice states are draft, issued, paid, overdue and cancelled.

`BillingProvider` creates, fetches and cancels provider-side invoices. The mock adapter returns identifiers, URLs and barcode strings explicitly marked as demonstration and not payable. It does not contact a bank, PSP, PIX network or boleto clearing system.

A real Asaas, Efí or other PSP adapter must add authenticated webhooks, signature verification, idempotency, reconciliation, secret rotation and careful redaction. URLs/barcodes should be shown only to authorized school staff and the owning student. Real financial integration is not implemented in 0.1.0.
