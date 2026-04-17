# Test Coverage Audit

## Scope And Project Type

- Declared project type: `fullstack` ([README.md](README.md), line 1).
- Static-only audit. No code, tests, scripts, containers, or package managers were run.
- Backend HTTP surface found:
- `63` versioned API endpoints under `/api/v1` in [internal/handler/router.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/handler/router.go:228).
- `1` auxiliary backend endpoint `GET /healthz` in [cmd/server/main.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/cmd/server/main.go:124).
- `71` server-rendered page routes in [internal/handler/router.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/handler/router.go:111).
- The mandatory API mapping table below covers `/api/v1/*` plus `GET /healthz`. Page routes are assessed separately under fullstack expectations because the prompt’s coverage criteria are API-centric.

## Backend Endpoint Inventory

### Auxiliary

- `GET /healthz`

### Auth

- `POST /api/v1/auth/login`
- `POST /api/v1/auth/logout`
- `GET /api/v1/auth/me`

### Tiers

- `GET /api/v1/tiers`
- `POST /api/v1/tiers`
- `GET /api/v1/tiers/:id`
- `PUT /api/v1/tiers/:id`
- `DELETE /api/v1/tiers/:id`
- `POST /api/v1/tiers/:id/restore`

### Customers

- `GET /api/v1/customers`
- `POST /api/v1/customers`
- `GET /api/v1/customers/:id`
- `PUT /api/v1/customers/:id`
- `DELETE /api/v1/customers/:id`
- `POST /api/v1/customers/:id/restore`

### Fulfillments

- `GET /api/v1/fulfillments`
- `POST /api/v1/fulfillments`
- `GET /api/v1/fulfillments/:id`
- `POST /api/v1/fulfillments/:id/transition`
- `PUT /api/v1/fulfillments/:id/shipping-address`
- `GET /api/v1/fulfillments/:id/timeline`
- `DELETE /api/v1/fulfillments/:id`
- `POST /api/v1/fulfillments/:id/restore`

### Exceptions

- `GET /api/v1/exceptions`
- `POST /api/v1/exceptions`
- `GET /api/v1/exceptions/:id`
- `PUT /api/v1/exceptions/:id/status`
- `POST /api/v1/exceptions/:id/events`

### Messaging

- `GET /api/v1/message-templates`
- `POST /api/v1/message-templates`
- `GET /api/v1/message-templates/:id`
- `PUT /api/v1/message-templates/:id`
- `DELETE /api/v1/message-templates/:id`
- `GET /api/v1/send-logs`
- `PUT /api/v1/send-logs/:id/printed`
- `PUT /api/v1/send-logs/:id/failed`
- `GET /api/v1/notifications`
- `PUT /api/v1/notifications/:id/read`
- `POST /api/v1/dispatch`

### Reports

- `GET /api/v1/reports/exports`
- `POST /api/v1/reports/exports`
- `GET /api/v1/reports/exports/:id`
- `POST /api/v1/reports/exports/:id/verify-checksum`
- `DELETE /api/v1/reports/exports/:id`

### Settings

- `GET /api/v1/settings`
- `PUT /api/v1/settings/:key`
- `GET /api/v1/settings/blackout-dates`
- `POST /api/v1/settings/blackout-dates`
- `DELETE /api/v1/settings/blackout-dates/:id`

### Audit

- `GET /api/v1/audit`

### Admin

- `GET /api/v1/admin/health`
- `GET /api/v1/admin/jobs/runs`
- `POST /api/v1/admin/jobs/:name/run`
- `GET /api/v1/admin/job-schedules`
- `PUT /api/v1/admin/job-schedules/:key`
- `GET /api/v1/admin/dr-drills`
- `POST /api/v1/admin/dr-drills`
- `PUT /api/v1/admin/dr-drills/:id`
- `GET /api/v1/admin/users`
- `POST /api/v1/admin/users`
- `GET /api/v1/admin/users/:id`
- `PUT /api/v1/admin/users/:id`
- `DELETE /api/v1/admin/users/:id`

## API Test Mapping Table

