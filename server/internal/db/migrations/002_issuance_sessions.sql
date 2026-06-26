-- XMiete Core Server — OID4VCI issuance session store
-- Run with: psql $DATABASE_URL -f 002_issuance_sessions.sql

CREATE TABLE IF NOT EXISTS issuance_sessions (
    id                  UUID        PRIMARY KEY,
    deposit_id          UUID        NOT NULL REFERENCES deposits(id) ON DELETE CASCADE,
    pre_authorized_code TEXT        NOT NULL UNIQUE,
    access_token        TEXT        UNIQUE,
    nonce               TEXT,
    state               TEXT        NOT NULL DEFAULT 'PENDING',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at          TIMESTAMPTZ NOT NULL,
    credential_id       TEXT        UNIQUE,
    valid_until         TEXT
);

CREATE INDEX IF NOT EXISTS idx_sessions_deposit_id ON issuance_sessions(deposit_id);
CREATE INDEX IF NOT EXISTS idx_sessions_state       ON issuance_sessions(state);
