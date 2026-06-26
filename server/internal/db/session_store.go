/*
 * Copyright 2026 XMiete Core Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package db

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xmiete/server/internal/issuance"
)

// PostgresSessionStore is a DB-backed implementation of issuance.SessionStore.
// It is safe for concurrent use and survives server restarts and horizontal scaling.
type PostgresSessionStore struct {
	pool *pgxpool.Pool
}

func NewPostgresSessionStore(pool *pgxpool.Pool) *PostgresSessionStore {
	return &PostgresSessionStore{pool: pool}
}

func (s *PostgresSessionStore) Create(ctx context.Context, depositID, validUntil string) (*issuance.Session, error) {
	sess := &issuance.Session{
		ID:                uuid.NewString(),
		DepositID:         depositID,
		PreAuthorizedCode: uuid.NewString(),
		Nonce:             uuid.NewString(),
		State:             issuance.SessionStatePending,
		CreatedAt:         time.Now().UTC(),
		ExpiresAt:         time.Now().UTC().Add(15 * time.Minute),
		ValidUntil:        validUntil,
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO issuance_sessions
		 (id, deposit_id, pre_authorized_code, nonce, state, created_at, expires_at, valid_until)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		sess.ID, sess.DepositID, sess.PreAuthorizedCode, sess.Nonce,
		string(sess.State), sess.CreatedAt, sess.ExpiresAt, sess.ValidUntil,
	)
	if err != nil {
		return nil, err
	}
	return sess, nil
}

func (s *PostgresSessionStore) GetByID(ctx context.Context, id string) (*issuance.Session, bool) {
	return s.queryOne(ctx, `SELECT id, deposit_id, pre_authorized_code,
		COALESCE(access_token,''), COALESCE(nonce,''), state, created_at, expires_at,
		COALESCE(credential_id,''), COALESCE(valid_until,'')
		FROM issuance_sessions WHERE id = $1`, id)
}

func (s *PostgresSessionStore) GetByCode(ctx context.Context, code string) (*issuance.Session, bool) {
	return s.queryOne(ctx, `SELECT id, deposit_id, pre_authorized_code,
		COALESCE(access_token,''), COALESCE(nonce,''), state, created_at, expires_at,
		COALESCE(credential_id,''), COALESCE(valid_until,'')
		FROM issuance_sessions WHERE pre_authorized_code = $1`, code)
}

func (s *PostgresSessionStore) GetByToken(ctx context.Context, token string) (*issuance.Session, bool) {
	return s.queryOne(ctx, `SELECT id, deposit_id, pre_authorized_code,
		COALESCE(access_token,''), COALESCE(nonce,''), state, created_at, expires_at,
		COALESCE(credential_id,''), COALESCE(valid_until,'')
		FROM issuance_sessions WHERE access_token = $1`, token)
}

// ExchangeCodeForToken atomically validates the pre-authorized code and sets an access token.
// Uses SELECT FOR UPDATE to prevent concurrent exchange of the same code.
func (s *PostgresSessionStore) ExchangeCodeForToken(ctx context.Context, code string) (string, string, bool) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		log.Printf("session ExchangeCodeForToken begin: %v", err)
		return "", "", false
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var id, state string
	var expiresAt time.Time
	err = tx.QueryRow(ctx,
		`SELECT id, state, expires_at FROM issuance_sessions WHERE pre_authorized_code = $1 FOR UPDATE`,
		code,
	).Scan(&id, &state, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", false
	}
	if err != nil {
		log.Printf("session ExchangeCodeForToken query: %v", err)
		return "", "", false
	}
	if state != string(issuance.SessionStatePending) || time.Now().After(expiresAt) {
		return "", "", false
	}

	token := uuid.NewString()
	nonce := uuid.NewString()
	_, err = tx.Exec(ctx,
		`UPDATE issuance_sessions SET access_token = $1, nonce = $2 WHERE id = $3`,
		token, nonce, id,
	)
	if err != nil {
		log.Printf("session ExchangeCodeForToken update: %v", err)
		return "", "", false
	}
	if err := tx.Commit(ctx); err != nil {
		log.Printf("session ExchangeCodeForToken commit: %v", err)
		return "", "", false
	}
	return token, nonce, true
}

// ConsumeByToken atomically validates the access token and marks the session as consumed.
// Uses SELECT FOR UPDATE to prevent double-issuance under concurrent requests.
func (s *PostgresSessionStore) ConsumeByToken(ctx context.Context, token, credentialID string) (*issuance.Session, bool) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		log.Printf("session ConsumeByToken begin: %v", err)
		return nil, false
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var id, depositID, code, nonce, state, validUntil string
	var createdAt, expiresAt time.Time
	err = tx.QueryRow(ctx,
		`SELECT id, deposit_id, pre_authorized_code, COALESCE(nonce,''), state, created_at, expires_at, COALESCE(valid_until,'')
		 FROM issuance_sessions WHERE access_token = $1 FOR UPDATE`,
		token,
	).Scan(&id, &depositID, &code, &nonce, &state, &createdAt, &expiresAt, &validUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false
	}
	if err != nil {
		log.Printf("session ConsumeByToken query: %v", err)
		return nil, false
	}
	if state != string(issuance.SessionStatePending) || time.Now().After(expiresAt) {
		return nil, false
	}

	_, err = tx.Exec(ctx,
		`UPDATE issuance_sessions SET state = $1, credential_id = $2 WHERE id = $3`,
		string(issuance.SessionStateConsumed), credentialID, id,
	)
	if err != nil {
		log.Printf("session ConsumeByToken update: %v", err)
		return nil, false
	}
	if err := tx.Commit(ctx); err != nil {
		log.Printf("session ConsumeByToken commit: %v", err)
		return nil, false
	}

	return &issuance.Session{
		ID:                id,
		DepositID:         depositID,
		PreAuthorizedCode: code,
		AccessToken:       token,
		Nonce:             nonce,
		State:             issuance.SessionStateConsumed,
		CreatedAt:         createdAt,
		ExpiresAt:         expiresAt,
		CredentialID:      credentialID,
		ValidUntil:        validUntil,
	}, true
}

// RevokeByDepositID marks all issued (CONSUMED) credentials for a deposit as REVOKED.
// Called when the deposit transitions to RELEASED or CLOSED.
func (s *PostgresSessionStore) RevokeByDepositID(ctx context.Context, depositID string) {
	_, err := s.pool.Exec(ctx,
		`UPDATE issuance_sessions SET state = $1 WHERE deposit_id = $2 AND state = $3`,
		string(issuance.SessionStateRevoked), depositID, string(issuance.SessionStateConsumed),
	)
	if err != nil {
		log.Printf("session RevokeByDepositID deposit=%s: %v", depositID, err)
	}
}

// CredentialStatus returns "active", "revoked", or "unknown" for a given credential ID.
func (s *PostgresSessionStore) CredentialStatus(ctx context.Context, credID string) (string, bool) {
	var state string
	err := s.pool.QueryRow(ctx,
		`SELECT state FROM issuance_sessions WHERE credential_id = $1`, credID,
	).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return "unknown", false
	}
	if err != nil {
		log.Printf("session CredentialStatus credID=%s: %v", credID, err)
		return "unknown", false
	}
	if issuance.SessionState(state) == issuance.SessionStateRevoked {
		return "revoked", true
	}
	return "active", true
}

// queryOne is a helper that scans a single session row.
func (s *PostgresSessionStore) queryOne(ctx context.Context, q string, arg any) (*issuance.Session, bool) {
	var sess issuance.Session
	var state string
	err := s.pool.QueryRow(ctx, q, arg).Scan(
		&sess.ID, &sess.DepositID, &sess.PreAuthorizedCode,
		&sess.AccessToken, &sess.Nonce, &state,
		&sess.CreatedAt, &sess.ExpiresAt,
		&sess.CredentialID, &sess.ValidUntil,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false
	}
	if err != nil {
		log.Printf("session queryOne: %v", err)
		return nil, false
	}
	sess.State = issuance.SessionState(state)
	return &sess, true
}
