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

	r.Route("/v1", func(r chi.Router) {
		r.Use(middleware.JWTMiddleware(jwtSecret))

		r.Post("/deposits", s.CreateDeposit)
		r.Get("/deposits/{id}", s.GetDeposit)
		r.Patch("/deposits/{id}/identity", s.UpdateIdentity)
		r.Post("/deposits/{id}/pledge", s.Pledge)
		r.Post("/deposits/{id}/release", s.Release)
		r.Post("/deposits/{id}/claim", s.Claim)
	})

	return r
}
