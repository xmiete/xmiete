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

package sdk

import (
	"context"
	"github.com/xmiete/xmiete-go-sdk/models"
)

// XMieteClient defines the interface for interacting with the XMiete API.
type XMieteClient interface {
	CreateDeposit(ctx context.Context, deposit models.Deposit) (*models.Deposit, error)
	GetDeposit(ctx context.Context, id string) (*models.Deposit, error)
	ConfirmPledge(ctx context.Context, id string, pledgeDate string) error
	ReleaseDeposit(ctx context.Context, id string, signatureToken string) error
}
