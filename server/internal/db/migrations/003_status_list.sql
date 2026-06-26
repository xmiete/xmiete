-- Migration 003: W3C Bitstring Status List support
--
-- Adds a status_list_index column to issuance_sessions so each credential is
-- permanently assigned a slot in the 131 072-entry revocation bitstring.
-- A single-row counter table provides atomic, race-free index allocation.

ALTER TABLE issuance_sessions
    ADD COLUMN IF NOT EXISTS status_list_index BIGINT;

CREATE TABLE IF NOT EXISTS status_list_counter (
    id         INT  PRIMARY KEY DEFAULT 1,
    next_index BIGINT NOT NULL  DEFAULT 0,
    CHECK (id = 1)  -- enforces single-row invariant
);

INSERT INTO status_list_counter (id, next_index)
    VALUES (1, 0)
    ON CONFLICT (id) DO NOTHING;
