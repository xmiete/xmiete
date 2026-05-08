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
	webhookURL := os.Getenv("WEBHOOK_URL")                                          // optional
	issuerURL := envOrDefault("ISSUER_URL", "https://api.xmiete.org")               // OID4VCI base URL

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo, err := db.NewPostgresRepo(ctx, dsn)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer repo.Close()

	srv := api.NewServer(repo, webhookURL, issuerURL)
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
