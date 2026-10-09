# Official v0.2.15 integration — 2026-10-09

- Custom base: `v0.2.14-custom.2`, `37a0488a4a157e2138878aaeda5ec505a49173eb`.
- Official merge parent: `official-v0.2.15`, `f2669c8cf62555cd92389b3f55920e9e6e7c6ff2`.
- Integration branch: `custom/0.2.15`; application version: `0.2.15-custom.1`.
- This change integrates source only. Tagging and deployment are separate operations.

## Merge decisions

Preserve the custom payment verification/recovery fixes, month-card billing,
multi-group API Keys, model permissions, and administrator leaderboard identities
and CSV export from custom.2. The payment/month-card and leaderboard implementation
files are unchanged from that base. Retain `golang.org/x/image v0.45.0` while adopting
the official Go 1.27.2/toolchain and dependency changes.

The new platform registry drives custom multi-group probes and composite model
catalogs/options. Cline and Command Code use their discovered or configured models;
they cannot inherit unrelated Claude defaults. OpenCode's explicit static catalog
and TypeSafe's Codex exclusion are preserved.

WebSocket pricing, key permissions, selected group, subscription/month-card
admission, and price time now use one snapshot per turn. A non-default selected
group is retained when the auth repository returns a different default group.
Changing a connection's platform or billing type requires a new session. Once a
turn succeeds, handler-level failover/same-account retries cannot restart the
proxy's turn counter: later failures close the connection, preventing stale-price
selection and repeated month-card billing identifiers. First-turn retries remain.

Frontend conflicts preserve the custom Escape guard, image-setting refresh,
model configuration assertions, and settings behavior while using the upstream
platform catalog. Failed batch-image settings loads remain retryable.

## Duplicate-model security backport

Ported the public fix from `lyy0709/sub2api-upstream` branch
`codex/duplicate-model-carriers`, PR #7951, pinned at
`1ccd9edea6bc9a3cc5821e596a1c20c217e2824a`. This includes the initial
#7909 protection and additional carrier coverage; these PRs were unmerged when
assessed. The integration adapts their intent to the custom routing stack.

Ambiguous model fields are rejected before group selection or rewriting, even
when model allowlists are disabled. Coverage includes repeated/case-variant/
escaped keys, null values, multipart fields, Live sessions, count-token endpoints,
and raw WebSocket frames (including session.update) before normalization. Multipart
filename classification matches the image parser. HTTP BOM/control-character
compatibility is inspected without copying or imposing a new body-size limit.
Protocol error shapes and configured body-size handling are retained. Legitimate
nested tool/input fields and multiple image files remain allowed.

Model replacement canonicalizes duplicate controls in a linear pass while
preserving other raw values, field order, and numeric precision.

## Database and validation

Official migration `242_drop_platform_check_constraints.sql` removes the platform
CHECK constraints from user platform quotas and composite routes so the registry
can accept the new providers. Application-layer platform validation remains.

Validation completed locally:

- Frontend typecheck, ESLint, full Vitest suite (415 files / 3,207 tests), and production build.
- golangci-lint v2.14.0: zero issues.
- Full Go unit and integration suites with Go 1.27.2; integration DSNs point to an isolated local PostgreSQL container.
- PostgreSQL month-card concurrency, billing, and payment recovery tests under the race detector.
- WebSocket non-default-group pricing/snapshot and established-turn retry regressions under the race detector.
- Linux amd64 build with embedded frontend.
- Deployment shell/Docker/Caddy checks and Ruby/Python release-policy checks.
- Independent security and WebSocket/platform reviews; all Important findings resolved.

The established-turn retry test also fails against an isolated overlay removing
the fix, confirming it exercises the prior bug. One initial Redis test-container
startup timeout was resolved by rerunning the integration suite with lower
package concurrency. Older duplicate-field expectations were updated to the
intentional 400/1008 rejection contract.

The local evidence directory is `../sub2api-0215-validation` (outside Git).
