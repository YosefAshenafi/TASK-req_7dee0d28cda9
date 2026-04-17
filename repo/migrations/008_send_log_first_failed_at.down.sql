DROP INDEX IF EXISTS idx_send_logs_failed_window;
ALTER TABLE send_logs DROP COLUMN IF EXISTS first_failed_at;
