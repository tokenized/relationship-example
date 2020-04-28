package node

import (
	"bytes"
	"context"

	"github.com/tokenized/envelope/pkg/golang/envelope"
	"github.com/tokenized/envelope/pkg/golang/envelope/v0"

	"github.com/tokenized/smart-contract/pkg/inspector"
	"github.com/tokenized/smart-contract/pkg/logger"

	"github.com/tokenized/specification/dist/golang/actions"
	"github.com/tokenized/specification/dist/golang/messages"

	"github.com/pkg/errors"
)

func (n *Node) ProcessMessage(ctx context.Context, itx *inspector.Transaction, index uint32,
	m *actions.Message) error {

	payload, err := n.DecryptPayload(ctx, itx, itx.MsgTx.TxOut[index].PkScript)
	if err != nil {
		return errors.Wrap(err, "decrypt payload")
	}

	p, err := messages.Deserialize(m.MessageCode, payload)
	if err != nil {
		return errors.Wrap(err, "deserialize message")
	}

	switch payload := p.(type) {
	case *messages.InitiateRelationship:
		return n.ProcessInitiateRelationship(ctx, itx, index, m, payload)
	}

	return nil
}

func (n *Node) DecryptPayload(ctx context.Context, itx *inspector.Transaction, script []byte) ([]byte, error) {

	// TODO Implement flag values for multi-party relationships --ce
	// An encryption secret from outside the tx will likely be used for those.

	// Reparse the full envelope message
	env, err := envelope.Deserialize(bytes.NewReader(script))
	if err != nil {
		return nil, errors.Wrap(err, "get full message")
	}

	payload := env.Payload() // Unencrypted

	// Convert to specific version of envelope
	env0, ok := env.(*v0.Message)
	if !ok {
		return payload, errors.New("Unsupported envelope version")
	}

	// Decrypt any payloads possible
	for i := 0; i < env0.EncryptedPayloadCount(); i++ {
		ep := env0.EncryptedPayload(i)
		wasDecrypted := false

		senderPublicKey, err := ep.SenderPublicKey(itx.MsgTx)
		if err != nil {
			return payload, errors.Wrap(err, "sender public key")
		}

		receiverAddresses, err := ep.ReceiverAddresses(itx.MsgTx)
		if err != nil {
			return payload, errors.Wrap(err, "receiver scripts")
		}

		// Check if sender key is ours
		senderAddress, err := senderPublicKey.RawAddress()
		if err != nil {
			return payload, errors.Wrap(err, "sender address")
		}

		address, err := n.wallet.FindAddress(ctx, senderAddress)
		if err != nil {
			return payload, errors.Wrap(err, "find address")
		}

		if address != nil {
			// Decrypt as sender
			key, err := n.wallet.GetKey(ctx, address.KeyType, address.KeyIndex)
			if err != nil {
				return payload, errors.Wrap(err, "get key")
			}

			for _, ra := range receiverAddresses {
				pubKey, err := ra.GetPublicKey()
				if err != nil {
					continue
				}

				decrypted, err := ep.SenderDecrypt(itx.MsgTx, key, pubKey)
				if err != nil {
					return payload, errors.Wrap(err, "sender decrypt")
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
			address, err := n.wallet.FindAddress(ctx, receiverAddress)
			if err != nil {
				return payload, errors.Wrap(err, "find address")
			}

			if address != nil {
				// Decrypt as receiver
				key, err := n.wallet.GetKey(ctx, address.KeyType, address.KeyIndex)
				if err != nil {
					return payload, errors.Wrap(err, "get key")
				}

				decrypted, err := ep.ReceiverDecrypt(itx.MsgTx, key)
				if err != nil {
					return payload, errors.Wrap(err, "receiver decrypt")
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

	return payload, nil
}
