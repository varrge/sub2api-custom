# Official v0.2.17 integration

Base: custom `aea973149b0c92bb80a3a677cd297cab0a5bfc54`
(`v0.2.16-custom.1`). Upstream: `060d0160417782cd962a0c7f460f6cd4565d4916`
(`v0.2.17`). Candidate work happens on `upgrade/0.2.17` before release approval.

## Compatibility decisions

- Keep the upstream order-type whitelist and balance-only fulfillment guard,
  adding validated `month_card` to order creation, fulfillment and admin retry.
  Month cards never fall through to balance credit.
- Keep the upstream pending-refund deduction hold, atomic failure rollback and
  prohibition on retrying a pending refund. Preserve the custom month-card claim
  fields before calling the gateway, no-balance-deduction policy, and separate
  card revocation/reconciliation.
- Adapt both callers of `latestRefundPendingDetail` to propagate audit read and
  parse errors. Recovery passes the original request time to the provider and
  preserves it in the pending audit JSON. Old audits use their creation time;
  interrupted requests with no audit use the durable refund claim's timestamp.
  This does not add a database migration.
- Retain stable Alipay refund IDs and reconstruction after a lost response,
  querying existing recorded IDs unchanged. Combine them with upstream business
  error checks, non-final states and the ten-minute missing-record grace period.
- Keep EasyPay's merchant, order, trade and amount confirmation. Carry signed
  notification `endtime` and query `endtime` through to month-card fulfillment,
  including nested responses. Reject non-finite queried amounts. Do not relax
  the official precedence of an explicitly present `trade_status`.
- Filter expanded multi-group catalogs and management choices using the shared
  actual-upstream retirement rules. Preserve API-key suppliers, valid explicit
  replacement mappings and eligible later groups. Composite aliases are checked
  against their endpoint-specific targets, then channel and account mappings are
  applied in scheduling order while keeping the original public name.
- Diagnose exhausted group probes using persistent model eligibility. Aggregate
  permanent rejections across candidates into `404 model_not_found` and business
  rejection counters; later valid groups may still serve the request. Temporary
  capacity failures retain 503, group RPM retains 429, and existing subscription
  403 and cache binding behavior remain covered. Compact diagnosis resolves the
  same target as scheduling.

## Validation and deployment boundary

Frontend typecheck, lint, production build and all 3,321 tests passed. Provider
and payment-service tests cover prepared month-card orders, admin retries,
refund claim durability, pending re-entry, corrupt audit rejection, stable IDs,
grace periods and payment-time propagation. A real `CreateOrder` integration
test uses PostgreSQL and a local EasyPay server to verify solo and coupon team
orders, consent, quote snapshots and coupon reservation. Multi-group tests cover OAuth-only,
API-key-only, mixed pools, aliases, management wildcard expansion, Composite
targets, later-group selection and permanent/temporary error classification.

The compact ticket test uses an available fallback model and separately checks
that a retired fallback is ignored without bypassing ticket requirements.

The PostgreSQL 18 month-card suite passes with race detection, using disposable
databases/schemas. Full backend and independent review results are recorded in
the deployment evidence before activation. Upstream did not change migrations,
Ent schema, dependencies or deployment workflows in this increment.

China staging is the `sub2api-multigroup-cn` project. Before replacing only its
application container, save and verify PostgreSQL, app data, Compose and an exact
running-container rollback image. Retain the internal network and database,
Redis and unrelated containers. Use a fixed candidate image and a loopback SSH
tunnel for acceptance. Do not publish a release tag or update production as part
of candidate deployment.

Upstream issue #8171 has no sufficient reproduction or payment-channel details.
Live merchant callback/query compatibility and real transfers are not established
by local gateway fixtures. The test environment remains isolated. Legacy
subscription expiry/revocation during a pending refund is an existing recovery
boundary; balance and month-card tests do not establish that those legacy
subscription entitlements are restored in every case.
