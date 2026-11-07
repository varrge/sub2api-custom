# Official v0.2.20 integration

Date: 2026-11-07 (Asia/Shanghai). Custom base:
`005397cf11bf77f0e9f8f3ef2bd777f545255ba0`; official tag commit:
`dab829a4e9b7d1cc307189aea8b0319eb29e29d0` (`v0.2.20`).
The candidate branch is `codex/merge-upstream-0.2.20`; the existing integration
branch remains `custom/0.2.19`. The source VERSION is updated to `0.2.20` because
the upstream tag precedes its VERSION synchronization commit. No release tag,
push or deployment is part of this integration.

## Compatibility decisions

- Keep all custom image/video studio components, drafts, persisted request
  snapshots, task recovery and original-key access. The upstream account size
  tiers complement the studio; no studio layout is replaced.
- Merge async image body-memory ownership with the account request lease and
  original GroupID. Cancellation releases both reservations. Existing accepted
  tasks are not silently resubmitted or billed through a different group.
- Preserve image/video creation-time pricing, original channel usage fields,
  video rate overrides and month-card settlement semantics while adopting
  upstream immutable usage metadata snapshots and bounded background work.
- Retain shared-credit personal usage costs for balance/subscription notification
  decisions. Quota persistence stays inside the usage task with a bounded
  detached context, including month-card/shared-credit recovery callers.
- Use auth snapshot version 33, remove the silently duplicated RequirePrivacySet
  field, keep enterprise identity refresh, and retain the multi-group RPM map.
  A successful nil override is distinct from an unqueried group; switching
  groups resets the cached GroupID and cannot reuse another group's override.
- Combine image size passes with primary/fallback account pools. Explicit image
  tiers also constrain the pinned previous-response fast path; unspecified sizes
  retain pinned ownership. Explicit image tiers are also checked during ordered
  multi-group preselection so a 1K-only group cannot mask a later 4K-capable group.
  A temporary image-size spillover does not overwrite
  the original session binding during an acquire-only pool probe.
- Keep upstream Gemini delta buffering and native tool media, while preserving
  explicit OpenAI delta repetitions, tool-call IDs and images for the custom
  Gemini alternate-protocol adapter.
- Route early custom group body reads through the new capacity/timeout semantics:
  memory pressure returns 503 + Retry-After, timeout returns 408, and disconnected
  clients end with an empty 499 response. Per-request size violations remain 413.
- Keep custom routes, feature flags, lifecycle cleanup and audit tests alongside
  upstream MCP. Regenerate Wire from the combined providers. Keep both the custom
  image response-header timeout and upstream transport-cooldown configuration.
- The new `249_mcp_oauth.sql` coexists with custom migration filenames, including
  `249_month_card_quota_adjustments.sql`; migration identity uses the full filename.

## Validation

Tests use the exact candidate source and its new dependency lockfiles.

- Frontend frozen-lockfile install, typecheck, ESLint, all 485 test files / 4,119 tests,
  and production build passed. Studio components/assets remain unchanged.
- Deployment lifecycle fixtures, Compose security/environment/resource checks,
  Caddy policy checks and release workflow checks passed.
- PostgreSQL MCP authorization/replay/concurrent code exchange and migration
  concurrency/idempotency/schema checks passed (7 top-level tests).
- PostgreSQL month-card, push and voting suites passed with the Go race detector.
- The standard backend suite (`go test -p 3 ./...`), full backend lint (0 issues),
  embedded server build and the resulting binary's `-version` check passed.
  The binary reports `0.2.20`.
- The final full unit-tag suite (`go test -p 3 -tags=unit ./...`) passed after
  the test isolation adjustment below; the service package completed in 229.095s.
  Lint on the adjusted service package also passed with 0 issues. The standard
  full suite and embedded build used identical production code; only this test
  and the integration notes changed afterward.
- Independent merge-interaction review found the explicit image size preselection
  regression. Four 4K cases failed before the patch; all 11 routing cases passed
  after the patch. The reviewer verified the fix and found no remaining blockers.
- A full unit-tag run exposed interference in the upstream orphan-precheck
  zero-allocation assertion: `AllocsPerRun` reads process-wide heap counters.
  The unchanged test passed 50 isolated repetitions. A temporary external overlay
  reproduced false counts with concurrent unrelated allocations and a larger
  escaped body (3/3 failures). Running the measurement in a bounded child process,
  following an existing repository test pattern, passed the same stress probe
  3/3 times. The original body, five measured runs and exact zero bound are retained
  in the repository test. Independent review approved this test-only adjustment.

PostgreSQL is a disposable local 16.14 instance; no production database or paid
generation API is contacted.

The repository's integration harness normally requires Docker, which is not
running here. PostgreSQL-only MCP/migration integration cases were executed using
an external Go overlay that substitutes only harness startup with the isolated
local PostgreSQL connection. The actual repositories, SQL migrations and test
bodies are unchanged. Redis-dependent integration cases are outside that run.

The initial full backend unit attempt supplied a URL DSN to legacy scoped-admin
fixtures that append keyword `search_path`. Its connection failures are test
configuration failures, not waived assertions; the final run uses the keyword
DSN for those fixtures and URI DSNs for fixtures that parse URLs.

Local validation logs are retained under `/tmp/sub2api-020-*`, including
`backend-unit-final.log`, `backend-tests.log`, `backend-lint-final.log`,
`backend-lint-isolation.log`, `frontend-tests-final.log`, `mcp-migrations.log`
and `backend-race-billing.log` (each name prefixed with `sub2api-020-`).

## Remaining limits

No live supplier output-format/size acceptance, full Docker-based Redis/PostgreSQL
18 integration suite, or production deployment is established by local checks.
The upstream image output format still depends on supplier capabilities. Existing
frontend bundle-size/Browserslist warnings do not block the build.
