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

// POST /validate
// Public, stateless endpoint.
// Accepts a deposit document and returns a conformance report.
//
// Request body:
//
//	{
//	  "deposit":      { ... deposit object ... },
//	  "monthly_rent": 1000.0   // optional — enables statutory cap checks
//	}
//
// Response: validator.Report — always 200 OK; check report.valid for conformance result.

import (
	"encoding/json"
	"net/http"

	"github.com/xmiete/server/internal/models"
	"github.com/xmiete/server/internal/validator"
)

type validateRequest struct {
	Deposit     models.Deposit `json:"deposit"`
	MonthlyRent float64        `json:"monthly_rent,omitempty"`
}

// Validate runs schema and jurisdiction checks against a submitted deposit document.
// It is public and stateless — no database access, no authentication required.
func (s *Server) Validate(w http.ResponseWriter, r *http.Request) {
	var req validateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON", "BAD_REQUEST")
		return
	}
	report := validator.Validate(&req.Deposit, req.MonthlyRent)
	writeJSON(w, http.StatusOK, report)
}
