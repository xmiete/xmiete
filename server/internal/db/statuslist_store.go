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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xmiete/server/internal/issuance"
)

// PostgresIndexAllocator atomically allocates status list indices via a single-row counter table.
// The UPDATE … RETURNING pattern is serialisable under Postgres default isolation.
type PostgresIndexAllocator struct {
	pool *pgxpool.Pool
}

func NewPostgresIndexAllocator(pool *pgxpool.Pool) *PostgresIndexAllocator {
	return &PostgresIndexAllocator{pool: pool}
}

// AllocateIndex increments the counter and returns the previously-held value as the new index.
func (a *PostgresIndexAllocator) AllocateIndex(ctx context.Context) (int, error) {
	var idx int
	err := a.pool.QueryRow(ctx,
		`UPDATE status_list_counter SET next_index = next_index + 1 WHERE id = 1 RETURNING next_index - 1`,
	).Scan(&idx)
	return idx, err
}

// Ensure PostgresIndexAllocator satisfies the interface at compile time.
var _ issuance.IndexAllocator = (*PostgresIndexAllocator)(nil)
