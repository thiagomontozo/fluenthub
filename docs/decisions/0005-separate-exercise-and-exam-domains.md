# Separate Exercise and Exam Domains

**Status:** Accepted

## Context
Practice and formal assessment have different timing, lifecycle, security and publication rules.

## Decision
Maintain separate aggregates, tables, attempts, answers and grades while permitting small internal helpers.

## Consequences
Some concepts are duplicated, but formal exam guarantees cannot be weakened by exercise evolution.
