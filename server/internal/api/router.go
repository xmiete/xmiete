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

			// QEAA issuance trigger — called by bank after pledge
			r.Post("/deposits/{id}/issue-credential", s.IssueCredential)
		})
	})

	return r
}
