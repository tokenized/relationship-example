package wallet

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/tokenized/smart-contract/pkg/inspector"

	"github.com/pkg/errors"
)

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
