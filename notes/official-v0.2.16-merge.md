# Official v0.2.16 integration — 2026-10-13

Custom baseline: `7f85c09ba1c94f7a48310ff5bbd48ef20670b04d`, comprising
`v0.2.15-custom.9` and the group Fast/Flex pricing editor fix. Official merge
parent: `01842576b3d5c1e2545b5a99ad555d36e28ed306` (`v0.2.16`).

## Merge decisions

Keep custom month-card billing and reset generations, multi-group API Keys,
per-model group restrictions, expected business-rejection aggregation, payment
verification/recovery, temporary group rates, and group Fast/Flex overrides.

Combine official group translations with the custom temporary-rate fields in
available channels, model plaza and payment plan responses. Localize every group
in a multi-group API Key response, including the custom `groups` collection.
Generate Ent and Wire from the combined schema and provider definitions.

Keep both official and custom critical frontend test selections. Dependencies and
release workflow policy do not change in this official increment. The source
version is `0.2.16-custom.1`; candidate builds use an explicit candidate version
and commit, and formal release builds obtain their version from the tag.

Gemini explicit caching needs custom integration at the protocol, selected-group
and billing boundaries. Normalize the `models/` resource prefix before policy
checks; cache management requires Gemini-capable accounts and cached resources
must retain their original group. Mutations must check the original public model
against current permissions. Quote and settle with the same request timestamp,
temporary group rate and pricing rules. Month-card charges must use the admitted
snapshot, card order, quota generations and balance supplement rules.

## Cache billing semantics

The official cache API charges cache input and the full requested TTL upfront.
Shortening or deleting a cache does not refund storage fees. Cache expiration uses
wall-clock time and does not pause when a local month card is frozen. These are
new cache product semantics; existing token-generation billing is unchanged.

GPT-6.1 Sol long-context exemption is based on the upstream OAuth/setup-token
account and actual model, not whether the downstream user holds a month card.
Custom interval prices and final ActualCost allocation must remain intact.

Fixed-charge reservations project all pending fixed fees through the existing
month-card allocator within a savepoint, then roll back the simulated charges.
Only the hold is persisted. The API Key quota and spending windows use the same
transaction connection and eight-decimal amount comparisons. Settlement and hold
removal share one transaction; existing durable billing recovery retries failures.
An orphan without a persisted billing command expires after a bounded lease.

This is an admission-time affordability projection, not an assignment of card
funds. Preserve the D19 contract: each request settles against its original cards,
order and quota generations. Out-of-order completion or other in-flight usage can
exhaust an earlier request's cards and debit its balance, including into debt;
newly purchased cards are not added to that earlier snapshot. Subsequent calls are
blocked on debt. No FIFO settlement or user-wide billing wait is introduced.
The upstream-success-to-first-local-persistence crash window remains; a persisted
billing command has recovery coverage, but the upstream call and local database
are not one transaction.

## Database

Official additions are `242_group_i18n.sql` and
`243_gemini_cached_contents.sql`, `244_gemini_cached_contents_channel_usage.sql`,
`245_gemini_cached_contents_mutation_lease.sql`. Custom addition
`252_month_card_fixed_charge_reservations.sql` stores bounded fixed-charge holds
and links their cleanup to the existing durable month-card billing command.
Existing migration filenames and
contents stay immutable. Numeric prefixes overlap custom migrations, but full
filenames do not; the runner identifies migrations by their full filename.

## Validation

Completed validation before candidate construction:

- Frontend typecheck, ESLint, all 429 Vitest files / 3,322 tests, production build.
- DTO localization, request-model and domain package tests.
- Deployment security/environment/resources/lifecycle/cache-policy checks and
  release workflow policy tests.
- Official migration empty-database and upgrade-path checks, including preservation
  of custom data and previously applied migration checksums/timestamps.

- Full backend unit and integration suites, including real PostgreSQL and Docker
  repository tests.
- Targeted cache ownership, selected-group/model restrictions, normalized model
  names, mixed month-card rights, concurrent holds and billing recovery.
- PostgreSQL migration checks: fresh 313 migrations, upgrade from the 308-migration
  custom release, repeat startup, historical checksum/data preservation, and
  old/new Ent read/write compatibility.

- Complete month-card PostgreSQL suite with the race detector and all three DSNs
  enabled; golangci-lint reports zero issues.

Independent review details are recorded in the candidate evidence bundle. Candidate deployment records contain the exact
commit, binary hash, backup, migration and API smoke-test results. This branch is
not a formal release; user acceptance on China staging precedes tagging.

The database changes are additive, but the old binary does not understand cache
hold recovery. Schema compatibility alone does not authorize rolling back while
new cache billing is in flight. China remains isolated; no live Gemini-provider
request or production-scale lock-duration benchmark is claimed.
