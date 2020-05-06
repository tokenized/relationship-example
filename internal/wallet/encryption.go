package wallet

import (
	"context"

	"github.com/tokenized/envelope/pkg/golang/envelope"
	"github.com/tokenized/envelope/pkg/golang/envelope/v0"

	"github.com/tokenized/specification/dist/golang/actions"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/logger"
	"github.com/tokenized/smart-contract/pkg/wire"

	"github.com/pkg/errors"
)

func (w *Wallet) DecryptActionDirect(ctx context.Context, tx *wire.MsgTx, index int,
	env envelope.BaseMessage) (actions.Action, bitcoin.Hash32, error) {

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

		if ep.EncryptionType() == 1 {
			continue // Indirect encryption (requires more context)
		}

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

		logger.Info(ctx, "Decrypt checking sender address : %s",
			bitcoin.NewAddressFromRawAddress(senderAddress, w.cfg.Net).String())

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

			if address.KeyHash != nil {
				key, err = bitcoin.NextKey(key, *address.KeyHash)
				if err != nil {
					return nil, bitcoin.Hash32{}, errors.Wrap(err, "next key")
				}
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
			logger.Info(ctx, "Decrypt checking receiver address : %s",
				bitcoin.NewAddressFromRawAddress(receiverAddress, w.cfg.Net).String())

			address, err := w.FindAddress(ctx, receiverAddress)
			if err != nil {
				return nil, bitcoin.Hash32{}, errors.Wrap(err, "find address")
			}

			if address != nil {
				logger.Info(ctx, "Address found")

				// Decrypt as receiver
				key, err := w.GetKey(ctx, address.KeyType, address.KeyIndex)
				if err != nil {
					return nil, bitcoin.Hash32{}, errors.Wrap(err, "get key")
				}

				if address.KeyHash != nil {
					logger.Info(ctx, "Adding hash")
					key, err = bitcoin.NextKey(key, *address.KeyHash)
					if err != nil {
						return nil, bitcoin.Hash32{}, errors.Wrap(err, "next key")
					}
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

func (w *Wallet) DecryptActionIndirect(ctx context.Context, env envelope.BaseMessage,
	encryptionKey bitcoin.Hash32) (actions.Action, error) {

	payload := env.Payload() // Unencrypted
	var decrypted []byte

	// Convert to specific version of envelope
	env0, ok := env.(*v0.Message)
	if !ok {
		return nil, errors.New("Unsupported envelope version")
	}

	logger.Verbose(ctx, "Decrypting %d payloads", env0.EncryptedPayloadCount())

	// Decrypt any payloads possible
	var err error
	for i := 0; i < env0.EncryptedPayloadCount(); i++ {
		ep := env0.EncryptedPayload(i)

		if ep.EncryptionType() != 1 {
			logger.Verbose(ctx, "Decryption not indirect")
			continue // Not indirect encryption
		}

		decrypted, err = ep.IndirectDecrypt(encryptionKey)
		if err != nil {
			return nil, errors.Wrap(err, "indirect decrypt")
		}

		payload = append(payload, decrypted...) // append decrypted data
	}

	a, err := actions.Deserialize(env.PayloadIdentifier(), payload)
	if err != nil {
		return nil, errors.Wrap(err, "deserialize action")
	}

	logger.Info(ctx, "Indirect decryption of %s", a.Code())

	return a, nil
}
