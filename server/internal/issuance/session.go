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
package issuance

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type SessionState string

const (
	SessionStatePending  SessionState = "PENDING"
	SessionStateConsumed SessionState = "CONSUMED"
	SessionStateRevoked  SessionState = "REVOKED"
)

// Session represents a single OID4VCI Pre-Authorized Code issuance session.
// It is created by the bank trigger and consumed when the wallet fetches the credential.
type Session struct {
	ID                string
	DepositID         string
	PreAuthorizedCode string
	AccessToken       string
	Nonce             string
	State             SessionState
	CreatedAt         time.Time
	ExpiresAt         time.Time
	CredentialID      string // set after credential is issued
	ValidUntil        string // ISO 8601 date; pledge end date passed by bank
}

// Store is a thread-safe in-memory SessionStore.
// Suitable for local development and tests; not suitable for production multi-instance deployments.
type Store struct {
	mu      sync.RWMutex
	byID    map[string]*Session
	byCode  map[string]*Session
	byToken map[string]*Session
	byCred  map[string]*Session
}

func NewStore() *Store {
	return &Store{
		byID:    make(map[string]*Session),
		byCode:  make(map[string]*Session),
		byToken: make(map[string]*Session),
		byCred:  make(map[string]*Session),
	}
}

func (s *Store) Create(_ context.Context, depositID, validUntil string) (*Session, error) {
	sess := &Session{
		ID:                uuid.NewString(),
		DepositID:         depositID,
		PreAuthorizedCode: uuid.NewString(),
		Nonce:             uuid.NewString(),
		State:             SessionStatePending,
		CreatedAt:         time.Now().UTC(),
		ExpiresAt:         time.Now().UTC().Add(15 * time.Minute),
		ValidUntil:        validUntil,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[sess.ID] = sess
	s.byCode[sess.PreAuthorizedCode] = sess
	return sess, nil
}

func (s *Store) GetByID(_ context.Context, id string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.byID[id]
	return sess, ok
}

func (s *Store) GetByCode(_ context.Context, code string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.byCode[code]
	return sess, ok
}

func (s *Store) GetByToken(_ context.Context, token string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.byToken[token]
	return sess, ok
}

// ExchangeCodeForToken validates the pre-authorized code and issues a short-lived access token.
// Returns (accessToken, nonce, ok).
func (s *Store) ExchangeCodeForToken(_ context.Context, code string) (string, string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.byCode[code]
	if !ok || sess.State != SessionStatePending || time.Now().After(sess.ExpiresAt) {
		return "", "", false
	}
	token := uuid.NewString()
	nonce := uuid.NewString()
	sess.AccessToken = token
	sess.Nonce = nonce
	s.byToken[token] = sess
	return token, nonce, true
}

// ConsumeByToken validates the access token, marks the session as consumed,
// records the issued credential ID, and returns the session.
func (s *Store) ConsumeByToken(_ context.Context, token, credentialID string) (*Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.byToken[token]
	if !ok || sess.State != SessionStatePending || time.Now().After(sess.ExpiresAt) {
		return nil, false
	}
	sess.State = SessionStateConsumed
	sess.CredentialID = credentialID
	s.byCred[credentialID] = sess
	return sess, true
}

// RevokeByDepositID marks all issued credentials for a deposit as revoked.
// Called when the deposit transitions to RELEASED or CLOSED.
func (s *Store) RevokeByDepositID(_ context.Context, depositID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sess := range s.byCred {
		if sess.DepositID == depositID {
			sess.State = SessionStateRevoked
		}
	}
}

// CredentialStatus returns the status of a credential by its ID.
// Returns ("active"|"revoked"|"unknown", bool found).
func (s *Store) CredentialStatus(_ context.Context, credID string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.byCred[credID]
	if !ok {
		return "unknown", false
	}
	if sess.State == SessionStateRevoked {
		return "revoked", true
	}
	return "active", true
}
