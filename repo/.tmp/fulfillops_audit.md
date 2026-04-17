# FulfillOps Rewards & Compliance Console Static Audit

## 1. Verdict
- Overall conclusion: **Partial Pass**

The repository is materially aligned to the prompt and has a real full-stack Go/Gin/Templ/PostgreSQL structure, but it has multiple material defects in compliance, recovery, and operational safety. The most serious issues are broken soft-delete cleanup against FK-linked data, use of soft-deleted entities in new fulfillments, unmasked shipping-address display in authenticated UI flows, and health checks that report success without performing the underlying checks.

## 2. Scope and Static Verification Boundary
- What was reviewed: repository structure, docs, config, migrations, route registration, middleware, handlers, services, repositories, views, and test files under `tests/` and `internal/**/*_test.go`.
- What was not reviewed: runtime behavior in a live server, Docker startup, browser rendering, background job execution against a running app, PostgreSQL restore execution, checksum generation against real files, and actual scheduled execution timing.
- What was intentionally not executed: project startup, Docker, tests, external services, browser flows.
- Claims requiring manual verification: actual page rendering quality in a browser, actual backup/restore behavior using `pg_dump`/`psql`, actual scheduler execution timing, actual export file generation, and any behavior dependent on runtime environment or container wiring.

## 3. Repository / Requirement Mapping Summary
- Prompt core goal mapped: offline reward-fulfillment console with tier management, inventory, purchase limits, fulfillment lifecycle, messaging/send logs, exception threads, audit trail, exports, health/admin operations, backups, and recovery.
- Main implementation areas reviewed: `cmd/server/main.go`, `internal/handler/router.go`, `internal/service/*`, `internal/repository/*`, `migrations/*.sql`, `internal/view/*.templ`, `internal/middleware/*`, and tests under `tests/` plus internal package tests.
- Major constraints checked: role-based access, optimistic locking, atomic transaction boundaries, AES-256 encryption-at-rest path, soft delete + 30-day restore, masked sensitive display, scheduled jobs, export permissions, and admin health/backup features.

## 4. Section-by-section Review

### 1. Hard Gates

#### 1.1 Documentation and static verifiability
- Conclusion: **Partial Pass**
- Rationale: The repository includes a README, env example, Dockerfile, compose files, migrations, and clear entry points. The docs are statically consistent with the main executable and environment structure. However, the documented test path is Docker-dependent and the tests themselves replace the real `/healthz` implementation with simplified stubs, which weakens static confidence in some operational claims.
- Evidence: `README.md:9`, `README.md:52`, `.env.example:1`, `cmd/server/main.go:28`, `docker/Dockerfile:1`, `docker/docker-compose.yml:1`, `tests/API_tests/setup_test.go:110`, `tests/e2e_tests/setup_test.go:109`
- Manual verification note: real startup, health endpoint behavior, and job execution still require manual verification.

#### 1.2 Material deviation from the prompt
- Conclusion: **Partial Pass**
- Rationale: The codebase is centered on the stated business scenario and implements the major domains. There are still material deviations: the dashboard does not compute “today’s pending fulfillments,” the admin health screen reports hardcoded success for several checks, and masked-display requirements are violated on fulfillment detail pages.
- Evidence: `internal/handler/page_dashboard.go:40`, `internal/handler/admin.go:42`, `internal/handler/page_admin.go:60`, `internal/handler/page_fulfillment.go:167`, `internal/view/fulfillments/detail.templ:121`

### 2. Delivery Completeness

#### 2.1 Core explicit requirements coverage
- Conclusion: **Partial Pass**
- Rationale: Core flows exist for tiers, customers, fulfillments, exceptions, templates, send logs, reports, audit, jobs, backups, and restore. Coverage is incomplete or defective for masked sensitive display, cleanup/recovery integrity, reliable health checks, and strict exclusion of soft-deleted entities from new fulfillments.
- Evidence: `internal/handler/router.go:52`, `internal/service/fulfillment.go:137`, `internal/service/export.go:64`, `internal/service/backup.go:52`, `internal/job/cleanup_job.go:11`, `internal/handler/page_fulfillment.go:167`, `internal/repository/tier.go:161`

