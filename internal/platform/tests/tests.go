package tests

import (
	"context"
	"crypto/rand"

	"github.com/tokenized/relationship-example/internal/platform/config"
	"github.com/tokenized/relationship-example/internal/wallet"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/inspector"
	"github.com/tokenized/smart-contract/pkg/logger"
	"github.com/tokenized/smart-contract/pkg/rpcnode"
	"github.com/tokenized/smart-contract/pkg/spynode"
	"github.com/tokenized/smart-contract/pkg/txbuilder"
	"github.com/tokenized/smart-contract/pkg/wire"

	"github.com/tokenized/specification/dist/golang/actions"

	"github.com/pkg/errors"
)

func Context() context.Context {
	logConfig := logger.NewDevelopmentConfig()
	logConfig.Main.Format |= logger.IncludeSystem
	logConfig.Main.MinLevel = logger.LevelVerbose

	logConfig.EnableSubSystem(rpcnode.SubSystem)
	logConfig.EnableSubSystem(spynode.SubSystem)
	logConfig.EnableSubSystem(txbuilder.SubSystem)

	logConfig.IsText = true

	return logger.ContextWithLogConfig(context.Background(), logConfig)
}

func NewMockConfig() *config.Config {
	return &config.Config{
		Entity: actions.EntityField{
			Name: "Test Wallet",
		},
		Net:        bitcoin.MainNet,
		IsTest:     true,
		DustLimit:  546,
		FeeRate:    1.0,
		AddressGap: 5,
		WalletPath: "m/7400'/0'/0'/0",
	}
}

func NewMockWallet(ctx context.Context, cfg *config.Config) (*wallet.Wallet, error) {
	xkey, err := bitcoin.GenerateMasterExtendedKey()
	if err != nil {
		return nil, errors.Wrap(err, "generate xkey")
	}

	result, err := wallet.NewWallet(cfg, xkey.String())
	if err != nil {
		return nil, errors.Wrap(err, "new wallet")
	}

	if err := result.Prepare(ctx); err != nil {
		return nil, errors.Wrap(err, "prepare wallet")
	}

	ra, err := result.GetUnusedRawAddress(ctx, wallet.KeyTypeExternal)
	if err != nil {
		return nil, errors.Wrap(err, "get address")
	}

	script, err := ra.LockingScript()
	if err != nil {
		return nil, errors.Wrap(err, "ra script")
	}

	ad, err := result.FindAddress(ctx, ra)
	if err != nil {
		return nil, errors.Wrap(err, "find address")
	}
	if ad == nil {
		return nil, errors.New("Failed to find address")
	}

	var rhash bitcoin.Hash32
	rand.Read(rhash[:])

	logger.Info(ctx, "Mock funding tx : %s", rhash.String())

	err = result.CreateUTXO(ctx, &wallet.UTXO{
		UTXO: bitcoin.UTXO{
			Hash:          rhash,
			Index:         1,
			Value:         100000,
			LockingScript: script,
		},
		KeyType:  ad.KeyType,
		KeyIndex: ad.KeyIndex,
	})

	result.MarkAddress(ctx, ad)

	return result, nil
}

type MockBroadcaster struct {
	cfg  *config.Config
	Msgs []*wire.MsgTx
}

func NewMockBroadcaster(cfg *config.Config) *MockBroadcaster {
	return &MockBroadcaster{
		cfg:  cfg,
		Msgs: make([]*wire.MsgTx, 0),
	}
}

func (mb *MockBroadcaster) BroadcastTx(ctx context.Context, tx *wire.MsgTx) error {
	logger.Info(ctx, "Mock broadcasting tx : \n%s\n", tx.StringWithAddresses(mb.cfg.Net))
	mb.Msgs = append(mb.Msgs, tx)
	return nil
}

func CreateInspector(ctx context.Context, cfg *config.Config, tx *wire.MsgTx, txs []*wire.MsgTx) (*inspector.Transaction, error) {
	itx, err := inspector.NewTransactionFromWire(ctx, tx, cfg.IsTest)
	if err != nil {
		return nil, err
	}

	itx.Inputs = nil
	for _, input := range tx.TxIn {
		ra, err := bitcoin.RawAddressFromUnlockingScript(input.SignatureScript)
		if err == nil {
			itx.Inputs = append(itx.Inputs, inspector.Input{Address: ra})
		} else {
			itx.Inputs = append(itx.Inputs, inspector.Input{})
		}
	}

	itx.Outputs = nil
	for _, output := range tx.TxOut {
		ra, err := bitcoin.RawAddressFromLockingScript(output.PkScript)
		if err == nil {
			itx.Outputs = append(itx.Outputs, inspector.Output{Address: ra})
		} else {
			itx.Outputs = append(itx.Outputs, inspector.Output{})
		}
	}

	return itx, nil
}
