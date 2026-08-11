DROP INDEX IF EXISTS billing_accounts_provider_customer_idx;
ALTER TABLE billing_accounts DROP COLUMN IF EXISTS provider_customer_reference;
