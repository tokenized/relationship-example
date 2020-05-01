package relationships

import (
	"bytes"
	"context"
	"fmt"

	"github.com/tokenized/envelope/pkg/golang/envelope/v0"

	"github.com/tokenized/relationship-example/internal/wallet"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/inspector"
	"github.com/tokenized/smart-contract/pkg/logger"
	"github.com/tokenized/smart-contract/pkg/txbuilder"

	"github.com/tokenized/specification/dist/golang/actions"
	"github.com/tokenized/specification/dist/golang/messages"
	"github.com/tokenized/specification/dist/golang/protocol"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
)

// AcceptRelationship creates and broadcasts an AcceptRelationship message corresponding to the
//   relationship specified.
// proofOfIdentity needs to be nil, or a proof of identity message like
//   messages.IdentityOracleProofField or messages.PaymailProofField
func (rs *Relationships) AcceptRelationship(ctx context.Context, r *Relationship,
	proofOfIdentity proto.Message) (*messages.AcceptRelationship, error) {

	if r.Accepted {
		return nil, errors.New("Already accepted")
	}

	// Private message fields
	accept := &messages.AcceptRelationship{}

	if proofOfIdentity != nil {
		switch proofOfIdentity.(type) {
		case *messages.IdentityOracleProofField:
			accept.ProofOfIdentityType = 2
		case *messages.PaymailProofField:
			accept.ProofOfIdentityType = 1
		default:
			return nil, errors.New("Unsupported proof of identity type")
		}

		var err error
		accept.ProofOfIdentity, err = proto.Marshal(proofOfIdentity)
		if err != nil {
			return nil, errors.Wrap(err, "marshal proof of identity")
		}
	}

	var acceptBuf bytes.Buffer
	if err := accept.Serialize(&acceptBuf); err != nil {
		return nil, errors.Wrap(err, "serialize accept")
	}

	tx := txbuilder.NewTxBuilder(rs.cfg.DustLimit, rs.cfg.FeeRate)
	senderIndex := uint32(0)

	// Public message fields
	publicMessage := &actions.Message{
		SenderIndexes: []uint32{senderIndex},
	}

	changeAddress, err := rs.wallet.GetUnusedAddress(ctx, wallet.KeyTypeInternal)
	if err != nil {
		return nil, errors.Wrap(err, "get change address")
	}

	logger.Info(ctx, "Using change address %d : %s", changeAddress.KeyIndex,
		bitcoin.NewAddressFromRawAddress(changeAddress.Address, rs.cfg.Net).String())

	if err := tx.SetChangeAddress(changeAddress.Address, ""); err != nil {
		return nil, errors.Wrap(err, "set change address")
	}

	baseKey, err := rs.wallet.GetKey(ctx, r.KeyType, r.KeyIndex)
	if err != nil {
		return nil, errors.Wrap(err, "get key")
	}
	nextKey, err := bitcoin.NextKey(baseKey, r.NextHash)
	if err != nil {
		return nil, errors.Wrap(err, "next key")
	}

	receivers := make([]bitcoin.PublicKey, 0, len(r.Members))
	if r.EncryptionType == 0 { // direct encryption
		for _, m := range r.Members {
			receivers = append(receivers, m.NextKey)

			// Add output to member
			receiverAddress, err := bitcoin.NewRawAddressPublicKey(m.NextKey)
			if err != nil {
				return nil, errors.Wrap(err, "receiver address")
			}

			publicMessage.ReceiverIndexes = append(publicMessage.ReceiverIndexes,
				uint32(len(tx.Outputs)))
			if err := tx.AddDustOutput(receiverAddress, false); err != nil {
				return nil, errors.Wrap(err, "add receiver")
			}
		}
	}

	// Create envelope
	env, err := protocol.WrapAction(publicMessage, rs.cfg.IsTest)
	if err != nil {
		return nil, errors.Wrap(err, "wrap action")
	}

	// Convert to specific version of envelope
	env0, ok := env.(*v0.Message)
	if !ok {
		return nil, errors.New("Unsupported envelope version")
	}

	// Private message fields
	privateMessage := &actions.Message{
		MessageCode:    messages.CodeAcceptRelationship,
		MessagePayload: acceptBuf.Bytes(),
	}

	privatePayload, err := proto.Marshal(privateMessage)
	if err != nil {
		return nil, errors.Wrap(err, "serialize private")
	}

	if r.EncryptionType == 0 { // direct encryption
		if _, err := env0.AddEncryptedPayloadDirect(privatePayload, tx.MsgTx, senderIndex, nextKey,
			receivers); err != nil {
			return nil, errors.Wrap(err, "add direct encrypted payload")
		}
	} else {
		encryptionKey := bitcoin.AddHashes(r.EncryptionKey, r.NextHash)

		if err := env0.AddEncryptedPayloadIndirect(privatePayload, tx.MsgTx, encryptionKey); err != nil {
			return nil, errors.Wrap(err, "add indirect encrypted payload")
		}
	}

	if len(r.Flag) > 0 {
		flagScript, err := protocol.SerializeFlagOutputScript(r.Flag)
		if err != nil {
			return nil, errors.Wrap(err, "serialize flag")
		}
		if err := tx.AddOutput(flagScript, 0, false, false); err != nil {
			return nil, errors.Wrap(err, "add flag op return")
		}
	}

	var scriptBuf bytes.Buffer
	if err := env0.Serialize(&scriptBuf); err != nil {
		return nil, errors.Wrap(err, "serialize envelope")
	}

	if err := tx.AddOutput(scriptBuf.Bytes(), 0, false, false); err != nil {
		return nil, errors.Wrap(err, "add message op return")
	}

	if err := rs.wallet.AddIndependentKey(ctx, nextKey.PublicKey(), r.KeyType, r.KeyIndex,
		r.NextHash); err != nil {
		return nil, errors.Wrap(err, "add independent key")
	}

	logger.Info(ctx, "Adding key funding")
	if err := rs.wallet.AddKeyFunding(ctx, r.KeyType, r.KeyIndex, r.NextHash, tx, rs.broadcastTx); err != nil {
		return nil, errors.Wrap(err, "add key funding")
	}

	// Increment hashes
	if err := r.IncrementHash(ctx, rs.wallet); err != nil {
		return nil, errors.Wrap(err, "increment hash")
	}
	if r.EncryptionType == 0 {
		// Member keys were included in this tx, so increment them too
		for _, m := range r.Members {
			m.IncrementHash()
		}
	}

	r.Accepted = true

	return accept, nil
}

