# 0004. SLA threshold secret handling: encrypted-in-repo, shared symmetric key

- **Status:** Accepted (mechanism deferred)
- **Date:** 2026-08-23

> Historical decision. ADR-0017 establishes that SLA thresholds are public and
> may be committed in plaintext. The threshold confidentiality and encryption
> requirements below are superseded and no longer apply. Backend credential
> handling remains in ADR-0005.

Superseded by [ADR-0017](0017-thresholds-are-publicly-accessible.md).

## Context

At the time, the harness needed real SLA thresholds at runtime while keeping
them out of plaintext, code, and readable Git history. Developers also needed
a practical way to obtain matching thresholds.

## Decision

The original decision was to commit real thresholds **only in a dedicated
encrypted folder**, using a shared symmetric key held only by Red Hat developers.
The proposal called for fake `thresholds.example.json` and schemas in plaintext
for other contributors.

At the time, encryption was not enabled. The decision called for selecting a
mechanism, key custody, and rotation process before committing real thresholds;
until then, real files were to stay outside tracked content and local
`thresholds.json` was to remain gitignored.

Under the proposal, decryption would produce a local file explicitly supplied
through `--thresholds` / `HARNESS_THRESHOLDS`, with no implicit default. Its
provenance would be stamped into every report. The key would not be committed
or shipped in the container.

## Decision notes

### Options considered

- **Encrypted copy (chosen):** keeps data with code and reduces KB retrieval;
  requires key management.
- **Gitignore-only:** keeps secrets out of history but requires separate fetches.
- **Authenticated runtime service:** revocable access, but adds infrastructure
  and credentials.

### Consequences

The original proposal noted that ciphertext would remain in history and a later
key leak could expose old versions. Tool selection, custody, and the relationship
between the canonical KB and encrypted copies/exporter were left open. Partner
threshold delivery was outside that decision.

Related: [ECOPROJECT-5273](https://redhat.atlassian.net/browse/ECOPROJECT-5273),
[ADR-0002](0002-repository-host-and-sdlc-workflow.md).
