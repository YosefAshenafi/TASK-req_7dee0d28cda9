DROP INDEX IF EXISTS idx_send_logs_dispatch_id;

ALTER TABLE send_logs
  DROP COLUMN IF EXISTS recipient_type,
  DROP COLUMN IF EXISTS dispatch_id;

-- Restore the FK (may fail if orphaned rows exist; operator must clean first).
ALTER TABLE send_logs
  ADD CONSTRAINT send_logs_recipient_id_fkey
    FOREIGN KEY (recipient_id) REFERENCES customers(id);
