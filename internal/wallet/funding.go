package wallet

import (
	"context"
	"fmt"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/logger"
	"github.com/tokenized/smart-contract/pkg/txbuilder"
	"github.com/tokenized/smart-contract/pkg/wire"

	"github.com/pkg/errors"
)

type BroadcastTx interface {
	BroadcastTx(context.Context, *wire.MsgTx) error
}

// AddKeyIndexFunding adds inputs to a transaction to fund it. It ensures the next input added is from
//   the key specified by keyType and keyIndex. Sometimes this requires creating a funding tx.
// This also broadcasts any supporting transactions as well as the tx.
func (w *Wallet) AddKeyIndexFunding(ctx context.Context, keyType, keyIndex uint32,
	tx *txbuilder.TxBuilder, broadcastTx BroadcastTx) error {

	utxos, err := w.GetKeyUTXOs(ctx, keyType, keyIndex)
	if err != nil {
		return errors.Wrap(err, "get key utxos")
	}

	if err = tx.AddFunding(ConvertUTXOs(utxos)); err == nil {
		keys, err := w.GetInputKeys(ctx, tx)
		if err != nil {
			return errors.Wrap(err, "get input keys")
		}

		// Sign transaction
		if err := tx.Sign(keys); err != nil {
			return errors.Wrap(err, "sign tx")
		}

		// Broadcast transaction
		if err := broadcastTx.BroadcastTx(ctx, tx.MsgTx); err != nil {
			return errors.Wrap(err, "broadcast funding tx")
		}

		if err := w.ProcessUTXOs(ctx, tx.MsgTx, false); err != nil {
			return errors.Wrap(err, "process utxos")
		}

		return nil
	}

	if !txbuilder.IsErrorCode(errors.Cause(err), txbuilder.ErrorCodeInsufficientValue) {
		return errors.Wrap(err, "add funding")
	}

	if len(tx.Inputs) > 0 {
		// There is at least one UTXO for authorization so just add additional funding from bitcoin
		//   funds
		butxos, err := w.GetBitcoinUTXOs(ctx)
		if err != nil {
			return errors.Wrap(err, "fetch bitcoin utxos")
		}

		if len(butxos) == 0 {
			return errors.New("No bitcoin funding found")
		}

		if err := tx.AddFunding(ConvertUTXOs(butxos)); err != nil {
			return errors.Wrap(err, "fund tx from bitcoin keys")
		}

		keys, err := w.GetInputKeys(ctx, tx)
		if err != nil {
			return errors.Wrap(err, "get input keys")
		}

		// Sign transaction
		if err := tx.Sign(keys); err != nil {
			return errors.Wrap(err, "sign tx")
		}

		// Broadcast transaction
		if err := broadcastTx.BroadcastTx(ctx, tx.MsgTx); err != nil {
			return errors.Wrap(err, "broadcast funding tx")
		}

		if err := w.ProcessUTXOs(ctx, tx.MsgTx, false); err != nil {
			return errors.Wrap(err, "process utxos")
		}

		return nil
	}

	// Create transaction to fund administration address
	fundTx := txbuilder.NewTxBuilder(w.cfg.DustLimit, w.cfg.FeeRate)

	// Get change bitcoin key
	changeAddress, err := w.GetUnusedAddress(ctx, KeyTypeInternal)
	if err != nil {
		return errors.Wrap(err, "get change address")
	}

	logger.Info(ctx, "Using change address %d : %s", changeAddress.KeyIndex,
		bitcoin.NewAddressFromRawAddress(changeAddress.Address, w.cfg.Net).String())

	fundTx.SetChangeAddress(changeAddress.Address, "")

	address := w.GetAddress(ctx, keyType, keyIndex)
	if address == nil {
		return fmt.Errorf("Address not found : %s %d", KeyTypeName[keyType], keyIndex)
	}

	fundingAmount := tx.EstimatedFee() + uint64(float32(txbuilder.MaximumP2PKHInputSize)*w.cfg.FeeRate)*2
	if fundingAmount < w.cfg.DustLimit {
		fundingAmount = 2 * w.cfg.DustLimit
	}

	for _, output := range tx.MsgTx.TxOut {
		fundingAmount += output.Value
	}

	// Add output to address for initial funding
	logger.Info(ctx, "Funding address %0.8f : %d : %s", float64(fundingAmount)/100000000.0,
		address.KeyIndex, bitcoin.NewAddressFromRawAddress(address.Address, w.cfg.Net).String())
	if err := fundTx.AddPaymentOutput(address.Address, fundingAmount, false); err != nil {
		return errors.Wrap(err, "add payment output")
	}

	// Fund transaction
	butxos, err := w.GetBitcoinUTXOs(ctx)
	if err != nil {
		return errors.Wrap(err, "fetch bitcoin utxos")
	}

	if len(butxos) == 0 {
		return errors.New("No bitcoin funding found")
	}

	if err := fundTx.AddFunding(ConvertUTXOs(butxos)); err != nil {
		return errors.Wrap(err, "fund funding tx")
	}

	keys, err := w.GetInputKeys(ctx, fundTx)
	if err != nil {
		return errors.Wrap(err, "get input keys")
	}

	// Sign transaction
	if err := fundTx.Sign(keys); err != nil {
		return errors.Wrap(err, "sign funding tx")
	}

	logger.Info(ctx, "Created funding tx : %s", fundTx.MsgTx.TxHash().String())

	// Broadcast transaction
	if err := broadcastTx.BroadcastTx(ctx, fundTx.MsgTx); err != nil {
		return errors.Wrap(err, "broadcast funding tx")
	}

	if err := w.ProcessUTXOs(ctx, fundTx.MsgTx, false); err != nil {
		return errors.Wrap(err, "process utxos")
	}

	// Fund transaction directly from funding tx above
	if err := tx.AddInput(wire.OutPoint{Hash: *fundTx.MsgTx.TxHash(), Index: 0},
		fundTx.MsgTx.TxOut[0].PkScript, fundTx.MsgTx.TxOut[0].Value); err != nil {
		return errors.Wrap(err, "add funding input")
	}

	keys, err = w.GetInputKeys(ctx, tx)
	if err != nil {
		return errors.Wrap(err, "get input keys")
	}

	// Sign transaction
	if err := tx.Sign(keys); err != nil {
		return errors.Wrap(err, "sign tx")
	}

	// Broadcast transaction
	if err := broadcastTx.BroadcastTx(ctx, tx.MsgTx); err != nil {
		return errors.Wrap(err, "broadcast funding tx")
	}

	if err := w.ProcessUTXOs(ctx, tx.MsgTx, false); err != nil {
		return errors.Wrap(err, "process utxos")
	}

	return nil
}