#### 2.2 Basic end-to-end deliverable vs partial/demo
- Conclusion: **Pass**
- Rationale: This is a structured application rather than a demo snippet. It has configuration, migrations, handlers, services, repositories, views, and multiple test suites.
- Evidence: `cmd/server/main.go:28`, `internal/handler/router.go:52`, `migrations/001_init.up.sql:1`, `tests/API_tests/setup_test.go:31`, `tests/e2e_tests/setup_test.go:31`

### 3. Engineering and Architecture Quality

#### 3.1 Structure and module decomposition
- Conclusion: **Pass**
- Rationale: The repository is decomposed into config, middleware, handlers, services, repositories, jobs, views, and migrations. Responsibilities are mostly separated cleanly.
- Evidence: `internal/handler/router.go:52`, `internal/service/fulfillment.go:91`, `internal/repository/tx.go:12`, `internal/job/scheduler.go:27`

#### 3.2 Maintainability and extensibility
- Conclusion: **Partial Pass**
- Rationale: The layering is maintainable, but several cross-cutting requirements are only partially enforced. Examples include health checks implemented as hardcoded values, soft-delete cleanup that ignores FK topology, and service comments that claim stricter enforcement than the code actually performs.
- Evidence: `internal/handler/admin.go:42`, `internal/handler/page_admin.go:60`, `internal/job/cleanup_job.go:27`, `internal/service/fulfillment.go:264`

### 4. Engineering Details and Professionalism

#### 4.1 Error handling, logging, validation, API design
- Conclusion: **Partial Pass**
- Rationale: The code has consistent JSON error handling, request logging, optimistic locking, and validation for several lifecycle rules. Material problems remain: health endpoints misreport status, cleanup can fail on relational data, and physical fulfillments can be marked `READY_TO_SHIP` without a shipping address despite service comments claiming the opposite.
- Evidence: `internal/middleware/errors.go:19`, `internal/middleware/logger.go:10`, `internal/service/fulfillment.go:224`, `internal/service/fulfillment.go:262`, `internal/handler/admin.go:42`, `internal/job/cleanup_job.go:27`
- Manual verification note: actual operational behavior of backup/restore and scheduler still requires manual verification.

#### 4.2 Real product/service vs demo level
- Conclusion: **Pass**
- Rationale: The overall shape is production-oriented rather than tutorial-level. It includes persistence, migrations, RBAC, jobs, audit, exports, and server-rendered UI.
- Evidence: `cmd/server/main.go:54`, `internal/handler/router.go:182`, `migrations/003_audit_immutability.up.sql:1`

### 5. Prompt Understanding and Requirement Fit

#### 5.1 Business goal, scenario, and constraint fit
- Conclusion: **Partial Pass**
- Rationale: The code reflects the offline fulfillment/compliance scenario, but several prompt constraints are not met reliably: masked display is violated on fulfillment detail pages, the dashboard does not implement “today’s pending fulfillments,” health checks are partly non-functional, and soft-deleted operational records are not handled safely.
- Evidence: `internal/handler/page_dashboard.go:40`, `internal/handler/page_fulfillment.go:167`, `internal/view/fulfillments/detail.templ:121`, `internal/handler/admin.go:42`, `internal/repository/tier.go:161`, `internal/service/fulfillment.go:148`

### 6. Aesthetics

#### 6.1 Visual and interaction quality
- Conclusion: **Cannot Confirm Statistically**
- Rationale: The repository includes a coherent Templ/CSS UI and status badges/modals, but static source inspection cannot prove actual browser rendering, responsiveness, hover states, or layout correctness.
- Evidence: `internal/view/layout.templ:1`, `internal/view/dashboard.templ:16`, `internal/view/fulfillments/detail.templ:30`, `static/css/app.css:1`
- Manual verification note: requires browser-based inspection.

## 5. Issues / Suggestions (Severity-Rated)

