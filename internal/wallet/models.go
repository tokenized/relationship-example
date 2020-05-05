package wallet

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/inspector"

	"github.com/pkg/errors"
)

type Address struct {
	Address   bitcoin.RawAddress
	PublicKey bitcoin.PublicKey
	KeyType   uint32
	KeyIndex  uint32
	KeyHash   *bitcoin.Hash32
	Used      bool
	Given     bool
}

type UTXO struct {
	UTXO     bitcoin.UTXO
	KeyType  uint32
	KeyIndex uint32
	KeyHash  *bitcoin.Hash32
	Pending  bool
	Reserved bool
	Deleted  bool
}

type Transaction struct {
	Itx *inspector.Transaction
}

func (a Address) Serialize(buf *bytes.Buffer) error {
	// Version
	if err := binary.Write(buf, binary.LittleEndian, uint8(0)); err != nil {
		return errors.Wrap(err, "version")
	}

	if err := a.Address.Serialize(buf); err != nil {
		return errors.Wrap(err, "address")
	}

	if err := binary.Write(buf, binary.LittleEndian, a.KeyType); err != nil {
		return errors.Wrap(err, "type")
	}

	if err := binary.Write(buf, binary.LittleEndian, a.KeyIndex); err != nil {
		return errors.Wrap(err, "index")
	}

	if a.KeyHash != nil {
		if err := binary.Write(buf, binary.LittleEndian, true); err != nil {
			return errors.Wrap(err, "key hash exists")
		}

		if err := a.KeyHash.Serialize(buf); err != nil {
			return errors.Wrap(err, "key hash")
		}
	} else {
		if err := binary.Write(buf, binary.LittleEndian, false); err != nil {
			return errors.Wrap(err, "key hash exists")
		}
	}

	if err := binary.Write(buf, binary.LittleEndian, a.Used); err != nil {
		return errors.Wrap(err, "used")
	}

	return nil
}

func (a *Address) Deserialize(buf *bytes.Reader) error {
	var version uint8
	if err := binary.Read(buf, binary.LittleEndian, &version); err != nil {
		return errors.Wrap(err, "version")
	}

	if version != 0 {
		return fmt.Errorf("Unsupported version : %d", version)
	}

	if err := a.Address.Deserialize(buf); err != nil {
		return errors.Wrap(err, "address")
	}

	if err := binary.Read(buf, binary.LittleEndian, &a.KeyType); err != nil {
		return errors.Wrap(err, "type")
	}

	if err := binary.Read(buf, binary.LittleEndian, &a.KeyIndex); err != nil {
		return errors.Wrap(err, "index")
	}

	var keyExists bool
	if err := binary.Read(buf, binary.LittleEndian, &keyExists); err != nil {
		return errors.Wrap(err, "key hash exists")
	}

	if keyExists {
		a.KeyHash = &bitcoin.Hash32{}
		if err := a.KeyHash.Deserialize(buf); err != nil {
			return errors.Wrap(err, "key hash")
		}
	} else {
		a.KeyHash = nil
	}

	if err := binary.Read(buf, binary.LittleEndian, &a.Used); err != nil {
		return errors.Wrap(err, "used")
	}

	return nil
}

func (tx Transaction) Serialize(buf *bytes.Buffer) error {
	// Version
	if err := binary.Write(buf, binary.LittleEndian, uint8(0)); err != nil {
		return errors.Wrap(err, "version")
	}

	if err := tx.Itx.Write(buf); err != nil {
		return errors.Wrap(err, "utxo")
	}

	return nil
}

func (tx *Transaction) Deserialize(buf *bytes.Reader, isTest bool) error {
	var version uint8
	if err := binary.Read(buf, binary.LittleEndian, &version); err != nil {
		return errors.Wrap(err, "version")
	}

	if version != 0 {
		return fmt.Errorf("Unsupported version : %d", version)
	}

	var itx inspector.Transaction
	if err := itx.Read(buf, isTest); err != nil {
		return errors.Wrap(err, "utxo")
	}
	tx.Itx = &itx

	return nil
}

func (u UTXO) Serialize(buf *bytes.Buffer) error {
	// Version
	if err := binary.Write(buf, binary.LittleEndian, uint8(0)); err != nil {
		return errors.Wrap(err, "version")
	}

	if err := u.UTXO.Write(buf); err != nil {
		return errors.Wrap(err, "utxo")
	}

	if err := binary.Write(buf, binary.LittleEndian, u.KeyType); err != nil {
		return errors.Wrap(err, "type")
	}

	if err := binary.Write(buf, binary.LittleEndian, u.KeyIndex); err != nil {
		return errors.Wrap(err, "index")
	}

	if u.KeyHash != nil {
		if err := binary.Write(buf, binary.LittleEndian, true); err != nil {
			return errors.Wrap(err, "key hash exists")
		}

		if err := u.KeyHash.Serialize(buf); err != nil {
			return errors.Wrap(err, "key hash")
		}
	} else {
		if err := binary.Write(buf, binary.LittleEndian, false); err != nil {
			return errors.Wrap(err, "key hash exists")
		}
	}

	if err := binary.Write(buf, binary.LittleEndian, u.Pending); err != nil {
		return errors.Wrap(err, "pending")
	}

	if err := binary.Write(buf, binary.LittleEndian, u.Reserved); err != nil {
		return errors.Wrap(err, "reserved")
	}

	if err := binary.Write(buf, binary.LittleEndian, u.Deleted); err != nil {
		return errors.Wrap(err, "deleted")
	}

	return nil
}

func (u *UTXO) Deserialize(buf *bytes.Reader) error {
	var version uint8
	if err := binary.Read(buf, binary.LittleEndian, &version); err != nil {
		return errors.Wrap(err, "version")
	}

	if version != 0 {
		return fmt.Errorf("Unsupported version : %d", version)
	}

	if err := u.UTXO.Read(buf); err != nil {
		return errors.Wrap(err, "utxo")
	}

	if err := binary.Read(buf, binary.LittleEndian, &u.KeyType); err != nil {
		return errors.Wrap(err, "type")
	}

	if err := binary.Read(buf, binary.LittleEndian, &u.KeyIndex); err != nil {
		return errors.Wrap(err, "index")
	}

	var keyExists bool
	if err := binary.Read(buf, binary.LittleEndian, &keyExists); err != nil {
		return errors.Wrap(err, "key hash exists")
	}

	if keyExists {
		u.KeyHash = &bitcoin.Hash32{}
		if err := u.KeyHash.Deserialize(buf); err != nil {
			return errors.Wrap(err, "key hash")
		}
	} else {
		u.KeyHash = nil
	}

	if err := binary.Read(buf, binary.LittleEndian, &u.Pending); err != nil {
		return errors.Wrap(err, "pending")
	}

	if err := binary.Read(buf, binary.LittleEndian, &u.Reserved); err != nil {
		return errors.Wrap(err, "reserved")
	}

	if err := binary.Read(buf, binary.LittleEndian, &u.Deleted); err != nil {
		return errors.Wrap(err, "deleted")
	}

	return nil
}

func ConvertUTXOs(utxos []*UTXO) []bitcoin.UTXO {
	result := make([]bitcoin.UTXO, 0, len(utxos))
	for _, utxo := range utxos {
		result = append(result, utxo.UTXO)
	}
	return result
}
