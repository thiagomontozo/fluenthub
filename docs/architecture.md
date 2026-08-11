# Architecture

## Modular monolith

FluentHub deploys as one Go API and one React application. Domain packages own their vocabulary and rules while PostgreSQL transactions coordinate cross-domain workflows. This avoids network-bound distributed transactions and preserves a path to extraction if scale evidence justifies it.

The backend composition root is `cmd/api/main.go`. It loads configuration, opens pgx, creates providers, injects stores/services into the HTTP router, starts the bounded scheduler and coordinates shutdown. Interfaces are defined by consumers and remain small.

## Backend modules

Identity covers schools, branding, users, sessions, roles and permissions. Academic structure covers units, courses, levels, modules, classes and enrollments. Learning covers lessons, live sessions, recordings and attendance. Assessment deliberately has separate exercise and exam packages. Operations covers billing, notification, support and audit. Certification depends on a closed approved academic result.

Handlers decode bounded input and produce standardized errors. Durable SQL belongs to domain stores/services rather than presentation code. Every query operating on tenant data must include school scope; teacher and student paths add class/enrollment scope.

## Frontend areas

React Router separates administrator, teacher, student and operator layouts. Guards improve navigation but never replace backend authorization. A branding provider loads public, privacy-safe identity values and applies CSS variables. Shared cards, tables, badges, progress, empty states and error patterns preserve consistency; the student experience is mobile-first.

## Infrastructure

PostgreSQL is authoritative for identity, academic state, assessment, billing metadata, support and audit. `ObjectStorage` keeps binary content outside the database. The initial local adapter uses random keys and a confined root; S3/MinIO/cloud adapters can preserve the same contract.

`LiveClassProvider` and `BillingProvider` isolate vendors. The mock implementations make demonstration state explicit and contain no real credentials. Live media remains at the selected provider; FluentHub stores control state and authorized references.

The scheduler uses one managed ticker, not one goroutine per row. Jobs query due work in batches and must be idempotent. SSE provides notification, ticket and simple lesson events from an in-memory hub.

```mermaid
flowchart TB
  Web[React areas] --> HTTP[HTTP middleware and router]
  HTTP --> Identity
  HTTP --> Academic
  HTTP --> Learning
  HTTP --> Assessment
  HTTP --> Operations
  Identity --> PG[(PostgreSQL)]
  Academic --> PG
  Learning --> PG
  Assessment --> PG
  Operations --> PG
  Learning --> Objects[(Object Storage)]
  Learning --> Live[Live provider]
  Operations --> Billing[Billing provider]
  Scheduler --> PG
  PG --> SSE[SSE hub]
```

## Authorization

Authentication resolves an active server-side session and user. RBAC checks explicit permission codes. Resource checks then enforce school ownership, assigned teacher/class membership, active student enrollment and operator assignment. Public certificate verification uses a purpose-built projection.

## Failure modes

- PostgreSQL unavailable: readiness fails; liveness stays healthy while the process can serve.
- Storage unavailable: readiness fails and uploads/downloads return bounded errors.
- Provider unavailable: affected live/billing operation fails without corrupting local state; liveness does not depend on the live provider.
- Browser disconnect: request context cancels; SSE subscriber is removed; attendance is recovered from last heartbeat.
- Scheduler retry: idempotency prevents duplicate notification or state transitions.

## Shutdown and scale

SIGINT/SIGTERM cancels the root context, stops scheduling, closes SSE clients, drains HTTP with a timeout, closes storage and then pgx. At multiple instances, PostgreSQL remains shared but SSE needs a shared event transport and scheduler needs leader/lease coordination. Object storage should move to a shared adapter. These are evolutionary changes, not reasons to begin with microservices.
