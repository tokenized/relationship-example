package wallet

import (
	"context"
	"fmt"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/logger"

	"github.com/pkg/errors"
)

// GetUnusedRawAddress returns an unused raw address of the specified type.
func (w *Wallet) GetUnusedRawAddress(ctx context.Context, keyType uint32) (bitcoin.RawAddress, error) {
	w.addressLock.Lock()
	defer w.addressLock.Unlock()

	for _, address := range w.addressesList[keyType] {
		if !address.Used && !address.Given {
			address.Given = true
			return address.Address, nil
		}
	}

	return bitcoin.RawAddress{}, errors.New("Not Available")
}

// GetUnusedAddress returns an unused address of the specified type.
func (w *Wallet) GetUnusedAddress(ctx context.Context, keyType uint32) (*Address, error) {
	w.addressLock.Lock()
	defer w.addressLock.Unlock()

	for _, address := range w.addressesList[keyType] {
		if !address.Used && !address.Given {
			address.Given = true
			return address, nil
		}
	}

	return nil, errors.New("Not Available")
}

// GetAddress gets an address by type and index.
func (w *Wallet) GetAddress(ctx context.Context, t, i uint32) *Address {
	if t >= KeyTypeCount {
		return nil
	}

	w.addressLock.Lock()
	defer w.addressLock.Unlock()

	if int(i) >= len(w.addressesList[t]) {
		return nil
	}
	return w.addressesList[t][i]
}

// FindAddress finds an address by the raw address.
func (w *Wallet) FindAddress(ctx context.Context, ra bitcoin.RawAddress) (*Address, error) {
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

		publicKey, err := ra.GetPublicKey()
		if err == nil && publicKey.Equal(result.PublicKey) {
			return result, nil
		}
	}

	return nil, nil
}

func (w *Wallet) MarkAddress(ctx context.Context, add *Address) error {
	if add.Used == true {
		return nil // already used
	}

	logger.Info(ctx, "Mark address : %s %d %s", KeyTypeName[add.KeyType], add.KeyIndex,
		bitcoin.NewAddressFromRawAddress(add.Address, w.cfg.Net).String())

	w.addressLock.Lock()
	defer w.addressLock.Unlock()

	add.Used = true

	if int(add.KeyIndex)+w.cfg.AddressGap >= len(w.addressesList[add.KeyType]) {
		if err := w.forwardScan(ctx, add.KeyType); err != nil {
			return errors.Wrap(err, "forward scan")
		}
	}

	return nil
}

func (w *Wallet) forwardScan(ctx context.Context, t uint32) error {
	if t >= KeyTypeCount {
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

	if unusedCount >= w.cfg.AddressGap {
		return nil
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

		var ra bitcoin.RawAddress
		if t == KeyTypeRelateIn {
			ra, err = bitcoin.NewRawAddressPublicKey(key.PublicKey())
			if err != nil {
				return errors.Wrap(err, "address")
			}
		} else {
			ra, err = key.RawAddress()
			if err != nil {
				return errors.Wrap(err, "address")
			}
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

		w.hashLock.Lock()
		for _, hash := range hashes {
			w.hashes[hash] = ra
			w.addressesMap[hash] = newAddress
		}
		w.hashLock.Unlock()
		w.addressesList[t] = append(w.addressesList[t], newAddress)

		nextIndex++
	}

	return nil
}
