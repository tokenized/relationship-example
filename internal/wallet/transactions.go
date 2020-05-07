package wallet

import (
	"context"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/inspector"
	"github.com/tokenized/smart-contract/pkg/logger"
	"github.com/tokenized/smart-contract/pkg/wire"

	"github.com/pkg/errors"
)

func (w *Wallet) AddWireTx(ctx context.Context, tx *wire.MsgTx) error {
	t := &Transaction{}
	var err error
	t.Itx, err = inspector.NewBaseTransactionFromWire(ctx, tx)
	if err != nil {
		return errors.Wrap(err, "new inspector tx")
	}

	if err := t.Itx.Setup(ctx, w.cfg.IsTest); err != nil {
		return errors.Wrap(err, "setup inspector tx")
	}

	if err := t.Itx.Validate(ctx); err != nil {
		return errors.Wrap(err, "validate inspector tx")
	}

	logger.Info(ctx, "Adding tx : %s", t.Itx.Hash.String())

	w.txLock.Lock()
	defer w.txLock.Unlock()

	w.txs[*t.Itx.Hash] = t
	return nil
}

func (w *Wallet) GetTx(ctx context.Context, txid bitcoin.Hash32) (*Transaction, error) {
	w.txLock.Lock()
	defer w.txLock.Unlock()

	tx, exists := w.txs[txid]
	if exists {
		return tx, nil
	}

	return nil, ErrNotFound
}
