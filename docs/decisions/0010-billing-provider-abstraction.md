# Abstract Billing Providers

**Status:** Accepted

## Context
Schools may choose different PSPs, while 0.1 must not issue real bank charges.

## Decision
Define `BillingProvider` and ship an explicitly non-payable mock adapter.

## Consequences
Core billing stays vendor-neutral. Real adapters must add webhooks, reconciliation, idempotency and secret management.
