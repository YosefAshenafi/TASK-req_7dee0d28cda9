-- Remove seed data
DELETE FROM system_settings WHERE key IN ('business_hours_start','business_hours_end','business_days','timezone');
DELETE FROM users WHERE username = 'admin';
