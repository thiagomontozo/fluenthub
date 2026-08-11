# API

All private routes are versioned below `/api/v1`. JSON errors contain `code`, safe `message` and `requestId`; stack traces are never returned.

## Identity and configuration

- `POST /api/v1/auth/login`, `/logout`, `/change-password`; `GET /auth/me`
- `GET /setup/status`; `POST /setup/complete` performs the one-time atomic setup.
- Authenticated resource APIs cover users, units, courses, classes, enrollments, lessons, exercises, exams, billing, notifications and support, with permission and resource-scope checks.
- Setup progresses through welcome, school, branding, main administrator, unit, defaults, policy and finish.

## Academic and learning

- `/courses`, `/levels`, `/modules`, `/classes`, `/enrollments`
- `/lessons`, `/live-sessions`, `/attendance`
- `POST /lessons/:lessonId/live-session`; `POST /live-sessions/:id/start|end`; `GET /lessons/:lessonId/live-join`; `POST /live-sessions/:id/recording/start|stop`
- `/exercises`, `/exercises/:id/attempts`, answer save, submit and grading
- `/exams`, `/exams/:id/attempts`, incremental answers, submit and grading
- `/grading`, `/academic`, `/certificates`, `/audit`

## Operations

- `/billing`, `/notifications`, `/support`
- `POST /api/v1/webhooks/asaas` is public but authenticated with `asaas-access-token`; duplicate event IDs are acknowledged without duplicate payment processing.
- Billing creation uses the configured provider; notifications support send/read/read-all.
- Support provides scoped ticket creation, assignment, messages, internal notes and state transitions.
- `GET /events` opens an authenticated SSE stream distributed through PostgreSQL `LISTEN/NOTIFY`.
- `GET /api/v1/public/certificates/verify/:code` and `GET /api/v1/public/branding` are public.

Example login:

```http
POST /api/v1/auth/login
Content-Type: application/json

{"email":"admin@example.invalid","password":"example-value-not-a-real-password"}
```

```json
{"id":"00000000-0000-0000-0000-000000000000","name":"Administrator Example","roles":["administrator"],"permissions":["school.manage"]}
```

Example validation failure:

```json
{"error":{"code":"VALIDATION_ERROR","message":"Invalid request body","requestId":"173e..."}}
```

Pagination should use bounded `limit` and stable `(created_at,id)` cursors. Sort columns must be allowlisted. UUID, email, dates, money, scores, policy weights, attempt windows, ticket fields and uploads are validated by the backend.
