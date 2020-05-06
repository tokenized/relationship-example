package relationships

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	"github.com/tokenized/envelope/pkg/golang/envelope"
	"github.com/tokenized/relationship-example/internal/platform/config"
	"github.com/tokenized/relationship-example/internal/platform/db"
	"github.com/tokenized/relationship-example/internal/wallet"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/inspector"
	"github.com/tokenized/smart-contract/pkg/logger"

	"github.com/tokenized/specification/dist/golang/actions"
	"github.com/tokenized/specification/dist/golang/protocol"

	"github.com/pkg/errors"
)

var (
	ErrUnknownFlag = errors.New("Unknown Flag")
	ErrNotFound    = errors.New("Not found")
)

const (
	relationshipsKey = "relationships"
)

// Relationships is a manager for all of the relationships associated with a wallet.
type Relationships struct {
	cfg         *config.Config
	wallet      *wallet.Wallet
	broadcastTx wallet.BroadcastTx
	lock        sync.Mutex

	Relationships []*Relationship
}

func NewRelationships(cfg *config.Config, wallet *wallet.Wallet, broadcastTx wallet.BroadcastTx) (*Relationships, error) {
	result := &Relationships{
		cfg:         cfg,
		wallet:      wallet,
		broadcastTx: broadcastTx,
	}

	return result, nil
}

func (rs *Relationships) ListRelationships(ctx context.Context) []*Relationship {
	rs.lock.Lock()
	defer rs.lock.Unlock()

	result := make([]*Relationship, 0, len(rs.Relationships))
	for _, r := range rs.Relationships {
		result = append(result, r)
	}

	return result
}

func (rs *Relationships) GetRelationship(ctx context.Context, keyType, keyIndex uint32) *Relationship {
	rs.lock.Lock()
	defer rs.lock.Unlock()

	for _, r := range rs.Relationships {
		if r.KeyType == keyType && r.KeyIndex == keyIndex {
			return r
		}
	}

	return nil
}

func (rs *Relationships) FindRelationshipForFlag(ctx context.Context, flag []byte) *Relationship {
	rs.lock.Lock()
	defer rs.lock.Unlock()

	return rs.findRelationshipForFlag(ctx, flag)
}

func (rs *Relationships) FindRelationshipForTxId(ctx context.Context, txid bitcoin.Hash32) *Relationship {
	rs.lock.Lock()
	defer rs.lock.Unlock()

	logger.Info(ctx, "Searching for relationship : %s", txid.String())

	for _, r := range rs.Relationships {
		logger.Info(ctx, "Checking relationship : %s", r.TxId.String())
		if r.TxId.Equal(&txid) {
			return r
		}
	}

	return nil
}

func (rs *Relationships) findRelationshipForFlag(ctx context.Context, flag []byte) *Relationship {
	for _, r := range rs.Relationships {
		if bytes.Equal(r.Flag, flag) {
			return r
		}
	}

	return nil
}

