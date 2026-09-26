# Official v0.2.8 integration

Integration branch: `custom/0.2.8`; candidate version: `0.2.8-custom.1`.

Parents:
- Custom production baseline: `v0.2.7-custom.13` (`58b0db9923b400d50eabf2b5861f17c31b5b1f2d`).
- Official stable release: `v0.2.8` (`fd80b08c90b55edcad5b00171b53f08721d30da1`, local ref `official-v0.2.8`).

## Integration decisions

Resolve the 34 overlapping files by preserving custom month-card transactional settlement, debt rejection, multi-group keys, model permissions, coupons, temporary group rates and custom UI. Add upstream simple-mode monetary key windows without charging cards or balances. Long-lived WebSocket turns must use the refreshed key/subscription snapshot both before admission and after concurrency slots are acquired.

Combine Codex ticket settings and secret masking with the official Claude Code version-sync settings and OpenCode Go account usage. Regenerate Wire with both sets of providers, including the plugin account directory and usage-billing shutdown cleanup.

Migrate custom model-plaza cards and bulk price import from the removed max-only field to `reasoning_effort_multipliers`. Preserve upstream default semantics: an absent multiplier is 1x.

Adopt the official release matrix and dry-run tooling. Preserve the custom release policy: resolve and validate one source commit, require all CI jobs before publication, recheck the release tag before publishing, and publish GHCR images under `ghcr.io/varrge/sub2api-custom`. Keep custom revisions as full releases. Do not copy the upstream post-release job that writes VERSION onto an unrelated default branch. This integration does not create or publish a release tag.

## Deployment considerations

The new official SQL files are `238b_content_moderation_engine_meta.sql`, `239_channel_reasoning_effort_multipliers.sql`, and `240_affiliate_ledger_operation_id.sql`. The migration runner identifies migrations by their full filename and checksum; retain the custom migrations and never renumber or edit already applied files.

Fable 5.1 Max no longer has the old implicit 3x multiplier. The production audit found two channel Fable price entries and two Fable 5.1 Max requests in the previous seven days, although no explicit legacy max multiplier was configured. Before a production cutover, decide whether to retain the old economics with explicit per-effort pricing. Code integration does not change live pricing.

Before production, validate the candidate in an isolated staging environment, take a fresh database/config snapshot, and verify custom card/coupon purchase, settlement, key permissions and real upstream streaming. Preserve the US Caddy membership API upstream `apikey.vsakura.top`. After new production writes, do not blindly restore an old database for application rollback.

## Validation

Tests used local isolated PostgreSQL/Redis, never the production database. Completed validation:

- Backend Go 1.27.0: `go test -tags=unit ./...` and `go test -tags=integration ./...` passed. Repository integration tests applied all migrations and validated the official reasoning multiplier migration alongside the custom schema.
- Dedicated PostgreSQL month-card tests: `go test -race ./internal/monthcard -count=1` passed with all three month-card DSNs configured, including transaction/concurrency and payment recovery cases.
- Final focused regression run passed for SimpleMode/month-card interaction, refreshed WebSocket turn admission, combined account states, pricing metadata and model plaza. The combined Claude/Codex settings round-trip test passed in the full handler/admin suite.
- `golangci-lint` v2.13.0: zero issues.
- Frontend: 384 test files / 2883 tests passed; full ESLint and production build (including TypeScript and locale validation) passed. Vite reports advisory bundle-size and browsers-data-age warnings.
- Embedded backend production build (`CGO_ENABLED=0`, `-tags=embed`, `-trimpath`) passed on Darwin arm64. Linux release artifacts are built by the release matrix before publication.
- Release policy tests passed, including all 16 CI outcome combinations, tag validation/pinning, moved/deleted-tag rejection, dry-run branches and verification before publication. All 10 release-helper tests passed with a mixed-case custom fork identity.
- Deployment script tests passed for Apple lifecycle, compose security/gateway/simple-mode configuration, runtime resources and Caddy caching/SSE handling.
- Independent code review: no unresolved Critical or Important findings. Its stale WebSocket key-limit finding was fixed before the final focused regression run.

No release tag was created, and no production application, pricing or database was changed. Isolated staging acceptance and production cutover remain separate deployment steps.
