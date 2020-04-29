package node

import (
	"context"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/inspector"

	"github.com/tokenized/specification/dist/golang/actions"
	"github.com/tokenized/specification/dist/golang/messages"

	"github.com/pkg/errors"
)

func (n *Node) ProcessMessage(ctx context.Context, itx *inspector.Transaction, index int,
	encryptionKey bitcoin.Hash32, message *actions.Message, flag []byte) error {

	p, err := messages.Deserialize(message.MessageCode, message.MessagePayload)
	if err != nil {
		return errors.Wrap(err, "deserialize message")
	}

	switch payload := p.(type) {
	case *messages.InitiateRelationship:
		_, err := n.rs.ParseRelationshipInitiation(ctx, itx, message, payload, encryptionKey)
		return err
	}

	return nil
}