| Endpoint | Covered | Test type | Test files | Evidence |
|---|---|---|---|---|
| `GET /healthz` | yes | true no-mock HTTP | `API_tests/auth_api_test.go`, `integration/integration_test.go` | `TestHealthz` at [API_tests/auth_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/auth_api_test.go:11), `TestHealthz` at [integration/integration_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/integration/integration_test.go:226) |
| `POST /api/v1/auth/login` | yes | true no-mock HTTP | `API_tests/auth_api_test.go` | `TestAuthLogin_Success`, `TestAuthLogin_WrongPassword`, `TestAuthLogin_MissingBody` at [API_tests/auth_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/auth_api_test.go:20) |
| `POST /api/v1/auth/logout` | yes | true no-mock HTTP | `API_tests/auth_api_test.go`, `API_tests/audit_fixes_api_test.go` | `TestAuthLogout` at [API_tests/auth_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/auth_api_test.go:66), `TestLogout_ClearsBothCookies` at [API_tests/audit_fixes_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/audit_fixes_api_test.go:115) |
| `GET /api/v1/auth/me` | yes | true no-mock HTTP | `API_tests/auth_api_test.go`, `API_tests/notifications_api_test.go` | `TestAuthMe_Authenticated`, `TestAuthMe_Unauthenticated` at [API_tests/auth_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/auth_api_test.go:52); `seedNotificationForAdmin` at [API_tests/notifications_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/notifications_api_test.go:13) |
| `GET /api/v1/tiers` | yes | true no-mock HTTP | `API_tests/tiers_api_test.go`, `e2e_tests/rbac_e2e_test.go` | `TestTiersList` at [API_tests/tiers_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/tiers_api_test.go:47), `TestRBAC_SpecialistCanReadButNotDeleteTiers` at [e2e_tests/rbac_e2e_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/e2e_tests/rbac_e2e_test.go:38) |
| `POST /api/v1/tiers` | yes | true no-mock HTTP | `API_tests/tiers_api_test.go` | `TestTiersCreate`, `TestTiersCreate_MissingName`, `TestTiersCreate_RequiresAdmin` at [API_tests/tiers_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/tiers_api_test.go:19) |
| `GET /api/v1/tiers/:id` | yes | true no-mock HTTP | `API_tests/tiers_api_test.go` | `TestTiersGet`, `TestTiersGet_NotFound` at [API_tests/tiers_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/tiers_api_test.go:77) |
| `PUT /api/v1/tiers/:id` | yes | true no-mock HTTP | `API_tests/tiers_api_test.go`, `e2e_tests/inventory_e2e_test.go` | `TestTiersUpdate`, `TestTiersUpdate_VersionConflict` at [API_tests/tiers_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/tiers_api_test.go:94), `TestVersionConflict_Tier` at [e2e_tests/inventory_e2e_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/e2e_tests/inventory_e2e_test.go:130) |
| `DELETE /api/v1/tiers/:id` | yes | true no-mock HTTP | `API_tests/tiers_api_test.go`, `e2e_tests/rbac_e2e_test.go` | `TestTiersSoftDeleteAndRestore`, `TestTiersDelete_NotFound` at [API_tests/tiers_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/tiers_api_test.go:126), `TestRBAC_SpecialistCanReadButNotDeleteTiers` at [e2e_tests/rbac_e2e_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/e2e_tests/rbac_e2e_test.go:38) |
| `POST /api/v1/tiers/:id/restore` | yes | true no-mock HTTP | `API_tests/tiers_api_test.go`, `e2e_tests/lifecycle_e2e_test.go` | `TestTiersSoftDeleteAndRestore` at [API_tests/tiers_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/tiers_api_test.go:126), `TestTierSoftDeleteAndRestore` at [e2e_tests/lifecycle_e2e_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/e2e_tests/lifecycle_e2e_test.go:204) |
| `GET /api/v1/customers` | yes | true no-mock HTTP | `API_tests/customers_api_test.go` | `TestCustomersList` at [API_tests/customers_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/customers_api_test.go:39) |
| `POST /api/v1/customers` | yes | true no-mock HTTP | `API_tests/customers_api_test.go` | `TestCustomersCreate`, `TestCustomersCreate_MissingName`, `TestCustomersCreate_PhoneAndEmailAreMaskedInResponse` at [API_tests/customers_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/customers_api_test.go:19) |
| `GET /api/v1/customers/:id` | yes | true no-mock HTTP | `API_tests/customers_api_test.go` | `TestCustomersGet`, `TestCustomersGet_NotFound` at [API_tests/customers_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/customers_api_test.go:70) |
| `PUT /api/v1/customers/:id` | yes | true no-mock HTTP | `API_tests/customers_api_test.go`, `API_tests/audit_fixes_api_test.go` | `TestCustomersUpdate` at [API_tests/customers_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/customers_api_test.go:87), `TestCustomerUpdate_NameOnlyPreservesEncryptedFields` at [API_tests/audit_fixes_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/audit_fixes_api_test.go:36) |
| `DELETE /api/v1/customers/:id` | yes | true no-mock HTTP | `API_tests/customers_api_test.go` | `TestCustomersSoftDeleteAndRestore` at [API_tests/customers_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/customers_api_test.go:102) |
| `POST /api/v1/customers/:id/restore` | yes | true no-mock HTTP | `API_tests/customers_api_test.go` | `TestCustomersSoftDeleteAndRestore` at [API_tests/customers_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/customers_api_test.go:102) |
| `GET /api/v1/fulfillments` | yes | true no-mock HTTP | `API_tests/fulfillments_api_test.go` | `TestFulfillmentsList` at [API_tests/fulfillments_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/fulfillments_api_test.go:97) |
| `POST /api/v1/fulfillments` | yes | true no-mock HTTP | `API_tests/fulfillments_api_test.go`, `e2e_tests/rbac_e2e_test.go` | `TestFulfillmentsCreate`, `TestFulfillmentsCreate_MissingTierID`, `TestFulfillmentsCreate_InventoryUnavailable` at [API_tests/fulfillments_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/fulfillments_api_test.go:55), `TestRBAC_SpecialistCanCreateFulfillments` at [e2e_tests/rbac_e2e_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/e2e_tests/rbac_e2e_test.go:117) |
| `GET /api/v1/fulfillments/:id` | yes | true no-mock HTTP | `API_tests/fulfillments_api_test.go`, `e2e_tests/setup_test.go` | `TestFulfillmentsGet`, `TestFulfillmentsGet_NotFound` at [API_tests/fulfillments_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/fulfillments_api_test.go:132), `transition` helper loads current version at [e2e_tests/setup_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/e2e_tests/setup_test.go:181) |
| `POST /api/v1/fulfillments/:id/transition` | yes | true no-mock HTTP | `API_tests/fulfillments_api_test.go`, `e2e_tests/lifecycle_e2e_test.go` | `TestFulfillmentsTransition_DraftToReadyToShip`, `...ShippedRequiresTracking`, `...Cancel` at [API_tests/fulfillments_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/fulfillments_api_test.go:160), workflow transitions in [e2e_tests/lifecycle_e2e_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/e2e_tests/lifecycle_e2e_test.go:13) |
| `PUT /api/v1/fulfillments/:id/shipping-address` | yes | true no-mock HTTP | `API_tests/fulfillments_api_test.go` | `TestFulfillmentShippingAddressUpdate_VersionConflict` at [API_tests/fulfillments_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/fulfillments_api_test.go:288) |
| `GET /api/v1/fulfillments/:id/timeline` | yes | true no-mock HTTP | `API_tests/fulfillments_api_test.go` | `TestFulfillmentsTimeline` at [API_tests/fulfillments_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/fulfillments_api_test.go:275) |
| `DELETE /api/v1/fulfillments/:id` | yes | true no-mock HTTP | `API_tests/fulfillments_api_test.go` | `TestFulfillmentsSoftDelete`, `TestFulfillmentsRestore` at [API_tests/fulfillments_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/fulfillments_api_test.go:315) |
| `POST /api/v1/fulfillments/:id/restore` | yes | true no-mock HTTP | `API_tests/fulfillments_api_test.go` | `TestFulfillmentsRestore` at [API_tests/fulfillments_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/fulfillments_api_test.go:325) |
| `GET /api/v1/exceptions` | yes | true no-mock HTTP | `API_tests/exceptions_api_test.go` | `TestExceptionsList` at [API_tests/exceptions_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/exceptions_api_test.go:49) |
| `POST /api/v1/exceptions` | yes | true no-mock HTTP | `API_tests/exceptions_api_test.go`, `e2e_tests/exports_e2e_test.go` | `TestExceptionsCreate` at [API_tests/exceptions_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/exceptions_api_test.go:33), `TestExceptionWorkflow` at [e2e_tests/exports_e2e_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/e2e_tests/exports_e2e_test.go:69) |
| `GET /api/v1/exceptions/:id` | yes | true no-mock HTTP | `API_tests/exceptions_api_test.go` | `TestExceptionsGet`, `TestExceptionsGet_NotFound` at [API_tests/exceptions_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/exceptions_api_test.go:54) |
| `PUT /api/v1/exceptions/:id/status` | yes | true no-mock HTTP | `API_tests/exceptions_api_test.go`, `e2e_tests/exports_e2e_test.go` | `TestExceptionsUpdateStatus_Investigating`, `TestExceptionsUpdateStatus_Resolved` at [API_tests/exceptions_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/exceptions_api_test.go:95), `TestExceptionWorkflow` at [e2e_tests/exports_e2e_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/e2e_tests/exports_e2e_test.go:69) |
| `POST /api/v1/exceptions/:id/events` | yes | true no-mock HTTP | `API_tests/exceptions_api_test.go`, `e2e_tests/exports_e2e_test.go` | `TestExceptionsAddEvent` at [API_tests/exceptions_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/exceptions_api_test.go:80), `TestExceptionWorkflow` at [e2e_tests/exports_e2e_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/e2e_tests/exports_e2e_test.go:69) |
| `GET /api/v1/message-templates` | yes | true no-mock HTTP | `API_tests/message_templates_api_test.go`, `API_tests/audit_fixes_api_test.go` | `TestMessageTemplatesList`, `TestMessageTemplates_AuditorForbidden`, `TestMessageTemplates_SpecialistCanRead` at [API_tests/message_templates_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/message_templates_api_test.go:26), `TestAuditor_CannotReadMessageTemplates` at [API_tests/audit_fixes_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/audit_fixes_api_test.go:80) |
| `POST /api/v1/message-templates` | yes | true no-mock HTTP | `API_tests/message_templates_api_test.go`, `API_tests/remediation_coverage_test.go` | `createTemplate` helper at [API_tests/message_templates_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/message_templates_api_test.go:11), `createDispatchableTemplate` at [API_tests/remediation_coverage_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/remediation_coverage_test.go:45) |
| `GET /api/v1/message-templates/:id` | yes | true no-mock HTTP | `API_tests/message_templates_api_test.go` | `TestMessageTemplatesGet`, `TestMessageTemplatesGet_NotFound`, `TestMessageTemplates_SpecialistCanRead` at [API_tests/message_templates_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/message_templates_api_test.go:60) |
| `PUT /api/v1/message-templates/:id` | yes | true no-mock HTTP | `API_tests/message_templates_api_test.go` | `TestMessageTemplatesUpdate`, `TestMessageTemplatesUpdate_VersionConflict`, `TestMessageTemplates_SpecialistCannotWrite` at [API_tests/message_templates_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/message_templates_api_test.go:86) |
| `DELETE /api/v1/message-templates/:id` | yes | true no-mock HTTP | `API_tests/message_templates_api_test.go` | `TestMessageTemplatesDelete`, `TestMessageTemplates_SpecialistCannotWrite` at [API_tests/message_templates_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/message_templates_api_test.go:134) |
| `GET /api/v1/send-logs` | yes | true no-mock HTTP | `API_tests/remediation_coverage_test.go`, `API_tests/audit_fixes_api_test.go` | `TestSendLog_Queued_MarkFailed_RetryCycle`, `TestSendLog_MarkPrinted` at [API_tests/remediation_coverage_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/remediation_coverage_test.go:74), `TestAuditor_CannotReadSendLogs` at [API_tests/audit_fixes_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/audit_fixes_api_test.go:91) |
| `PUT /api/v1/send-logs/:id/printed` | yes | true no-mock HTTP | `API_tests/remediation_coverage_test.go` | `TestSendLog_MarkPrinted`, `TestSendLog_MarkPrinted_ForbiddenForAuditor` at [API_tests/remediation_coverage_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/remediation_coverage_test.go:239) |
| `PUT /api/v1/send-logs/:id/failed` | yes | true no-mock HTTP | `API_tests/remediation_coverage_test.go` | `TestSendLog_Queued_MarkFailed_RetryCycle`, `TestSendLog_MarkFailed_ForbiddenForAuditor` at [API_tests/remediation_coverage_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/remediation_coverage_test.go:74) |
| `GET /api/v1/notifications` | yes | true no-mock HTTP | `API_tests/notifications_api_test.go` | `TestNotificationsList`, `TestNotificationsList_UnreadFilter` at [API_tests/notifications_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/notifications_api_test.go:42) |
| `PUT /api/v1/notifications/:id/read` | yes | true no-mock HTTP | `API_tests/notifications_api_test.go` | `TestNotificationsMarkRead`, `TestNotificationsMarkRead_InvalidID` at [API_tests/notifications_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/notifications_api_test.go:96) |
| `POST /api/v1/dispatch` | yes | true no-mock HTTP | `API_tests/remediation_coverage_test.go` | `TestSendLog_Queued_MarkFailed_RetryCycle`, `TestSendLog_MarkPrinted` at [API_tests/remediation_coverage_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/remediation_coverage_test.go:74) |
| `GET /api/v1/reports/exports` | yes | true no-mock HTTP | `API_tests/reports_api_test.go`, `API_tests/remediation_coverage_test.go` | `TestReportsList`, `TestReports_RequiresAuth` at [API_tests/reports_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/reports_api_test.go:15), `TestReportsList_AuditorFilterAppliedBeforePagination` at [API_tests/remediation_coverage_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/remediation_coverage_test.go:158) |
| `POST /api/v1/reports/exports` | yes | true no-mock HTTP | `API_tests/reports_api_test.go`, `API_tests/audit_fixes_api_test.go` | `TestReportsCreate`, `TestReportsSensitive_AuditorCreateForbidden` at [API_tests/reports_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/reports_api_test.go:52), `TestReportsCreate_InvalidTypeRejectedSynchronously` at [API_tests/audit_fixes_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/audit_fixes_api_test.go:20) |
| `GET /api/v1/reports/exports/:id` | yes | true no-mock HTTP | `API_tests/reports_api_test.go`, `e2e_tests/exports_e2e_test.go` | `TestReportsGet`, `TestReportsGet_NotFound`, `TestReportsSensitive_AuditorGetAndVerifyForbidden` at [API_tests/reports_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/reports_api_test.go:63), `TestExportWorkflow_CreateAndVerifyChecksum` at [e2e_tests/exports_e2e_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/e2e_tests/exports_e2e_test.go:12) |
| `POST /api/v1/reports/exports/:id/verify-checksum` | yes | true no-mock HTTP | `API_tests/reports_api_test.go`, `e2e_tests/exports_e2e_test.go` | `TestReportsVerifyChecksum_AfterCompletion`, `TestReportsSensitive_AuditorGetAndVerifyForbidden` at [API_tests/reports_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/reports_api_test.go:83), `TestExportWorkflow_CreateAndVerifyChecksum` at [e2e_tests/exports_e2e_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/e2e_tests/exports_e2e_test.go:12) |
| `DELETE /api/v1/reports/exports/:id` | yes | true no-mock HTTP | `API_tests/reports_api_test.go` | `TestReportsDelete` at [API_tests/reports_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/reports_api_test.go:113) |
| `GET /api/v1/settings` | yes | true no-mock HTTP | `e2e_tests/exports_e2e_test.go` | `TestSettingsGetAll` at [e2e_tests/exports_e2e_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/e2e_tests/exports_e2e_test.go:131) |
| `PUT /api/v1/settings/:key` | yes | true no-mock HTTP | `API_tests/settings_api_test.go` | `TestSettingsSet`, `TestSettingsSet_SpecialistForbidden`, `TestSettingsSet_AuditorForbidden`, `TestSettingsSet_MissingValue` at [API_tests/settings_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/settings_api_test.go:10) |
| `GET /api/v1/settings/blackout-dates` | yes | true no-mock HTTP | `API_tests/settings_api_test.go`, `API_tests/audit_fixes_api_test.go` | `TestSettingsBlackoutDatesList`, `TestSettingsBlackoutDatesList_SpecialistAllowed`, `TestSettingsBlackoutDatesList_AuditorForbidden` at [API_tests/settings_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/settings_api_test.go:58), `TestAuditor_CannotReadSettingsBlackouts` at [API_tests/audit_fixes_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/audit_fixes_api_test.go:102) |
| `POST /api/v1/settings/blackout-dates` | yes | true no-mock HTTP | `API_tests/settings_api_test.go`, `e2e_tests/exports_e2e_test.go` | seeding inside `TestSettingsBlackoutDatesList` at [API_tests/settings_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/settings_api_test.go:58), `TestBlackoutDatesCreateAndDelete` at [e2e_tests/exports_e2e_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/e2e_tests/exports_e2e_test.go:136) |
| `DELETE /api/v1/settings/blackout-dates/:id` | yes | true no-mock HTTP | `API_tests/settings_api_test.go`, `e2e_tests/exports_e2e_test.go` | cleanup inside `TestSettingsBlackoutDatesList` at [API_tests/settings_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/settings_api_test.go:58), `TestBlackoutDatesCreateAndDelete` at [e2e_tests/exports_e2e_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/e2e_tests/exports_e2e_test.go:136) |
| `GET /api/v1/audit` | yes | true no-mock HTTP | `API_tests/audit_api_test.go`, `e2e_tests/rbac_e2e_test.go` | `TestAuditList`, `TestAuditList_RequiresAuth`, `TestAuditList_ContainsEntries`, `TestAuditList_SpecialistForbidden` at [API_tests/audit_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/audit_api_test.go:10), `TestRBAC_AuditorCanReadAuditButNotCreateTier` at [e2e_tests/rbac_e2e_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/e2e_tests/rbac_e2e_test.go:67) |
| `GET /api/v1/admin/health` | yes | true no-mock HTTP | `API_tests/admin_api_test.go`, `e2e_tests/rbac_e2e_test.go` | `TestAdminHealth`, `TestAdminHealth_SpecialistForbidden`, `TestAdminHealth_AuditorForbidden` at [API_tests/admin_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/admin_api_test.go:12), `TestRBAC_SpecialistCanReadButNotDeleteTiers` at [e2e_tests/rbac_e2e_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/e2e_tests/rbac_e2e_test.go:38) |
| `GET /api/v1/admin/jobs/runs` | yes | true no-mock HTTP | `API_tests/remediation_coverage_test.go` | `TestAdminJobHistory_SurfacesFailedRunWithError` at [API_tests/remediation_coverage_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/remediation_coverage_test.go:201) |
| `POST /api/v1/admin/jobs/:name/run` | yes | true no-mock HTTP | `API_tests/admin_api_test.go` | `TestAdminTriggerJob`, `TestAdminTriggerJob_ForbiddenForSpecialist` at [API_tests/admin_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/admin_api_test.go:49) |
| `GET /api/v1/admin/job-schedules` | yes | true no-mock HTTP | `API_tests/admin_api_test.go`, `API_tests/remediation_coverage_test.go` | `TestAdminJobSchedulesList` at [API_tests/admin_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/admin_api_test.go:72), `TestAdminSchedules_ForbiddenForSpecialist`, `...Auditor` at [API_tests/remediation_coverage_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/remediation_coverage_test.go:286) |
| `PUT /api/v1/admin/job-schedules/:key` | yes | true no-mock HTTP | `API_tests/admin_api_test.go` | `TestAdminJobScheduleUpdate`, `TestAdminJobScheduleUpdate_VersionConflict` at [API_tests/admin_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/admin_api_test.go:100) |
| `GET /api/v1/admin/dr-drills` | yes | true no-mock HTTP | `API_tests/admin_api_test.go`, `API_tests/remediation_coverage_test.go` | `TestAdminDRDrillsList` at [API_tests/admin_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/admin_api_test.go:177), `TestAdminDRDrills_ForbiddenForSpecialist`, `...Auditor` at [API_tests/remediation_coverage_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/remediation_coverage_test.go:298) |
| `POST /api/v1/admin/dr-drills` | yes | true no-mock HTTP | `API_tests/admin_api_test.go` | `TestAdminDRDrillCreate`, `TestAdminDRDrillCreate_MissingScheduledFor` at [API_tests/admin_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/admin_api_test.go:192) |
| `PUT /api/v1/admin/dr-drills/:id` | yes | true no-mock HTTP | `API_tests/admin_api_test.go` | `TestAdminDRDrillUpdate`, `TestAdminDRDrillUpdate_InvalidOutcome`, `TestAdminDRDrillUpdate_NotFound` at [API_tests/admin_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/admin_api_test.go:225) |
| `GET /api/v1/admin/users` | yes | true no-mock HTTP | `API_tests/users_api_test.go` | `TestUsersList`, `TestUsers_RequiresAdminRole` at [API_tests/users_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/users_api_test.go:10) |
| `POST /api/v1/admin/users` | yes | true no-mock HTTP | `API_tests/users_api_test.go` | `TestUsersCreate`, `TestUsersCreate_DuplicateUsername`, `TestUsersCreate_InvalidRole` at [API_tests/users_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/users_api_test.go:47) |
| `GET /api/v1/admin/users/:id` | yes | true no-mock HTTP | `API_tests/users_api_test.go` | `TestUsersGet` at [API_tests/users_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/users_api_test.go:96) |
| `PUT /api/v1/admin/users/:id` | yes | true no-mock HTTP | `API_tests/users_api_test.go` | `TestUsersUpdate` at [API_tests/users_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/users_api_test.go:115) |
| `DELETE /api/v1/admin/users/:id` | yes | true no-mock HTTP | `API_tests/users_api_test.go` | `TestUsersDeactivate` at [API_tests/users_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/users_api_test.go:141) |