// AddKeyFunding adds inputs to a transaction to fund it. It ensures the next input added is from
//   the key specified. Sometimes this requires creating a funding tx.
// This also broadcasts any supporting transactions as well as the tx.
func (w *Wallet) AddKeyFunding(ctx context.Context, keyType, keyIndex uint32, keyHash bitcoin.Hash32,
	tx *txbuilder.TxBuilder, broadcastTx BroadcastTx) error {

	utxos, err := w.GetKeyHashUTXOs(ctx, keyType, keyIndex, keyHash)
	if err != nil {
		return errors.Wrap(err, "get key utxos")
	}

	if err = tx.AddFunding(ConvertUTXOs(utxos)); err == nil {
		keys, err := w.GetInputKeys(ctx, tx)
		if err != nil {
			return errors.Wrap(err, "get input keys")
		}

		// Sign transaction
		if err := tx.Sign(keys); err != nil {
			return errors.Wrap(err, "sign tx")
		}

		// Broadcast transaction
		if err := broadcastTx.BroadcastTx(ctx, tx.MsgTx); err != nil {
			return errors.Wrap(err, "broadcast funding tx")
		}

		if err := w.ProcessUTXOs(ctx, tx.MsgTx, false); err != nil {
			return errors.Wrap(err, "process utxos")
		}

		return nil
	}

	if !txbuilder.IsErrorCode(errors.Cause(err), txbuilder.ErrorCodeInsufficientValue) {
		return errors.Wrap(err, "add funding")
	}

	if len(tx.Inputs) > 0 {
		// There is at least one UTXO for authorization so just add additional funding from bitcoin
		//   funds
		butxos, err := w.GetBitcoinUTXOs(ctx)
		if err != nil {
			return errors.Wrap(err, "fetch bitcoin utxos")
		}

		if len(butxos) == 0 {
			return errors.New("No bitcoin funding found")
		}

		if err := tx.AddFunding(ConvertUTXOs(butxos)); err != nil {
			return errors.Wrap(err, "fund tx from bitcoin keys")
		}

		keys, err := w.GetInputKeys(ctx, tx)
		if err != nil {
			return errors.Wrap(err, "get input keys")
		}

		// Sign transaction
		if err := tx.Sign(keys); err != nil {
			return errors.Wrap(err, "sign tx")
		}

		// Broadcast transaction
		if err := broadcastTx.BroadcastTx(ctx, tx.MsgTx); err != nil {
			return errors.Wrap(err, "broadcast funding tx")
		}

		if err := w.ProcessUTXOs(ctx, tx.MsgTx, false); err != nil {
			return errors.Wrap(err, "process utxos")
		}

		return nil
	}

	// Create transaction to fund administration address
	fundTx := txbuilder.NewTxBuilder(w.cfg.DustLimit, w.cfg.FeeRate)

	// Get change bitcoin key
	changeAddress, err := w.GetUnusedAddress(ctx, KeyTypeInternal)
	if err != nil {
		return errors.Wrap(err, "get change address")
	}

	logger.Info(ctx, "Using change address %d : %s", changeAddress.KeyIndex,
		bitcoin.NewAddressFromRawAddress(changeAddress.Address, w.cfg.Net).String())

	fundTx.SetChangeAddress(changeAddress.Address, "")

	key, err := w.GetKey(ctx, keyType, keyIndex)
	if err != nil {
		return errors.Wrap(err, "get key")
	}

	key, err = bitcoin.NextKey(key, keyHash)
	if err != nil {
		return errors.Wrap(err, "next key")
	}

	ra, err := key.RawAddress()
	if err != nil {
		return errors.Wrap(err, "raw address")
	}

	fundingAmount := tx.EstimatedFee() + uint64(float32(txbuilder.MaximumP2PKHInputSize)*w.cfg.FeeRate)*2
	if fundingAmount < w.cfg.DustLimit {
		fundingAmount = 2 * w.cfg.DustLimit
	}

	for _, output := range tx.MsgTx.TxOut {
		fundingAmount += output.Value
	}

	// Add output to address for initial funding
	logger.Info(ctx, "Funding address %0.8f : %s", float64(fundingAmount)/100000000.0,
		bitcoin.NewAddressFromRawAddress(ra, w.cfg.Net).String())
	if err := fundTx.AddPaymentOutput(ra, fundingAmount, false); err != nil {
		return errors.Wrap(err, "add payment output")
	}

	// Fund transaction
	butxos, err := w.GetBitcoinUTXOs(ctx)
	if err != nil {
		return errors.Wrap(err, "fetch bitcoin utxos")
	}

	if len(butxos) == 0 {
		return errors.New("No bitcoin funding found")
	}

	if err := fundTx.AddFunding(ConvertUTXOs(butxos)); err != nil {
		return errors.Wrap(err, "fund funding tx")
	}

	keys, err := w.GetInputKeys(ctx, fundTx)
	if err != nil {
		return errors.Wrap(err, "get input keys")
	}

	// Sign transaction
	if err := fundTx.Sign(keys); err != nil {
		return errors.Wrap(err, "sign funding tx")
	}

	logger.Info(ctx, "Created funding tx : %s", fundTx.MsgTx.TxHash().String())

	// Broadcast transaction
	if err := broadcastTx.BroadcastTx(ctx, fundTx.MsgTx); err != nil {
		return errors.Wrap(err, "broadcast funding tx")
	}

	if err := w.ProcessUTXOs(ctx, fundTx.MsgTx, false); err != nil {
		return errors.Wrap(err, "process utxos")
	}

	// Fund transaction directly from funding tx above
	if err := tx.AddInput(wire.OutPoint{Hash: *fundTx.MsgTx.TxHash(), Index: 0},
		fundTx.MsgTx.TxOut[0].PkScript, fundTx.MsgTx.TxOut[0].Value); err != nil {
		return errors.Wrap(err, "add funding input")
	}

	keys, err = w.GetInputKeys(ctx, tx)
	if err != nil {
		return errors.Wrap(err, "get input keys")
	}

	// Sign transaction
	if err := tx.Sign(keys); err != nil {
		return errors.Wrap(err, "sign tx")
	}

	// Broadcast transaction
	if err := broadcastTx.BroadcastTx(ctx, tx.MsgTx); err != nil {
		return errors.Wrap(err, "broadcast funding tx")
	}

	if err := w.ProcessUTXOs(ctx, tx.MsgTx, false); err != nil {
		return errors.Wrap(err, "process utxos")
	}

	return nil
}

