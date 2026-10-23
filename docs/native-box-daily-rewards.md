# Native box and daily leaderboard rewards

This change is opt-in. It does not deploy, change live rates/coupons, enable an
event, move cards, or grant production rewards by itself.

## Ownership and identity

Sub2API owns authenticated user entry, usage accounting, ranking and immutable
daily manifests. KaWang continues owning box campaigns, prizes, tier weights,
cards, draws and prize delivery. The old embedded page and new native page use
the same underlying cards and draw idempotency keys.

Configure the Sub server environment:

```text
NATIVE_BOX_SERVICE_URL=https://<trusted-kawang-service>
NATIVE_BOX_CONNECTION_ID=<connection-for-this-sub-server>
```

The native route is `/activities/box`. Its authenticated API exchanges the user's
already-verified Sub JWT in a server-to-server POST. The KaWang connection is
operator-selected and KaWang validates the token against that Sub instance.
Client-supplied user IDs, arbitrary destinations and credentials in query strings
are not accepted. The delegated box session never leaves the server. HTTPS is
required except for loopback development; redirects are disabled.

New draws carry both the request ID and expected box campaign. Unknown results
are persisted per authenticated user before sending. Lookup-only recovery stays
available after the campaign closes and never consumes another campaign's card.
The server validates result identity and shape before the page clears retry state.

## Setup sequence

1. Deploy the Sub migration `257_activity_reward_manifests.sql` and both codebases.
2. Verify KaWang's saved connection/admin credential targets this Sub server.
3. In Sub activity leaderboard settings, enable daily mode and automatic reward
   settlement. Boundaries must be midnight Asia/Shanghai.
4. For the requested event set start `2026-10-24T00:00:00+08:00` and exclusive end
   `2026-10-28T00:00:00+08:00`. Disable demo data. Titles and prize descriptions
   remain operator-configured, not hardcoded.
5. Create/schedule the target box campaign on the matching connection. Its start
   must cover the source start, and its end must be at least 24 hours plus the
   ten-minute settlement grace after source end. Suggested redemption end:
   `2026-10-31T00:00:00+08:00`.
6. In KaWang box administration, select that campaign under automatic leaderboard
   rewards and bind the current source event. One task per connection/source event.
7. Verify native bootstrap/prize display and synthetic staging draws before
   replacing the existing embedded navigation entry. Keep the old page available
   for rollback; do not duplicate balances/card inventory.

## Accounting and delivery

- The live leaderboard rolls each Shanghai day. The stored overall activity
  dates are not rewritten daily.
- At 00:10 after each day, the KaWang maintenance loop requests the full private
  Sub snapshot. It retries every minute and catches up all unprocessed days after
  downtime. The last requested day is October 27, not October 28.
- The Sub transaction freezes all eligible callers, not merely the visible top
  20. Positive actual_cost ranks preserve full PostgreSQL decimal precision and
  existing tie ordering. Free usage receives rank 0 and a participation award.
  Admin, enterprise and deleted users are excluded.
- Top 1: high x1. Top 2: medium x2. Top 3: medium x1. Everyone else: low x1.
  Top-three users do not also get a participation card. Each day is independent.
- Date membership uses usage_logs.created_at, matching the existing leaderboard.
  A request recorded after midnight belongs to the next day. A persisted
  month-card recovery command older than day-end blocks freezing until recovery.
- Freeze stores exact manifest JSON plus SHA-256. Retries return that same list;
  later refunds, manual changes or late writes do not silently rerank issued
  awards. Corrections need an explicit audited operational process.
- KaWang verifies source identity, day boundaries, hash, uniqueness, amount order
  and tier/count rules. It stores the entire manifest and grant obligations.
  Issuing all daily cards and marking all recipients granted is one SQLite
  transaction, with unique grant and card source IDs. A crash either commits the
  whole day or leaves it pending; retries cannot issue extra cards.
- A disabled task, inactive connection or expired/wrong target prevents grants.
  Failures stay visible in the admin task list. No compensation cards or
  substitute prize pools are chosen automatically.

The admin task page can pause/resume, safely retry, and export every recipient as
CSV with date, identity, amount, rank, tier/count and issuance status. This is a
private admin export; public leaderboard responses remain anonymized.

## Operational limits

Keep the original source event configured until all its days are settled. Moving
or disabling the event intentionally stops new snapshots; it does not revoke
already issued cards. Snapshot dates are immutable.

A bound connection cannot be retargeted to another base URL. Historical jobs also
reserve their date ranges by pinned source URL, so soft-deleting/recreating a
connection or extending/rebinding an overlapping campaign cannot repeat rewards.
For a different server create a new connection; for another event use a
non-overlapping source period. Resolve a changed deployment URL explicitly rather
than silently reassigning already captured identities.

Extending box lifetime also extends its existing automatic earning rules. Review
those rules before enabling a longer redemption window; daily leaderboard
rewards are additional to existing earning rules, not a replacement.

The ten-minute grace period is not a promise of instantaneous settlement:
pending recovery, service downtime, identity/configuration mismatch and an
expired box campaign require resolution before delivery. The native frontend
does not change these accounting rules.

## Implementation verification (2026-10-23)

- Native frontend created by Kimi K3 using the configured `codeg-managed` model
  alias and interactive `--yolo` mode; root finished safety logic after subsequent
  upstream connection failures. No substitute model claimed as Kimi.
- Frontend: 451 files / 3520 tests passing; production build and ESLint passing.
- Go: full unit suite and targeted authenticated gateway/settlement tests passing;
  real isolated PostgreSQL snapshot concurrency regression passes with `-race`.
- KaWang: 38 box-related tests passing. Full suite has 426 tests, 376 passing and
  50 failures. The untouched base has the identical 50 unrelated membership
  failures; no new failing test names were introduced.
- Playwright: desktop/mobile/dark rendering, no horizontal overflow, no page
  errors, lost response + reload + same-ID recovery passed using local fixtures.
- Independent reviews resolved all findings on tenant pinning, overlap/recreate
  duplicates, midnight pending billing, old-campaign recovery, nonterminal reward
  recovery persistence and malformed prize responses.
- Production connectivity, real prize credits and operator event configuration
  have not been exercised or changed. Both repositories must deploy together.
