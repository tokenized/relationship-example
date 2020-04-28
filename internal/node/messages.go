package node

import (
	"context"

	"github.com/tokenized/smart-contract/pkg/inspector"

	"github.com/tokenized/specification/dist/golang/actions"
	"github.com/tokenized/specification/dist/golang/messages"

	"github.com/pkg/errors"
)

func (n *Node) ProcessMessage(ctx context.Context, itx *inspector.Transaction, index uint32) error {

	unencrypted, err := n.wallet.DecryptScript(ctx, itx.MsgTx, itx.MsgTx.TxOut[index].PkScript)
	if err != nil {
		return errors.Wrap(err, "decrypt payload")
	}

	a, err := actions.Deserialize([]byte(actions.CodeMessage), unencrypted)
	if err != nil {
		return errors.Wrap(err, "deserialize action")
	}

	m, ok := a.(*actions.Message)
	if !ok {
		return errors.New("Action not a message")
	}

	p, err := messages.Deserialize(m.MessageCode, m.MessagePayload)
	if err != nil {
		return errors.Wrap(err, "deserialize message")
	}

	switch payload := p.(type) {
	case *messages.InitiateRelationship:
		return n.ProcessInitiateRelationship(ctx, itx, index, m, payload)
	}

	return nil
}
