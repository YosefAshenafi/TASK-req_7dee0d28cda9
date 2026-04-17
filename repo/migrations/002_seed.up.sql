-- Seed: default system settings only.
-- The initial administrator account is created at first startup via the
-- FULFILLOPS_BOOTSTRAP_ADMIN_EMAIL / FULFILLOPS_BOOTSTRAP_ADMIN_PASSWORD
-- environment variables — no hard-coded credentials are stored here.

-- Default business hours settings
INSERT INTO system_settings (key, value) VALUES
    ('business_hours_start', '"08:00"'),
    ('business_hours_end',   '"18:00"'),
    ('business_days',        '[1,2,3,4,5]'),
    ('timezone',             '"America/New_York"')
ON CONFLICT (key) DO NOTHING;
