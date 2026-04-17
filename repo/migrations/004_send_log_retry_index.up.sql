-- Index to speed up retry scans for FAILED send_logs.
-- The original idx_send_logs_status_retry was scoped to status='QUEUED', but
-- the retry policy now also sweeps FAILED rows waiting for their next_retry_at.

CREATE INDEX IF NOT EXISTS idx_send_logs_failed_retry
    ON send_logs(status, next_retry_at)
    WHERE status = 'FAILED';
