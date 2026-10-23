# Native Box and Daily Rewards

## Scope

Native Vue opening page in the Sub2API fork; KaWang remains the sole owner of
cards, prizes, draws and prize delivery. No production configuration, deployments,
real grants, shared branch merges or releases in this implementation task.

## Contract

- Authenticated Sub user APIs: GET `/api/v1/activities/box/bootstrap`, POST
  `/api/v1/activities/box/draws` with `{requestId, cardTier}`. Responses use the
  existing KaWang bootstrap/draw data wrapped in the Sub API envelope. Server
  authenticates the Sub JWT and exchanges it with the configured trusted KaWang
  server; browser never receives admin credentials or places tokens in URLs.
- GET bootstrap includes existing `campaign`, `availableCards`,
  `availableCardsByTier`, `prizes`, `history`, `progress`, `user` fields. Inspect
  KaWang's `buildBootstrap` for exact response shape. All animations reveal a
  server-recorded draw, never select a prize. Retries preserve requestId.
- Private Sub settlement API freezes a complete daily manifest for the configured
  leaderboard campaign: Shanghai calendar days, start inclusive/end exclusive,
  positive actual_cost ranks using existing tie ordering; other eligible callers
  receive low cards, including zero-cost usage. Admin/enterprise/deleted accounts
  remain excluded. Fail closed while billing recovery is pending.
- KaWang scheduled job polls enabled reward campaign configuration and catches up
  all ended days. Persist snapshot identity + complete manifest + grants before
  marking completion. SQLite uniqueness/transactions prevent duplicate cards
  across overlapping ticks, restart, timeout and retry.
- Daily awards: positive-spend rank 1 high x1, rank 2 medium x2, rank 3 medium x1;
  all other eligible callers low x1; no participant award stacking with top three.
- Card redemption deadline must follow the final daily settlement and leave a
  redemption grace window. Never grant cards into ended/wrong-connection campaigns.
- Preserve existing leaderboard defaults. New daily mode/config opt-in. Existing
  four-day campaign defaults in docs use Oct 24 00:00 through Oct 28 00:00 +08.

## Tasks

1. Kimi K3 (`codeg-managed`, --yolo): native page, menu/route, API types/tests only.
2. Root: Go trusted box gateway + immutable daily manifests + daily leaderboard mode.
3. Root: KaWang automatic settlement/grants, admin manifests/export and tests.
4. Integration, security/retry/timezone regression tests, browser screenshots,
   independent review, operational setup documentation.

## Progress

- Existing dirty worktrees preserved; two clean feature worktrees created.
- Kimi model alias verified locally to resolve to kimi-k3. No model substitution.
- Ruling: "activity ends" means each daily window ends, consistent with the user's
  daily awards for Oct 24-27; restart catches up all unprocessed days.
- Kimi K3 ran via the verified codeg-managed alias in --yolo mode. The CLI disallows
  --prompt with --yolo, so the task used an interactive session. Kimi implemented
  the native page, route/menu, locales, API types and focused tests.
- Kimi's follow-up requests repeatedly failed with APIConnectionError. Root
  retained its UI design, stopped that session, and finished request recovery,
  account isolation, card terminology and strict response validation.
- Root implemented Go gateway/manifests and KaWang daily jobs, transactional
  grants, administrator controls and full recipient export.
- Independent reviews found and resolved: pending billing crossing midnight;
  connection retargeting; extended/rebound overlapping periods; deleted/recreated
  connection duplicates; ended-campaign recovery; malformed prize responses;
  nonterminal delivery recovery persistence.
- Local browser checks passed desktop/mobile/dark and timeout/reload retry with
  the same request ID. All displayed prize values in previews were test fixtures.
- KaWang's full suite has 50 pre-existing failures in unrelated membership tests;
  running the untouched base commit reproduced the same failure names exactly.
- No production deployment, rates/coupon edits, real card grants or release tags.
- Follow-up correction: cumulative leaderboard display/export is independent of
  daily reward settlement. A regression reproduced loss of earlier-day usage
  from the display; removed rolling display windows and retained daily award
  selection only. This does not change the user's specified daily prize rules.
