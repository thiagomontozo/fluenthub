ALTER TABLE billing_accounts
    ADD COLUMN provider_customer_reference text;

CREATE INDEX billing_accounts_provider_customer_idx
    ON billing_accounts(provider_customer_reference)
    WHERE provider_customer_reference IS NOT NULL;
