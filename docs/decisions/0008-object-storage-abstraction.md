# Abstract Object Storage

**Status:** Accepted

## Context
Recordings, audio, materials, attachments and certificates should not be database blobs or fixed to one cloud.

## Decision
Use a small `ObjectStorage` interface and local-filesystem first adapter with generated keys.

## Consequences
S3/MinIO/cloud migration is contained. Authorization and metadata remain application responsibilities.
