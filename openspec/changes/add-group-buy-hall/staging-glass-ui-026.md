# China staging: glass group-buy UI on custom 0.2.6

Date: 2026-09-20 (Asia/Shanghai). Scope: deploy completed session 260 UI to the existing China multi-group test environment.

## Source and compatibility

- Base: `v0.2.6-custom.2` (`e496f756dd3af79170f8c37dbad2da7a74978215`).
- Branch: `feature/group-buy-glass-cn`.
- Application commit: `59b2dbdfe37d9e7dfd0811ed80fae9625eb4ea0b`.
- Runtime version: `0.2.6-custom.3-candidate.glass-ui.59b2dbdfe`.
- Session 260 changes were extracted against its 2026-09-18 source snapshot and integrated onto the latest tag in a separate worktree. The original dirty `custom/0.2.0` checkout was preserved.
- Includes glass styles for group-buy catalog, hall, cards, checkout and administration; API-key group wrapping; quota progression; payment coupon UI and required server-side implementation.
- Preserves month-card freeze policy, recruitment cancellation/manual refund handling, independent subscription pages, ordered API-key groups, and per-key model restrictions.
- Additive migration: `245_month_card_coupons.sql`; existing migrations are unchanged. A regression test verifies cancelled-team handling takes precedence over an expired/reallocated coupon.
- This is a locally built staging candidate. No new GitHub tag/Release was created, and GitHub CI was not run for this unpublished candidate.

## Artifact identity

- Fixed app image: `sub2api-custom:0.2.6-custom.3-candidate.glass-ui.59b2dbdfe`.
- Image ID: `sha256:debd5ec5c8f6c737bb1c5fcb990deb59e5b1b137a7b25a621921ebc52e4e7ac1`.
- Linux amd64 binary SHA-256: `c9a622d4723df42d5df68c12e7147a8af1b9a05cc76ba4cbf22132cc3f634274`.
- Source archive SHA-256: `d89424668b75322045c4135a4d19be487afa629fe1bf409f84b39359719f3b87`.
- Image context SHA-256: `7c46b85fba1be9236bb643c6f8002d19547222944f65c45aacf653543ee25cce`.
- Runtime base: `sub2api-cn-runtime-base:20260918T022208Z`, verified SHA `87df7c0716c495cef1ce9ea369e3dd4f3fcb619e16f09bc46475ad116dd6da10`.
- Source/artifacts: `/home/yinan/sub2api-staging/glass-ui-20260920T062904Z/artifacts` (private server directory).

## Backup and cutover

- Project: `sub2api-multigroup-cn`; only `sub2api` recreated with `--no-deps`.
- Private Compose: `/home/yinan/sub2api-staging/multigroup-20260918T082757Z/runtime/compose.json`. Contains credentials; do not print or commit it.
- Backup: `/home/yinan/sub2api-staging/glass-ui-20260920T062904Z/backup`; PostgreSQL archive 65,852,251 bytes, verified with `pg_restore --list`.
- Backup SHA-256: `d4c2bc91355f160c0f3919bc922a39a1ae7e6787f6fc23a5440577ca64e036d9`.
- Exact rollback image: `sub2api-restore:glass-ui-20260920t062904z`.
- New application container: `0b0f35afc153b54cb4cdea6453171c29391af4fc1c2ac26dceb6580fbe3e31eb`.
- Healthy after 7.8 seconds; explicit restart healthy after 6.6 seconds.
- All other container IDs remained unchanged, including PostgreSQL, Redis, the older China staging project and unrelated applications.
- Network remains `internal=true`, with no published application ports. Japan production was not updated.

| Core table | Total before/after | Active before/after |
|---|---:|---:|
| users | 154 | 151 |
| accounts | 57 | 19 |
| api_keys | 223 | 158 |
| user_subscriptions | 49 | 24 |

## Validation

- 173 focused frontend tests passed; i18n completeness, full ESLint check, Vue TypeScript and production build passed.
- Focused payment/month-card/coupon Go tests passed in service and handler packages.
- Full month-card package passed with a disposable local PostgreSQL server, including coupon concurrency, cancellation/reallocation, frozen-price and cancelled-team precedence tests.
- Independent code review completed; findings addressed before deployment.
- Browser checks passed for desktop/mobile in light/dark themes and long-text mobile fixtures. Coupon application/price payload and group sorting were verified using mocked API fixtures; no real payment was made.
- Live China read-only smoke passed for products, teams, cards, coupon administration/authentication, freeze policy, key model options and key response contracts.
- Live page routes and health passed. 51 CSS/UI JavaScript assets fetched through the SSH tunnel match the local build by SHA-256.
- No fatal/panic/migration errors; core counts unchanged. Upstream inference and external payments were not exercised in the isolated network.

## Access and rollback

- Hall: <http://127.0.0.1:18081/group-buy>.
- Keys: <http://127.0.0.1:18081/keys>.
- Use the existing China test login; no password changes.
- SSH local tunnel: `127.0.0.1:18081 -> 172.26.0.4:8080`; control socket `/tmp/sub2api-cn-glass-20260920.sock` on this Mac.
- To roll back, change only the private Compose app image to `sub2api-restore:glass-ui-20260920t062904z`, retain `pull_policy: never`, validate Compose, and recreate only `sub2api` with `--no-deps`. Keep additive coupon tables; do not restore the database for an app rollback.

## Follow-up: restore missing month-card entry points

The first deployment smoke checked SPA HTTP responses and mocked browser settings, which missed the China environment's real `payment_enabled=false` setting. With a real authenticated admin session, `/group-buy` and `/purchase?tab=group-buy` redirected to the dashboard. `/my-group-buy` and direct `/admin/group-buy` remained available, but payment-gated navigation was hidden.

Restored only the China payment `enabled` setting through `PUT /api/v1/admin/payment/config` with `{"enabled":true}`. The prior payment configuration was saved privately as `artifacts/payment-config-before-ui-access.json`; all other payment configuration fields were verified unchanged. The payment-config endpoint does not invalidate the embedded HTML settings cache, so the public API returned true while `window.__APP_CONFIG__` still contained false. Restarted only the application (healthy after 6.7 seconds); HTML and API settings now both return true. All other container IDs and the internal network stayed unchanged.

A real authenticated browser check now passes for `/my-group-buy`, `/group-buy`, `/purchase?tab=group-buy`, and `/admin/group-buy`. It also asserts that month-card purchase buttons, the coupon-management tab, and the admin menu entry render. Reusable local diagnostic: `/tmp/sub2api-cn-monthcard-routes.cjs`; sanitized results: `/tmp/sub2api-glass-deploy-20260920T062904Z/monthcard-route-check.json`. Credentials remain only in memory and are not printed or recorded.

Future staging checks must exercise authenticated browser routes using actual public settings, in addition to mocked interaction tests and SPA HTTP status checks. Current candidate code and version were not changed by this configuration fix.

## Subsequent model-mode candidate

The later API-key allow/deny candidate and validation are recorded in [staging-model-mode.md](../multi-group-api-keys/staging-model-mode.md). Its rollback compatibility requirements supersede the image-only rollback instructions above whenever deny policies have been saved.
