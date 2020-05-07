package node

import (
	"context"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/logger"
	"github.com/tokenized/smart-contract/pkg/spynode/handlers"
	"github.com/tokenized/smart-contract/pkg/wire"

	"github.com/tokenized/specification/dist/golang/protocol"
)

// Implement spynode Listener interface

// Block add and revert messages.
func (n *Node) HandleBlock(ctx context.Context, msgType int, block *handlers.BlockMessage) error {
	ctx = logger.ContextWithOutLogSubSystem(ctx)

	switch msgType {
	case handlers.ListenerMsgBlock:
		if block.Height > n.blockHeight {
			n.blockHeight = block.Height
			logger.Info(ctx, "New Block (%d) : %s", block.Height, block.Hash.String())
		} else {
			logger.Info(ctx, "Refeed Block (%d) : %s", block.Height, block.Hash.String())
		}
	case handlers.ListenerMsgBlockRevert:
		logger.Info(ctx, "Reverted Block (%d) : %s", block.Height, block.Hash.String())
	}

	val := n.refeedNeeded.Load()
	refeedNeeded, ok := val.(bool)
	if ok && refeedNeeded {
		logger.Info(ctx, "Setting refeed blocks from %d", n.blockHeight-2)
		n.spy.RefeedBlocksFromHeight(ctx, n.blockHeight-2)
		n.refeedNeeded.Store(false)
	}

	return nil
}

// Full message for a transaction broadcast on the network.
// Return true for txs that are relevant to ensure spynode sends further notifications for that tx.
func (n *Node) HandleTx(ctx context.Context, tx *wire.MsgTx) (bool, error) {
	ctx = logger.ContextWithOutLogSubSystem(ctx)

	t, err := n.wallet.GetTx(ctx, *tx.TxHash())
	if err == nil && t != nil {
		return true, nil // already have tx. this happens when reprocessing a block
	}

	for index, input := range tx.TxIn {
		// Check for owned public keys in unlock scripts.
		pubkeys, err := bitcoin.PubKeysFromSigScript(input.SignatureScript)
		if err != nil {
			return false, err
		}
		pkhs := make([]bitcoin.Hash20, 0, len(pubkeys))
		for _, pubkey := range pubkeys {
			pkh, err := bitcoin.NewHash20FromData(pubkey)
			if err != nil {
				return false, err
			}

			pkhs = append(pkhs, *pkh)
		}

		m, ra := n.wallet.AreHashesMonitored(pkhs)
		if m {
			address := bitcoin.NewAddressFromRawAddress(ra, n.cfg.Net)
			logger.Info(ctx, "Found tx input %d for %s : %s", index, address.String(),
				tx.TxHash().String())
			if err := n.PreprocessTx(ctx, tx); err != nil {
				logger.Error(ctx, "Failed preprocess : %s : %s", err, tx.TxHash().String())
			} else {
				n.wallet.AddWireTx(ctx, tx)
			}
			return true, nil
		}

		// Check for owned utxos.
		utxo, err := n.wallet.FindUTXO(ctx, input.PreviousOutPoint.Hash, input.PreviousOutPoint.Index)
		if err != nil {
			return false, err
		}
		if utxo != nil {
			logger.Info(ctx, "Found tx input %d for utxo %s %d : %s", index,
				utxo.UTXO.Hash.String(), utxo.UTXO.Index, tx.TxHash().String())
			if err := n.PreprocessTx(ctx, tx); err != nil {
				logger.Error(ctx, "Failed preprocess : %s : %s", err, tx.TxHash().String())
			} else {
				n.wallet.AddWireTx(ctx, tx)
			}
			return true, nil
		}
	}

	for index, output := range tx.TxOut {
		// Check for owned public keys or public key hashes in locking scripts.
		pkhs, err := bitcoin.PKHsFromLockingScript(output.PkScript)
		if err != nil {
			return false, err
		}

		m, ra := n.wallet.AreHashesMonitored(pkhs)
		if m {
			address := bitcoin.NewAddressFromRawAddress(ra, n.cfg.Net)
			logger.Info(ctx, "Found tx output %d for %s : %s", index, address.String(),
				tx.TxHash().String())
			if err := n.PreprocessTx(ctx, tx); err != nil {
				logger.Error(ctx, "Failed preprocess : %s : %s", err, tx.TxHash().String())
			} else {
				n.wallet.AddWireTx(ctx, tx)
			}
			return true, nil
		}

		// Check for flags for known relationships.
		flag, err := protocol.DeserializeFlagOutputScript(output.PkScript)
		if err == nil {
			r := n.rs.FindRelationshipForFlag(ctx, flag)
			if r != nil {
				logger.Info(ctx, "Found tx output %d for flag %x : %s", index, flag,
					tx.TxHash().String())
				if err := n.PreprocessTx(ctx, tx); err != nil {
					logger.Error(ctx, "Failed preprocess : %s : %s", err, tx.TxHash().String())
				} else {
					n.wallet.AddWireTx(ctx, tx)
				}
				return true, nil
			}
		}
	}

	return false, nil
}

