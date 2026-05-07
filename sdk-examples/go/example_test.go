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
	"github.com/xmiete/xmiete-go-sdk/models"
)

type MockClient struct{}

func (m *MockClient) CreateDeposit(ctx context.Context, d models.Deposit) (*models.Deposit, error) {
	return &d, nil
}

func (m *MockClient) GetDeposit(ctx context.Context, id string) (*models.Deposit, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *MockClient) ConfirmPledge(ctx context.Context, id string, date string) error {
	fmt.Printf("Mock: Confirming pledge for %s on %s\n", id, date)
	return nil
}

func (m *MockClient) ReleaseDeposit(ctx context.Context, id string, token string) error {
	return nil
}

func main() {
	client := &MockClient{}
	ctx := context.Background()

	fmt.Println("XMiete Go SDK Example Usage")

	err := client.ConfirmPledge(ctx, "DEP-789", "2026-05-07")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Println("Successfully initiated pledge confirmation.")
}
