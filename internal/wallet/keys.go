package wallet

import (
	"context"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/logger"

	"github.com/pkg/errors"
)

const (
	KeyTypeExternal  = uint32(0)
	KeyTypeInternal  = uint32(1)
	KeyTypeRelateOut = uint32(2) // Outgoing - Used for initiating relationships
	KeyTypeRelateIn  = uint32(3) // Incoming - Used for receiving relationship requests

	KeyTypeCount = 4
)

var (
	KeyTypeName = []string{
		"External",
		"Internal",
		"Relate Out",
		"Relate In",
	}
)

func (w *Wallet) GetUnusedKey(ctx context.Context, keyType uint32) (bitcoin.Key, uint32, error) {
	w.addressLock.Lock()
	defer w.addressLock.Unlock()

	for _, address := range w.addressesList[keyType] {
		if !address.Used && !address.Given {
			address.Given = true
			key, err := w.GetKey(ctx, keyType, address.KeyIndex)
			return key, address.KeyIndex, err
		}
	}

	return bitcoin.Key{}, 0, errors.New("Not Available")
}

func (w *Wallet) GetKey(ctx context.Context, t, i uint32) (bitcoin.Key, error) {
	parentKey, err := w.walletKey.ChildKey(t)
	if err != nil {
		return bitcoin.Key{}, errors.Wrap(err, "parent key")
	}

	key, err := parentKey.ChildKey(i)
	if err != nil {
		return bitcoin.Key{}, errors.Wrap(err, "address key")
	}

	return key.Key(bitcoin.InvalidNet), nil
}

// AddIndependentKey adds a key derived outside the wallet to wallet tx filtering.
func (w *Wallet) AddIndependentKey(ctx context.Context, pk bitcoin.PublicKey,
	keyType, keyIndex uint32, hash bitcoin.Hash32) error {

	ra, err := pk.RawAddress()
	if err != nil {
		return errors.Wrap(err, "raw address")
	}

	logger.Info(ctx, "Adding independent key for address : %s",
		bitcoin.NewAddressFromRawAddress(ra, w.cfg.Net).String())

	pkra, err := bitcoin.NewRawAddressPublicKey(pk)
	if err != nil {
		return errors.Wrap(err, "raw address")
	}

	logger.Info(ctx, "Adding independent key for pk address : %s",
		bitcoin.NewAddressFromRawAddress(pkra, w.cfg.Net).String())

	hashes, err := ra.Hashes()
	if err != nil {
		return errors.Wrap(err, "new address hashes")
	}

	w.hashLock.Lock()
	for _, hash := range hashes {
		w.hashes[hash] = ra
	}
	w.hashLock.Unlock()

	ad := &Address{
		Address:   ra,
		PublicKey: pk,
		KeyType:   keyType,
		KeyIndex:  keyIndex,
		KeyHash:   &hash,
	}

	w.addressLock.Lock()
	for _, hash := range hashes {
		w.addressesMap[hash] = ad
	}
	w.addressLock.Unlock()

	return nil
}
