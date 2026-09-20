# Sub2API v0.2.6-custom.4 release record

Date: 2026-09-20 (Asia/Shanghai).

- Official baseline: 0.2.6.
- Branch: `feature/group-buy-glass-cn`.
- Annotated tag: `v0.2.6-custom.4`.
- Tag object: `36e55a5f6aa0306f49a52719246ca2958bbfaac7`.
- Application commit: `892ab46dfdc155fc103af517d169300913408543`.
- Release: https://github.com/varrge/sub2api-custom/releases/tag/v0.2.6-custom.4.
- Fixed GHCR tag: `ghcr.io/varrge/sub2api-custom:0.2.6-custom.4`.

## Scope and validation

This release includes the glass group-buy/month-card UI, payment coupon support, API-key group layout and allow/deny model selection already validated in China. See [China deployment](staging-model-mode.md) for API/browser checks, backups, counts and rollback requirements.

The additional release fix marks coupon query cleanup and test transaction rollback errors as intentionally ignored, matching the package's existing cleanup conventions. It changes no SQL, policy, model routing or UI behavior. Local month-card tests passed; the exact release commit passed all four jobs in [branch CI](https://github.com/varrge/sub2api-custom/actions/runs/35498690278), including database integration and PostgreSQL month-card concurrency/payment recovery tests.

## Earlier custom.3 tag

`v0.2.6-custom.3` points to `bfb80b7b6c6b8ca2ba3ef5807d90864e3ce9a3b6`. Its CI found seven unchecked cleanup return values in coupon code/tests. The unfinished Release workflow was cancelled; no GitHub Release was published for custom.3. Its immutable tag was preserved. The cleanup fix was committed separately and published under custom.4 after successful branch CI.

## Deployment state

This tag operation publishes artifacts. China continues running the already-validated `0.2.6-custom.3-candidate.model-mode.4052228a3` image; the cleanup-only release fix has not been deployed there. Japan production was not changed. Existing deny-mode rollback restrictions still apply.

## Final publication verification

- [Tag CI](https://github.com/varrge/sub2api-custom/actions/runs/35499413869): success, all four jobs passed.
- [Release workflow](https://github.com/varrge/sub2api-custom/actions/runs/35499413830): success.
- Release is published, not a draft or prerelease, and `/releases/latest` returns `v0.2.6-custom.4`.
- Assets: checksums plus Linux amd64/arm64, macOS amd64/arm64 and Windows amd64 archives. Checksums list all five archives.
- GHCR index digest: `sha256:1c4267e7dcc3569d8aeff6b984f43335cffe1ca399f740fd2a99ed7eb44b4f30`.
- Linux amd64 manifest: `sha256:bee8a937f66bdd39404b15c8f086a13b123c147a9fb385090c62d572e9ce3cb6`.
- Linux arm64 manifest: `sha256:9bd82856bedc976641cf5214b8d9e4ed6b49c3cd0248d59c4155dde9099de3e0`.
- Both platform manifests/config blobs were verified by SHA-256; their architecture, version and revision labels match the release tag and commit.