// GetRelationshipForTx finds the relationship based on the senders and receivers of the
//   transaction and also increments all of the hashes for the keys involved.
// Returns:
//   *Relationship - matching relationship. nil if not found
//   bool - true if we are the sender
//   uint32 - the index of the member that sent the tx
//   error - if applicable
func (rs *Relationships) GetRelationshipForTx(ctx context.Context, itx *inspector.Transaction,
	message *actions.Message, flag []byte) (*Relationship, bool, uint32, error) {

	var r *Relationship
	if len(flag) > 0 {
		r = rs.FindRelationshipForFlag(ctx, flag)
	}

	if len(message.SenderIndexes) == 0 { // No sender indexes means use the first input
		message.SenderIndexes = append(message.SenderIndexes, 0)
	}

	areSender := false
	memberIndex := uint32(0)
	for _, senderIndex := range message.SenderIndexes {
		if int(senderIndex) >= len(itx.MsgTx.TxIn) {
			return nil, false, 0, fmt.Errorf("Sender index out of range : %d/%d", senderIndex,
				len(itx.MsgTx.TxIn))
		}

		pk, err := bitcoin.PublicKeyFromUnlockingScript(itx.MsgTx.TxIn[senderIndex].SignatureScript)
		if err != nil {
			return nil, false, 0, errors.Wrap(err, "sender parse script")
		}

		publicKey, err := bitcoin.PublicKeyFromBytes(pk)
		if err != nil {
			return nil, false, 0, errors.Wrap(err, "sender public key")
		}

		ra, err := publicKey.RawAddress()
		if err != nil {
			return nil, false, 0, errors.Wrap(err, "sender address")
		}

		ad, err := rs.wallet.FindAddress(ctx, ra)
		if err != nil {
			if errors.Cause(err) == bitcoin.ErrUnknownScriptTemplate {
				continue
			}
			return nil, false, 0, errors.Wrap(err, "find sender address")
		}

		if ad != nil &&
			(ad.KeyType == wallet.KeyTypeRelateIn || ad.KeyType == wallet.KeyTypeRelateOut) {

			if r != nil {
				if ad.KeyType != r.KeyType || ad.KeyIndex != r.KeyIndex {
					return nil, false, 0, errors.New("Wrong key for relationship")
				}
			} else {
				r = rs.GetRelationship(ctx, ad.KeyType, ad.KeyIndex)
				if r == nil {
					return nil, false, 0, ErrNotFound
				}
			}

			areSender = true

			if ad.PublicKey.Equal(r.NextKey) {
				if err := r.IncrementHash(ctx, rs.wallet); err != nil {
					return nil, false, 0, errors.Wrap(err, "increment hash")
				}
			}
		}
	}

	if len(message.ReceiverIndexes) == 0 { // No receiver indexes means use the first input
		message.ReceiverIndexes = append(message.ReceiverIndexes, 0)
	}

	if !areSender {
		for _, receiverIndex := range message.ReceiverIndexes {
			if int(receiverIndex) >= len(itx.Outputs) {
				return nil, false, 0, fmt.Errorf("Receiver index out of range : %d/%d", receiverIndex,
					len(itx.Outputs))
			}

			ad, err := rs.wallet.FindAddress(ctx, itx.Outputs[receiverIndex].Address)
			if err != nil {
				if errors.Cause(err) == bitcoin.ErrUnknownScriptTemplate {
					continue
				}
				return nil, false, 0, errors.Wrap(err, "find receiver address")
			}

			if ad != nil &&
				(ad.KeyType == wallet.KeyTypeRelateIn || ad.KeyType == wallet.KeyTypeRelateOut) {

				if r != nil {
					if ad.KeyType != r.KeyType || ad.KeyIndex != r.KeyIndex {
						return nil, false, 0, errors.New("Wrong key for relationship")
					}
				} else {
					r = rs.GetRelationship(ctx, ad.KeyType, ad.KeyIndex)
					if r == nil {
						return nil, false, 0, ErrNotFound
					}

					if ad.PublicKey.Equal(r.NextKey) {
						if err := r.IncrementHash(ctx, rs.wallet); err != nil {
							return nil, false, 0, errors.Wrap(err, "increment hash")
						}
					}
				}
			}
		}
	}

	if r == nil {
		return nil, false, 0, ErrNotFound
	}

	for _, senderIndex := range message.SenderIndexes {
		pk, err := bitcoin.PublicKeyFromUnlockingScript(itx.MsgTx.TxIn[senderIndex].SignatureScript)
		if err != nil {
			return nil, false, 0, errors.Wrap(err, "sender parse script")
		}

		publicKey, err := bitcoin.PublicKeyFromBytes(pk)
		if err != nil {
			return nil, false, 0, errors.Wrap(err, "sender public key")
		}

		for index, m := range r.Members {
			if publicKey.Equal(m.NextKey) {
				if !areSender {
					memberIndex = uint32(index)
				}
				m.IncrementHash()
				break
			}
		}
	}

	for _, receiverIndex := range message.ReceiverIndexes {
		publicKey, err := itx.Outputs[receiverIndex].Address.GetPublicKey()
		if err != nil {
			if errors.Cause(err) == bitcoin.ErrWrongType {
				continue
			}
			return nil, false, 0, errors.Wrap(err, "get public key")
		}

		for _, m := range r.Members {
			if publicKey.Equal(m.NextKey) {
				m.IncrementHash()
				break
			}
		}
	}

	logger.Info(ctx, "Found relationship : %s", r.TxId.String())

	return r, areSender, memberIndex, nil
}

func (rs *Relationships) FindHash(ctx context.Context, r *Relationship,
	publicKey bitcoin.PublicKey) (bitcoin.Hash32, error) {

	if r.NextKey.Equal(publicKey) {
		logger.Info(ctx, "Key matches our next key %d", r.NextIndex)
		return r.NextHash, nil
	}

	// Check current keys
	for i, m := range r.Members {
		if m.NextKey.Equal(publicKey) {
			logger.Info(ctx, "Key matches next key %d for member %d", m.NextIndex, i)
			return m.NextHash, nil
		}
	}

	// Check past keys
	baseKey, err := rs.wallet.GetKey(ctx, r.KeyType, r.KeyIndex)
	if err != nil {
		return bitcoin.Hash32{}, errors.Wrap(err, "get key")
	}

	h, in, err := r.FindKey(baseKey.PublicKey(), publicKey)
	if err == nil {
		logger.Info(ctx, "Key matches our index %d", in)
		return h, nil
	} else if errors.Cause(err) != ErrKeyNotFound {
		return bitcoin.Hash32{}, errors.Wrap(err, "find key")
	}

	for i, m := range r.Members {
		h, in, err := m.FindKey(publicKey, r.Seed)
		if err != nil {
			if errors.Cause(err) == ErrKeyNotFound {
				continue
			}
			return bitcoin.Hash32{}, errors.Wrap(err, "find key")
		}

		logger.Info(ctx, "Key matches index %d for member %d", in, i)
		return bitcoin.AddHashes(r.EncryptionKey, h), nil
	}

	return bitcoin.Hash32{}, ErrKeyNotFound
}

