# Attendance

1. **JOIN:** an authorized enrolled student creates/resumes an attendance session and opens a presence segment.
2. **Heartbeat:** every 30 seconds the frontend refreshes `lastSeenAt`; the backend clips overlaps and rejects impossible future timestamps.
3. **LEAVE:** closes the active segment when available.
4. **Recovery:** a background job closes stale segments at their last heartbeat plus bounded grace.

`totalPresentSeconds` is the sum of valid segments and cannot exceed lesson duration. Basis points are `present / duration × 10,000`. At or above the configurable policy is `present`; from half the threshold is `partial`; otherwise `absent`. No 70% value is hardcoded.

`RecordingView` separately tracks replay consumption. Replay supports learning analytics but never changes live attendance, because asynchronous viewing is not evidence of participation in a scheduled live session.
