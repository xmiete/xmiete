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
package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/xmiete/server/internal/api"
	"github.com/xmiete/server/internal/db"
	"github.com/xmiete/server/internal/issuance"
	"github.com/xmiete/server/internal/mailer"
	"github.com/xmiete/server/internal/models"
)

const testJWTSecret = "conformance-test-secret"

// ── In-memory repository ───────────────────────────────────────────────────────

type memRepo struct {
	mu       sync.Mutex
	deposits map[string]*models.Deposit
}

func newMemRepo() *memRepo {
	return &memRepo{deposits: make(map[string]*models.Deposit)}
}

func (r *memRepo) Create(_ context.Context, d *models.Deposit) (*models.Deposit, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d.ID = uuid.NewString()
	d.Deposit.LifecycleState = models.StateRequested
	d.Meta.Timestamp = time.Now().UTC()
	d.Deposit.History = []models.HistoryEntry{{
		State:     models.StateRequested,
		Timestamp: d.Meta.Timestamp,
		Actor:     "TENANT",
	}}
	cp := *d
	r.deposits[cp.ID] = &cp
	ret := cp
	return &ret, nil
}

func (r *memRepo) GetByID(_ context.Context, id string) (*models.Deposit, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.deposits[id]
	if !ok {
		return nil, db.ErrNotFound
	}
	cp := *d
	return &cp, nil
}

func (r *memRepo) UpdateState(_ context.Context, id string, newState models.LifecycleState, entry models.HistoryEntry, patch func(*models.Deposit)) (*models.Deposit, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	d, ok := r.deposits[id]
	if !ok {
		return nil, db.ErrNotFound
	}
	if patch != nil {
		patch(d)
	}
	d.Deposit.LifecycleState = newState
	entry.Timestamp = time.Now().UTC()
	d.Deposit.History = append(d.Deposit.History, entry)
	cp := *d
	return &cp, nil
}

// ── Test server ────────────────────────────────────────────────────────────────

func newTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	sessions := issuance.NewStore()
	allocator := &issuance.MemIndexAllocator{}
	srv := api.NewServer(newMemRepo(), sessions, allocator, "", "http://localhost:8080", mailer.NoOp{})
	router := api.NewRouter(srv, testJWTSecret)
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)
	return ts, makeJWT()
}

func makeJWT() string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "conformance-test",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	signed, err := token.SignedString([]byte(testJWTSecret))
	if err != nil {
		panic("makeJWT: " + err.Error())
	}
	return signed
}

// ── HTTP helpers ───────────────────────────────────────────────────────────────

// doJSON sends a JSON request and returns the response without consuming the body.
func doJSON(t *testing.T, ts *httptest.Server, method, path string, body any, token string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}
	req, err := http.NewRequest(method, ts.URL+path, &buf)
	if err != nil {
		t.Fatalf("new request %s %s: %v", method, path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

// doForm sends an application/x-www-form-urlencoded POST request.
func doForm(t *testing.T, ts *httptest.Server, path string, values url.Values) *http.Response {
	t.Helper()
	resp, err := http.PostForm(ts.URL+path, values)
	if err != nil {
		t.Fatalf("POST form %s: %v", path, err)
	}
	return resp
}

// mustDo asserts HTTP status and drains+closes the body.
func mustDo(t *testing.T, resp *http.Response, wantStatus int) {
	t.Helper()
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		var errBody map[string]any
		json.NewDecoder(resp.Body).Decode(&errBody) //nolint:errcheck
		t.Fatalf("HTTP %d, want %d — %v", resp.StatusCode, wantStatus, errBody)
	}
}

// mustDoJSON asserts HTTP status, decodes the body into v, and closes the body.
// Pass v=nil to just drain without decoding.
func mustDoJSON(t *testing.T, resp *http.Response, wantStatus int, v any) {
	t.Helper()
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		var errBody map[string]any
		json.NewDecoder(resp.Body).Decode(&errBody) //nolint:errcheck
		t.Fatalf("HTTP %d, want %d — %v", resp.StatusCode, wantStatus, errBody)
	}
	if v != nil {
		if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
			t.Fatalf("decode response body: %v", err)
		}
	}
}

// ── Test fixtures ──────────────────────────────────────────────────────────────

// minDeposit returns a minimal valid deposit body.
func minDeposit(depositType, currency string, amount float64) map[string]any {
	return map[string]any{
		"meta":     map[string]any{"version": "2.2.0"},
		"tenant":   map[string]any{"first_name": "Test", "last_name": "Tenant", "email": "tenant@example.com"},
		"landlord": map[string]any{"name": "Test Landlord GmbH"},
		"property": map[string]any{
			"address": map[string]any{"street": "Teststr. 1", "zip": "10115", "city": "Berlin", "country": "DE"},
		},
		"deposit": map[string]any{"amount": amount, "currency": currency, "type": depositType},
	}
}

// ── Shared lifecycle helpers ───────────────────────────────────────────────────

// createDeposit POSTs a new deposit and returns its ID.
func createDeposit(t *testing.T, ts *httptest.Server, token string, body map[string]any) string {
	t.Helper()
	var d map[string]any
	mustDoJSON(t, doJSON(t, ts, http.MethodPost, "/v1/deposits", body, token), http.StatusCreated, &d)
	id, _ := d["id"].(string)
	if id == "" {
		t.Fatal("missing id in deposit creation response")
	}
	return id
}

// getDepositState GETs the deposit and returns its lifecycle state.
func getDepositState(t *testing.T, ts *httptest.Server, token, id string) models.LifecycleState {
	t.Helper()
	var d struct {
		Deposit struct {
			LifecycleState string `json:"lifecycle_state"`
		} `json:"deposit"`
	}
	mustDoJSON(t, doJSON(t, ts, http.MethodGet, "/v1/deposits/"+id, nil, token), http.StatusOK, &d)
	return models.LifecycleState(d.Deposit.LifecycleState)
}

// requireState fails the test if the deposit is not in the expected state.
func requireState(t *testing.T, ts *httptest.Server, token, id string, want models.LifecycleState) {
	t.Helper()
	got := getDepositState(t, ts, token, id)
	if got != want {
		t.Fatalf("deposit %s: state got %q, want %q", id, got, want)
	}
}

// createAndPledge creates a deposit, verifies eID, and pledges it. Returns the deposit ID.
func createAndPledge(t *testing.T, ts *httptest.Server, token string) string {
	t.Helper()
	id := createDeposit(t, ts, token, minDeposit("CASH_EQUIVALENT", "EUR", 1500.0))
	mustDo(t,
		doJSON(t, ts, http.MethodPatch, "/v1/deposits/"+id+"/identity",
			map[string]any{"eid_status": "VERIFIED"}, token),
		http.StatusOK)
	mustDo(t,
		doJSON(t, ts, http.MethodPost, "/v1/deposits/"+id+"/pledge",
			map[string]any{"pledge_date": "2026-06-26", "is_confirmed_by_bank": true}, token),
		http.StatusOK)
	return id
}
