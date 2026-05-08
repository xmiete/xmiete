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
package mailer

import "github.com/xmiete/server/internal/models"

// Mailer sends deposit PDF documents to the tenant by email.
type Mailer interface {
	SendReceipt(d *models.Deposit, pdf []byte) error
	SendReleaseReceipt(d *models.Deposit, pdf []byte) error
}

// NoOp is used when no SMTP config is provided (e.g. dev/test environments).
type NoOp struct{}

func (NoOp) SendReceipt(_ *models.Deposit, _ []byte) error        { return nil }
func (NoOp) SendReleaseReceipt(_ *models.Deposit, _ []byte) error { return nil }