### High

#### 1. Scheduled soft-delete cleanup is likely broken against relational data
- Severity: **High**
- Conclusion: **Fail**
- Evidence: `internal/job/cleanup_job.go:27`, `migrations/001_init.up.sql:49`, `migrations/001_init.up.sql:50`, `migrations/001_init.up.sql:76`, `migrations/001_init.up.sql:85`, `migrations/001_init.up.sql:98`, `migrations/001_init.up.sql:110`, `migrations/001_init.up.sql:149`, `migrations/001_init.up.sql:150`
- Impact: The 30-day recovery-window cleanup job deletes parent tables directly and in an unsafe order, while the schema uses normal foreign keys without `ON DELETE CASCADE`. In realistic data, purge runs can fail and stop retention enforcement.
- Minimum actionable fix: Redesign purge order and dependency handling, or add explicit archival/cascade strategy for all dependent tables before hard delete.

#### 2. New fulfillments can be created against soft-deleted tiers and customers
- Severity: **High**
- Conclusion: **Fail**
- Evidence: `internal/service/fulfillment.go:148`, `internal/repository/tier.go:161`, `internal/repository/customer.go:109`, `migrations/001_init.up.sql:27`, `migrations/001_init.up.sql:42`
- Impact: Soft-deleted operational entities remain usable for new business records. Deleted tiers can still be loaded by `GetByIDForUpdate`, and customers are never validated at creation time, so soft-deleted customers remain FK-valid. This breaks recovery semantics and data integrity.
- Minimum actionable fix: Ensure create-time validation rejects soft-deleted tiers/customers and make locked tier lookup respect `deleted_at IS NULL`.

#### 3. Fulfillment detail page decrypts and displays full shipping addresses to any authenticated role
- Severity: **High**
- Conclusion: **Fail**
- Evidence: `internal/handler/router.go:84`, `internal/handler/router.go:114`, `internal/handler/page_fulfillment.go:167`, `internal/view/fulfillments/detail.templ:121`
- Impact: The prompt requires masked display of sensitive fields and names auditors as review/export users, but the fulfillment detail UI decrypts and renders full address lines and postal details on routes available to any authenticated user, including auditors.
- Minimum actionable fix: Render masked address values in detail views by default and require an explicit privileged path for full-address maintenance.

#### 4. Admin health endpoints report success for checks they do not perform
- Severity: **High**
- Conclusion: **Fail**
- Evidence: `internal/handler/admin.go:42`, `internal/handler/admin.go:45`, `internal/handler/page_admin.go:60`, `internal/handler/page_admin.go:63`
- Impact: The admin health screen is supposed to monitor local jobs and operational readiness, but encryption, directories, and scheduler are hardcoded as healthy. Operators can be misled during incident response or acceptance review.
- Minimum actionable fix: Replace hardcoded `"ok"` values with real filesystem, key-file, scheduler-state, and configured-directory checks.

### Medium

#### 5. Deleted fulfillments are not discoverable from the recovery UI
- Severity: **Medium**
- Conclusion: **Fail**
- Evidence: `internal/handler/page_admin.go:137`, `internal/repository/fulfillment.go:48`, `internal/repository/fulfillment.go:51`
- Impact: The admin recovery page tries to surface deleted fulfillments, but it calls a repository list function that always filters `deleted_at IS NULL`. Deleted fulfillments therefore cannot appear for restore through the advertised UI.
- Minimum actionable fix: Add an include-deleted fulfillment query path and use it in the recovery page.

#### 6. Dashboard “pending fulfillments” count does not implement the prompt’s “today’s pending fulfillments”
- Severity: **Medium**
- Conclusion: **Fail**
- Evidence: `internal/handler/page_dashboard.go:40`, `internal/handler/page_dashboard.go:43`, `internal/handler/page_dashboard.go:59`
- Impact: The dashboard metric counts all `DRAFT` and `READY_TO_SHIP` fulfillments, not pending items for today. This is a prompt mismatch and can materially mislead operations staff.
- Minimum actionable fix: Apply a same-day date filter or define and implement the intended “today” semantics explicitly.

