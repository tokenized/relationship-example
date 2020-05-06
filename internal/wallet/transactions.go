package wallet

import (
	"context"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
)

func (w *Wallet) AddTx(ctx context.Context, t Transaction) error {
	w.txLock.Lock()
	defer w.txLock.Unlock()

	w.txs[*t.Itx.Hash] = &t
	return nil
}

func (w *Wallet) GetTx(ctx context.Context, txid bitcoin.Hash32) (Transaction, error) {
	w.txLock.Lock()
	defer w.txLock.Unlock()

	tx, exists := w.txs[txid]
	if exists {
		return *tx, nil
	}

	return Transaction{}, ErrNotFound
}
