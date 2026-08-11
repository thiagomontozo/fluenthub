# Use Go for the Backend

**Status:** Accepted

## Context
The API needs explicit concurrency, HTTP streaming, predictable operations and a small dependency surface.

## Decision
Use Go with `net/http`, pgx, slog, contexts and explicit dependency injection.

## Consequences
The binary is operationally simple and SSE/scheduler cancellation is direct. Teams must keep domain boundaries deliberate without a framework enforcing them.