#### 7. Physical fulfillments can reach `READY_TO_SHIP` without any shipping address
- Severity: **Medium**
- Conclusion: **Fail**
- Evidence: `internal/service/fulfillment.go:262`, `internal/service/fulfillment.go:265`, `internal/view/fulfillments/detail.templ:246`
- Impact: The service comment says physical fulfillments require a shipping address at `READY_TO_SHIP`, but the code only validates an address if one was provided. This allows incomplete shipment records.
- Minimum actionable fix: Enforce address presence for physical `READY_TO_SHIP` transitions and fail when it is absent.

#### 8. Retry implementation for failed sends is incomplete and internally inconsistent
- Severity: **Medium**
- Conclusion: **Partial Fail**
- Evidence: `internal/service/messaging.go:56`, `internal/service/messaging.go:79`, `internal/service/messaging.go:93`, `internal/repository/send_log.go:137`, `internal/job/notify_job.go:10`
- Impact: Dispatch creates `QUEUED` offline send logs, while retry scanning only picks up `FAILED` rows. There is no production code path that transitions queued offline send logs into failed/retryable states, so the “retry failed sends up to 3 times over 30 minutes” requirement is only partially represented.
- Minimum actionable fix: Define the failed-send lifecycle explicitly and make retry scanning, state transitions, and job comments consistent with it.

## 6. Security Review Summary

### Authentication entry points
- Conclusion: **Pass**
- Evidence: `internal/handler/router.go:184`, `internal/handler/auth.go:30`, `internal/middleware/auth.go:21`
- Reasoning: API login establishes session cookies and protected routes require session auth. Page auth uses a separate cookie path for server-rendered routes.

### Route-level authorization
- Conclusion: **Partial Pass**
- Evidence: `internal/handler/router.go:193`, `internal/handler/router.go:194`, `internal/handler/router.go:195`, `internal/middleware/auth.go:64`
- Reasoning: RBAC is applied consistently at route-group level for admin, specialist, and auditor routes. However, many operational read routes are available to any authenticated user, and the fulfillment detail page exposes decrypted address data under those broader read permissions.

### Object-level authorization
- Conclusion: **Partial Pass**
- Evidence: `internal/handler/report.go:143`, `internal/handler/report.go:171`, `internal/repository/notification.go:86`, `internal/handler/page_report.go:121`
- Reasoning: Sensitive export access is checked per-record, and notifications are marked read only for the owning user. Object-level controls are not consistently applied across other sensitive operational records.

### Function-level authorization
- Conclusion: **Pass**
- Evidence: `internal/handler/router.go:200`, `internal/handler/router.go:218`, `internal/handler/router.go:268`, `internal/handler/router.go:274`
- Reasoning: Mutating admin functions are behind admin-only route groups; specialist mutation scope is narrower.

### Tenant / user data isolation
- Conclusion: **Not Applicable**
- Evidence: `migrations/001_init.up.sql:5`, `migrations/001_init.up.sql:18`, `migrations/001_init.up.sql:47`
- Reasoning: The prompt does not describe a multi-tenant model. The repository is implemented as a single-organization internal console.

### Admin / internal / debug protection
- Conclusion: **Pass**
- Evidence: `internal/handler/router.go:266`, `internal/handler/router.go:268`, `internal/handler/router.go:270`
- Reasoning: Admin health and job-trigger endpoints are behind admin-only middleware. No separate debug endpoints were found.

## 7. Tests and Logging Review

### Unit tests
- Conclusion: **Partial Pass**
- Evidence: `internal/service/fulfillment_test.go:69`, `internal/job/cleanup_job_test.go:11`, `internal/middleware/middleware_test.go:129`, `tests/unit_tests/domain_test.go:1`
- Reasoning: Unit and package tests exist for services, jobs, middleware, masking, encryption, and domain logic. Coverage is weak around the strongest defects found in this audit.

