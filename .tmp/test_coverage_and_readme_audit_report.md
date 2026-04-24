# Test Coverage Audit

## Scope and Method
- Mode: static inspection only (no code/test execution).
- Evidence sources: `repo/internal/handler/router.go`, `repo/API_tests/*.go`, `repo/integration/integration_test.go`, `repo/e2e_tests/*.go`, `repo/internal/**/*_test.go`, `repo/static/js/*`, `repo/run_tests.sh`, `repo/README.md`.
- Project type declaration: `fullstack` (`repo/README.md:1`).

## Backend Endpoint Inventory

### API and health endpoints (coverage-calculated set)
1. `GET /healthz`
2. `POST /api/v1/auth/login`
3. `POST /api/v1/auth/logout`
4. `GET /api/v1/auth/me`
5. `GET /api/v1/tiers`
6. `POST /api/v1/tiers`
7. `GET /api/v1/tiers/:id`
8. `PUT /api/v1/tiers/:id`
9. `DELETE /api/v1/tiers/:id`
10. `POST /api/v1/tiers/:id/restore`
11. `GET /api/v1/customers`
12. `POST /api/v1/customers`
13. `GET /api/v1/customers/:id`
14. `PUT /api/v1/customers/:id`
15. `DELETE /api/v1/customers/:id`
16. `POST /api/v1/customers/:id/restore`
17. `GET /api/v1/fulfillments`
18. `POST /api/v1/fulfillments`
19. `GET /api/v1/fulfillments/:id`
20. `POST /api/v1/fulfillments/:id/transition`
21. `PUT /api/v1/fulfillments/:id/shipping-address`
22. `GET /api/v1/fulfillments/:id/timeline`
23. `DELETE /api/v1/fulfillments/:id`
24. `POST /api/v1/fulfillments/:id/restore`
25. `GET /api/v1/exceptions`
26. `POST /api/v1/exceptions`
27. `GET /api/v1/exceptions/:id`
28. `PUT /api/v1/exceptions/:id/status`
29. `POST /api/v1/exceptions/:id/events`
30. `GET /api/v1/message-templates`
31. `POST /api/v1/message-templates`
32. `GET /api/v1/message-templates/:id`
33. `PUT /api/v1/message-templates/:id`
34. `DELETE /api/v1/message-templates/:id`
35. `GET /api/v1/send-logs`
36. `PUT /api/v1/send-logs/:id/printed`
37. `PUT /api/v1/send-logs/:id/failed`
38. `GET /api/v1/notifications`
39. `PUT /api/v1/notifications/:id/read`
40. `POST /api/v1/dispatch`
41. `GET /api/v1/reports/exports`
42. `POST /api/v1/reports/exports`
43. `GET /api/v1/reports/exports/:id`
44. `POST /api/v1/reports/exports/:id/verify-checksum`
45. `DELETE /api/v1/reports/exports/:id`
46. `GET /api/v1/settings`
47. `PUT /api/v1/settings/:key`
48. `GET /api/v1/settings/blackout-dates`
49. `POST /api/v1/settings/blackout-dates`
50. `DELETE /api/v1/settings/blackout-dates/:id`
51. `GET /api/v1/audit`
52. `GET /api/v1/admin/health`
53. `GET /api/v1/admin/jobs/runs`
54. `POST /api/v1/admin/jobs/:name/run`
55. `GET /api/v1/admin/job-schedules`
56. `PUT /api/v1/admin/job-schedules/:key`
57. `GET /api/v1/admin/dr-drills`
58. `POST /api/v1/admin/dr-drills`
59. `PUT /api/v1/admin/dr-drills/:id`
60. `GET /api/v1/admin/users`
61. `POST /api/v1/admin/users`
62. `GET /api/v1/admin/users/:id`
63. `PUT /api/v1/admin/users/:id`
64. `DELETE /api/v1/admin/users/:id`

Evidence for route inventory: `repo/internal/handler/router.go:224-334` and health route setup in `repo/API_tests/setup_test.go:120-123`, `repo/integration/integration_test.go:124-128`, `repo/e2e_tests/setup_test.go:115-118`.

