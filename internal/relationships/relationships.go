package relationships

import (
	"bytes"
	"context"

	"github.com/tokenized/relationship-example/internal/platform/config"
	"github.com/tokenized/relationship-example/internal/platform/db"
	"github.com/tokenized/relationship-example/internal/wallet"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/inspector"

	"github.com/tokenized/specification/dist/golang/actions"

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

func (rs *Relationships) FindRelationship(ctx context.Context, flag []byte) *Relationship {
	for _, r := range rs.Relationships {
		if bytes.Equal(r.Flag, flag) {
			return r
		}
	}

	return nil
}

func (rs *Relationships) DecryptAction(ctx context.Context, itx *inspector.Transaction, index int,
	flag []byte) (actions.Action, bitcoin.Hash32, error) {

	if len(flag) == 0 { // Not related to a relationship with a indirect encryption
		return rs.wallet.DecryptActionDirect(ctx, itx.MsgTx, index)
	}

	// Find relationship
	r := rs.FindRelationship(ctx, flag)
	if r == nil { // Not related to a relationship with a indirect encryption
		return rs.wallet.DecryptActionDirect(ctx, itx.MsgTx, index)
	}

	// Find appropriate encryption key based on sender public key.
	// The encryption key is based on which key is used to create the message.
	// For example if the sender used their 5th derived key to send the message, then the encryption
	//   key is the 5th derived encryption key for the relationship.
	// TODO Pull information from encrypted payload about which input is the sender. --ce
	// TODO Implement system for checking more than just the expected index key as the sender. For
	//   example if the sender sends two messages and we see them out of order. --ce
	for i, _ := range itx.Inputs {
		publicKey, err := itx.GetPublicKeyForInput(i)
		if err != nil {
			if errors.Cause(err) == bitcoin.ErrWrongType {
				continue // not a script that provides a public key
			}
			return nil, bitcoin.Hash32{}, errors.Wrap(err, "get public key")
		}

		encryptionKey, err := r.FindEncryptionKey(publicKey)
		if err != nil {
			if errors.Cause(err) == ErrKeyNotFound {
				continue // this input is not part of this relationship
			}
			return nil, bitcoin.Hash32{}, errors.Wrap(err, "find encryption key")
		}

		action, err := rs.wallet.DecryptActionIndirect(ctx, itx.MsgTx.TxOut[index].PkScript,
			encryptionKey)
		if err != nil {
			return nil, bitcoin.Hash32{}, errors.Wrap(err, "decrypt action indirect")
		}
		if action != nil {
			return action, bitcoin.Hash32{}, err
		}
	}

	return rs.wallet.DecryptActionDirect(ctx, itx.MsgTx, index)
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
