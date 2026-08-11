CREATE TABLE asaas_webhook_events (
    event_id text PRIMARY KEY,
    event_type text NOT NULL,
    provider_reference text NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz
);

CREATE INDEX asaas_webhook_events_received_idx
    ON asaas_webhook_events(received_at DESC);

ALTER TABLE invoices ADD COLUMN provider_synced_at timestamptz;

CREATE INDEX invoices_provider_reconcile_idx
    ON invoices(provider, provider_synced_at NULLS FIRST)
    WHERE provider_reference IS NOT NULL AND status IN ('issued','overdue','paid');

CREATE UNIQUE INDEX payments_invoice_provider_reference_idx
    ON payments(invoice_id, provider_reference)
    WHERE provider_reference IS NOT NULL;

CREATE TABLE billing_reconciliation_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    provider text NOT NULL,
    started_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz,
    inspected_count integer NOT NULL DEFAULT 0,
    updated_count integer NOT NULL DEFAULT 0,
    failure_count integer NOT NULL DEFAULT 0
);
