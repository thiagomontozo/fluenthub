# Live Classes

`LiveClassProvider` creates, starts and ends sessions; returns short-lived role-specific join information; and starts/stops recording. The API verifies the teacher assignment or student enrollment before calling it. Provider credentials never reach logs or persistent browser storage.

The LiveKit adapter maps lessons to rooms through the authenticated Twirp API, converts `wss` endpoints for server calls, signs HS256 JWTs with 15-minute role grants and exposes only URL/token/expiry to the browser. Teachers can publish; students subscribe by default. The React workspace connects through `livekit-client`. Start/stop recording uses LiveKit Egress; its output destination is configured in the Egress deployment, never in browser data.

Routes cover room creation, start/end, role-scoped join, and recording start/stop. The service verifies school ownership, teacher assignment or active enrollment before calling LiveKit. The mock adapter remains for offline demonstrations. External validation requires a LiveKit project and Egress deployment; the deterministic validator exercises the same HTTP and token contracts locally.

Failure leaves the lesson and session in a recoverable state. Provider availability is not part of liveness. Webhook ingestion, retry idempotency, participant privacy, regional hosting, recording consent and Egress-to-ObjectStorage ingestion remain deployment responsibilities.
