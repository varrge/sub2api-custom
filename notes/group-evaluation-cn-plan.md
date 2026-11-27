# Group evaluation: China acceptance build

Scope approved 2026-11-26: add group-level candy and pelican evaluations to Channel Status; configure per-group tests, model, effort, attempts per cycle and interval seconds; user views independently show the latest 10 records of each kind. Deploy a first version to China staging only. No production update or release tag.

## Implementation contract

- Real implementation starts from v0.2.25-custom.8 on `codex/group-evaluation-cn`; prototype remains separate.
- Administrator configuration lives in the existing Channel Monitor admin page. Settings and manual run require administrator authentication, step-up for mutations, existing audit logging, and reject restricted admin keys unless explicitly supported.
- User APIs enforce authenticated available-group scope and Channel Monitor feature gate. No account identities, upstream URLs, credentials or raw errors are public.
- Backend persists configurations and results in PostgreSQL; defaults disabled. Execution is asynchronous, globally bounded and one run per group, using a database claim so instances cannot double-run.
- Scheduler interval is integer seconds (10–86400), defaults 600. Tests do not auto-start on page view. Global concurrency 2; each group has a daily request cap default 100. No retries for ambiguous model requests.
- Each candy response is one history item; each pelican response one history item. Latest ten per group/type, descending by timestamp and id; fewer records show actual count. Successful and failed attempts both visible.
- Safe HTML/SVG preview must not access parent storage, scripts, network or credentials; renderer uses sandbox without same-origin, restrictive CSP. No HTML injected directly into the application DOM.
- Detection is diagnostic only: test answers do not change priority, models, account enable/disable or user charges. Real 401/429 transport errors continue to use the gateway's existing authentication/rate-limit protection.
- Candidate deployment preserves existing China DB/Redis/proxy and network isolation; scoped mock upstream may be used for deterministic acceptance without real provider charges.

## API

Root `/api/v1`; existing response envelope `{code:0,data:...}`.

- GET `/admin/group-evaluations` -> `{items: Config[]}` (all active, non-deleted groups with saved config or disabled defaults).
- PUT `/admin/group-evaluations/:group_id` with Config editable fields + `revision` -> Config.
- POST `/admin/group-evaluations/:group_id/run` -> HTTP 202 `{accepted:true}`; disabled/busy/cap failures are 4xx.
- GET `/group-evaluations` -> `{items: PublicGroup[]}` enabled groups within authenticated viewer scope.
- GET `/group-evaluations/:group_id` -> `{group: PublicGroup, records:{candy: Record[],pelican: Record[]}}`; also admin GET `/admin/group-evaluations/:group_id`.

Config: `group_id`, `group_name`, `revision`, `enabled`, `candy_enabled`, `pelican_enabled`, `automatic`, `interval_seconds`, `model`, `effort` (`low|medium|high`), `runs` (1–10 candy calls per cycle), `daily_limit` (1–1000), `last_run_at`, `next_run_at`, `running`.
PublicGroup: group_id/group_name/enabled/candy_enabled/pelican_enabled/automatic/interval_seconds/model/effort/last_run_at/next_run_at/running.
Record: `id`, `kind` (`candy|pelican`), `status` (`passed|mismatch|generated|error`), `model`, `effort`, `created_at`, `answer`, `html`, `error` (bounded generic error), `duration_ms`. `html` bounded 64 KiB; results otherwise bounded. Only records from currently enabled test kinds are returned publicly.

## Work

1. Frontend components + monitor integration, translations and focused tests.
2. Backend store, scheduler, transport adapter, handlers/routes/wiring and focused tests.
3. Independent review; build and test; record any implementation assumptions.
4. China backup, fixed candidate deployment, authenticated acceptance, tunnel and handover. Do not enable real recurring work without checking configured source and budget.

## Rulings

- This first version reuses stored group accounts server-side; user views never trigger calls. Account selection and provider support must be explicitly reported after transport investigation; unsupported providers fail closed.
- The public result labels describe diagnostic sampling rather than a guarantee that every group account was covered.
- Group cap counts outbound requests, including errors; this version does not claim a precise currency budget across heterogeneous upstream pricing.
- Candy wording allows selecting by shape: 9 round + 12 star guarantees a pair, minimum 21. Clarify that choice explicitly in the prompt, use final-answer parsing, expose/persist `expected_answer:"21"` and `prompt_version:"candy-shape-selection-v1"`. Exhaustive shape-count enumeration validates the reference. Blind draws without shape selection would require 29 and are a different task. Pelican uses `prompt_version:"pelican-static-v1"`.
- First version executes only OpenAI-platform groups with supported OAuth/API-key accounts; all other platforms are visible as unsupported in admin and cannot be enabled. This is administrator-funded diagnostic sampling, not full end-user billing/entitlement simulation.