func (rs *Relationships) FindEncryptionKey(ctx context.Context, r *Relationship,
	publicKey bitcoin.PublicKey) (bitcoin.Hash32, error) {

	hash, err := rs.FindHash(ctx, r, publicKey)
	if err != nil {
		return bitcoin.Hash32{}, errors.Wrap(err, "find hash")
	}

	return bitcoin.AddHashes(r.EncryptionKey, hash), nil
}

func (rs *Relationships) DecryptAction(ctx context.Context, itx *inspector.Transaction, index int,
	flag []byte) (actions.Action, bitcoin.Hash32, error) {

	// Reparse the full envelope message
	env, err := envelope.Deserialize(bytes.NewReader(itx.MsgTx.TxOut[index].PkScript))
	if err != nil {
		return nil, bitcoin.Hash32{}, errors.Wrap(err, "deserialize envelope")
	}

	if !bytes.Equal(env.PayloadProtocol(), protocol.GetProtocolID(rs.cfg.IsTest)) {
		return nil, bitcoin.Hash32{}, protocol.ErrNotTokenized
	}

	logger.Info(ctx, "Decrypting action in output %d", index)

	rs.lock.Lock()
	defer rs.lock.Unlock()

	if len(flag) == 0 { // Not related to a relationship with a indirect encryption
		return rs.wallet.DecryptActionDirect(ctx, itx.MsgTx, index, env)
	}

	// Find relationship
	r := rs.findRelationshipForFlag(ctx, flag)
	if r == nil { // Not related to a relationship with a indirect encryption
		return rs.wallet.DecryptActionDirect(ctx, itx.MsgTx, index, env)
	}

	logger.Info(ctx, "Found relationship for decryption : %s", r.TxId.String())

	if r.EncryptionType == 0 { // Relationship uses direct encryption
		logger.Info(ctx, "Relationship uses direct encryption : %s", r.TxId.String())
		return rs.wallet.DecryptActionDirect(ctx, itx.MsgTx, index, env)
	}

	logger.Info(ctx, "Relationship uses indirect encryption : %s", r.TxId.String())

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

		logger.Info(ctx, "Checking input %d address : %s", i,
			bitcoin.NewAddressFromRawAddress(itx.Inputs[i].Address, rs.cfg.Net).String())

		encryptionKey, err := rs.FindEncryptionKey(ctx, r, publicKey)
		if err != nil {
			if errors.Cause(err) == ErrKeyNotFound {
				logger.Info(ctx, "Key not found")
				continue // this input is not part of this relationship
			}
			return nil, bitcoin.Hash32{}, errors.Wrap(err, "find encryption key")
		}

		logger.Info(ctx, "Decrypting action with indirect key")
		action, err := rs.wallet.DecryptActionIndirect(ctx, env, encryptionKey)
		if err != nil {
			logger.Info(ctx, "Failed to decrypt action indirect : %s", err)
			return nil, bitcoin.Hash32{}, errors.Wrap(err, "decrypt action indirect")
		}
		if action != nil {
			return action, bitcoin.Hash32{}, err
		}
	}

	return rs.wallet.DecryptActionDirect(ctx, itx.MsgTx, index, env)
}

func (rs *Relationships) Load(ctx context.Context, dbConn *db.DB) error {
	rs.lock.Lock()
	defer rs.lock.Unlock()

	b, err := dbConn.Fetch(ctx, relationshipsKey)
	if err == nil {
		if err := rs.Deserialize(bytes.NewReader(b)); err != nil {
			return errors.Wrap(err, "deserialize wallet")
		}
	} else if err != db.ErrNotFound {
		return errors.Wrap(err, "fetch wallet")
	}

	// Calculate relationship next key values
	for _, r := range rs.Relationships {
		key, err := rs.wallet.GetKey(ctx, r.KeyType, r.KeyIndex)
		if err != nil {
			return errors.Wrap(err, "get key")
		}

		r.NextKey, err = bitcoin.NextPublicKey(key.PublicKey(), r.NextHash)
		if err != nil {
			return errors.Wrap(err, "next key")
		}
	}

	return nil
}

func (rs *Relationships) Save(ctx context.Context, dbConn *db.DB) error {
	rs.lock.Lock()
	defer rs.lock.Unlock()

	var buf bytes.Buffer
	if err := rs.Serialize(&buf); err != nil {
		return errors.Wrap(err, "serialize wallet")
	}

	if err := dbConn.Put(ctx, relationshipsKey, buf.Bytes()); err != nil {
		return errors.Wrap(err, "put wallet")
	}

	return nil
}
