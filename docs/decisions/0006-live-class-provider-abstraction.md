# Abstract Live Class Providers

**Status:** Accepted

## Context
FluentHub must coordinate live teaching without becoming a video infrastructure product.

## Decision
Define `LiveClassProvider`; ship a mock and add LiveKit/Jitsi adapters later.

## Consequences
Vendor choice and failure are isolated. Provider callbacks, media and credentials require adapter-specific hardening.
