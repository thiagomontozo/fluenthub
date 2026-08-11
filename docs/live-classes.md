# Live Classes

`LiveClassProvider` creates, starts and ends sessions; returns short-lived role-specific join information; and starts/stops recording. The API verifies the teacher assignment or student enrollment before calling it. Provider credentials never reach logs or persistent browser storage.

The included mock adapter returns explicit demonstration join data and is not video conferencing. A LiveKit or Jitsi adapter should map provider IDs, sign short-lived claims, validate callbacks, make operations idempotent and download/ingest recording output through `ObjectStorage`.

Failure leaves the lesson and session in a recoverable state. Provider availability is not part of liveness. Callback authentication, participant privacy, regional hosting and recording consent remain deployment responsibilities.
