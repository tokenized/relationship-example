package relationships

import (
	"bytes"
	"encoding/binary"

	"github.com/tokenized/relationship-example/internal/platform/config"
	"github.com/tokenized/relationship-example/internal/wallet"

	"github.com/tokenized/smart-contract/pkg/bitcoin"

	"github.com/pkg/errors"
)

// Relationships is a manager for all of the relationships associated with a wallet.
type Relationships struct {
	cfg         *config.Config
	wallet      *wallet.Wallet
	broadcastTx wallet.BroadcastTx

	Relationships []*Relationship
}

// Relationship represents a relationship, a private communication channel between two or more
//   parties.
type Relationship struct {
	KeyIndex      uint32
	NextHash      bitcoin.Hash32
	NextIndex     uint64
	Seed          []byte
	Flag          []byte
	EncryptionKey bitcoin.Hash32
	Members       []*Member
}

// Member represents a member of a relationship.
type Member struct {
	// The base public key used for deriving keys
	BaseKey bitcoin.PublicKey

	// Next expected hash to be used in a message
	NextHash  bitcoin.Hash32
	NextIndex uint64
}

func (m Member) Serialize(buf *bytes.Buffer) error {
	if err := m.BaseKey.Serialize(buf); err != nil {
		return errors.Wrap(err, "base key")
	}

	if err := m.NextHash.Serialize(buf); err != nil {
		return errors.Wrap(err, "next hash")
	}

	if err := binary.Write(buf, binary.LittleEndian, m.NextIndex); err != nil {
		return errors.Wrap(err, "next index")
	}

	return nil
}

func (m *Member) Deserialize(buf *bytes.Reader) error {
	if err := m.BaseKey.Deserialize(buf); err != nil {
		return errors.Wrap(err, "base key")
	}

	if err := m.NextHash.Deserialize(buf); err != nil {
		return errors.Wrap(err, "next hash")
	}

	if _, err := buf.Read(m.NextHash[:]); err != nil {
		return errors.Wrap(err, "next hash")
	}

	if err := binary.Read(buf, binary.LittleEndian, &m.NextIndex); err != nil {
		return errors.Wrap(err, "next index")
	}

	return nil
}

func (r Relationship) Serialize(buf *bytes.Buffer) error {
	if err := binary.Write(buf, binary.LittleEndian, r.KeyIndex); err != nil {
		return errors.Wrap(err, "key index")
	}

	if err := r.NextHash.Serialize(buf); err != nil {
		return errors.Wrap(err, "next hash")
	}

	if err := binary.Write(buf, binary.LittleEndian, r.NextIndex); err != nil {
		return errors.Wrap(err, "next index")
	}

	if err := binary.Write(buf, binary.LittleEndian, uint16(len(r.Seed))); err != nil {
		return errors.Wrap(err, "seed size")
	}
	if _, err := buf.Write(r.Seed); err != nil {
		return errors.Wrap(err, "seed")
	}

	if err := binary.Write(buf, binary.LittleEndian, uint16(len(r.Flag))); err != nil {
		return errors.Wrap(err, "flag size")
	}
	if _, err := buf.Write(r.Flag); err != nil {
		return errors.Wrap(err, "flag")
	}

	if err := r.EncryptionKey.Serialize(buf); err != nil {
		return errors.Wrap(err, "encryption key")
	}

	if err := binary.Write(buf, binary.LittleEndian, uint64(len(r.Members))); err != nil {
		return errors.Wrap(err, "members size")
	}
	for _, m := range r.Members {
		if err := m.Serialize(buf); err != nil {
			return errors.Wrap(err, "member")
		}
	}

	return nil
}

func (r *Relationship) Deserialize(buf *bytes.Reader) error {
	if err := binary.Read(buf, binary.LittleEndian, &r.KeyIndex); err != nil {
		return errors.Wrap(err, "key index")
	}

	if err := r.NextHash.Deserialize(buf); err != nil {
		return errors.Wrap(err, "next hash")
	}

	if err := binary.Read(buf, binary.LittleEndian, &r.NextIndex); err != nil {
		return errors.Wrap(err, "next index")
	}

	var size uint16
	if err := binary.Read(buf, binary.LittleEndian, &size); err != nil {
		return errors.Wrap(err, "seed size")
	}
	r.Seed = make([]byte, size)
	if _, err := buf.Read(r.Seed); err != nil {
		return errors.Wrap(err, "seed")
	}

	if err := binary.Read(buf, binary.LittleEndian, &size); err != nil {
		return errors.Wrap(err, "flag size")
	}
	r.Flag = make([]byte, size)
	if _, err := buf.Read(r.Flag); err != nil {
		return errors.Wrap(err, "flag")
	}

	if err := r.EncryptionKey.Deserialize(buf); err != nil {
		return errors.Wrap(err, "encryption key")
	}

	var count uint64
	if err := binary.Read(buf, binary.LittleEndian, &count); err != nil {
		return errors.Wrap(err, "members size")
	}
	r.Members = make([]*Member, 0, count)
	for i := uint64(0); i < count; i++ {
		var m Member
		if err := m.Deserialize(buf); err != nil {
			return errors.Wrap(err, "member")
		}

		r.Members = append(r.Members, &m)
	}

	return nil
}

func (rs Relationships) Serialize(buf *bytes.Buffer) error {
	if err := binary.Write(buf, binary.LittleEndian, uint64(len(rs.Relationships))); err != nil {
		return errors.Wrap(err, "relationships size")
	}
	for _, r := range rs.Relationships {
		if err := r.Serialize(buf); err != nil {
			return errors.Wrap(err, "relationship")
		}
	}

	return nil
}

func (rs *Relationships) Deserialize(buf *bytes.Reader) error {
	var count uint64
	if err := binary.Read(buf, binary.LittleEndian, &count); err != nil {
		return errors.Wrap(err, "relationships size")
	}
	rs.Relationships = make([]*Relationship, 0, count)
	for i := uint64(0); i < count; i++ {
		var r Relationship
		if err := r.Deserialize(buf); err != nil {
			return errors.Wrap(err, "relationship")
		}

		rs.Relationships = append(rs.Relationships, &r)
	}

	return nil
}