### Non-API page routes observed (not included in API coverage %)
- Page routes are also extensive in `repo/internal/handler/router.go:110-222` (e.g., `/auth/login`, `/tiers`, `/reports`, `/admin/...`).
- These are verified by page-oriented tests in `repo/API_tests/pages_api_test.go::TestPageRoutesSmoke` and `repo/e2e_tests/pages_e2e_test.go::TestPageRoutesRender`.

## API Test Mapping Table
| Endpoint | Covered | Test type | Test files | Evidence (function reference) |
|---|---|---|---|---|
| `GET /healthz` | yes | true no-mock HTTP | `API_tests/auth_api_test.go`, `integration/integration_test.go` | `TestHealthz` |
| `POST /api/v1/auth/login` | yes | true no-mock HTTP | `API_tests/auth_api_test.go` | `TestAuthLogin_Success`, `TestAuthLogin_WrongPassword`, `TestAuthLogin_MissingBody` |
| `POST /api/v1/auth/logout` | yes | true no-mock HTTP | `API_tests/auth_api_test.go`, `API_tests/audit_fixes_api_test.go` | `TestAuthLogout`, `TestLogout_ClearsBothCookies` |
| `GET /api/v1/auth/me` | yes | true no-mock HTTP | `API_tests/auth_api_test.go` | `TestAuthMe_Authenticated`, `TestAuthMe_Unauthenticated` |
| `GET /api/v1/tiers` | yes | true no-mock HTTP | `API_tests/tiers_api_test.go`, `integration/integration_test.go`, `e2e_tests/rbac_e2e_test.go` | `TestTiersList`, `TestRBACAccessControl` |
| `POST /api/v1/tiers` | yes | true no-mock HTTP | `API_tests/tiers_api_test.go`, `integration/integration_test.go`, `e2e_tests/lifecycle_e2e_test.go` | `TestTiersCreate`, `TestFulfillmentLifecycle` |
| `GET /api/v1/tiers/:id` | yes | true no-mock HTTP | `API_tests/tiers_api_test.go`, `integration/integration_test.go` | `TestTiersGet`, `TestSoftDeleteRestore` |
| `PUT /api/v1/tiers/:id` | yes | true no-mock HTTP | `API_tests/tiers_api_test.go`, `integration/integration_test.go`, `e2e_tests/inventory_e2e_test.go` | `TestTiersUpdate`, `TestConflictOnStaleVersion` |
| `DELETE /api/v1/tiers/:id` | yes | true no-mock HTTP | `API_tests/tiers_api_test.go`, `integration/integration_test.go`, `e2e_tests/lifecycle_e2e_test.go` | `TestTiersSoftDeleteAndRestore`, `TestSoftDeleteRestore` |
| `POST /api/v1/tiers/:id/restore` | yes | true no-mock HTTP | `API_tests/tiers_api_test.go`, `integration/integration_test.go`, `e2e_tests/lifecycle_e2e_test.go` | `TestTiersSoftDeleteAndRestore`, `TestSoftDeleteRestore` |
| `GET /api/v1/customers` | yes | true no-mock HTTP | `API_tests/customers_api_test.go` | `TestCustomersList` |
| `POST /api/v1/customers` | yes | true no-mock HTTP | `API_tests/customers_api_test.go`, `integration/integration_test.go`, `e2e_tests/*` | `TestCustomersCreate`, `TestFulfillmentLifecycle` |
| `GET /api/v1/customers/:id` | yes | true no-mock HTTP | `API_tests/customers_api_test.go` | `TestCustomersGet`, `TestCustomersGet_NotFound` |
| `PUT /api/v1/customers/:id` | yes | true no-mock HTTP | `API_tests/customers_api_test.go`, `API_tests/audit_fixes_api_test.go` | `TestCustomersUpdate`, `TestCustomerUpdate_NameOnlyPreservesEncryptedFields` |
| `DELETE /api/v1/customers/:id` | yes | true no-mock HTTP | `API_tests/customers_api_test.go` | `TestCustomersSoftDeleteAndRestore` |
| `POST /api/v1/customers/:id/restore` | yes | true no-mock HTTP | `API_tests/customers_api_test.go` | `TestCustomersSoftDeleteAndRestore` |
| `GET /api/v1/fulfillments` | yes | true no-mock HTTP | `API_tests/fulfillments_api_test.go` | `TestFulfillmentsList` |
| `POST /api/v1/fulfillments` | yes | true no-mock HTTP | `API_tests/fulfillments_api_test.go`, `integration/integration_test.go`, `e2e_tests/*` | `TestFulfillmentsCreate`, `TestFulfillmentLifecycle` |
| `GET /api/v1/fulfillments/:id` | yes | true no-mock HTTP | `API_tests/fulfillments_api_test.go`, `integration/integration_test.go` | `TestFulfillmentsGet`, `TestFulfillmentLifecycle` |
| `POST /api/v1/fulfillments/:id/transition` | yes | true no-mock HTTP | `API_tests/fulfillments_api_test.go`, `integration/integration_test.go`, `e2e_tests/lifecycle_e2e_test.go` | `TestFulfillmentsTransition_*`, `TestPhysicalFulfillmentLifecycle` |
| `PUT /api/v1/fulfillments/:id/shipping-address` | yes | true no-mock HTTP | `API_tests/fulfillments_api_test.go` | `TestFulfillmentShippingAddressUpdate_VersionConflict` |
| `GET /api/v1/fulfillments/:id/timeline` | yes | true no-mock HTTP | `API_tests/fulfillments_api_test.go`, `integration/integration_test.go`, `e2e_tests/lifecycle_e2e_test.go` | `TestFulfillmentsTimeline`, `TestFulfillmentLifecycle` |
| `DELETE /api/v1/fulfillments/:id` | yes | true no-mock HTTP | `API_tests/fulfillments_api_test.go` | `TestFulfillmentsSoftDelete`, `TestFulfillmentsRestore` |
| `POST /api/v1/fulfillments/:id/restore` | yes | true no-mock HTTP | `API_tests/fulfillments_api_test.go` | `TestFulfillmentsRestore` |
| `GET /api/v1/exceptions` | yes | true no-mock HTTP | `API_tests/exceptions_api_test.go` | `TestExceptionsList` |
| `POST /api/v1/exceptions` | yes | true no-mock HTTP | `API_tests/exceptions_api_test.go`, `integration/integration_test.go`, `e2e_tests/exports_e2e_test.go` | `TestExceptionsCreate`, `TestExceptionFlow` |
| `GET /api/v1/exceptions/:id` | yes | true no-mock HTTP | `API_tests/exceptions_api_test.go` | `TestExceptionsGet`, `TestExceptionsGet_NotFound` |
| `PUT /api/v1/exceptions/:id/status` | yes | true no-mock HTTP | `API_tests/exceptions_api_test.go`, `integration/integration_test.go`, `e2e_tests/exports_e2e_test.go` | `TestExceptionsUpdateStatus_*`, `TestExceptionFlow` |
| `POST /api/v1/exceptions/:id/events` | yes | true no-mock HTTP | `API_tests/exceptions_api_test.go`, `integration/integration_test.go`, `e2e_tests/exports_e2e_test.go` | `TestExceptionsAddEvent`, `TestExceptionFlow` |
| `GET /api/v1/message-templates` | yes | true no-mock HTTP | `API_tests/message_templates_api_test.go`, `API_tests/audit_fixes_api_test.go` | `TestMessageTemplatesList`, `TestAuditor_CannotReadMessageTemplates` |
| `POST /api/v1/message-templates` | yes | true no-mock HTTP | `API_tests/remediation_coverage_test.go`, `e2e_tests/pages_e2e_test.go` | `createDispatchableTemplate`, `TestPageRoutesRender` |
| `GET /api/v1/message-templates/:id` | yes | true no-mock HTTP | `API_tests/message_templates_api_test.go` | `TestMessageTemplatesGet`, `TestMessageTemplates_SpecialistCanRead` |
| `PUT /api/v1/message-templates/:id` | yes | true no-mock HTTP | `API_tests/message_templates_api_test.go` | `TestMessageTemplatesUpdate`, `TestMessageTemplatesUpdate_VersionConflict` |
| `DELETE /api/v1/message-templates/:id` | yes | true no-mock HTTP | `API_tests/message_templates_api_test.go` | `TestMessageTemplatesDelete`, `TestMessageTemplates_SpecialistCannotWrite` |
| `GET /api/v1/send-logs` | yes | true no-mock HTTP | `API_tests/remediation_coverage_test.go`, `API_tests/audit_fixes_api_test.go` | `TestSendLog_Queued_MarkFailed_RetryCycle`, `TestAuditor_CannotReadSendLogs` |
| `PUT /api/v1/send-logs/:id/printed` | yes | true no-mock HTTP | `API_tests/remediation_coverage_test.go` | `TestSendLog_MarkPrinted`, `TestSendLog_MarkPrinted_ForbiddenForAuditor` |
| `PUT /api/v1/send-logs/:id/failed` | yes | true no-mock HTTP | `API_tests/remediation_coverage_test.go` | `TestSendLog_Queued_MarkFailed_RetryCycle`, `TestSendLog_MarkFailed_ForbiddenForAuditor` |
| `GET /api/v1/notifications` | yes | true no-mock HTTP | `API_tests/notifications_api_test.go` | `TestNotificationsList`, `TestNotificationsList_UnreadFilter` |
| `PUT /api/v1/notifications/:id/read` | yes | true no-mock HTTP | `API_tests/notifications_api_test.go` | `TestNotificationsMarkRead`, `TestNotificationsMarkRead_InvalidID` |
| `POST /api/v1/dispatch` | yes | true no-mock HTTP | `API_tests/remediation_coverage_test.go` | `TestSendLog_Queued_MarkFailed_RetryCycle`, `TestSendLog_MarkPrinted` |
| `GET /api/v1/reports/exports` | yes | true no-mock HTTP | `API_tests/reports_api_test.go`, `API_tests/remediation_coverage_test.go`, `e2e_tests/exports_e2e_test.go` | `TestReportsList`, `TestReportsList_AuditorFilterAppliedBeforePagination` |
| `POST /api/v1/reports/exports` | yes | true no-mock HTTP | `API_tests/reports_api_test.go`, `integration/integration_test.go`, `e2e_tests/exports_e2e_test.go` | `TestReportsCreate`, `TestReportExportFlow` |
| `GET /api/v1/reports/exports/:id` | yes | true no-mock HTTP | `API_tests/reports_api_test.go`, `integration/integration_test.go`, `e2e_tests/exports_e2e_test.go` | `TestReportsGet`, `TestReportExportFlow` |
| `POST /api/v1/reports/exports/:id/verify-checksum` | yes | true no-mock HTTP | `API_tests/reports_api_test.go`, `integration/integration_test.go`, `e2e_tests/exports_e2e_test.go` | `TestReportsVerifyChecksum_AfterCompletion`, `TestReportExportFlow` |
| `DELETE /api/v1/reports/exports/:id` | yes | true no-mock HTTP | `API_tests/reports_api_test.go` | `TestReportsDelete` |
| `GET /api/v1/settings` | yes | true no-mock HTTP | `e2e_tests/exports_e2e_test.go` | `TestSettingsGetAll` |
| `PUT /api/v1/settings/:key` | yes | true no-mock HTTP | `API_tests/settings_api_test.go` | `TestSettingsSet`, `TestSettingsSet_*Forbidden`, `TestSettingsSet_MissingValue` |
| `GET /api/v1/settings/blackout-dates` | yes | true no-mock HTTP | `API_tests/settings_api_test.go`, `API_tests/audit_fixes_api_test.go` | `TestSettingsBlackoutDatesList`, `TestAuditor_CannotReadSettingsBlackouts` |
| `POST /api/v1/settings/blackout-dates` | yes | true no-mock HTTP | `API_tests/settings_api_test.go`, `e2e_tests/exports_e2e_test.go` | `TestSettingsBlackoutDatesList`, `TestBlackoutDatesCreateAndDelete` |
| `DELETE /api/v1/settings/blackout-dates/:id` | yes | true no-mock HTTP | `API_tests/settings_api_test.go`, `e2e_tests/exports_e2e_test.go` | `TestSettingsBlackoutDatesList`, `TestBlackoutDatesCreateAndDelete` |
| `GET /api/v1/audit` | yes | true no-mock HTTP | `API_tests/audit_api_test.go`, `integration/integration_test.go`, `e2e_tests/rbac_e2e_test.go` | `TestAuditList`, `TestRBACAccessControl` |
| `GET /api/v1/admin/health` | yes | true no-mock HTTP | `API_tests/admin_api_test.go`, `integration/integration_test.go`, `e2e_tests/rbac_e2e_test.go` | `TestAdminHealth`, `TestRBACAccessControl` |
| `GET /api/v1/admin/jobs/runs` | yes | true no-mock HTTP | `API_tests/remediation_coverage_test.go` | `TestAdminJobHistory_SurfacesFailedRunWithError` |
| `POST /api/v1/admin/jobs/:name/run` | yes | true no-mock HTTP | `API_tests/admin_api_test.go` | `TestAdminTriggerJob`, `TestAdminTriggerJob_ForbiddenForSpecialist` |
| `GET /api/v1/admin/job-schedules` | yes | true no-mock HTTP | `API_tests/admin_api_test.go`, `API_tests/remediation_coverage_test.go` | `TestAdminJobSchedulesList`, `TestAdminSchedules_*Forbidden` |
| `PUT /api/v1/admin/job-schedules/:key` | yes | true no-mock HTTP | `API_tests/admin_api_test.go` | `TestAdminJobScheduleUpdate`, `TestAdminJobScheduleUpdate_VersionConflict` |
| `GET /api/v1/admin/dr-drills` | yes | true no-mock HTTP | `API_tests/admin_api_test.go`, `API_tests/remediation_coverage_test.go` | `TestAdminDRDrillsList`, `TestAdminDRDrills_*Forbidden` |
| `POST /api/v1/admin/dr-drills` | yes | true no-mock HTTP | `API_tests/admin_api_test.go` | `TestAdminDRDrillCreate`, `TestAdminDRDrillCreate_MissingScheduledFor` |
| `PUT /api/v1/admin/dr-drills/:id` | yes | true no-mock HTTP | `API_tests/admin_api_test.go` | `TestAdminDRDrillUpdate`, `TestAdminDRDrillUpdate_NotFound` |
| `GET /api/v1/admin/users` | yes | true no-mock HTTP | `API_tests/users_api_test.go` | `TestUsersList`, `TestUsers_RequiresAdminRole` |
| `POST /api/v1/admin/users` | yes | true no-mock HTTP | `API_tests/users_api_test.go`, `e2e_tests/pages_e2e_test.go` | `TestUsersCreate`, `TestPageRoutesRender` |
| `GET /api/v1/admin/users/:id` | yes | true no-mock HTTP | `API_tests/users_api_test.go` | `TestUsersGet` |
| `PUT /api/v1/admin/users/:id` | yes | true no-mock HTTP | `API_tests/users_api_test.go` | `TestUsersUpdate` |
| `DELETE /api/v1/admin/users/:id` | yes | true no-mock HTTP | `API_tests/users_api_test.go` | `TestUsersDeactivate` |

