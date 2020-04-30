package relationships

import (
	"bytes"
	"context"

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
func (rs *Relationships) AcceptRelationship(ctx context.Context, r *Relationship) (*messages.AcceptRelationship, error) {
	if r.Accepted {
		return nil, errors.New("Already accepted")
	}

	// Public message fields
	publicMessage := &actions.Message{
		MessageCode: messages.CodeAcceptRelationship,
	}
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
	accept := &messages.AcceptRelationship{
		ProofOfIdentityType: 2, // Identity oracle
		// ProofOfIdentity      []byte   `protobuf:"bytes,2,opt,name=ProofOfIdentity,proto3" json:"ProofOfIdentity,omitempty"`
	}

	var acceptBuf bytes.Buffer
	if err := accept.Serialize(&acceptBuf); err != nil {
		return nil, errors.Wrap(err, "serialize accept")
	}

	privateMessage := &actions.Message{
		MessagePayload: acceptBuf.Bytes(),
	}

	privatePayload, err := proto.Marshal(privateMessage)
	if err != nil {
		return nil, errors.Wrap(err, "serialize private")
	}

	tx := txbuilder.NewTxBuilder(rs.cfg.DustLimit, rs.cfg.FeeRate)
	senderIndex := uint32(0)

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
	nextKey, err := r.NextKey(baseKey)
	if err != nil {
		return nil, errors.Wrap(err, "next key")
	}
	nextAddress, err := nextKey.RawAddress()
	if err != nil {
		return nil, errors.Wrap(err, "next key")
	}

	if r.EncryptionType == 0 { // direct encryption
		receivers := make([]bitcoin.PublicKey, 0, len(r.Members))
		for _, m := range r.Members {
			memberKey, err := m.NextKey()
			if err != nil {
				return nil, errors.Wrap(err, "member next key")
			}
			receivers = append(receivers, memberKey)

			// Add output to member
			receiverAddress, err := bitcoin.NewRawAddressPublicKey(memberKey)
			if err != nil {
				return nil, errors.Wrap(err, "receiver address")
			}

			if err := tx.AddDustOutput(receiverAddress, false); err != nil {
				return nil, errors.Wrap(err, "add receiver")
			}
		}

		if _, err := env0.AddEncryptedPayloadDirect(privatePayload, tx.MsgTx, senderIndex, nextKey, receivers); err != nil {
			return nil, errors.Wrap(err, "add direct encrypted payload")
		}
	} else {
		encryptionKey := bitcoin.AddHashes(r.EncryptionKey, r.NextHash)

		if err := env0.AddEncryptedPayloadIndirect(privatePayload, tx.MsgTx, encryptionKey); err != nil {
			return nil, errors.Wrap(err, "add indirect encrypted payload")
		}
	}

	var scriptBuf bytes.Buffer
	if err := env0.Serialize(&scriptBuf); err != nil {
		return nil, errors.Wrap(err, "serialize envelope")
	}

	if err := tx.AddOutput(scriptBuf.Bytes(), 0, false, false); err != nil {
		return nil, errors.Wrap(err, "add op return")
	}

	if err := rs.wallet.AddIndependentKey(ctx, nextAddress, nextKey.PublicKey(), r.KeyType,
		r.KeyIndex, r.NextHash); err != nil {
		return nil, errors.Wrap(err, "add independent key")
	}

	logger.Info(ctx, "Adding key funding")
	if err := rs.wallet.AddKeyFunding(ctx, r.KeyType, r.KeyIndex, r.NextHash, tx, rs.broadcastTx); err != nil {
		return nil, errors.Wrap(err, "add key funding")
	}

	// Increment hashes
	// r.IncrementHash()
	// if r.EncryptionType == 0 {
	// 	// Member keys were included in this tx, so increment them too
	// 	for _, m := range r.Members {
	// 		m.IncrementHash()
	// 	}
	// }

	return accept, nil
}

func (rs *Relationships) ProcessAcceptRelationship(ctx context.Context, itx *inspector.Transaction,
	message *actions.Message, accept *messages.AcceptRelationship) error {
	return nil
}