func (rs *Relationships) ProcessAcceptRelationship(ctx context.Context, itx *inspector.Transaction,
	message *actions.Message, accept *messages.AcceptRelationship, flag []byte) error {

	// Find relationship
	r := rs.FindRelationship(ctx, flag)
	if r == nil {
		return ErrUnknownFlag
	}

	if len(message.SenderIndexes) > 1 {
		return fmt.Errorf("More than one sender not supported : %d",
			len(message.SenderIndexes))
	}

	if len(message.SenderIndexes) == 0 { // No sender indexes means use the first input
		message.SenderIndexes = append(message.SenderIndexes, 0)
	}

	for _, senderIndex := range message.SenderIndexes {
		if int(senderIndex) >= len(itx.MsgTx.TxIn) {
			return fmt.Errorf("Sender index out of range : %d/%d", senderIndex,
				len(itx.MsgTx.TxIn))
		}

		pk, err := bitcoin.PublicKeyFromUnlockingScript(itx.MsgTx.TxIn[senderIndex].SignatureScript)
		if err != nil {
			return errors.Wrap(err, "sender parse script")
		}

		publicKey, err := bitcoin.PublicKeyFromBytes(pk)
		if err != nil {
			return errors.Wrap(err, "sender public key")
		}

		ra, err := publicKey.RawAddress()
		if err != nil {
			return errors.Wrap(err, "sender address")
		}

		ad, err := rs.wallet.FindAddress(ctx, ra)
		if err != nil {
			return errors.Wrap(err, "sender address")
		}

		if ad != nil {
			// We are sender. Mark relationship as accepted
			r.Accepted = true

			nextKey, err := bitcoin.NextPublicKey(publicKey, r.NextHash)
			if err != nil {
				return errors.Wrap(err, "next key")
			}

			if err := rs.wallet.AddIndependentKey(ctx, nextKey, r.KeyType, r.KeyIndex,
				r.NextHash); err != nil {
				return errors.Wrap(err, "add independent key")
			}

			continue
		}

		// Find which member to mark relationship accepted.
		for _, member := range r.Members {
			if member.NextKey.Equal(publicKey) {
				member.Accepted = true
				member.IncrementHash()
				break
			}
		}
	}

	return nil
}
