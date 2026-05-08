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
package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/xmiete/server/internal/api/middleware"
)

func NewRouter(s *Server, jwtSecret string) http.Handler {
	r := chi.NewRouter()

	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.SetHeader("Content-Type", "application/json"))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
	})

	// OID4VCI well-known discovery — public, no auth
	r.Get("/.well-known/openid-credential-issuer", s.IssuerMetadata)
	r.Get("/.well-known/jwks.json", s.JWKS)

	r.Route("/v1", func(r chi.Router) {
		// OID4VCI wallet-facing endpoints — authenticated by their own tokens, not JWT
		r.Get("/credential-offers/{sessionId}", s.GetCredentialOffer)
		r.Post("/token", s.Token)
		r.Post("/credential", s.Credential)
		r.Get("/credentials/{credentialId}/status", s.CredentialStatus)

		// Deposit lifecycle — JWT-authenticated
		r.Group(func(r chi.Router) {
			r.Use(middleware.JWTMiddleware(jwtSecret))

			r.Post("/deposits", s.CreateDeposit)
			r.Get("/deposits/{id}", s.GetDeposit)
			r.Patch("/deposits/{id}/identity", s.UpdateIdentity)
			r.Post("/deposits/{id}/pledge", s.Pledge)
			r.Post("/deposits/{id}/release", s.Release)
			r.Post("/deposits/{id}/claim", s.Claim)

			// PDF receipt — fallback for tenants without an EUDI wallet (available from PLEDGED onwards)
			r.Get("/deposits/{id}/receipt", s.GetReceipt)

			// QEAA issuance trigger — called by bank after pledge
			r.Post("/deposits/{id}/issue-credential", s.IssueCredential)

			// OpenID4VP — landlord initiates presentation request (JWT-authenticated)
			r.Post("/deposits/{id}/vp-request", s.CreateVpRequest)
		})

		// OpenID4VP wallet response — no JWT; wallet self-authenticates via KB-JWT in vp_token
		r.Post("/deposits/{id}/vp-response", s.ReceiveVpResponse)
	})

	return r
}
