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

// InitiateRelationship creates and broadcasts an InitiateRelationship message to the receivers
//   specified.
// proofOfIdentity needs to be nil, or a proof of identity message like
//   messages.IdentityOracleProofField or messages.PaymailProofField
func (rs *Relationships) InitiateRelationship(ctx context.Context,
	receivers []bitcoin.PublicKey, proofOfIdentity proto.Message) (*messages.InitiateRelationship, error) {

	if len(receivers) == 0 {
		return nil, errors.New("No receivers provided")
	}

	senderKey, senderKeyIndex, err := rs.wallet.GetUnusedKey(ctx, wallet.KeyTypeRelateOut)
	if err != nil {
		return nil, errors.Wrap(err, "get relationship key")
	}

	seedValue, err := bitcoin.GenerateSeedValue()
	if err != nil {
		return nil, errors.Wrap(err, "seed value")
	}

	hash, _ := bitcoin.NewHash32(bitcoin.Sha256(seedValue.Bytes()))

	r := &Relationship{
		KeyType:   wallet.KeyTypeRelateOut,
		KeyIndex:  senderKeyIndex,
		Seed:      seedValue.Bytes(),
		NextHash:  *hash,
		NextIndex: 1,
		Accepted:  true,
	}

	r.NextKey, err = bitcoin.NextPublicKey(senderKey.PublicKey(), *hash)
	if err != nil {
		return nil, errors.Wrap(err, "next key")
	}

	if len(receivers) > 1 {
		flagValue, err := bitcoin.GenerateSeedValue()
		if err != nil {
			return nil, errors.Wrap(err, "flag value")
		}

		r.Flag = flagValue.Bytes()
	}

	for _, receiver := range receivers {
		nextKey, err := bitcoin.NextPublicKey(receiver, *hash)
		if err != nil {
			return nil, errors.Wrap(err, "next key")
		}

		r.Members = append(r.Members, &Member{
			BaseKey:   receiver,
			NextHash:  *hash,
			NextIndex: 1,
			NextKey:   nextKey,
		})
	}

	// Private message fields
	initiate := &messages.InitiateRelationship{
		Type: 0, // Conversation
		Seed: r.Seed,
		Flag: r.Flag,
		// ChannelParties       []*ChannelPartyField
	}

	if proofOfIdentity != nil {
		switch proofOfIdentity.(type) {
		case *messages.IdentityOracleProofField:
			initiate.ProofOfIdentityType = 2
		case *messages.PaymailProofField:
			initiate.ProofOfIdentityType = 1
		default:
			return nil, errors.New("Unsupported proof of identity type")
		}

		initiate.ProofOfIdentity, err = proto.Marshal(proofOfIdentity)
		if err != nil {
			return nil, errors.Wrap(err, "marshal proof of identity")
		}
	}

	if len(receivers) > 1 {
		initiate.EncryptionType = 1
	}

	var initiateBuf bytes.Buffer
	if err := initiate.Serialize(&initiateBuf); err != nil {
		return nil, errors.Wrap(err, "serialize initiate")
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

	for _, receiver := range receivers {
		receiverAddress, err := bitcoin.NewRawAddressPublicKey(receiver)
		if err != nil {
			return nil, errors.Wrap(err, "receiver address")
		}

		publicMessage.ReceiverIndexes = append(publicMessage.ReceiverIndexes,
			uint32(len(tx.Outputs)))
		if err := tx.AddDustOutput(receiverAddress, false); err != nil {
			return nil, errors.Wrap(err, "add receiver")
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
		MessageCode:    messages.CodeInitiateRelationship,
		MessagePayload: initiateBuf.Bytes(),
	}

	privatePayload, err := proto.Marshal(privateMessage)
	if err != nil {
		return nil, errors.Wrap(err, "serialize private")
	}

	encryptionKey, err := env0.AddEncryptedPayloadDirect(privatePayload, tx.MsgTx, senderIndex,
		senderKey, receivers)
	if err != nil {
		return nil, errors.Wrap(err, "add encrypted payload")
	}

	if initiate.EncryptionType > 0 {
		// Save encryption key used in this message as the base encryption key for indirect
		//   encryption in future messages.
		r.EncryptionKey = encryptionKey
	}

	var scriptBuf bytes.Buffer
	if err := env0.Serialize(&scriptBuf); err != nil {
		return nil, errors.Wrap(err, "serialize envelope")
	}

	if err := tx.AddOutput(scriptBuf.Bytes(), 0, false, false); err != nil {
		return nil, errors.Wrap(err, "add op return")
	}

	logger.Info(ctx, "Adding key funding")
	if err := rs.wallet.AddKeyIndexFunding(ctx, wallet.KeyTypeRelateOut, r.KeyIndex, tx,
		rs.broadcastTx); err != nil {
		return nil, errors.Wrap(err, "add key funding")
	}

	rs.Relationships = append(rs.Relationships, r)

	r.TxId = *tx.MsgTx.TxHash()

	if err := rs.wallet.AddIndependentKey(ctx, r.NextKey, r.KeyType, r.KeyIndex,
		r.NextHash); err != nil {
		return nil, errors.Wrap(err, "add independent key")
	}

	return initiate, nil
}

func (rs *Relationships) ProcessInitiateRelationship(ctx context.Context,
	itx *inspector.Transaction, message *actions.Message, initiate *messages.InitiateRelationship,
	encryptionKey bitcoin.Hash32) error {

	// Check for pre-existing
	for _, r := range rs.Relationships {
		if r.TxId.Equal(itx.Hash) {
			return nil // already exists
		}
	}

	hash, _ := bitcoin.NewHash32(bitcoin.Sha256(initiate.Seed))
	keyFound := false

	r := &Relationship{
		NextHash:       *hash,
		NextIndex:      1,
		Seed:           initiate.Seed,
		Flag:           initiate.Flag,
		EncryptionType: initiate.EncryptionType,
	}

	if initiate.EncryptionType != 0 {
		r.EncryptionKey = encryptionKey
	}

	if len(message.SenderIndexes) > 1 {
		return fmt.Errorf("More than one sender not supported : %d", len(message.SenderIndexes))
	}

	// TODO Other Fields --ce
	// initiate.Type
	// initiate.ProofOfIdentityType
	// initiate.ProofOfIdentity
	// initiate.ChannelParties

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

		if !keyFound {
			ra, err := publicKey.RawAddress()
			if err != nil {
				return errors.Wrap(err, "sender address")
			}

			ad, err := rs.wallet.FindAddress(ctx, ra)
			if err != nil {
				return errors.Wrap(err, "sender address")
			}

			if ad != nil {
				// We are sender
				if ad.KeyType != wallet.KeyTypeRelateOut {
					return fmt.Errorf("Wrong key type for relationship sender : %s",
						wallet.KeyTypeName[ad.KeyType])
				}
				r.KeyType = ad.KeyType
				r.KeyIndex = ad.KeyIndex
				keyFound = true

				r.NextKey, err = bitcoin.NextPublicKey(publicKey, r.NextHash)
				if err != nil {
					return errors.Wrap(err, "next key")
				}

				if err := rs.wallet.AddIndependentKey(ctx, r.NextKey, r.KeyType,
					r.KeyIndex, r.NextHash); err != nil {
					return errors.Wrap(err, "add independent key")
				}

				continue
			}
		}

		// Sender is someone else
		nextKey, err := bitcoin.NextPublicKey(publicKey, *hash)
		if err != nil {
			return errors.Wrap(err, "next key")
		}

		r.Members = append(r.Members, &Member{
			BaseKey:   publicKey,
			NextHash:  *hash,
			NextIndex: 1,
			NextKey:   nextKey,
		})
	}

	if len(message.ReceiverIndexes) == 0 { // No receiver indexes means use the first input
		message.ReceiverIndexes = append(message.ReceiverIndexes, 0)
	}

	for _, receiverIndex := range message.ReceiverIndexes {
		if int(receiverIndex) >= len(itx.Outputs) {
			return fmt.Errorf("Receiver index out of range : %d/%d", receiverIndex,
				len(itx.Outputs))
		}

		if itx.Outputs[receiverIndex].Address.Type() != bitcoin.ScriptTypePK {
			return errors.New("Receiver locking script not P2PK")
		}

		publicKey, err := itx.Outputs[receiverIndex].Address.GetPublicKey()
		if err != nil {
			return errors.Wrap(err, "get public key")
		}

		if !keyFound {
			ad, err := rs.wallet.FindAddress(ctx, itx.Outputs[receiverIndex].Address)
			if err != nil {
				return errors.Wrap(err, "receiver address")
			}

			if ad != nil {
				// We are receiver
				if ad.KeyType != wallet.KeyTypeRelateIn {
					return fmt.Errorf("Wrong key type for relationship sender : %s",
						wallet.KeyTypeName[ad.KeyType])
				}
				r.KeyType = ad.KeyType
				r.KeyIndex = ad.KeyIndex
				keyFound = true

				r.NextKey, err = bitcoin.NextPublicKey(publicKey, r.NextHash)
				if err != nil {
					return errors.Wrap(err, "next key")
				}

				if err := rs.wallet.AddIndependentKey(ctx, r.NextKey, r.KeyType, r.KeyIndex,
					r.NextHash); err != nil {
					return errors.Wrap(err, "add independent key")
				}

				continue
			}
		}

		nextKey, err := bitcoin.NextPublicKey(publicKey, *hash)
		if err != nil {
			return errors.Wrap(err, "next key")
		}

		r.Members = append(r.Members, &Member{
			BaseKey:   publicKey,
			NextHash:  *hash,
			NextIndex: 1,
			NextKey:   nextKey,
		})
	}

	if !keyFound {
		return errors.New("Not a member of relationship")
	}

	rs.Relationships = append(rs.Relationships, r)

	return nil
}
