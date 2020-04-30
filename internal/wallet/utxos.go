package wallet

import (
	"context"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/logger"
	"github.com/tokenized/smart-contract/pkg/txbuilder"
	"github.com/tokenized/smart-contract/pkg/wire"

	"github.com/pkg/errors"
)

func (w *Wallet) FindUTXO(ctx context.Context, hash bitcoin.Hash32, index uint32) (*UTXO, error) {
	w.utxoLock.Lock()
	defer w.utxoLock.Unlock()

	utxos, exists := w.utxos[hash]
	if !exists {
		return nil, nil
	}

	for _, utxo := range utxos {
		if utxo.UTXO.Index == index {
			return utxo, nil
		}
	}

	return nil, nil
}

func (w *Wallet) GetKeyUTXOs(ctx context.Context, keyType, keyIndex uint32) ([]*UTXO, error) {
	w.utxoLock.Lock()
	defer w.utxoLock.Unlock()

	result := make([]*UTXO, 0)
	for _, utxos := range w.utxos {
		for _, utxo := range utxos {
			if !utxo.Reserved && utxo.KeyHash == nil && utxo.KeyType == keyType &&
				utxo.KeyIndex == keyIndex {
				result = append(result, utxo)
			}
		}
	}

	return result, nil
}

func (w *Wallet) GetKeyHashUTXOs(ctx context.Context, keyType, keyIndex uint32, keyHash bitcoin.Hash32) ([]*UTXO, error) {
	w.utxoLock.Lock()
	defer w.utxoLock.Unlock()

	result := make([]*UTXO, 0)
	for _, utxos := range w.utxos {
		for _, utxo := range utxos {
			if !utxo.Reserved && utxo.KeyHash != nil && keyHash.Equal(utxo.KeyHash) &&
				utxo.KeyType == keyType && utxo.KeyIndex == keyIndex {
				result = append(result, utxo)
			}
		}
	}

	return result, nil
}

func (w *Wallet) GetBitcoinUTXOs(ctx context.Context) ([]*UTXO, error) {
	w.utxoLock.Lock()
	defer w.utxoLock.Unlock()

	// TODO UTXOs with hashes, from relationship derived keys need to be randomly used as bitcoin
	//   funding because those keys should not be reused so there is no point in saving the UTXOs.

	result := make([]*UTXO, 0)
	for _, utxos := range w.utxos {
		for _, utxo := range utxos {
			if !utxo.Reserved && (utxo.KeyType == KeyTypeExternal || utxo.KeyType == KeyTypeInternal) {
				result = append(result, utxo)
			}
		}
	}

	return result, nil
}

func (w *Wallet) GetInputKeys(ctx context.Context, tx *txbuilder.TxBuilder) ([]bitcoin.Key, error) {
	w.utxoLock.Lock()
	defer w.utxoLock.Unlock()

	result := make([]bitcoin.Key, 0, len(tx.Inputs))
	for _, input := range tx.MsgTx.TxIn {
		utxos, exists := w.utxos[input.PreviousOutPoint.Hash]
		if !exists {
			return result, errors.New("UTXO hash not found")
		}
		found := false
		for _, utxo := range utxos {
			if utxo.UTXO.Index == input.PreviousOutPoint.Index {
				found = true
				key, err := w.GetKey(ctx, utxo.KeyType, utxo.KeyIndex)
				if err != nil {
					return result, errors.Wrap(err, "get key")
				}
				if utxo.KeyHash != nil {
					key, err = bitcoin.NextKey(key, *utxo.KeyHash)
					if err != nil {
						return result, errors.Wrap(err, "next key")
					}
				}
				result = append(result, key)
			}
		}
		if !found {
			return result, errors.New("UTXO index not found")
		}
	}

	return result, nil
}

func (w *Wallet) CreateUTXO(ctx context.Context, utxo *UTXO) error {
	w.utxoLock.Lock()
	defer w.utxoLock.Unlock()

	utxos, exists := w.utxos[utxo.UTXO.Hash]
	if !exists {
		w.utxos[utxo.UTXO.Hash] = []*UTXO{utxo}
		return nil
	}

	utxos = append(utxos, utxo)
	w.utxos[utxo.UTXO.Hash] = utxos
	return nil
}

func (w *Wallet) DeleteUTXO(ctx context.Context, hash bitcoin.Hash32, index uint32) (*UTXO, error) {
	w.utxoLock.Lock()
	defer w.utxoLock.Unlock()

	utxos, exists := w.utxos[hash]
	if !exists {
		return nil, nil
	}

	for i, utxo := range utxos {
		if utxo.UTXO.Index == index {
			w.utxos[hash] = append(utxos[:i], utxos[i+1:]...)
			return utxo, nil
		}
	}

	return nil, nil
}

