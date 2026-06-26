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
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xmiete/server/internal/models"
)

var ErrNotFound = errors.New("deposit not found")

type PostgresRepo struct {
	pool *pgxpool.Pool
}

func NewPostgresRepo(ctx context.Context, dsn string) (*PostgresRepo, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("db ping: %w", err)
	}
	return &PostgresRepo{pool: pool}, nil
}

func (r *PostgresRepo) Close() {
	r.pool.Close()
}

func (r *PostgresRepo) Pool() *pgxpool.Pool { return r.pool }

func (r *PostgresRepo) Create(ctx context.Context, d *models.Deposit) (*models.Deposit, error) {
	d.ID = uuid.NewString()
	d.Deposit.LifecycleState = models.StateRequested
	d.Meta.Timestamp = time.Now().UTC()

	d.Deposit.History = []models.HistoryEntry{{
		State:     models.StateRequested,
		Timestamp: d.Meta.Timestamp,
		Actor:     "TENANT",
	}}

	raw, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}

	_, err = r.pool.Exec(ctx,
		`INSERT INTO deposits (id, lifecycle_state, data) VALUES ($1, $2, $3)`,
		d.ID, string(models.StateRequested), raw,
	)
	if err != nil {
		return nil, fmt.Errorf("insert deposit: %w", err)
	}

	_, err = r.pool.Exec(ctx,
		`INSERT INTO deposit_history (deposit_id, state, timestamp, actor) VALUES ($1, $2, $3, $4)`,
		d.ID, models.StateRequested, d.Meta.Timestamp, "TENANT",
	)
	if err != nil {
		return nil, fmt.Errorf("insert history: %w", err)
	}

	return d, nil
}

func (r *PostgresRepo) GetByID(ctx context.Context, id string) (*models.Deposit, error) {
	var raw []byte
	err := r.pool.QueryRow(ctx,
		`SELECT data FROM deposits WHERE id = $1`, id,
	).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query deposit: %w", err)
	}

	var d models.Deposit
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("unmarshal deposit: %w", err)
	}
	return &d, nil
}

// UpdateState runs in a transaction: applies patch to the deposit, appends a history entry,
// updates the lifecycle_state column, and persists the mutated JSON blob.
func (r *PostgresRepo) UpdateState(
	ctx context.Context,
	id string,
	newState models.LifecycleState,
	entry models.HistoryEntry,
	patch func(*models.Deposit),
) (*models.Deposit, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var raw []byte
	err = tx.QueryRow(ctx,
		`SELECT data FROM deposits WHERE id = $1 FOR UPDATE`, id,
	).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock deposit: %w", err)
	}

	var d models.Deposit
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, err
	}

	if patch != nil {
		patch(&d)
	}
	d.Deposit.LifecycleState = newState
	entry.Timestamp = time.Now().UTC()
	d.Deposit.History = append(d.Deposit.History, entry)

	updated, err := json.Marshal(&d)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(ctx,
		`UPDATE deposits SET lifecycle_state = $1, data = $2 WHERE id = $3`,
		string(newState), updated, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update deposit: %w", err)
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO deposit_history (deposit_id, state, timestamp, actor, comment, signature)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		id, string(newState), entry.Timestamp, entry.Actor, entry.Comment, entry.Signature,
	)
	if err != nil {
		return nil, fmt.Errorf("insert history: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &d, nil
}
