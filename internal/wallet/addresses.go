package wallet

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/logger"

	"github.com/pkg/errors"
)

var (
	KeyTypeName = []string{
		"External",
		"Internal",
		"Relationship",
	}
)

func (w *Wallet) FindAddress(ctx context.Context, t, i uint32) *Address {
	if t > 2 {
		return nil
	}

	w.addressLock.Lock()
	defer w.addressLock.Unlock()
	return w.addressesList[t][i]
}

func (w *Wallet) FindAddressByAddress(ctx context.Context, ra bitcoin.RawAddress) (*Address, error) {
	hashes, err := ra.Hashes()
	if err != nil {
		return nil, errors.Wrap(err, "address hash")
	}

	w.addressLock.Lock()
	defer w.addressLock.Unlock()

	for _, hash := range hashes {
		result, exists := w.addressesMap[hash]
		if !exists {
			continue
		}

		if result.Address.Equal(ra) {
			return result, nil
		}
	}

	return nil, nil
}

func (w *Wallet) MarkAddress(ctx context.Context, add *Address) error {
	if add.Used == true {
		return nil // already used
	}

	w.addressLock.Lock()
	defer w.addressLock.Unlock()

	add.Used = true

	if int(add.KeyIndex)+w.cfg.AddressGap > len(w.addressesList[add.KeyType]) {
		if err := w.ForwardScan(ctx, add.KeyType); err != nil {
			return errors.Wrap(err, "forward scan")
		}
	}

	return nil
}

func (w *Wallet) ForwardScan(ctx context.Context, t uint32) error {
	if t > 2 {
		return fmt.Errorf("Invalid address type : %d", t)
	}

	list := w.addressesList[t]

	unusedCount := 0
	for i := len(list) - 1; i >= 0; i-- {
		if list[i].Used {
			break
		}
		unusedCount++
		if unusedCount >= w.cfg.AddressGap {
			return nil
		}
	}

	parentKey, err := w.walletKey.ChildKey(t)
	if err != nil {
		return errors.Wrap(err, "parent key")
	}

	nextIndex := uint32(len(list))
	for i := unusedCount; i < w.cfg.AddressGap; i++ {
		key, err := parentKey.ChildKey(nextIndex)
		if err != nil {
			return errors.Wrap(err, "address key")
		}

		ra, err := key.RawAddress()
		if err != nil {
			return errors.Wrap(err, "address")
		}

		hashes, err := ra.Hashes()
		if err != nil {
			return errors.Wrap(err, "address hash")
		}

		logger.Info(ctx, "Generated address : %s %d %s", KeyTypeName[t], nextIndex,
			bitcoin.NewAddressFromRawAddress(ra, w.cfg.Net).String())

		newAddress := &Address{
			Address:   ra,
			PublicKey: key.PublicKey(),
			KeyType:   t,
			KeyIndex:  nextIndex,
		}

		for _, hash := range hashes {
			if err := w.addMonitoredHash(ctx, hash); err != nil {
				return errors.Wrap(err, "add hash")
			}
			w.addressesMap[hash] = newAddress
		}
		w.addressesList[t] = append(w.addressesList[t], newAddress)

		nextIndex++
	}

	return nil
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

	if err := binary.Read(buf, binary.LittleEndian, &a.Used); err != nil {
		return errors.Wrap(err, "used")
	}

	return nil
}