## API Test Classification

### 1. True No-Mock HTTP

- `API_tests/*.go`: real Gin router, real repositories/services, live PostgreSQL, real request path through `handler.RegisterRoutes`; evidence in [API_tests/setup_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/setup_test.go:32).
- `e2e_tests/*.go`: same pattern with real router and DB; evidence in [e2e_tests/setup_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/e2e_tests/setup_test.go:31).
- `integration/integration_test.go`: real router and DB, same no-mock pattern; evidence in [integration/integration_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/integration/integration_test.go:31).

### 2. HTTP With Mocking

- [internal/handler/customer_update_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/handler/customer_update_test.go:82): `recordingCustomerRepo` and `identityEncryptionSvc` replace real dependencies.
- [internal/handler/report_validation_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/handler/report_validation_test.go:23): `stubReportRepo` replaces repository/storage.
- [internal/handler/coverage_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/handler/coverage_test.go:159): `stubBackupService` replaces backup service.
- [internal/handler/auditor_masking_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/handler/auditor_masking_test.go:25): stub fulfillment, tier, customer, timeline, shipping, and exception repositories.
- [internal/handler/message_filter_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/handler/message_filter_test.go:20): `captureTemplateRepo` replaces template repository.

### 3. Non-HTTP (unit/integration without HTTP)

