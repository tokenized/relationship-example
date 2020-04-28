package node

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

func (n *Node) CreateInitiateRelationship(ctx context.Context, receiver bitcoin.PublicKey) error {

	// Public message fields
	publicMessage := &actions.Message{} // Full message encrypted
	env, err := protocol.WrapAction(publicMessage, n.cfg.IsTest)
	if err != nil {
		return errors.Wrap(err, "wrap action")
	}

	// Convert to specific version of envelope
	env0, ok := env.(*v0.Message)
	if !ok {
		return errors.New("Unsupported envelope version")
	}

	seedValue, err := bitcoin.GenerateSeedValue()
	if err != nil {
		return errors.Wrap(err, "seed value")
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
		return errors.Wrap(err, "serialize initiate")
	}

	privateMessage := &actions.Message{
		MessageCode:    messages.CodeInitiateRelationship,
		MessagePayload: initiateBuf.Bytes(),
	}

	privatePayload, err := proto.Marshal(privateMessage)
	if err != nil {
		return errors.Wrap(err, "serialize private")
	}

	senderKey, senderKeyIndex, err := n.wallet.GetRelationshipKey(ctx)
	if err != nil {
		return errors.Wrap(err, "get relationship key")
	}

	tx := txbuilder.NewTxBuilder(n.cfg.DustLimit, n.cfg.FeeRate)

	changeAddress, err := n.wallet.GetChangeAddress(ctx)
	if err != nil {
		return errors.Wrap(err, "get change address")
	}

	logger.Info(ctx, "Using change address %d : %s", changeAddress.KeyIndex,
		bitcoin.NewAddressFromRawAddress(changeAddress.Address, n.cfg.Net).String())

	if err := tx.SetChangeAddress(changeAddress.Address, ""); err != nil {
		return errors.Wrap(err, "set change address")
	}

	senderIndex := uint32(len(tx.Inputs))

	receiverAddress, err := bitcoin.NewRawAddressPublicKey(receiver)
	if err != nil {
		return errors.Wrap(err, "receiver address")
	}

	if err := tx.AddDustOutput(receiverAddress, false); err != nil {
		return errors.Wrap(err, "add receiver")
	}

	if err := env0.AddEncryptedPayload(privatePayload, tx.MsgTx, senderIndex, senderKey,
		[]bitcoin.PublicKey{receiver}); err != nil {
		return errors.Wrap(err, "add encrypted payload")
	}

	var scriptBuf bytes.Buffer
	if err := env0.Serialize(&scriptBuf); err != nil {
		return errors.Wrap(err, "serialize envelope")
	}

	if err := tx.AddOutput(scriptBuf.Bytes(), 0, false, false); err != nil {
		return errors.Wrap(err, "add op return")
	}

	if err := n.wallet.AddKeyFunding(ctx, wallet.KeyTypeRelationship, senderKeyIndex, tx, n); err != nil {
		return errors.Wrap(err, "add key funding")
	}

	return nil
}

func (n *Node) ProcessInitiateRelationship(ctx context.Context, itx *inspector.Transaction,
	index uint32, m *actions.Message, i *messages.InitiateRelationship) error {

	if len(m.SenderIndexes) > 1 {
		return fmt.Errorf("Too many senders : %d", len(m.SenderIndexes))
	}

	senderIndex := uint32(0)
	if len(m.SenderIndexes) > 0 {
		senderIndex = m.SenderIndexes[0]
	}

	if int(senderIndex) >= len(itx.Inputs) {
		return fmt.Errorf("Sender index out of range : %d/%d", senderIndex, len(itx.Inputs))
	}

	address, err := n.wallet.FindAddress(ctx, itx.Inputs[senderIndex].Address)
	if err != nil {
		return errors.Wrap(err, "find address")
	}

	if address == nil { // not the sender
		return n.ProcessInitiateRelationshipReceive(ctx, itx, index, m, i, senderIndex)
	}

	// r, err := relationship.ParseRelationshipInitiation(address.KeyIndex, secret []byte, itx, m, i)
	// if err != nil {
	// 	return errors.Wrap(err, "parse relationship")
	// }

	return nil
}

func (n *Node) ProcessInitiateRelationshipReceive(ctx context.Context, itx *inspector.Transaction,
	index uint32, m *actions.Message, i *messages.InitiateRelationship, senderIndex uint32) error {

	for _, receiverIndex := range m.ReceiverIndexes {
		if int(receiverIndex) >= len(itx.Outputs) {
			return fmt.Errorf("Receiver index out of range : %d/%d", receiverIndex,
				len(itx.Outputs))
		}

		if itx.Outputs[receiverIndex].Address.Type() != bitcoin.ScriptTypePK {
			return errors.New("Receiver locking script not P2PK")
		}

		// pk, err := itx.Outputs[receiverIndex].Address.GetPublicKey()
		// if err != nil {
		// 	return errors.Wrap(err, "get public key")
		// }

	}

	// TODO Implement receive

	return nil
}