func (w *Wallet) ReserveUTXO(ctx context.Context, hash bitcoin.Hash32, index uint32) (*UTXO, error) {
	w.utxoLock.Lock()
	defer w.utxoLock.Unlock()

	utxos, exists := w.utxos[hash]
	if !exists {
		return nil, nil
	}

	for i, utxo := range utxos {
		if utxo.UTXO.Index == index {
			utxos[i].Reserved = true
			w.utxos[hash] = utxos
			return utxo, nil
		}
	}

	return nil, nil
}

func (w *Wallet) ProcessUTXOs(ctx context.Context, tx *wire.MsgTx, isFinal bool) error {

	// Delete spent UTXOs
	for _, input := range tx.TxIn {
		if isFinal {
			utxo, err := w.DeleteUTXO(ctx, input.PreviousOutPoint.Hash,
				input.PreviousOutPoint.Index)
			if err != nil {
				return errors.Wrap(err, "delete utxo")
			}

			if utxo != nil {
				address := w.GetAddress(ctx, utxo.KeyType, utxo.KeyIndex)
				if address != nil {
					logger.Info(ctx, "Deleted UTXO (%d) : [%s %d %s] %s %d", utxo.UTXO.Value,
						KeyTypeName[utxo.KeyType], utxo.KeyIndex,
						bitcoin.NewAddressFromRawAddress(address.Address, w.cfg.Net).String(),
						utxo.UTXO.Hash.String(), utxo.UTXO.Index)
				} else {
					logger.Info(ctx, "Deleted UTXO (%d) : [%s %d %s] %s %d", utxo.UTXO.Value,
						KeyTypeName[utxo.KeyType], utxo.KeyIndex, "unknown",
						utxo.UTXO.Hash.String(), utxo.UTXO.Index)
				}
			}
		} else {
			utxo, err := w.ReserveUTXO(ctx, input.PreviousOutPoint.Hash,
				input.PreviousOutPoint.Index)
			if err != nil {
				return errors.Wrap(err, "reserve utxo")
			}

			if utxo != nil {
				address := w.GetAddress(ctx, utxo.KeyType, utxo.KeyIndex)
				if address != nil {
					logger.Info(ctx, "Reserved UTXO (%d) : [%s %d %s] %s %d", utxo.UTXO.Value,
						KeyTypeName[address.KeyType], address.KeyIndex,
						bitcoin.NewAddressFromRawAddress(address.Address, w.cfg.Net).String(),
						utxo.UTXO.Hash.String(), utxo.UTXO.Index)
				} else {
					logger.Info(ctx, "Reserved UTXO (%d) : [%s %d %s] %s %d", utxo.UTXO.Value,
						KeyTypeName[address.KeyType], address.KeyIndex, "unknown",
						utxo.UTXO.Hash.String(), utxo.UTXO.Index)
				}
			}
		}
	}

	// Add new UTXOs
	for index, output := range tx.TxOut {
		ra, err := bitcoin.RawAddressFromLockingScript(output.PkScript)
		if err != nil {
			continue
		}

		// Check if we own the address
		address, err := w.FindAddress(ctx, ra)
		if err != nil {
			logger.Error(ctx, "Failed to find address : %s", err)
		}
		if address == nil {
			continue
		}

		if output.Value > 0 {
			// Add the UTXO
			utxo := &UTXO{
				UTXO: bitcoin.UTXO{
					Hash:          *tx.TxHash(),
					Index:         uint32(index),
					Value:         output.Value,
					LockingScript: output.PkScript,
				},
				KeyType:  address.KeyType,
				KeyIndex: address.KeyIndex,
				KeyHash:  address.KeyHash,
			}

			if err := w.CreateUTXO(ctx, utxo); err != nil {
				return errors.Wrap(err, "create utxo")
			}

			logger.Info(ctx, "Created UTXO (%d) : [%s %d %s] %s %d", utxo.UTXO.Value,
				KeyTypeName[address.KeyType], address.KeyIndex,
				bitcoin.NewAddressFromRawAddress(ra, w.cfg.Net).String(), utxo.UTXO.Hash.String(),
				utxo.UTXO.Index)
		}

		if err := w.MarkAddress(ctx, address); err != nil {
			return errors.Wrap(err, "mark address")
		}
	}

	return nil
}
