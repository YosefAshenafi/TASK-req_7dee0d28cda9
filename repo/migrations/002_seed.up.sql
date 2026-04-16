-- Seed: default system settings and admin user

-- Default business hours settings
INSERT INTO system_settings (key, value) VALUES
    ('business_hours_start', '"08:00"'),
    ('business_hours_end',   '"18:00"'),
    ('business_days',        '[1,2,3,4,5]'),
    ('timezone',             '"America/New_York"')
ON CONFLICT (key) DO NOTHING;

-- Default admin user
-- Password: Admin@FulfillOps1  (bcrypt hash below)
-- Change immediately after first login in production
INSERT INTO users (username, email, password_hash, role, is_active)
VALUES (
    'admin',
    'admin@fulfillops.local',
    '$2a$12$GD5oQyNa42QJ/0k3b8sZH.84OrVY7tJFTR421aZu1B5LZ2n4vzKB6',
    'ADMINISTRATOR',
    TRUE
)
ON CONFLICT (username) DO NOTHING;
