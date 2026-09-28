# Official v0.2.9 integration

Integration branch: `custom/0.2.9`; candidate version: `0.2.9-custom.1`.

Parents:
- Custom baseline: `v0.2.8-custom.1` (`5e010e47f2b54a6ce6751811f8b3fee1eee292a0`).
- Official stable release: `v0.2.9` (`4c00df2e0183e2c70b7fa8ba45914205e36aad0c`, local ref `official-v0.2.9`).

## Integration decisions

Resolve five conflicting files by combining the official video-independent pricing implementation with the existing custom model plaza. Preserve the shared reasoning multiplier helper, channel-price reference, model cards, monthly-card and coupon behavior, multi-group keys and concrete per-Key model permissions. Video card and detail prices both clamp independent video rates to zero, matching the official billing behavior. Keep the official response fields required and update custom fixtures accordingly.

Adopt the official group allowlist glob expansion while retaining the separate exact, case-sensitive Key ACL. A handler regression covers suffix and middle globs combined with per-Key allow/deny restrictions, including catalog intersection and blocked request candidates.

Independent review identified an existing WebSocket/month-card integration bug: the settlement callback rewrites `RequestID`, after which native ingress used that internal billing identifier for upstream response continuation and sticky routing. Capture the upstream response identifier before calling the billing callback. A two-turn WebSocket regression verifies continuation within a window, removal of the old continuation on a new Codex window, and distinct settlement IDs.

Preserve all custom migrations and release safeguards. This integration has no database migration or dependency updates, and does not include unmerged official PRs such as prompt-cache PR #7547.

## Deployment considerations

Channel image prices left unset now inherit catalog/LiteLLM/builtin prices, falling back to text prices if the catalog has no image price. Explicit image output price `0` remains free. A read-only production audit found 24 token-pricing rows: 9 unset image-output prices, 9 unset image-input prices and 15 explicit zero image-output prices. No usage rows in the preceding seven days reported nonzero image input/output tokens. This does not prove that every image request was captured; before deploying, explicitly determine the intended price of each affected row. Code integration makes no live pricing changes.

First validate in the isolated China staging environment, including real streamed calls, CC Switch import, month-card/coupon purchase and settlement, multi-group model permissions, and image-price display/charges. Take fresh database/config backups before a production cutover. Preserve the US membership API upstream and the existing isolated networks. Deployment and tag publication are separate authorized steps.

## Validation

Completed validation, using local isolated services only:

- Full backend `go test -tags=unit ./...` and `go test -tags=integration ./...` passed on the final tree after the WebSocket fix. Integration used Docker PostgreSQL/Redis plus isolated DSNs for model-access, scheduled-task and leaderboard tests.
- PostgreSQL month-card `go test -race ./internal/monthcard -count=1` passed with all three DSNs configured, including payment recovery and concurrency.
- The WebSocket regression first failed on the old implementation (`ws-turn:test:1` instead of `resp_auto_prev_1`), then passed after the fix. The exact-key/group-glob handler regression also passed.
- Frontend full run covered 385 files / 2,902 tests, with one negative-video-rate failure in a run started before the final source edits. On the final tree, the entire model-plaza suite passed (5 files / 74 tests), including that case. All other 384 files / 2,901 tests passed in the full run. Final full ESLint passed; typecheck passed.
- Deployment shell and compose checks passed. Release-policy checks passed for all 16 CI result combinations; all 10 release-helper tests passed.
- Independent review and follow-up review completed, with no unresolved Critical or Important findings after the WebSocket fix.

- Backend `golangci-lint` v2.13.0 passed with zero issues.
- Frontend production build passed, including TypeScript and locale validation. Vite reported advisory chunk-size and browsers-data-age warnings.

- Linux amd64 embedded backend build passed (`CGO_ENABLED=0`, `-tags=embed`, `-trimpath`), with the newly built frontend assets.

No release tag was created and no application deployment or live pricing/database changes were performed. Real upstream staging acceptance remains a separate deployment step.
