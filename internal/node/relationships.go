package node

import (
	"context"
	"fmt"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/inspector"

	"github.com/tokenized/specification/dist/golang/actions"
	"github.com/tokenized/specification/dist/golang/messages"

	"github.com/pkg/errors"
)

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