## API Test Classification
1. True No-Mock HTTP:
- `repo/API_tests/*.go`, `repo/integration/integration_test.go`, `repo/e2e_tests/*.go`.
- Evidence: real Gin engine + real route registration + real repositories/services wiring in `repo/API_tests/setup_test.go:61-137`, `repo/integration/integration_test.go:68-159`, `repo/e2e_tests/setup_test.go:60-131`.

2. HTTP with Mocking:
- None found in API/e2e/integration suites.

3. Non-HTTP (unit/integration without HTTP):
- `repo/internal/service/*_test.go`, `repo/internal/repository/*_test.go`, `repo/internal/middleware/*_test.go`, `repo/internal/handler/*_test.go`, `repo/internal/job/*_test.go`, `repo/unit_tests/*`, `repo/internal/util/*_test.go`, `repo/static/js/app.test.js`.

## Mock Detection
- No `jest.mock`, `vi.mock`, `sinon.stub` found in backend API/e2e/integration tests.
- Unit-level fakes/stubs present (non-HTTP tests):
  - `repo/internal/job/jobs_test.go` (`fakeJobRunRepo`, `fakeBackupService`, `fakeMessagingService`, etc.).
  - `repo/internal/middleware/auth_revalidation_test.go` (`fakeUserLookup`).