- Domain/util/unit tests: [unit_tests/domain_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/unit_tests/domain_test.go:1), [unit_tests/csv_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/unit_tests/csv_test.go:1), [unit_tests/encryption_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/unit_tests/encryption_test.go:1), [unit_tests/maskpii_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/unit_tests/maskpii_test.go:1).
- Service tests: [internal/service/fulfillment_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/service/fulfillment_test.go:1), [internal/service/messaging_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/service/messaging_test.go:1), [internal/service/encryption_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/service/encryption_test.go:1), plus related service tests in the same directory.
- Repository tests: [internal/repository/repo_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/repository/repo_test.go:1), [internal/repository/repo_extra_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/repository/repo_extra_test.go:1).
- Middleware/config/job tests: [internal/middleware/auth_revalidation_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/middleware/auth_revalidation_test.go:1), [internal/middleware/middleware_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/middleware/middleware_test.go:1), [internal/config/config_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/config/config_test.go:1), job tests under `internal/job/*_test.go`.

## Mock Detection

| What is mocked/stubbed | Where |
|---|---|
| Customer repository and encryption service | [internal/handler/customer_update_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/handler/customer_update_test.go:20) |
| Report export repository | [internal/handler/report_validation_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/handler/report_validation_test.go:23) |
| Backup service | [internal/handler/coverage_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/handler/coverage_test.go:159) |
| Fulfillment, tier, customer, timeline, shipping, and exception repositories | [internal/handler/auditor_masking_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/handler/auditor_masking_test.go:25) |
| Message-template repository capture double | [internal/handler/message_filter_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/handler/message_filter_test.go:20) |
| Frontend globals and timers: `confirm`, `form.submit`, timers | [static/js/app.test.js](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/static/js/app.test.js:58) via `vi.stubGlobal`, `vi.spyOn`, `vi.useFakeTimers` |

