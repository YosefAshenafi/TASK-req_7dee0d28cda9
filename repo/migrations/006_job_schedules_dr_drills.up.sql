-- Persisted job schedules — cadences are now admin-managed, not hard-coded.
CREATE TABLE job_schedules (
  id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
  job_key          VARCHAR(100) NOT NULL UNIQUE,
  interval_seconds INT          CHECK (interval_seconds IS NULL OR interval_seconds > 0),
  daily_hour       INT          CHECK (daily_hour IS NULL OR daily_hour BETWEEN 0 AND 23),
  daily_minute     INT          CHECK (daily_minute IS NULL OR daily_minute BETWEEN 0 AND 59),
  enabled          BOOLEAN      NOT NULL DEFAULT TRUE,
  updated_by       UUID         REFERENCES users(id),
  updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
  version          INT          NOT NULL DEFAULT 1
);

-- Seed with current hard-coded defaults so the scheduler boots unchanged.
INSERT INTO job_schedules (job_key, interval_seconds) VALUES
  ('overdue-check', 900),
  ('notify-retry',  600);

INSERT INTO job_schedules (job_key, daily_hour, daily_minute) VALUES
  ('cleanup',            3,  0),
  ('export-cleanup',     3, 30),
  ('stats',              2,  0),
  ('backup',             1,  0),
  ('scheduled-reports',  2, 30);

CREATE INDEX idx_job_schedules_key ON job_schedules(job_key);

-- Quarterly DR drill tracking.
CREATE TABLE dr_drills (
  id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  scheduled_for TIMESTAMPTZ NOT NULL,
  executed_at   TIMESTAMPTZ,
  executed_by   UUID        REFERENCES users(id),
  outcome       VARCHAR(20) CHECK (outcome IN ('PASS', 'FAIL', 'PENDING')),
  notes         TEXT,
  artifact_path VARCHAR(500),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_dr_drills_scheduled_for ON dr_drills(scheduled_for DESC);
