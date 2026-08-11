# Preserve Grade Revision History

**Status:** Accepted

## Context
Overwriting a published grade removes academic accountability.

## Decision
Require a reason and append `GradeRevision` plus audit metadata for every post-publication score change.

## Consequences
Changes remain explainable. Services must update revision and grade atomically.
