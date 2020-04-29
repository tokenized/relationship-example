package wallet

import (
	"bytes"
	"context"

	"github.com/tokenized/envelope/pkg/golang/envelope"
	"github.com/tokenized/envelope/pkg/golang/envelope/v0"
	"github.com/tokenized/specification/dist/golang/actions"
	"github.com/tokenized/specification/dist/golang/protocol"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/logger"
	"github.com/tokenized/smart-contract/pkg/wire"

	"github.com/pkg/errors"
)

func (w *Wallet) DecryptActionDirect(ctx context.Context, tx *wire.MsgTx,
	index int) (actions.Action, bitcoin.Hash32, error) {

	// Reparse the full envelope message
	env, err := envelope.Deserialize(bytes.NewReader(tx.TxOut[index].PkScript))
	if err != nil {
		return nil, bitcoin.Hash32{}, errors.Wrap(err, "deserlialize envelope")
	}

	if !bytes.Equal(env.PayloadProtocol(), protocol.GetProtocolID(w.cfg.IsTest)) {
		return nil, bitcoin.Hash32{}, protocol.ErrNotTokenized
	}

	payload := env.Payload() // Unencrypted
	var encryptionKey bitcoin.Hash32
	var decrypted []byte

	// Convert to specific version of envelope
	env0, ok := env.(*v0.Message)
	if !ok {
		return nil, bitcoin.Hash32{}, errors.New("Unsupported envelope version")
	}

	logger.Verbose(ctx, "Decrypting %d payloads", env0.EncryptedPayloadCount())

	// Decrypt any payloads possible
	for i := 0; i < env0.EncryptedPayloadCount(); i++ {
		ep := env0.EncryptedPayload(i)
		wasDecrypted := false

		senderPublicKey, err := ep.SenderPublicKey(tx)
		if err != nil {
			return nil, bitcoin.Hash32{}, errors.Wrap(err, "sender public key")
		}

		receiverAddresses, err := ep.ReceiverAddresses(tx)
		if err != nil {
			return nil, bitcoin.Hash32{}, errors.Wrap(err, "receiver scripts")
		}

		// Check if sender key is ours
		senderAddress, err := senderPublicKey.RawAddress()
		if err != nil {
			return nil, bitcoin.Hash32{}, errors.Wrap(err, "sender address")
		}

		address, err := w.FindAddress(ctx, senderAddress)
		if err != nil {
			return nil, bitcoin.Hash32{}, errors.Wrap(err, "find address")
		}

		if address != nil {
			// Decrypt as sender
			logger.Verbose(ctx, "Decrypting payload %d as sender %s", i,
				bitcoin.NewAddressFromRawAddress(senderAddress, w.cfg.Net).String())
			key, err := w.GetKey(ctx, address.KeyType, address.KeyIndex)
			if err != nil {
				return nil, bitcoin.Hash32{}, errors.Wrap(err, "get key")
			}

			for _, ra := range receiverAddresses {
				pubKey, err := ra.GetPublicKey()
				if err != nil {
					continue
				}

				decrypted, encryptionKey, err = ep.SenderDecryptKey(tx, key, pubKey)
				if err != nil {
					return nil, bitcoin.Hash32{}, errors.Wrap(err, "sender decrypt")
				}

				wasDecrypted = true
				payload = append(payload, decrypted...) // append decrypted data
				break
			}

			if !wasDecrypted {
				logger.Warn(ctx, "Couldn't decrypt payload as sender : receiver public key not included")
			} else {
				continue
			}
		}

		// Check if receiver keys are ours
		for _, receiverAddress := range receiverAddresses {
			address, err := w.FindAddress(ctx, receiverAddress)
			if err != nil {
				return nil, bitcoin.Hash32{}, errors.Wrap(err, "find address")
			}

			if address != nil {
				// Decrypt as receiver
				key, err := w.GetKey(ctx, address.KeyType, address.KeyIndex)
				if err != nil {
					return nil, bitcoin.Hash32{}, errors.Wrap(err, "get key")
				}

				decrypted, encryptionKey, err = ep.ReceiverDecryptKey(tx, key)
				if err != nil {
					return nil, bitcoin.Hash32{}, errors.Wrap(err, "receiver decrypt")
				}

				wasDecrypted = true
				payload = append(payload, decrypted...) // append decrypted data
				break
			}
		}

		if !wasDecrypted {
			logger.Warn(ctx, "Couldn't decrypt payload as receiver : receiver key not owned")
		}
	}

	a, err := actions.Deserialize(env.PayloadIdentifier(), payload)
	if err != nil {
		return nil, bitcoin.Hash32{}, errors.Wrap(err, "deserialize action")
	}

	return a, encryptionKey, nil
}