- Frontend unit test spying/stubbing present:
  - `repo/static/js/app.test.js` uses `vi.spyOn` and `vi.stubGlobal` in `describe('confirmAction', ...)`.

## Coverage Summary
- Total endpoints in coverage set: **64** (63 API + `/healthz`).
- Endpoints with HTTP tests: **64**.
- Endpoints with true no-mock HTTP tests: **64**.
- HTTP coverage: **100.0%**.
- True API coverage: **100.0%**.

## Unit Test Summary

### Backend Unit Tests
- Test files (sample classes, non-exhaustive):
  - services: `repo/internal/service/*_test.go`
  - repositories: `repo/internal/repository/repo_test.go`, `repo/internal/repository/repo_extra_test.go`
  - handlers/controllers: `repo/internal/handler/coverage_test.go`, `repo/internal/handler/report_validation_test.go`, `repo/internal/handler/customer_update_test.go`, `repo/internal/handler/admin_health_test.go`
  - auth/guards/middleware: `repo/internal/middleware/middleware_test.go`, `repo/internal/middleware/auth_revalidation_test.go`
  - jobs/config/util/domain: `repo/internal/job/*_test.go`, `repo/internal/config/config_test.go`, `repo/internal/util/*_test.go`, `repo/unit_tests/*`

- Covered module categories:
  - controllers/handlers: covered.
  - services: covered.
  - repositories: covered.
  - auth/guards/middleware: covered.