// Tx confirm, cancel, unsafe, and revert messages.
func (n *Node) HandleTxState(ctx context.Context, msgType int, txid bitcoin.Hash32) error {
	ctx = logger.ContextWithOutLogSubSystem(ctx)

	switch msgType {
	case handlers.ListenerMsgTxStateSafe:
		logger.Info(ctx, "Tx Safe : %s", txid.String())
		t, err := n.wallet.GetTx(ctx, txid)
		if err != nil {
			logger.Error(ctx, "Failed get tx : %s : %s", err, txid.String())
		} else if t != nil {
			if err := n.ProcessTx(ctx, t); err != nil {
				logger.Error(ctx, "Failed to process tx : %s", err)
				// TODO Stop the daemon
			}
		} else {
			logger.Info(ctx, "Tx Safe not found : %s", txid.String())
		}

	case handlers.ListenerMsgTxStateConfirm:
		logger.Info(ctx, "Tx Confirmed : %s", txid.String())
		t, err := n.wallet.GetTx(ctx, txid)
		if err != nil {
			logger.Error(ctx, "Failed get tx : %s : %s", err, txid.String())
		} else if t != nil {
			if err := n.ProcessTx(ctx, t); err != nil {
				logger.Error(ctx, "Failed to process tx : %s", err)
				// TODO Stop the daemon
			}
		} else {
			logger.Info(ctx, "Tx Confirm not found : %s", txid.String())
		}

	case handlers.ListenerMsgTxStateCancel:
		logger.Info(ctx, "Canceling tx : %s", txid.String())
		t, err := n.wallet.GetTx(ctx, txid)
		if err != nil {
			logger.Error(ctx, "Failed get tx : %s : %s", err, txid.String())
		} else if t != nil {
			if err := n.RevertTx(ctx, t); err != nil {
				logger.Error(ctx, "Failed to revert tx : %s", err)
				// TODO Stop the daemon
			}
		}

	case handlers.ListenerMsgTxStateUnsafe:
		logger.Info(ctx, "Tx Unsafe : %s", txid.String())
		t, err := n.wallet.GetTx(ctx, txid)
		if err != nil {
			logger.Error(ctx, "Failed get tx : %s : %s", err, txid.String())
		} else if t != nil {
			if err := n.RevertTx(ctx, t); err != nil {
				logger.Error(ctx, "Failed to revert tx : %s", err)
				// TODO Stop the daemon
			}
		}

	case handlers.ListenerMsgTxStateRevert:
		logger.Info(ctx, "Reverting tx : %s", txid.String())
		t, err := n.wallet.GetTx(ctx, txid)
		if err != nil {
			logger.Error(ctx, "Failed get tx : %s : %s", err, txid.String())
		} else if t != nil {
			if err := n.RevertTx(ctx, t); err != nil {
				logger.Error(ctx, "Failed to revert tx : %s", err)
				// TODO Stop the daemon
			}
		}

	}

	return nil
}

// When in sync with network
func (n *Node) HandleInSync(ctx context.Context) error {
	ctx = logger.ContextWithOutLogSubSystem(ctx)
	n.isInSync.Store(true)
	logger.Info(ctx, "In Sync")
	return nil
}
