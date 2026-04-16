-- FulfillOps initial schema
-- All tables in foreign-key dependency order

-- ── Users ─────────────────────────────────────────────────────────────────────
CREATE TABLE users (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    username      VARCHAR(100) NOT NULL UNIQUE,
    email         VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role          VARCHAR(30)  NOT NULL CHECK (role IN ('ADMINISTRATOR','FULFILLMENT_SPECIALIST','AUDITOR')),
    is_active     BOOLEAN      NOT NULL DEFAULT TRUE,
    version       INT          NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ── Customers ─────────────────────────────────────────────────────────────────
CREATE TABLE customers (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name              VARCHAR(200) NOT NULL,
    phone_encrypted   BYTEA,
    email_encrypted   BYTEA,
    address_encrypted BYTEA,
    version           INT          NOT NULL DEFAULT 1,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMPTZ,
    deleted_by        UUID REFERENCES users(id)
);

-- ── Reward Tiers ──────────────────────────────────────────────────────────────
CREATE TABLE reward_tiers (
    id              UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(200) NOT NULL,
    description     TEXT,
    inventory_count INT          NOT NULL CHECK (inventory_count >= 0),
    purchase_limit  INT          NOT NULL DEFAULT 2 CHECK (purchase_limit > 0),
    alert_threshold INT          NOT NULL DEFAULT 10 CHECK (alert_threshold >= 0),
    version         INT          NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ,
    deleted_by      UUID REFERENCES users(id)
);

-- ── Fulfillments ──────────────────────────────────────────────────────────────
CREATE TABLE fulfillments (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tier_id             UUID        NOT NULL REFERENCES reward_tiers(id),
    customer_id         UUID        NOT NULL REFERENCES customers(id),
    type                VARCHAR(20) NOT NULL CHECK (type IN ('PHYSICAL','VOUCHER')),
    status              VARCHAR(30) NOT NULL DEFAULT 'DRAFT'
                            CHECK (status IN ('DRAFT','READY_TO_SHIP','SHIPPED','DELIVERED',
                                              'VOUCHER_ISSUED','COMPLETED','ON_HOLD','CANCELED')),
    carrier_name        VARCHAR(100),
    tracking_number     VARCHAR(30),
    voucher_code_encrypted BYTEA,
    voucher_expiration  TIMESTAMPTZ,
    hold_reason         TEXT,
    cancel_reason       TEXT,
    ready_at            TIMESTAMPTZ,
    shipped_at          TIMESTAMPTZ,
    delivered_at        TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    version             INT         NOT NULL DEFAULT 1,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at          TIMESTAMPTZ,
    deleted_by          UUID REFERENCES users(id)
);

-- ── Reservations ──────────────────────────────────────────────────────────────
CREATE TABLE reservations (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tier_id        UUID        NOT NULL REFERENCES reward_tiers(id),
    fulfillment_id UUID        NOT NULL REFERENCES fulfillments(id),
    status         VARCHAR(20) NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE','VOIDED')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Shipping Addresses ────────────────────────────────────────────────────────
CREATE TABLE shipping_addresses (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    fulfillment_id    UUID        NOT NULL UNIQUE REFERENCES fulfillments(id),
    line_1_encrypted  BYTEA       NOT NULL,
    line_2_encrypted  BYTEA,
    city              VARCHAR(100) NOT NULL,
    state             CHAR(2)     NOT NULL,
    zip_code          VARCHAR(10) NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Fulfillment Timeline (append-only) ───────────────────────────────────────
CREATE TABLE fulfillment_timeline (
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    fulfillment_id UUID        NOT NULL REFERENCES fulfillments(id),
    from_status    VARCHAR(30),
    to_status      VARCHAR(30) NOT NULL,
    reason         TEXT,
    metadata       JSONB       NOT NULL DEFAULT '{}',
    changed_by     UUID        REFERENCES users(id),
    changed_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Fulfillment Exceptions ────────────────────────────────────────────────────
CREATE TABLE fulfillment_exceptions (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    fulfillment_id  UUID        NOT NULL REFERENCES fulfillments(id),
    type            VARCHAR(30) NOT NULL CHECK (type IN ('OVERDUE_SHIPMENT','OVERDUE_VOUCHER','MANUAL')),
    status          VARCHAR(30) NOT NULL DEFAULT 'OPEN'
                        CHECK (status IN ('OPEN','INVESTIGATING','ESCALATED','RESOLVED')),
    resolution_note TEXT,
    opened_by       UUID REFERENCES users(id),
    resolved_by     UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Exception Events ──────────────────────────────────────────────────────────
CREATE TABLE exception_events (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    exception_id UUID         NOT NULL REFERENCES fulfillment_exceptions(id),
    event_type   VARCHAR(50)  NOT NULL,
    content      TEXT         NOT NULL,
    created_by   UUID         REFERENCES users(id),
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ── Message Templates ─────────────────────────────────────────────────────────
CREATE TABLE message_templates (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(200) NOT NULL,
    category      VARCHAR(50)  NOT NULL
                      CHECK (category IN ('BOOKING_RESULT','BOOKING_CHANGE','EXPIRATION','FULFILLMENT_PROGRESS')),
    channel       VARCHAR(20)  NOT NULL CHECK (channel IN ('IN_APP','SMS','EMAIL')),
    body_template TEXT         NOT NULL,
    version       INT          NOT NULL DEFAULT 1,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ,
    deleted_by    UUID REFERENCES users(id)
);

-- ── Send Logs ─────────────────────────────────────────────────────────────────
CREATE TABLE send_logs (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    template_id   UUID        REFERENCES message_templates(id),
    recipient_id  UUID        NOT NULL REFERENCES customers(id),
    channel       VARCHAR(20) NOT NULL CHECK (channel IN ('IN_APP','SMS','EMAIL')),
    status        VARCHAR(20) NOT NULL DEFAULT 'QUEUED'
                      CHECK (status IN ('QUEUED','SENT','PRINTED','FAILED')),
    attempt_count INT         NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ,
    printed_by    UUID        REFERENCES users(id),
    printed_at    TIMESTAMPTZ,
    context       JSONB       NOT NULL DEFAULT '{}',
    error_message TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Notifications ─────────────────────────────────────────────────────────────
CREATE TABLE notifications (
    id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID         NOT NULL REFERENCES users(id),
    title      VARCHAR(300) NOT NULL,
    body       TEXT,
    is_read    BOOLEAN      NOT NULL DEFAULT FALSE,
    context    JSONB        NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ── Report Exports ────────────────────────────────────────────────────────────
CREATE TABLE report_exports (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    report_type       VARCHAR(50)  NOT NULL,
    filters           JSONB        NOT NULL DEFAULT '{}',
    file_path         VARCHAR(500),
    file_size_bytes   BIGINT,
    checksum_sha256   VARCHAR(64),
    include_sensitive BOOLEAN      NOT NULL DEFAULT FALSE,
    status            VARCHAR(20)  NOT NULL DEFAULT 'QUEUED'
                          CHECK (status IN ('QUEUED','PROCESSING','COMPLETED','FAILED')),
    expires_at        TIMESTAMPTZ,
    generated_by      UUID         REFERENCES users(id),
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ── System Settings ───────────────────────────────────────────────────────────
CREATE TABLE system_settings (
    id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    key        VARCHAR(100) NOT NULL UNIQUE,
    value      JSONB        NOT NULL,
    updated_by UUID         REFERENCES users(id),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ── Blackout Dates ────────────────────────────────────────────────────────────
CREATE TABLE blackout_dates (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    date        DATE         NOT NULL UNIQUE,
    description VARCHAR(200),
    created_by  UUID         REFERENCES users(id),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ── Job Run History ───────────────────────────────────────────────────────────
CREATE TABLE job_run_history (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    job_name          VARCHAR(100) NOT NULL,
    status            VARCHAR(20)  NOT NULL DEFAULT 'RUNNING'
                          CHECK (status IN ('RUNNING','COMPLETED','FAILED')),
    started_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    finished_at       TIMESTAMPTZ,
    records_processed INT          NOT NULL DEFAULT 0,
    error_stack       TEXT,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ── Audit Logs (append-only) ──────────────────────────────────────────────────
CREATE TABLE audit_logs (
    id           UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    table_name   VARCHAR(100) NOT NULL,
    record_id    UUID,
    operation    VARCHAR(20)  NOT NULL,
    performed_by UUID         REFERENCES users(id),
    before_state JSONB,
    after_state  JSONB,
    ip_address   VARCHAR(45),
    request_id   VARCHAR(100),
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ── Indexes ───────────────────────────────────────────────────────────────────

-- Fulfillments
CREATE INDEX idx_fulfillments_tier_id     ON fulfillments(tier_id);
CREATE INDEX idx_fulfillments_customer_id ON fulfillments(customer_id);
CREATE INDEX idx_fulfillments_status      ON fulfillments(status);
CREATE INDEX idx_fulfillments_created_at  ON fulfillments(created_at);
CREATE INDEX idx_fulfillments_deleted_at  ON fulfillments(deleted_at) WHERE deleted_at IS NOT NULL;

-- Fulfillment timeline
CREATE INDEX idx_timeline_fulfillment_id ON fulfillment_timeline(fulfillment_id);

-- Exceptions
CREATE INDEX idx_exceptions_fulfillment_status ON fulfillment_exceptions(fulfillment_id, status);

-- Exception events
CREATE INDEX idx_exception_events_exception_id ON exception_events(exception_id);

-- Send logs
CREATE INDEX idx_send_logs_recipient_channel ON send_logs(recipient_id, channel);
CREATE INDEX idx_send_logs_status_retry      ON send_logs(status, next_retry_at) WHERE status = 'QUEUED';

-- Notifications
CREATE INDEX idx_notifications_user_read ON notifications(user_id, is_read);

-- Audit logs
CREATE INDEX idx_audit_table_record     ON audit_logs(table_name, record_id);
CREATE INDEX idx_audit_performed_by     ON audit_logs(performed_by, created_at);

-- Reservations
CREATE INDEX idx_reservations_tier_status ON reservations(tier_id, status);

-- Soft-delete queries
CREATE INDEX idx_customers_deleted_at    ON customers(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX idx_tiers_deleted_at        ON reward_tiers(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX idx_templates_deleted_at    ON message_templates(deleted_at) WHERE deleted_at IS NOT NULL;

-- Customers
CREATE INDEX idx_customers_name ON customers(name);

-- Report exports
CREATE INDEX idx_exports_generated_by ON report_exports(generated_by, created_at);
CREATE INDEX idx_exports_expires_at   ON report_exports(expires_at) WHERE status = 'COMPLETED';

-- Job history
CREATE INDEX idx_job_runs_name_started ON job_run_history(job_name, started_at DESC);
