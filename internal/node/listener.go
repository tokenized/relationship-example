package node

import (
	"context"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/logger"
	"github.com/tokenized/smart-contract/pkg/spynode/handlers"
	"github.com/tokenized/smart-contract/pkg/wire"
)

// Implement spynode Listener interface

// Block add and revert messages.
func (n *Node) HandleBlock(ctx context.Context, msgType int, block *handlers.BlockMessage) error {
	ctx = logger.ContextWithOutLogSubSystem(ctx)

	switch msgType {
	case handlers.ListenerMsgBlock:
		logger.Info(ctx, "New Block (%d) : %s", block.Height, block.Hash.String())
	case handlers.ListenerMsgBlockRevert:
		logger.Info(ctx, "Reverted Block (%d) : %s", block.Height, block.Hash.String())
	}

	return nil
}

// Full message for a transaction broadcast on the network.
// Return true for txs that are relevant to ensure spynode sends further notifications for that tx.
func (n *Node) HandleTx(ctx context.Context, tx *wire.MsgTx) (bool, error) {
	ctx = logger.ContextWithOutLogSubSystem(ctx)

	// TODO Multi-key addresses might not contain our public key if not signed by us. --ce
	// We might need to monitor utxos or something.
	for index, input := range tx.TxIn {
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
			logger.Info(ctx, "Found tx input %d for %s : %s", index, address.String(), tx.TxHash().String())
			n.SetTx(tx)
			if err := n.PreprocessTx(ctx, tx); err != nil {
				logger.Error(ctx, "Failed preprocess : %s : %s", err, tx.TxHash().String())
			}
			return true, nil
		}
	}

	for index, output := range tx.TxOut {
		pkhs, err := bitcoin.PKHsFromLockingScript(output.PkScript)
		if err != nil {
			return false, err
		}

		m, ra := n.wallet.AreHashesMonitored(pkhs)
		if m {
			address := bitcoin.NewAddressFromRawAddress(ra, n.cfg.Net)
			logger.Info(ctx, "Found tx output %d for %s : %s", index, address.String(), tx.TxHash().String())
			n.SetTx(tx)
			if err := n.PreprocessTx(ctx, tx); err != nil {
				logger.Error(ctx, "Failed preprocess : %s : %s", err, tx.TxHash().String())
			}
			return true, nil
		}
	}

	return false, nil
}

// Tx confirm, cancel, unsafe, and revert messages.
func (n *Node) HandleTxState(ctx context.Context, msgType int, txid bitcoin.Hash32) error {
	ctx = logger.ContextWithOutLogSubSystem(ctx)

	switch msgType {
	case handlers.ListenerMsgTxStateSafe:
		fallthrough
	case handlers.ListenerMsgTxStateConfirm:
		tx := n.GetTx(txid)
		if tx != nil {
			n.RemoveTx(txid)
			if err := n.ProcessTx(ctx, tx); err != nil {
				logger.Error(ctx, "Failed to process tx : %s", err)
				// TODO Stop the daemon
			}
		}

	case handlers.ListenerMsgTxStateCancel:
		fallthrough
	case handlers.ListenerMsgTxStateUnsafe:
		logger.Info(ctx, "Canceling tx : %s", txid.String())
		n.RemoveTx(txid)
	case handlers.ListenerMsgTxStateRevert:
		logger.Info(ctx, "Reverted tx : %s", txid.String())
		// TODO Revert
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
