# API

All private routes are versioned below `/api/v1`. JSON errors contain `code`, safe `message` and `requestId`; stack traces are never returned.

## Identity and configuration

- `POST /api/v1/auth/login`, `/logout`, `/change-password`; `GET /auth/me`
- `/school`, `/branding`, `/units`, `/users`, `/roles`, `/permissions`
- Setup progresses through welcome, school, branding, main administrator, unit, defaults, policy and finish.

## Academic and learning

- `/courses`, `/levels`, `/modules`, `/classes`, `/enrollments`
- `/lessons`, `/live-sessions`, `/attendance`
- `/exercises`, `/exercises/:id/attempts`, answer save, submit and grading
- `/exams`, `/exams/:id/attempts`, incremental answers, submit and grading
- `/grading`, `/academic`, `/certificates`, `/audit`

## Operations

- `/billing`, `/notifications`, `/support`
- `GET /events` opens an authenticated SSE stream.
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
