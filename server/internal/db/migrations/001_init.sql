-- XMiete Core Server — initial schema
-- Run with: psql $DATABASE_URL -f 001_init.sql

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS deposits (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    lifecycle_state TEXT        NOT NULL DEFAULT 'REQUESTED',
    data            JSONB       NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Immutable audit log — one row per state transition.
CREATE TABLE IF NOT EXISTS deposit_history (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    deposit_id  UUID        NOT NULL REFERENCES deposits(id) ON DELETE CASCADE,
    state       TEXT        NOT NULL,
    timestamp   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actor       TEXT,
    comment     TEXT,
    signature   TEXT
);

CREATE INDEX IF NOT EXISTS idx_deposits_state     ON deposits(lifecycle_state);
CREATE INDEX IF NOT EXISTS idx_history_deposit_id ON deposit_history(deposit_id);

-- Auto-update updated_at on row change.
CREATE OR REPLACE FUNCTION touch_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;

CREATE TRIGGER deposits_updated_at
    BEFORE UPDATE ON deposits
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
