# Adopt a Modular Monolith

**Status:** Accepted

## Context
The product has many related domains but no proven need for distributed deployment.

## Decision
Deploy one API while isolating modules by business vocabulary and narrow interfaces.

## Consequences
Transactions and local development stay simple. Future extraction remains possible, but boundaries require code-review discipline.
