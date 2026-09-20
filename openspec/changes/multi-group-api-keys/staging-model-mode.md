# API-key model allow/deny selection — China staging

Date: 2026-09-20 (Asia/Shanghai).

## Behavior

The model-restriction editor in user API keys and admin user-key management now has two buttons:

- **仅允许勾选**: allow only selected concrete public model IDs. An enabled empty selection is rejected.
- **勾选禁用**: deny selected IDs, retaining other eligible group models. An empty selection adds no model restriction; new group models remain available.

Switching modes preserves the selection. Group permissions/subscriptions still apply; quotas and rate counters remain shared across the entire key. Disabling model restrictions preserves the chosen mode and selection for later use.

Existing policies without `mode` retain allow semantics. API-key JSON stores optional `mode: allow | deny`; no schema migration is needed. Group policies remain allow-only. Auth snapshot version is 28. User/admin updates to an existing deny policy must include an explicit mode; stale editors cannot silently convert it into an allow list. Other updates that omit the policy retain it.

## Source and runtime

- Branch: `feature/group-buy-glass-cn`; application commit: `4052228a3feb531da2ed018cfdb4be7fe1e42606`.
- Fixed candidate image: `sub2api-custom:0.2.6-custom.3-candidate.model-mode.4052228a3`.
- Image ID: `sha256:0315f5449916a8182ebe9e9ed68298879898b877d854590917aeb5ae35c58b28`.
- Binary SHA-256: `ae993df02c9eebfa9943a2549cc7d0d7dbc31ba34e267b49217dac6c25c80499`.
- Source SHA-256: `81c454756fd3e63183c0f6ccc1dc8b39c6708762cda018ed597df6c82a71fed0`.
- Image context SHA-256: `fa1c04e07d16341174f3c359af3e3bb8909204df176b16ad4a83b69f611e6abc`.
- Private server artifacts: `/home/yinan/sub2api-staging/model-mode-20260920T072546Z/artifacts`.
- China project: `sub2api-multigroup-cn`; app container: `1dd1c493585c46a950a69aac39bfcdd614df9c274e692ecfbf5aa9c98f79acad`.
- App-only cutover used `--no-deps`; healthy after 7.8 seconds, explicit restart healthy after 9.6 seconds. All other container IDs remained unchanged; internal network and unpublished ports preserved.
- No new formal tag, GitHub Release or push. Japan production was not changed.

## Validation

- 43 focused frontend tests passed; full ESLint, locale completeness, Vue TypeScript and production build passed.
- Focused tagged Go tests passed in repository, service, server, handler, admin handler and DTO packages, including API contracts, SQLite policy persistence, warm-cache invalidation, protocol model catalogs and first/later WebSocket turns.
- Regression checks reject stale editor writes before policy/counter mutations. Independent reviewer found no remaining blocking defects.
- Desktop/mobile light/dark browser checks passed for switching, retained selection, save/reopen and layout.
- Live China API smoke used a temporary multi-group key (groups 3 and 4, baseline 12 models): deny persistence, catalog exclusion, denied request HTTP 404, inverse allow behavior, warm-cache refresh, admin edits, empty deny, invalid mode/empty allow rejection and unchanged shared limits all passed.
- Live authenticated browser edited a separate temporary key, saved deny, reopened it, switched to allow and confirmed persisted values. No JavaScript errors. Both temporary keys were deleted.
- Live authenticated month-card routes `/my-group-buy`, `/group-buy`, `/purchase?tab=group-buy`, `/admin/group-buy` still render. Public API and embedded HTML both retain `payment_enabled=true`.
- Eight relevant deployed JS/CSS assets match local SHA-256 values through the tunnel. No upstream inference or payment was made; Docker/Postgres/Redis repository integration harness was not run for this change.

| Table | Total before | Total after smoke | Nondeleted before/after |
|---|---:|---:|---:|
| users | 154 | 154 | 151 |
| accounts | 57 | 57 | 19 |
| api_keys | 223 | 225 | 158 |
| user_subscriptions | 49 | 49 | 24 |

Two additional API-key rows are the soft-deleted smoke keys.

## Backup, access and rollback compatibility

- Fresh PostgreSQL backup and private Compose snapshot: `/home/yinan/sub2api-staging/model-mode-20260920T072546Z/backup`.
- Database archive: 65807447 bytes, verified with `pg_restore --list`; SHA-256 `e9b3aab3171ba1ab5091d513773374f0db0cdeeb09d7d816724934076e7bd577`.
- Exact prior-app rollback image: `sub2api-restore:model-mode-20260920t072546z`. Its binary SHA matches the previous running app in an isolated temporary container.
- Access: <http://127.0.0.1:18081/keys>; current tunnel `127.0.0.1:18081 -> 172.26.0.4:8080`, control socket `/tmp/sub2api-cn-glass-20260920.sock`. Refresh existing browser tabs to load the mode buttons.

**Do not run an older allow-only binary while nondeleted deny policies exist.** It ignores `mode` and interprets blocked IDs as allowed IDs. This also applies to disabled deny policies if later re-enabled. Do not run mixed application versions or follow earlier image-only rollback instructions without this check.

The guarded rollback utility is `/home/yinan/sub2api-staging/model-mode-20260920T072546Z/artifacts/rollback.py`. Run it on China with the deployment directory as the first argument and `--check-only` to verify eligibility without changing the app. It refuses rollback if any nondeleted key has `mode=deny`, checks the exact previous binary hash, and performs only an app rollback when executed without `--check-only`. It never restores the database. The check passed immediately after smoke cleanup with zero remaining deny policies; check again at rollback time.

Once real deny policies are saved, use a deny-aware rollback build or a separately reviewed policy conversion. Do not silently discard or invert model restrictions.
