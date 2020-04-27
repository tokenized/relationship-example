package wallet

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/logger"
	"github.com/tokenized/smart-contract/pkg/wire"

	"github.com/pkg/errors"
)

func (w *Wallet) CreateUTXO(ctx context.Context, utxo UTXO) error {
	w.utxoLock.Lock()
	defer w.utxoLock.Unlock()

	utxos, exists := w.utxos[utxo.UTXO.Hash]
	if !exists {
		w.utxos[utxo.UTXO.Hash] = []UTXO{utxo}
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
			return &utxo, nil
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
			return &utxo, nil
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
				address := w.FindAddress(ctx, utxo.KeyType, utxo.KeyIndex)
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
				address := w.FindAddress(ctx, utxo.KeyType, utxo.KeyIndex)
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
		address, err := w.FindAddressByAddress(ctx, ra)
		if err != nil {
			logger.Error(ctx, "Failed to find address : %s", err)
		}
		if address == nil {
			continue
		}

		if output.Value > 0 {
			// Add the UTXO
			utxo := UTXO{
				UTXO: bitcoin.UTXO{
					Hash:          *tx.TxHash(),
					Index:         uint32(index),
					Value:         output.Value,
					LockingScript: output.PkScript,
				},
				KeyType:  address.KeyType,
				KeyIndex: address.KeyIndex,
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

func (u UTXO) Serialize(buf *bytes.Buffer) error {
	// Version
	if err := binary.Write(buf, binary.LittleEndian, uint8(0)); err != nil {
		return errors.Wrap(err, "version")
	}

	if err := u.UTXO.Write(buf); err != nil {
		return errors.Wrap(err, "utxo")
	}

	if err := binary.Write(buf, binary.LittleEndian, u.KeyType); err != nil {
		return errors.Wrap(err, "type")
	}

	if err := binary.Write(buf, binary.LittleEndian, u.KeyIndex); err != nil {
		return errors.Wrap(err, "index")
	}

	if err := binary.Write(buf, binary.LittleEndian, u.Reserved); err != nil {
		return errors.Wrap(err, "reserved")
	}

	return nil
}

func (u *UTXO) Deserialize(buf *bytes.Reader) error {
	var version uint8
	if err := binary.Read(buf, binary.LittleEndian, &version); err != nil {
		return errors.Wrap(err, "version")
	}

	if version != 0 {
		return fmt.Errorf("Unsupported version : %d", version)
	}

	if err := u.UTXO.Read(buf); err != nil {
		return errors.Wrap(err, "utxo")
	}

	if err := binary.Read(buf, binary.LittleEndian, &u.KeyType); err != nil {
		return errors.Wrap(err, "type")
	}

	if err := binary.Read(buf, binary.LittleEndian, &u.KeyIndex); err != nil {
		return errors.Wrap(err, "index")
	}

	if err := binary.Read(buf, binary.LittleEndian, &u.Reserved); err != nil {
		return errors.Wrap(err, "reserved")
	}

	return nil
}
