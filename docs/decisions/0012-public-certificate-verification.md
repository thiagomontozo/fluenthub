# Provide Public Certificate Verification

**Status:** Accepted

## Context
Recipients need a low-friction authenticity check without exposing student records.

## Decision
Use random verification codes and a narrow revocation-aware public projection.

## Consequences
Verification is useful and privacy-conscious. Codes must be unguessable, rate-limited and retained after revocation.
