# Security

## Authentication and sessions

Passwords are bcrypt hashes with length validation. Login failures return one generic message. Random 256-bit session tokens are sent in HttpOnly, SameSite=Lax cookies; only SHA-256 token digests are stored. Production cookies are Secure. Password changes and user disablement revoke sessions.

## Authorization and isolation

RBAC is checked on the API. Every tenant query includes `school_id`. Teachers are further limited to assigned classes and enrolled students; students to their active enrollments and own records; operators to permitted administrative fields and assigned/authorized queues. Menus are only a usability layer.

The final privileged administrator must be protected with a transaction/lock check. Grade revision and academic override need special permissions, reasons and audit. Exam endpoints never serialize correct flags during attempts, never accept browser-computed scores and enforce expiration server-side.

## Files, support and certificates

Storage keys are generated, paths confined and uploads size/MIME checked. New content is quarantined, scanned using ClamAV's `INSTREAM` protocol and encrypted in independently authenticated AES-256-GCM chunks. Production startup requires both a valid 32-byte base64 key and ClamAV. Download authorization is checked from the owning lesson, attempt, ticket or certificate—not possession of a key. Internal support messages are excluded from student/teacher projections. Public certificate results expose a minimal projection.

## Logging and audit

Structured logs use request IDs and avoid passwords, session tokens, authorization headers, full answers, audio, documents and sensitive billing instructions. Audit captures actor, school, action, resource, safe metadata, IP and user agent.

Asaas webhooks use a dedicated secret compared in constant time through the `asaas-access-token` header. Bodies are limited to 256 KiB, errors do not reveal database/provider details, event IDs enforce at-least-once idempotency and financial transitions execute transactionally.

## Limitations

This architecture provides privacy-oriented controls but does not claim automatic legal compliance. Storage backups are local encrypted-object snapshots with checksums; PostgreSQL backup, off-site copies, key management/rotation and restore drills remain deployment responsibilities. Deployment must also address TLS, trusted proxies, incident response, consent, regional requirements, dependency review and rate limiting. No penetration test has been performed.