### API / integration tests
- Conclusion: **Partial Pass**
- Evidence: `tests/API_tests/setup_test.go:31`, `tests/API_tests/fulfillments_api_test.go:55`, `tests/API_tests/reports_api_test.go:125`, `tests/e2e_tests/rbac_e2e_test.go:38`, `tests/integration/integration_test.go:700`
- Reasoning: API/integration coverage exists for auth, lifecycle flows, RBAC, exports, and optimistic locking. It is largely happy-path and route-smoke oriented and does not catch the recovery, masking, cleanup-topology, or real-health-check issues.

### Logging categories / observability
- Conclusion: **Partial Pass**
- Evidence: `internal/middleware/logger.go:10`, `internal/job/scheduler.go:137`, `internal/service/backup.go:100`
- Reasoning: Request logging and job logging exist, and job run history persists status/error stacks. The admin health implementation undermines observability by reporting unverified statuses.

### Sensitive-data leakage risk in logs / responses
- Conclusion: **Partial Pass**
- Evidence: `internal/middleware/logger.go:22`, `internal/handler/customer.go:49`, `internal/handler/fulfillment.go:65`, `internal/handler/page_fulfillment.go:167`
- Reasoning: Request logs do not log bodies, which is positive. API responses mask customer/voucher data. The fulfillment page path still decrypts and displays full shipping addresses, creating a sensitive-display exposure.

## 8. Test Coverage Assessment (Static Audit)

### 8.1 Test Overview
- Unit tests and package-level tests exist under `internal/**/*_test.go` and `tests/unit_tests/`.
- API/integration tests exist under `tests/API_tests/`, `tests/e2e_tests/`, and `tests/integration/`.
- Test entry points are `run_tests.sh` and direct `go test` package execution.
- Documentation provides test commands, but they are Docker-dependent and outside this audit boundary.
- Evidence: `run_tests.sh:1`, `README.md:52`, `tests/API_tests/setup_test.go:31`, `tests/e2e_tests/setup_test.go:31`, `tests/integration/integration_test.go:36`

### 8.2 Coverage Mapping Table

