-- Add first_failed_at to enforce the 30-minute retry window.
-- This timestamp is set once (on the first FAILED transition) and never updated,
-- so the retry policy can compute absolute deadlines without drift.
ALTER TABLE send_logs ADD COLUMN first_failed_at TIMESTAMPTZ;

-- Index to speed up the FAILED-only retry scan.
CREATE INDEX IF NOT EXISTS idx_send_logs_failed_window
    ON send_logs(first_failed_at)
    WHERE status = 'FAILED' AND next_retry_at IS NOT NULL;
