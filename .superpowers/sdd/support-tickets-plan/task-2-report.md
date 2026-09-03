# Task 2 report: backend API, feature gate, and private images

## Implementation

- Added the complete `support_ticket_enabled` setting pipeline with a default of `false`: persisted/default parsing, runtime fail-closed lookup, admin GET/PUT response and update merge, settings audit diff, public settings DTO, and SSR injection payload.
- Added `SupportTicketService` validation/orchestration for user/admin create, reply, list, detail, explicit read, unread count, attachment, status, and priority operations. User actor IDs always come from the authenticated context at the handler boundary.
- Added the required user routes under `/api/v1/tickets` and ungated admin routes under `/api/v1/admin/tickets`, with `unread-count` registered before `:id` routes.
- Added bounded 16 MiB multipart parsing, at most 3 repeated `images`, 5 MiB raw-image limits, actual JPEG/PNG/WebP decode-config and full-decode validation, a 25,000,000-pixel ceiling, metadata-stripping re-encoding, and a 10 MiB normalized-output limit. JPEG is stored at quality 90; PNG/WebP are stored as PNG.
- Added private attachment delivery using the persisted normalized content type and bytes, `X-Content-Type-Options: nosniff`, and `Cache-Control: private, no-store`.
- Added ticket/list/detail/message/attachment DTOs with safe user identity enrichment. User responses omit email; admin responses include username/email when available.
- Added `FEATURE_DISABLED` and focused ticket/image validation errors using the existing application error/response envelope.
- Added create/reply route templates to the central audit body-omission list. Audit metadata remains available while multipart content, filenames, and bytes are never captured by the audit middleware.
- Registered repository, service, and handlers through Wire and regenerated `backend/cmd/server/wire_gen.go`.

## Task 1 integration correction

Task 1's repository contract documented and implemented `OpenForUser`/`OpenForAdmin` as read-position mutations. The binding API requires detail fetches not to mutate and exposes explicit `/read` actions. The minimal correction retains the existing method names for interface compatibility but changes them to read-only detail fetches, adds explicit `MarkReadForUser`/`MarkReadForAdmin`, and updates the existing integration tests accordingly. Ownership remains enforced in the repository predicate, so another user's detail/read/attachment returns the normal support-ticket not-found error.

## Tests

- `go test ./internal/service ./internal/handler ./internal/handler/admin ./internal/server/routes ./internal/server/middleware -run 'SupportTicket|NormalizeSupportTicketImage' -count=1` — PASS (all five packages).
- `go test -tags=unit ./internal/service ./internal/handler ./internal/handler/dto -run 'SupportTicket|PublicSettingsInjectionPayload_SchemaDoesNotDrift' -count=1` — PASS (all three packages).
- `go test -tags=integration ./internal/repository -run '^TestSupportTicketRepository' -count=1` — PASS.
- `go test ./... -run '^$'` — PASS; all backend packages compiled.
- `git diff --check` — PASS.

Focused coverage includes text normalization/rune validation delegation, context-owned actor identity, closed-reply propagation and status literals, actual-format spoof resistance, corrupt/truncated images, WebP-to-PNG conversion, metadata removal, raw/body/image-count/pixel/normalized-output limits, private attachment headers, cross-owner 404 semantics, explicit read behavior, fail-closed runtime settings, user 403 versus admin availability, exact route registration, public/SSR setting exposure, admin update/audit contracts, and audit omission on rejected multipart requests.

## Files

- Domain/service: `backend/internal/domain/support_ticket.go`, `backend/internal/service/support_ticket.go`, `backend/internal/service/support_ticket_image.go`, setting split files and focused tests.
- Repository/DI: `backend/internal/repository/support_ticket_repo.go`, repository integration tests, repository/service/handler Wire sets, generated `backend/cmd/server/wire_gen.go`.
- HTTP/API: `backend/internal/handler/support_ticket_handler.go`, `backend/internal/handler/dto/support_ticket.go`, user/admin route registration and focused tests.
- Privacy/audit: `backend/internal/server/middleware/audit_log.go` and focused middleware tests.
- No frontend files were changed.

## Self-review

- Verified every user ticket route is behind one fail-closed group middleware and no admin ticket route uses that gate.
- Verified route declaration order keeps both static `unread-count` paths ahead of `:id`.
- Verified handlers never accept user/admin IDs from request fields and do not log content, filenames, or image bytes.
- Verified attachment DTOs never expose stored bytes or original filenames, and attachment responses use only persisted normalized metadata/data.
- Verified ownership constraints are applied before user detail/read/attachment access and map to the same not-found response.
- Verified status transition and pending-to-in-progress behavior remain atomic in the Task 1 repository transaction.
- Verified no new Go dependency or frontend change was introduced.

## Concerns

None. The legacy `OpenForUser`/`OpenForAdmin` names now mean read-only detail fetch for compatibility; explicit read-state mutation is solely through the new `MarkRead...` methods and HTTP `/read` actions.
