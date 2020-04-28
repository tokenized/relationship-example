package relationships

import (
	"bytes"
	"context"
	"fmt"

	"github.com/tokenized/envelope/pkg/golang/envelope/v0"

	"github.com/tokenized/relationship-example/internal/platform/config"
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

type Relationships struct {
	cfg         *config.Config
	wallet      *wallet.Wallet
	broadcastTx wallet.BroadcastTx
}

func NewRelationships(cfg *config.Config, wallet *wallet.Wallet, broadcastTx wallet.BroadcastTx) (*Relationships, error) {
	result := &Relationships{
		cfg:         cfg,
		wallet:      wallet,
		broadcastTx: broadcastTx,
	}

	return result, nil
}

func (r *Relationships) CreateInitiateRelationship(ctx context.Context,
	receiver bitcoin.PublicKey) (*messages.InitiateRelationship, error) {

	// Public message fields
	publicMessage := &actions.Message{} // Full message encrypted
	env, err := protocol.WrapAction(publicMessage, r.cfg.IsTest)
	if err != nil {
		return nil, errors.Wrap(err, "wrap action")
	}

	// Convert to specific version of envelope
	env0, ok := env.(*v0.Message)
	if !ok {
		return nil, errors.New("Unsupported envelope version")
	}

	seedValue, err := bitcoin.GenerateSeedValue()
	if err != nil {
		return nil, errors.Wrap(err, "seed value")
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

	var initiateBuf bytes.Buffer
	if err := initiate.Serialize(&initiateBuf); err != nil {
		return nil, errors.Wrap(err, "serialize initiate")
	}

	privateMessage := &actions.Message{
		MessageCode:    messages.CodeInitiateRelationship,
		MessagePayload: initiateBuf.Bytes(),
	}

	privatePayload, err := proto.Marshal(privateMessage)
	if err != nil {
		return nil, errors.Wrap(err, "serialize private")
	}

	senderKey, senderKeyIndex, err := r.wallet.GetRelationshipKey(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get relationship key")
	}

	tx := txbuilder.NewTxBuilder(r.cfg.DustLimit, r.cfg.FeeRate)

	changeAddress, err := r.wallet.GetChangeAddress(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get change address")
	}

	logger.Info(ctx, "Using change address %d : %s", changeAddress.KeyIndex,
		bitcoin.NewAddressFromRawAddress(changeAddress.Address, r.cfg.Net).String())

	if err := tx.SetChangeAddress(changeAddress.Address, ""); err != nil {
		return nil, errors.Wrap(err, "set change address")
	}

	senderIndex := uint32(len(tx.Inputs))

	receiverAddress, err := bitcoin.NewRawAddressPublicKey(receiver)
	if err != nil {
		return nil, errors.Wrap(err, "receiver address")
	}

	if err := tx.AddDustOutput(receiverAddress, false); err != nil {
		return nil, errors.Wrap(err, "add receiver")
	}

	if err := env0.AddEncryptedPayload(privatePayload, tx.MsgTx, senderIndex, senderKey,
		[]bitcoin.PublicKey{receiver}); err != nil {
		return nil, errors.Wrap(err, "add encrypted payload")
	}

	var scriptBuf bytes.Buffer
	if err := env0.Serialize(&scriptBuf); err != nil {
		return nil, errors.Wrap(err, "serialize envelope")
	}

	if err := tx.AddOutput(scriptBuf.Bytes(), 0, false, false); err != nil {
		return nil, errors.Wrap(err, "add op return")
	}

	logger.Info(ctx, "Adding key funding")
	if err := r.wallet.AddKeyFunding(ctx, wallet.KeyTypeRelateOut, senderKeyIndex, tx,
		r.broadcastTx); err != nil {
		return nil, errors.Wrap(err, "add key funding")
	}

	return initiate, nil
}

func ParseRelationshipInitiation(keyIndex uint32, secret []byte, itx *inspector.Transaction,
	message *actions.Message, initiate *messages.InitiateRelationship) (Relationship, error) {

	result := Relationship{
		KeyIndex: keyIndex,
	}

	result.Seed = initiate.SeedValue
	result.Flag = initiate.FlagValue

	// TODO Other Fields --ce
	// initiate.Type
	// initiate.ProofOfIdentityType
	// initiate.ProofOfIdentity
	// initiate.ChannelParties

	for _, receiverIndex := range message.ReceiverIndexes {
		if int(receiverIndex) >= len(itx.Outputs) {
			return result, fmt.Errorf("Receiver index out of range : %d/%d", receiverIndex,
				len(itx.Outputs))
		}

		if itx.Outputs[receiverIndex].Address.Type() != bitcoin.ScriptTypePK {
			return result, errors.New("Receiver locking script not P2PK")
		}

		pk, err := itx.Outputs[receiverIndex].Address.GetPublicKey()
		if err != nil {
			return result, errors.Wrap(err, "get public key")
		}

		result.Members = append(result.Members, Member{
			BaseKey: pk,
		})
	}

	return result, nil
}
