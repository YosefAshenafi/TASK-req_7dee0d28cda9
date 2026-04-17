-- Audit immutability: block UPDATE/DELETE on audit_logs at the DB layer so
-- even an attacker with app-level write access cannot rewrite history.
-- The application role is expected to be `fulfillops` (the username embedded
-- in the default DATABASE_URL). Adjust here if that changes.

CREATE OR REPLACE FUNCTION audit_logs_deny_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs is append-only: % is forbidden', TG_OP;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_audit_logs_no_update ON audit_logs;
DROP TRIGGER IF EXISTS trg_audit_logs_no_delete ON audit_logs;

CREATE TRIGGER trg_audit_logs_no_update
BEFORE UPDATE ON audit_logs
FOR EACH ROW EXECUTE FUNCTION audit_logs_deny_mutation();

CREATE TRIGGER trg_audit_logs_no_delete
BEFORE DELETE ON audit_logs
FOR EACH ROW EXECUTE FUNCTION audit_logs_deny_mutation();

-- Also revoke UPDATE/DELETE grants on audit_logs from the application role.
-- PUBLIC is the safest fallback; the app role inherits PUBLIC unless explicitly
-- GRANTed. REVOKE is idempotent.
REVOKE UPDATE, DELETE, TRUNCATE ON audit_logs FROM PUBLIC;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'fulfillops') THEN
        EXECUTE 'REVOKE UPDATE, DELETE, TRUNCATE ON audit_logs FROM fulfillops';
    END IF;
END $$;