## Coverage Summary

- Total backend endpoints audited for API coverage: `64`
- HTTP-tested endpoints: `64`
- Endpoints with true no-mock HTTP tests: `64`
- HTTP coverage: `100%` (`64/64`)
- True API coverage: `100%` for `/api/v1/*` (`63/63`)

## Unit Test Summary

### Backend Unit Tests

- Controllers/handlers covered:
- `CustomerHandler.Update` via [internal/handler/customer_update_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/handler/customer_update_test.go:82)
- `ReportHandler.Create` validation via [internal/handler/report_validation_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/handler/report_validation_test.go:52)
- `AdminHandler.Health` via [internal/handler/admin_health_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/handler/admin_health_test.go:18)
- `PageAdminHandler.PostRestoreBackup`, `PageReportHandler` download/verify paths via [internal/handler/coverage_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/handler/coverage_test.go:59)
- `PageMessageHandler.ListTemplates` filter forwarding via [internal/handler/message_filter_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/handler/message_filter_test.go:44)
- `PageFulfillmentHandler.ShowDetail` masking behavior via [internal/handler/auditor_masking_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/handler/auditor_masking_test.go:168)
- Services covered:
- fulfillment, inventory, transition rules, shipping update, encryption, messaging retry/mark-failed, bootstrap via tests under [internal/service](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/service)
- Repositories covered:
- repository integration/behavior tests in [internal/repository/repo_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/repository/repo_test.go:1) and [internal/repository/repo_extra_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/repository/repo_extra_test.go:1)
- Auth/guards/middleware covered:
- session revalidation and middleware behavior in [internal/middleware/auth_revalidation_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/middleware/auth_revalidation_test.go:1) and [internal/middleware/middleware_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/internal/middleware/middleware_test.go:1)
- Important backend modules not directly unit-tested:
- `internal/handler/tier.go`, `internal/handler/exception.go`, `internal/handler/settings.go`, `internal/handler/user.go`, `internal/handler/auth.go`, `internal/handler/audit.go`
- `internal/service/export.go`, `internal/service/backup.go`, `internal/service/exception.go`, `internal/service/user.go`, `internal/service/sla.go`
- Most page POST/PUT/DELETE handlers in `internal/handler/page_*.go` are only smoke-covered or indirectly covered, not unit-tested.

