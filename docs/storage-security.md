# Storage Security and Backup

## Upload pipeline

`SecureObjectStorage` writes incoming content to a restrictive quarantine file, enforces `MAX_UPLOAD_MB`, and sends the stream to ClamAV using `INSTREAM`. A positive or indeterminate scan rejects the upload; unscanned content is never committed when scanning is enabled. Clamd must run on a private network because its TCP protocol has no built-in authentication or encryption.

After a clean result, FluentHub encrypts the content in 1 MiB AES-256-GCM chunks. Every chunk has a fresh nonce and authenticated length metadata. Reads authenticate every chunk before returning plaintext. The envelope has a versioned magic header so future migrations are possible. `STORAGE_ALLOW_LEGACY_PLAINTEXT` is disabled by default and should only be temporary during a controlled migration.

`STORAGE_ENCRYPTION_KEY` is a base64 encoding of exactly 32 random bytes. It must come from a secret manager, never Git. Losing it makes objects and backups unreadable; exposing it compromises both. Rotation requires a planned decrypt/re-encrypt migration and verified backups.

## Backups

The scheduler creates a snapshot immediately at startup and then every `BACKUP_INTERVAL_HOURS`. Each atomic `tar.gz` contains the already encrypted object files and has a SHA-256 sidecar. `BACKUP_RETENTION_HOURS` removes expired local snapshots. Symbolic links are refused. `VerifyBackup` checks the sidecar and parses the full archive.

These snapshots do not include PostgreSQL and are not an off-site strategy. A real deployment needs database-native backups, encrypted off-site replication, access controls, monitoring, restore drills, documented RPO/RTO and independent key custody. Never place `BACKUP_PATH` inside `STORAGE_PATH`.

## Validation

`go run ./cmd/validate-integrations` starts local contract servers, exercises ClamAV framing, verifies encrypted round-trip without plaintext leakage, creates a snapshot, checks its digest and parses its archive. This validates implementation behavior without transmitting real data or requiring provider credentials.