| Requirement / Risk Point | Mapped Test Case(s) | Key Assertion / Fixture / Mock | Coverage Assessment | Gap | Minimum Test Addition |
| --- | --- | --- | --- | --- | --- |
| Authenticated API requires login | `tests/API_tests/auth_api_test.go:61`, `tests/e2e_tests/rbac_e2e_test.go:102` | 401 assertions on protected endpoints at `tests/API_tests/auth_api_test.go:62`, `tests/e2e_tests/rbac_e2e_test.go:110` | basically covered | Real main health route is stubbed in tests | Add tests against the real `/healthz` implementation rather than the stubbed route |
| RBAC for admin/specialist/auditor routes | `tests/e2e_tests/rbac_e2e_test.go:38`, `tests/API_tests/reports_api_test.go:125`, `tests/API_tests/audit_api_test.go:32` | 403 assertions at `tests/e2e_tests/rbac_e2e_test.go:59`, `tests/e2e_tests/rbac_e2e_test.go:63`, `tests/API_tests/reports_api_test.go:132` | basically covered | No tests for over-broad read access to decrypted fulfillment detail pages | Add page-level tests for auditor access to masked vs full sensitive fields |
| Fulfillment lifecycle + validations | `tests/API_tests/fulfillments_api_test.go:121`, `internal/service/fulfillment_test.go:69` | invalid transition and tracking assertions at `tests/API_tests/fulfillments_api_test.go:145`, `internal/service/fulfillment_test.go:185` | sufficient | No test requiring shipping address for physical `READY_TO_SHIP` | Add failing test for physical ready transition without address |
| Optimistic locking | `internal/service/fulfillment_test.go:340`, `tests/integration/integration_test.go:700`, `tests/API_tests/tiers_api_test.go:94` | conflict assertions at `internal/service/fulfillment_test.go:369`, `tests/integration/integration_test.go:727` | sufficient | No test for stale fulfillment transition via HTTP | Add API test for stale fulfillment `version` during transition |
| Purchase limit and inventory | `internal/service/fulfillment_test.go:253`, `tests/e2e_tests/inventory_e2e_test.go:90` | limit/inventory assertions at `internal/service/fulfillment_test.go:279`, `internal/service/fulfillment_test.go:247` | sufficient | No test for deleted tier/customer misuse | Add tests rejecting fulfillment create against soft-deleted tier/customer |
| Masked sensitive display | `tests/unit_tests/maskpii_test.go:1`, `tests/unit_tests/encryption_test.go:1` | utility/encryption coverage only | insufficient | No API/page test validates masked output vs full sensitive display | Add page/API assertions for masked customer/address output and forbidden full-address display |
| Soft delete + 30-day recovery | `tests/API_tests/customers_api_test.go:89`, `tests/API_tests/tiers_api_test.go:114`, `tests/integration/integration_test.go:466` | restore happy-path assertions | basically covered | No coverage for deleted fulfillments in recovery UI or cleanup with FK-linked records | Add tests for fulfillment recovery listing and cleanup against relational fixtures |
| Export permission + checksum | `tests/API_tests/reports_api_test.go:51`, `tests/API_tests/reports_api_test.go:125`, `tests/integration/integration_test.go:659` | checksum and admin-only sensitive export assertions at `tests/API_tests/reports_api_test.go:73`, `tests/API_tests/reports_api_test.go:145` | basically covered | No coverage for download auditing or page-level role-based masking around exports | Add tests around page download path and audit record creation on export actions |
| Health/admin observability | `tests/API_tests/auth_api_test.go:11` | only stubbed `/healthz` status check | missing | Tests replace real `/healthz` and do not exercise admin health checks | Add tests for actual DB ping/error handling and real admin health component checks |
| Retry failed sends | `internal/service/coverage_extra_test.go:98` | stubbed retry repository at `internal/service/coverage_extra_test.go:74` | insufficient | No end-to-end test of failed-send lifecycle; production path to FAILED is untested and unclear | Add integration tests covering QUEUED -> FAILED -> retry progression |

### 8.3 Security Coverage Audit
- Authentication: **basically covered**. Tests check successful login and 401s, but they do not exercise the real main health route or full cookie-option behavior. Evidence: `tests/API_tests/auth_api_test.go:20`, `tests/API_tests/auth_api_test.go:61`.
- Route authorization: **basically covered**. Specialist/auditor/admin route distinctions are tested. Evidence: `tests/e2e_tests/rbac_e2e_test.go:38`, `tests/e2e_tests/rbac_e2e_test.go:67`.
- Object-level authorization: **insufficient**. Sensitive export per-record authorization is tested, but not broader sensitive-object exposure such as decrypted fulfillment addresses. Evidence: `tests/API_tests/reports_api_test.go:135`.
- Tenant / data isolation: **not applicable** for multi-tenancy, but user-bound notification ownership is not directly tested beyond repo behavior.
- Admin / internal protection: **basically covered** at route level. Evidence: `tests/e2e_tests/rbac_e2e_test.go:63`.

Severe defects could still remain undetected because the tests do not verify masking on authenticated pages, do not exercise cleanup against realistic FK-linked fixtures, and stub out the real `/healthz` implementation.

### 8.4 Final Coverage Judgment
- **Partial Pass**

The test suite covers many happy paths, RBAC basics, optimistic locking, and core lifecycle rules. It does not cover several high-risk defects found in this audit: soft-delete cleanup on relational data, creating fulfillments against soft-deleted entities, real health-check behavior, and masked-vs-full sensitive display on fulfillment pages. Because of those gaps, the tests could still pass while serious compliance and operational defects remain.

## 9. Final Notes
- The repository is substantially more than a demo and is close to an acceptable delivery shape.
- The remaining defects are not cosmetic; they affect compliance, recovery, operational trust, and data-handling guarantees stated in the prompt.
- Manual verification is still required for runtime-only claims, especially backup/restore, export generation, scheduler execution, and browser rendering.