### Frontend Unit Tests

- Frontend unit tests: PRESENT
- Frontend test files:
- [static/js/app.test.js](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/static/js/app.test.js:1)
- Frameworks/tools detected:
- `vitest`, `jsdom` in [static/js/package.json](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/static/js/package.json:1)
- Components/modules covered:
- actual frontend module [static/js/app.js](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/static/js/app.js:1)
- functions and DOM behaviors: `confirmAction`, `openModal`, `closeModal`, `validateTracking`, backdrop click, Escape close, flash auto-dismiss, required-field submit guard, dashboard auto-refresh
- Important frontend components/modules NOT tested:
- all Templ-rendered pages under `internal/view/**`
- `static/css/app.css`
- real browser rendering and page-to-page interactions
- server-rendered form flows for create/update/delete actions

### Cross-Layer Observation

- This is backend-heavy. Backend API coverage is extensive and mostly real-HTTP. Frontend unit coverage exists, but it is narrow and limited to one static JS file. There is no browser-level end-to-end suite exercising full page flows with JS, form submission, and rendered DOM assertions.

## Tests Check

- API observability:
- Strong for most core API tests. Many tests show exact method/path, payload, status, and selected response fields.
- Weak for some negative/smoke cases that only assert status or “not 200”, for example `TestAdminDRDrillCreate_MissingScheduledFor` at [API_tests/admin_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/admin_api_test.go:214) and `TestSettingsSet_MissingValue` at [API_tests/settings_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/settings_api_test.go:49).
- Success/failure/edge coverage:
- Strong on auth, RBAC, version conflicts, lifecycle transitions, inventory limits, send-log retry behavior, report sensitivity restrictions, soft-delete/restore.
- Weaker on page-layer write routes and browser behaviors.
- Assertions:
- Mostly meaningful on status plus payload fields, role restrictions, and persisted state.
- Some smoke tests are shallow by design, especially page render checks in [API_tests/pages_api_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/API_tests/pages_api_test.go:55) and [e2e_tests/pages_e2e_test.go](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/e2e_tests/pages_e2e_test.go:50).
- `run_tests.sh` check:
- Primary execution is Docker-based and satisfies the stated requirement; see [run_tests.sh](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/run_tests.sh:1).
- Minor leakage: coverage post-processing prefers local `go tool cover` when Go is installed, then falls back to Docker. This is not a runtime test dependency, but it is not purely Docker-contained.
- End-to-end expectations for fullstack:
- Partial only. There are end-to-end HTTP workflow tests and page-render smoke tests, but no real browser FE↔BE automation. The “E2E” suites use `httptest`, not a browser driver. Strong API coverage partially compensates; it does not fully satisfy strict fullstack FE↔BE E2E expectations.

