# Sub2API v0.2.7-custom.1 release record

Date: 2026-09-20 (Asia/Shanghai).

- Official baseline: `v0.2.7`.
- Source branch: `feature/merge-upstream-0.2.7`.
- Annotated tag: `v0.2.7-custom.1`.
- Tag object: `43be44ec222748ee1684f68a40dd81ce034a2f2f`.
- Application commit: `8bfcb5e4ca35d4e5c14fbb55ca949393e0a05b99`.
- Release: https://github.com/varrge/sub2api-custom/releases/tag/v0.2.7-custom.1.
- Fixed image: `ghcr.io/varrge/sub2api-custom:0.2.7-custom.1`.

## Scope and validation

This release combines official v0.2.7 with custom multigroup keys, allow/deny model policies, shared key limits, group-buy/month-card billing and Codex tickets. It includes conditional scheduled model tests and Seedance routing, task ownership, frozen token pricing and entitlement, durable settlement/recovery, and mandatory usage submission. Missing Seedance prices fail before creation; explicitly configured zero rates remain valid. See [integration notes](upstream-0.2.7-integration.md) for compatibility decisions.

The tag points to the exact application commit that passed [branch CI](https://github.com/varrge/sub2api-custom/actions/runs/35505624265) and [China candidate acceptance](upstream-0.2.7-deployment-cn.md). Later branch commits only document that acceptance and publication. Local frontend/backend checks, real PostgreSQL month-card concurrency/recovery, API/browser checks and application restart verification passed. No real supplier video call was made.

## Publication verification

- [Tag CI](https://github.com/varrge/sub2api-custom/actions/runs/35514029058): success, all four jobs passed.
- [Release workflow](https://github.com/varrge/sub2api-custom/actions/runs/35514029133): success.
- Published release is neither draft nor prerelease; `/releases/latest` returns this tag.
- Assets: checksums plus Linux amd64/arm64, macOS amd64/arm64 and Windows amd64 archives; all five archives appear in checksums.
- GHCR index digest: `sha256:1d4acba188f3ff005a966bb21ccf2454720c7d69b3470e25f9ca529e5684d72a`.
- Linux amd64 manifest: `sha256:cbcec7184bc52187dcf2f09684b93040c7bae4ea4a203b38eacda181ba445f2a`.
- Linux arm64 manifest: `sha256:36339dc559917d3128fb6a456a3b9f8f8387452c62547308eeb4b5c330b1a399`.
- Both platform manifests and config blobs pass SHA-256 verification; architecture, version and revision labels match the release tag and application commit.

## Deployment state

This operation publishes the tag and release artifacts. China continues running `0.2.7-custom.1-candidate.8bfcb5e4c`, built from the same application commit; this tag operation does not replace its image. Japan production was not changed. Backup and app-only rollback requirements, including disabling enabled conditional plans before using an older binary, remain in the candidate deployment record.
