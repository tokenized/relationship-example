package relationships

import (
	"bytes"
	"context"
	"fmt"

	"github.com/tokenized/envelope/pkg/golang/envelope/v0"

	"github.com/tokenized/relationship-example/internal/platform/config"
	"github.com/tokenized/relationship-example/internal/platform/db"
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

const (
	relationshipsKey = "relationships"
)

func NewRelationships(cfg *config.Config, wallet *wallet.Wallet, broadcastTx wallet.BroadcastTx) (*Relationships, error) {
	result := &Relationships{
		cfg:         cfg,
		wallet:      wallet,
		broadcastTx: broadcastTx,
	}

	return result, nil
}

func (rs *Relationships) Load(ctx context.Context, dbConn *db.DB) error {
	b, err := dbConn.Fetch(ctx, relationshipsKey)
	if err == nil {
		if err := rs.Deserialize(bytes.NewReader(b)); err != nil {
			return errors.Wrap(err, "deserialize wallet")
		}
	} else if err != db.ErrNotFound {
		return errors.Wrap(err, "fetch wallet")
	}

	return nil
}

func (rs *Relationships) Save(ctx context.Context, dbConn *db.DB) error {
	var buf bytes.Buffer
	if err := rs.Serialize(&buf); err != nil {
		return errors.Wrap(err, "serialize wallet")
	}

	if err := dbConn.Put(ctx, relationshipsKey, buf.Bytes()); err != nil {
		return errors.Wrap(err, "put wallet")
	}

	return nil
}

func (rs *Relationships) InitiateRelationship(ctx context.Context,
	receivers []bitcoin.PublicKey) (*messages.InitiateRelationship, error) {

	if len(receivers) == 0 {
		return nil, errors.New("No receivers provided")
	}

	senderKey, senderKeyIndex, err := rs.wallet.GetRelationshipKey(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get relationship key")
	}

	seedValue, err := bitcoin.GenerateSeedValue()
	if err != nil {
		return nil, errors.Wrap(err, "seed value")
	}

	r := &Relationship{
		KeyIndex: senderKeyIndex,
		Seed:     seedValue.Bytes(),
	}

	if len(receivers) > 1 {
		flagValue, err := bitcoin.GenerateSeedValue()
		if err != nil {
			return nil, errors.Wrap(err, "flag value")
		}

		r.Flag = flagValue.Bytes()
	}

	hash, _ := bitcoin.NewHash32(bitcoin.Sha256(r.Seed))

	for _, receiver := range receivers {
		// Save
		r.Members = append(r.Members, &Member{
			BaseKey:   receiver,
			NextHash:  *hash,
			NextIndex: 1,
		})
	}

	// Public message fields
	publicMessage := &actions.Message{
		MessageCode: messages.CodeInitiateRelationship,
	} // Full message encrypted
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
	initiate := &messages.InitiateRelationship{
		Type:      0, // Conversation
		SeedValue: seedValue.Bytes(),
		// FlagValue            []byte // Not needed for two party
		// ProofOfIdentityType  uint32 // Skipping for now
		// ProofOfIdentity      []byte
		// ChannelParties       []*ChannelPartyField
	}

	if len(receivers) > 1 {
		initiate.EncryptionType = 1
	}

	var initiateBuf bytes.Buffer
	if err := initiate.Serialize(&initiateBuf); err != nil {
		return nil, errors.Wrap(err, "serialize initiate")
	}

	privateMessage := &actions.Message{
		MessagePayload: initiateBuf.Bytes(),
	}

	privatePayload, err := proto.Marshal(privateMessage)
	if err != nil {
		return nil, errors.Wrap(err, "serialize private")
	}

	tx := txbuilder.NewTxBuilder(rs.cfg.DustLimit, rs.cfg.FeeRate)

	changeAddress, err := rs.wallet.GetChangeAddress(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get change address")
	}

	logger.Info(ctx, "Using change address %d : %s", changeAddress.KeyIndex,
		bitcoin.NewAddressFromRawAddress(changeAddress.Address, rs.cfg.Net).String())

	if err := tx.SetChangeAddress(changeAddress.Address, ""); err != nil {
		return nil, errors.Wrap(err, "set change address")
	}

	senderIndex := uint32(len(tx.Inputs))

	for _, receiver := range receivers {
		receiverAddress, err := bitcoin.NewRawAddressPublicKey(receiver)
		if err != nil {
			return nil, errors.Wrap(err, "receiver address")
		}

		if err := tx.AddDustOutput(receiverAddress, false); err != nil {
			return nil, errors.Wrap(err, "add receiver")
		}
	}

	encryptionKey, err := env0.AddEncryptedPayloadDirect(privatePayload, tx.MsgTx, senderIndex, senderKey,
		receivers)
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
	if err := rs.wallet.AddKeyFunding(ctx, wallet.KeyTypeRelateOut, r.KeyIndex, tx,
		rs.broadcastTx); err != nil {
		return nil, errors.Wrap(err, "add key funding")
	}

	rs.Relationships = append(rs.Relationships, r)

	return initiate, nil
}

func (rs *Relationships) ParseRelationshipInitiation(ctx context.Context,
	itx *inspector.Transaction, message *actions.Message, initiate *messages.InitiateRelationship,
	encryptionKey bitcoin.Hash32) (*Relationship, error) {

	hash, _ := bitcoin.NewHash32(bitcoin.Sha256(initiate.SeedValue))
	keyFound := false

	result := &Relationship{
		NextHash:  *hash,
		NextIndex: 1,
		Seed:      initiate.SeedValue,
		Flag:      initiate.FlagValue,
	}

	if initiate.EncryptionType != 0 {
		result.EncryptionKey = encryptionKey
	}

	if len(message.SenderIndexes) > 1 {
		return nil, fmt.Errorf("More than one sender not supported : %d",
			len(message.SenderIndexes))
	}

	// TODO Other Fields --ce
	// initiate.Type
	// initiate.ProofOfIdentityType
	// initiate.ProofOfIdentity
	// initiate.ChannelParties

	if len(message.SenderIndexes) == 0 {
		message.SenderIndexes = append(message.SenderIndexes, 0)
	}

	for _, senderIndex := range message.SenderIndexes {
		if int(senderIndex) >= len(itx.MsgTx.TxIn) {
			return nil, fmt.Errorf("Sender index out of range : %d/%d", senderIndex,
				len(itx.MsgTx.TxIn))
		}

		pk, err := bitcoin.PublicKeyFromUnlockingScript(itx.MsgTx.TxIn[senderIndex].SignatureScript)
		if err != nil {
			return nil, errors.Wrap(err, "sender parse script")
		}

		publicKey, err := bitcoin.PublicKeyFromBytes(pk)
		if err != nil {
			return nil, errors.Wrap(err, "sender public key")
		}

		if keyFound {
			// Sender is someone else
			result.Members = append(result.Members, &Member{
				BaseKey:   publicKey,
				NextHash:  *hash,
				NextIndex: 1,
			})
			continue
		}

		ra, err := publicKey.RawAddress()
		if err != nil {
			return nil, errors.Wrap(err, "sender address")
		}

		ad, err := rs.wallet.FindAddress(ctx, ra)
		if err != nil {
			return nil, errors.Wrap(err, "sender address")
		}

		if ad != nil {
			// We are sender
			if ad.KeyType != wallet.KeyTypeRelateOut {
				return nil, fmt.Errorf("Wrong key type for relationship sender : %s",
					wallet.KeyTypeName[ad.KeyType])
			}
			result.KeyIndex = ad.KeyIndex
			keyFound = true
		} else {
			// Sender is someone else
			result.Members = append(result.Members, &Member{
				BaseKey:   publicKey,
				NextHash:  *hash,
				NextIndex: 1,
			})
		}
	}

	for _, receiverIndex := range message.ReceiverIndexes {
		if int(receiverIndex) >= len(itx.Outputs) {
			return nil, fmt.Errorf("Receiver index out of range : %d/%d", receiverIndex,
				len(itx.Outputs))
		}

		if itx.Outputs[receiverIndex].Address.Type() != bitcoin.ScriptTypePK {
			return nil, errors.New("Receiver locking script not P2PK")
		}

		publicKey, err := itx.Outputs[receiverIndex].Address.GetPublicKey()
		if err != nil {
			return nil, errors.Wrap(err, "get public key")
		}

		if keyFound {
			// Receiver is someone else
			result.Members = append(result.Members, &Member{
				BaseKey:   publicKey,
				NextHash:  *hash,
				NextIndex: 1,
			})
			continue
		}

		ad, err := rs.wallet.FindAddress(ctx, itx.Outputs[receiverIndex].Address)
		if err != nil {
			return nil, errors.Wrap(err, "receiver address")
		}

		if ad != nil {
			// We are receiver
			if ad.KeyType != wallet.KeyTypeRelateIn {
				return nil, fmt.Errorf("Wrong key type for relationship sender : %s",
					wallet.KeyTypeName[ad.KeyType])
			}
			result.KeyIndex = ad.KeyIndex
			keyFound = true
		} else {
			result.Members = append(result.Members, &Member{
				BaseKey:   publicKey,
				NextHash:  *hash,
				NextIndex: 1,
			})
		}
	}

	rs.Relationships = append(rs.Relationships, result)

	return result, nil
}
