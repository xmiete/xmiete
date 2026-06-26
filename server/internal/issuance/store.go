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

import "context"

// SessionStore is the storage abstraction for OID4VCI issuance sessions.
// The in-memory Store satisfies this interface for testing and local development.
// Production deployments use a DB-backed implementation (db.PostgresSessionStore).
type SessionStore interface {
	Create(ctx context.Context, depositID, validUntil string) (*Session, error)
	GetByID(ctx context.Context, id string) (*Session, bool)
	GetByCode(ctx context.Context, code string) (*Session, bool)
	GetByToken(ctx context.Context, token string) (*Session, bool)
	ExchangeCodeForToken(ctx context.Context, code string) (string, string, bool)
	ConsumeByToken(ctx context.Context, token, credentialID string) (*Session, bool)
	RevokeByDepositID(ctx context.Context, depositID string)
	CredentialStatus(ctx context.Context, credID string) (string, bool)
}
