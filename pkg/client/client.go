package client

import (
	"context"

	"github.com/tokenized/relationship-example/internal/platform/config"
	"github.com/tokenized/relationship-example/internal/platform/db"
	"github.com/tokenized/relationship-example/internal/wallet"
	"github.com/tokenized/smart-contract/pkg/logger"

	"github.com/pkg/errors"
)

type Client struct {
	Config *config.Config
	Wallet *wallet.Wallet
}

func NewClient(ctx context.Context) (*Client, error) {
	result := &Client{}

	// -------------------------------------------------------------------------
	// Configuration

	cfg, err := config.Environment()
	if err != nil {
		return nil, errors.Wrap(err, "get config")
	}

	result.Config, err = cfg.Config()
	if err != nil {
		return nil, errors.Wrap(err, "convert config")
	}

	// -------------------------------------------------------------------------
	// Data

	masterDB, err := db.New(&db.StorageConfig{
		Bucket:     cfg.Storage.Bucket,
		Root:       cfg.Storage.Root,
		MaxRetries: cfg.AWS.MaxRetries,
		RetryDelay: cfg.AWS.RetryDelay,
	})
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize storage : %s", err)
	}

	// -------------------------------------------------------------------------
	// Wallet

	result.Wallet, err = wallet.NewWallet(result.Config, cfg.Key)
	if err != nil {
		return nil, errors.Wrap(err, "new wallet")
	}

	if err := result.Wallet.Load(ctx, masterDB); err != nil {
		return nil, errors.Wrap(err, "load wallet")
	}

	return result, nil
}
