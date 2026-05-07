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
