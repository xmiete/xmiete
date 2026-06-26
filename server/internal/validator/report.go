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
package validator

// Severity classifies the impact of a finding.
type Severity string

const (
	SeverityError   Severity = "ERROR"   // conformance failure — instance is non-compliant
	SeverityWarning Severity = "WARNING" // likely problem, review recommended
	SeverityInfo    Severity = "INFO"    // informational only
)

// Finding is a single validation result.
type Finding struct {
	Code     string   `json:"code"`
	Severity Severity `json:"severity"`
	Field    string   `json:"field,omitempty"`
	Message  string   `json:"message"`
	Rule     string   `json:"rule,omitempty"` // statutory or schema reference
}

// Report is the full conformance report returned by POST /validate.
type Report struct {
	Valid         bool      `json:"valid"`                    // false if any ERROR finding is present
	Jurisdiction  string    `json:"jurisdiction,omitempty"`   // ISO 3166-1 alpha-2 derived from property.address.country
	SchemaVersion string    `json:"schema_version,omitempty"` // from meta.version
	Findings      []Finding `json:"findings"`
	CheckedAt     string    `json:"checked_at"`
}

func (r *Report) add(sev Severity, code, field, message, rule string) {
	r.Findings = append(r.Findings, Finding{
		Code:     code,
		Severity: sev,
		Field:    field,
		Message:  message,
		Rule:     rule,
	})
}

func (r *Report) hasErrors() bool {
	for _, f := range r.Findings {
		if f.Severity == SeverityError {
			return true
		}
	}
	return false
}