- Important backend modules not directly unit-tested (file-level direct tests absent):
  - `repo/cmd/server/main.go` (bootstrapping path is validated only indirectly via HTTP suites).
  - Several page handler internals rely mainly on HTTP smoke/e2e coverage rather than dedicated unit-level tests (e.g., `repo/internal/handler/page_*` files not all have matching focused unit tests).

### Frontend Unit Tests (STRICT)
- Frontend test files detected:
  - `repo/static/js/app.test.js`
- Frameworks/tools detected:
  - Vitest imports: `repo/static/js/app.test.js:22`
  - Vitest config + jsdom: `repo/static/js/vitest.config.js:1-15`
  - package script/tooling: `repo/static/js/package.json:6-13`
- Components/modules covered:
  - `repo/static/js/app.js` behaviors tested: confirm modal flow, modal open/close, backdrop click/Escape handling, tracking validation, flash auto-dismiss, submit guard, dashboard auto-refresh.
- Important frontend components/modules not tested:
  - No additional frontend module files detected under `repo/static/js/` besides `app.js`; no component framework tree exists to audit beyond this file.

**Frontend unit tests: PRESENT**

### Cross-Layer Observation
- Backend testing is substantially broader (API + integration + e2e + unit).
- Frontend unit testing exists but is limited to one JS module (`static/js/app.js`); there is no browser-level FE↔BE end-to-end automation.

