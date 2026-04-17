DROP TRIGGER IF EXISTS trg_audit_logs_no_update ON audit_logs;
DROP TRIGGER IF EXISTS trg_audit_logs_no_delete ON audit_logs;
DROP FUNCTION IF EXISTS audit_logs_deny_mutation();

-- Restore permissive defaults (if needed).
GRANT UPDATE, DELETE ON audit_logs TO PUBLIC;
