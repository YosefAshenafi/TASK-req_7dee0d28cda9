-- Seed a dedicated system actor used by scheduler-originated writes (e.g.,
-- overdue-exception auto-open) so that opened_by and audit_logs.performed_by
-- have a stable, auditable identity. The account uses a fixed UUID to make the
-- referencing service code deterministic.
--
-- The password_hash is an intentionally invalid bcrypt blob so this account
-- cannot authenticate through the normal login path; is_active=false also
-- prevents session creation. Its sole purpose is FK-valid attribution.
INSERT INTO users (id, username, email, password_hash, role, is_active)
VALUES (
    '00000000-0000-0000-0000-00000000f0f0',
    'system',
    'system@fulfillops.local',
    '!disabled',
    'ADMINISTRATOR',
    FALSE
)
ON CONFLICT (id) DO NOTHING;