## API Observability Check
- Strong observability in most API tests:
  - explicit method+path requests,
  - request payloads/query params,
  - response assertions (status + JSON fields).
- Evidence: repeated `admin/as/do` requests with request bodies and `mustStatus`/`decodeJSON` assertions in `repo/API_tests/*.go` and `repo/integration/integration_test.go`.
- Weak areas:
  - some RBAC/forbidden tests assert status only (minimal response-body checks), e.g. `repo/API_tests/remediation_coverage_test.go::TestAdminSchedules_ForbiddenForSpecialist`.

## Tests Check
- Success paths: strong (CRUD + happy lifecycle + exports + notifications).
- Failure/negative paths: strong (401/403/404/409, validation failures, stale version conflicts).
- Edge cases: moderate-to-strong (concurrency, retry transitions, pagination/filter ordering).
- Validation/authz depth: strong.
- Integration boundaries: strong for backend HTTP+DB path.
- Assertion quality: generally meaningful; some tests are status-only in authorization checks.
- `run_tests.sh` audit:
  - Docker-based execution present (`repo/run_tests.sh:3-4`, docker orchestration throughout).
  - No hard requirement for local language/runtime package installs; execution model is containerized.

## End-to-End Expectations
- For fullstack projects, real FE↔BE tests are expected.
- Present status:
  - API/e2e backend HTTP workflows are strong.
  - Page-route smoke/e2e exists (`repo/API_tests/pages_api_test.go`, `repo/e2e_tests/pages_e2e_test.go`).
  - No true browser-driven frontend-UI-to-backend test harness (Playwright/Cypress) found.
