-- Add recipient_type and dispatch_id to send_logs.
-- recipient_type distinguishes customer-facing from user-facing deliveries.
-- dispatch_id groups all channel outputs from a single dispatch call.
-- The FK on recipient_id is dropped so it can hold either customer or user UUIDs.

ALTER TABLE send_logs
  ADD COLUMN recipient_type VARCHAR(20) NOT NULL DEFAULT 'CUSTOMER'
    CHECK (recipient_type IN ('CUSTOMER', 'USER')),
  ADD COLUMN dispatch_id UUID;

-- Drop the hard FK so recipient_id can reference customers OR users.
ALTER TABLE send_logs DROP CONSTRAINT send_logs_recipient_id_fkey;

CREATE INDEX idx_send_logs_dispatch_id ON send_logs(dispatch_id)
  WHERE dispatch_id IS NOT NULL;
