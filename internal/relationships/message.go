package relationships

import (
	"bytes"
	"context"
	"encoding/json"
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

func (rs *Relationships) SendMessage(ctx context.Context, r *Relationship, message messages.Message) error {
	logger.Info(ctx, "Creating message for relationship : %s", r.TxId.String())

	if !r.Accepted {
		return errors.New("Relationship not accepted")
	}

	tx := txbuilder.NewTxBuilder(rs.cfg.DustLimit, rs.cfg.FeeRate)

	senderIndex := uint32(0)

	// Public message fields
	publicMessage := &actions.Message{
		SenderIndexes: []uint32{senderIndex},
	}

	changeAddress, err := rs.wallet.GetUnusedAddress(ctx, wallet.KeyTypeInternal)
	if err != nil {
		return errors.Wrap(err, "get change address")
	}

	logger.Info(ctx, "Using change address %d : %s", changeAddress.KeyIndex,
		bitcoin.NewAddressFromRawAddress(changeAddress.Address, rs.cfg.Net).String())

	if err := tx.SetChangeAddress(changeAddress.Address, ""); err != nil {
		return errors.Wrap(err, "set change address")
	}

	baseKey, err := rs.wallet.GetKey(ctx, r.KeyType, r.KeyIndex)
	if err != nil {
		return errors.Wrap(err, "get key")
	}
	nextKey, err := bitcoin.NextKey(baseKey, r.NextHash)
	if err != nil {
		return errors.Wrap(err, "next key")
	}
	nextAddress, err := nextKey.RawAddress()
	if err != nil {
		return errors.Wrap(err, "next key")
	}

	logger.Info(ctx, "Sending message from address : %s",
		bitcoin.NewAddressFromRawAddress(nextAddress, rs.cfg.Net).String())

	receivers := make([]bitcoin.PublicKey, 0, len(r.Members))
	if r.EncryptionType == 0 { // direct encryption
		for _, m := range r.Members {
			receivers = append(receivers, m.NextKey)

			// Add output to member
			receiverAddress, err := bitcoin.NewRawAddressPublicKey(m.NextKey)
			if err != nil {
				return errors.Wrap(err, "receiver address")
			}
			logger.Info(ctx, "Sending message to address : %s",
				bitcoin.NewAddressFromRawAddress(receiverAddress, rs.cfg.Net).String())

			publicMessage.ReceiverIndexes = append(publicMessage.ReceiverIndexes,
				uint32(len(tx.Outputs)))
			if err := tx.AddDustOutput(receiverAddress, false); err != nil {
				return errors.Wrap(err, "add receiver")
			}
		}
	}

	// Create envelope
	env, err := protocol.WrapAction(publicMessage, rs.cfg.IsTest)
	if err != nil {
		return errors.Wrap(err, "wrap action")
	}

	// Convert to specific version of envelope to access encryption
	env0, ok := env.(*v0.Message)
	if !ok {
		return errors.New("Unsupported envelope version")
	}

	messagePayload, err := message.Bytes()
	if err != nil {
		return errors.Wrap(err, "Serialize message")
	}

	privateMessage := &actions.Message{
		MessageCode:    message.Code(),
		MessagePayload: messagePayload,
	}

	privatePayload, err := proto.Marshal(privateMessage)
	if err != nil {
		return errors.Wrap(err, "serialize private")
	}

	if r.EncryptionType == 0 { // direct encryption
		if _, err := env0.AddEncryptedPayloadDirect(privatePayload, tx.MsgTx, senderIndex, nextKey,
			receivers); err != nil {
			return errors.Wrap(err, "add direct encrypted payload")
		}
	} else {
		encryptionKey := bitcoin.AddHashes(r.EncryptionKey, r.NextHash)
		if err := env0.AddEncryptedPayloadIndirect(privatePayload, tx.MsgTx, encryptionKey); err != nil {
			return errors.Wrap(err, "add indirect encrypted payload")
		}
	}

	if len(r.Flag) > 0 {
		flagScript, err := protocol.SerializeFlagOutputScript(r.Flag)
		if err != nil {
			return errors.Wrap(err, "serialize flag")
		}
		if err := tx.AddOutput(flagScript, 0, false, false); err != nil {
			return errors.Wrap(err, "add flag op return")
		}
	}

	var scriptBuf bytes.Buffer
	if err := env0.Serialize(&scriptBuf); err != nil {
		return errors.Wrap(err, "serialize envelope")
	}

	if err := tx.AddOutput(scriptBuf.Bytes(), 0, false, false); err != nil {
		return errors.Wrap(err, "add message op return")
	}

	logger.Info(ctx, "Adding key funding")
	if err := rs.wallet.AddKeyFunding(ctx, r.KeyType, r.KeyIndex, r.NextHash, tx, rs.broadcastTx); err != nil {
		return errors.Wrap(err, "add key funding")
	}

	// Increment hashes
	if err := r.IncrementHash(ctx, rs.wallet); err != nil {
		return errors.Wrap(err, "increment hash")
	}
	if r.EncryptionType == 0 {
		// Member keys were included in this tx, so increment them too
		for _, m := range r.Members {
			m.IncrementHash()
		}
	}

	return nil
}

func (rs *Relationships) ProcessPrivateMessage(ctx context.Context, itx *inspector.Transaction,
	message *actions.Message, privateMessage *messages.PrivateMessage, flag []byte) error {

	logger.Info(ctx, "Processing private message for relationship")

	// Get relationship
	r, areSender, memberIndex, err := rs.GetRelationshipForTx(ctx, itx, message, flag)
	if err != nil {
		return errors.Wrap(err, "get relationship")
	}
	if r == nil {
		return ErrNotFound
	}

	if len(message.SenderIndexes) > 1 {
		return fmt.Errorf("More than one sender not supported : %d", len(message.SenderIndexes))
	}

	if areSender {
		logger.Info(ctx, "We are sender")
	} else {
		ra, err := r.Members[memberIndex].BaseKey.RawAddress()
		if err == nil {
			logger.Info(ctx, "Message from %s",
				bitcoin.NewAddressFromRawAddress(ra, rs.cfg.Net).String())
		}
	}

	if js, err := json.MarshalIndent(privateMessage, "", "    "); err == nil {
		logger.Info(ctx, "Message contents : \n%s\n", js)
	}

	return nil
}
