# Use Heartbeat-Based Attendance

**Status:** Accepted

## Context
Page-open events and browser leave events are not reliable attendance evidence.

## Decision
Aggregate recoverable presence segments from JOIN, 30-second heartbeats and LEAVE/stale recovery.

## Consequences
Attendance is more defensible and replay remains separate. The API and scheduler must handle overlap and stale sessions.
