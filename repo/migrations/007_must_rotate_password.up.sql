-- Add must_rotate_password flag for bootstrap admin forced-rotation flow.
ALTER TABLE users ADD COLUMN must_rotate_password BOOLEAN NOT NULL DEFAULT FALSE;
