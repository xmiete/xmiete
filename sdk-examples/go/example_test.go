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
