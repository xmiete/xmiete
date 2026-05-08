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
package db

import (
	"context"

	"github.com/xmiete/server/internal/models"
)

// Repository is the storage abstraction for deposits.
// Swap the postgres implementation with any other backend by satisfying this interface.
type Repository interface {
	Create(ctx context.Context, d *models.Deposit) (*models.Deposit, error)
	GetByID(ctx context.Context, id string) (*models.Deposit, error)
	UpdateState(ctx context.Context, id string, newState models.LifecycleState, entry models.HistoryEntry, patch func(*models.Deposit)) (*models.Deposit, error)
}