## Test Coverage Score (0-100)

- `84/100`

## Score Rationale

- High score earned by complete backend API route coverage, extensive true no-mock HTTP testing, real database wiring, and good failure-path coverage.
- Score reduced because fullstack expectations are not fully met:
- page-layer write routes are not comprehensively tested through the page surface
- frontend unit coverage is narrow
- no browser-level FE↔BE E2E exists
- several handler-level HTTP tests rely on stubs rather than full app bootstrap

## Key Gaps

- No browser-driven end-to-end suite for the server-rendered UI. Fullstack coverage stops at `httptest` page rendering and static-js unit tests.
- Page POST/PUT/DELETE endpoints are sparsely covered compared with API endpoints.
- Frontend tests cover `static/js/app.js` only; views/templates/CSS and real client-side flows are untested.
- Some negative tests are weakly observable and assert only status shape rather than response body semantics.

## Confidence & Assumptions

- Confidence: high for backend API coverage; medium for page-surface sufficiency.
- Assumptions:
- Coverage accounting treats `/api/v1/*` plus `/healthz` as the backend API surface for the mandatory mapping table.
- Page routes are counted separately as fullstack surface, not mixed into the API coverage percentage.
- Static audit only. No claim is made that the suites currently pass.

# README Audit

## Hard Gate Failures

