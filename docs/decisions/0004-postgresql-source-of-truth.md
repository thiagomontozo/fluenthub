# Use PostgreSQL as Source of Truth

**Status:** Accepted

## Context
Academic, financial and certification records need constraints, transactions and durable relationships.

## Decision
Persist durable metadata in PostgreSQL through versioned SQL migrations.

## Consequences
Strong integrity and familiar operations outweigh adding specialized stores. Binary content remains in object storage.