// AddBitcoinFunding adds inputs to a transaction to fund it.
// This also broadcasts any supporting transactions as well as the tx.
func (w *Wallet) AddBitcoinFunding(ctx context.Context, tx *txbuilder.TxBuilder,
	broadcastTx BroadcastTx) error {

	// Fund transaction
	butxos, err := w.GetBitcoinUTXOs(ctx)
	if err != nil {
		return errors.Wrap(err, "fetch bitcoin utxos")
	}

	if len(butxos) == 0 {
		return errors.New("No bitcoin funding found")
	}

	if err := tx.AddFunding(ConvertUTXOs(butxos)); err != nil {
		return errors.Wrap(err, "fund funding tx")
	}

	keys, err := w.GetInputKeys(ctx, tx)
	if err != nil {
		return errors.Wrap(err, "get input keys")
	}

	// Sign transaction
	if err := tx.Sign(keys); err != nil {
		return errors.Wrap(err, "sign tx")
	}

	// Broadcast transaction
	if err := broadcastTx.BroadcastTx(ctx, tx.MsgTx); err != nil {
		return errors.Wrap(err, "broadcast funding tx")
	}

	if err := w.ProcessUTXOs(ctx, tx.MsgTx, false); err != nil {
		return errors.Wrap(err, "process utxos")
	}

	return nil
}