- None found.

## High Priority Issues

- Fullstack verification is operationally strong, but README overstates test location accuracy. It says API/integration/E2E tests live in `tests/` in [README.md](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/README.md:19); actual directories are `API_tests/`, `e2e_tests/`, `integration/`, and `unit_tests/`.
- Demo credentials are not uniformly ready after `docker-compose up`. Only the administrator is auto-seeded; specialist and auditor require manual post-start API seeding in [README.md](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/README.md:54). This is usable, but it weakens the “demo-ready” experience for a strict reviewer.

## Medium Priority Issues

- Architecture explanation is thin. The README lists directories and stack, but it does not explain key runtime boundaries such as server-rendered pages vs JSON API, session model, scheduler role, or how the static JS layer fits into the system.
- The “Running Tests” section is accurate about Docker-first execution, but it does not mention the separate `integration/` directory explicitly even though integration tests exist.
- The verification flow is admin-centric. It does not provide a concrete verification path for specialist and auditor roles after seeding, despite those roles being documented.

## Low Priority Issues

- Command style is inconsistent: startup uses `docker-compose up`, stop instructions use `docker compose down` in [README.md](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/README.md:28) and [README.md](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/README.md:42).
- “Running Tests” says “All test suites run inside Docker,” but `run_tests.sh` opportunistically uses local `go tool cover` for post-processing when available. The statement is directionally correct but not literally absolute.

## Formatting

- Pass. The markdown is structured, readable, and scannable.

## Startup Instructions

- Pass. `docker-compose up` is present and clearly positioned for backend/fullstack startup in [README.md](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/README.md:25).

## Access Method

- Pass. URL and port are explicit: `http://localhost:8080/auth/login` in [README.md](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/README.md:33).

## Verification Method

- Pass. Concrete curl/UI verification steps are present in [README.md](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/README.md:99).

## Environment Rules

- Pass with note. README explicitly disallows manual runtime installs in [README.md](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/README.md:30). No manual DB setup is required. One-time role seeding is done through the running app API, not direct DB manipulation.

## Demo Credentials

- Pass with note. README provides username/email/password for all roles in [README.md](/Users/yosef/Documents/Projects/TASK-req_7dee0d28cda9/repo/README.md:48), but two roles require manual creation after startup.

## Engineering Quality

- Tech stack clarity: good
- Architecture explanation: moderate
- Testing instructions: good but path accuracy is imperfect
- Security/roles: good
- Workflows/presentation: good

## README Verdict

- `PARTIAL PASS`

## Final Verdicts

- Test Coverage Audit verdict: `STRONG BACKEND API COVERAGE, PARTIAL FULLSTACK COVERAGE`
- README Audit verdict: `PARTIAL PASS`
