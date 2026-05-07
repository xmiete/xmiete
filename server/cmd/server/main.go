package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/xmiete/server/internal/api"
	"github.com/xmiete/server/internal/db"
)

func main() {
	dsn := mustEnv("DATABASE_URL")
	jwtSecret := mustEnv("JWT_SECRET")
	port := envOrDefault("PORT", "8080")
	webhookURL := os.Getenv("WEBHOOK_URL") // optional

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo, err := db.NewPostgresRepo(ctx, dsn)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer repo.Close()

	srv := api.NewServer(repo, webhookURL)
	router := api.NewRouter(srv, jwtSecret)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("xmiete-server listening on %s", addr)

	httpSrv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	if err := httpSrv.ListenAndServe(); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required env var %s is not set", key)
	}
	return v
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