- Compensation: strong backend API/e2e coverage partially compensates, but not a full replacement for browser-level FE↔BE tests.

## Test Coverage Score (0-100)
**91/100**

## Score Rationale
- + High endpoint completeness: all inventoried API/health endpoints mapped to HTTP tests.
- + True no-mock HTTP pattern consistently used in API/integration/e2e suites.
- + Good negative and authorization coverage breadth.
- - No browser-level FE↔BE E2E automation despite `fullstack` classification.
- - Some assertion blocks are shallow (status-only checks in selected RBAC tests).
- - Unit-testing density is backend-heavy; frontend scope is narrow (single JS module).

## Key Gaps
1. Missing browser-level FE↔BE end-to-end test suite (critical for fullstack confidence at UI integration boundary).
2. Status-only assertions in several RBAC tests reduce observability depth.
3. Page-layer behaviors are mostly smoke-tested; deeper page mutation-path assertions are limited.

## Confidence & Assumptions
- Confidence: high for route inventory and API coverage mapping.
- Assumptions:
  - Coverage % is computed for API routes in `RegisterRoutes` plus `/healthz`.
  - Non-API page routes are tracked separately and not mixed into API coverage percentages.

## Test Coverage Verdict
**PASS with notable gaps** (coverage breadth is strong; browser-level FE↔BE testing remains the principal deficiency).

---

# README Audit

## README Location
- Found at `repo/README.md`.

## Hard Gate Evaluation

### Formatting
- PASS: markdown is readable and structured with sections/tables/code blocks.
- Evidence: `repo/README.md:3-203`.

### Startup Instructions (fullstack/backend requires `docker-compose up`)
- PASS: explicit `docker-compose up` included.
- Evidence: `repo/README.md:33-35`.

### Access Method
- PASS: URL and port explicitly provided.
- Evidence: `repo/README.md:39` (`http://localhost:8080/auth/login`).

### Verification Method
- PASS: concrete verification flow with curl and expected outcomes.
- Evidence: `repo/README.md:96-147`.

### Environment Rules (Docker-contained; no runtime manual installs)
- PASS: explicitly states Docker-only and disallows manual installs.
- Evidence: `repo/README.md:31-38`.

### Demo Credentials (auth exists)
- PASS (strictly): credentials for all roles are documented, with explicit one-time seeding for specialist/auditor and complete role table.
- Evidence: `repo/README.md:55-93`.

## Engineering Quality
- Tech stack clarity: strong (`repo/README.md:7`).
- Architecture explanation: moderate; folder-map and controls exist but request/data-flow narrative is limited (`repo/README.md:11-26`, `197-203`).
- Testing instructions: clear and actionable (`repo/README.md:168-177`).
- Security/roles clarity: moderate-to-strong, including compliance controls and role credentials (`repo/README.md:55-93`, `197-203`).
- Workflow clarity: good for startup and verification; medium for operational/day-2 procedures.
- Presentation quality: good.

## High Priority Issues
- None.

## Medium Priority Issues
1. Architecture explanation is mostly directory-oriented; it lacks a concise request-flow narrative (UI/page routes vs API routes vs services vs repositories).
2. Demo account setup is split between auto-seeded admin and manual role seeding; while valid, this increases first-run friction and could be condensed into a clearer quickstart path.

## Low Priority Issues
1. Some operational sections could link directly to exact API route groups for faster navigation by new contributors.
2. Verification flow is API-heavy; a short end-user UI checklist could improve fullstack onboarding.

## Hard Gate Failures
- None.

## README Verdict (PASS / PARTIAL PASS / FAIL)
**PASS**

## README Final Verdict
**PASS**
