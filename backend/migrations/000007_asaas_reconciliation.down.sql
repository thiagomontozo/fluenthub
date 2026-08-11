DROP TABLE IF EXISTS billing_reconciliation_runs;
DROP INDEX IF EXISTS payments_invoice_provider_reference_idx;
DROP INDEX IF EXISTS invoices_provider_reconcile_idx;
ALTER TABLE invoices DROP COLUMN IF EXISTS provider_synced_at;
DROP TABLE IF EXISTS asaas_webhook_events;
