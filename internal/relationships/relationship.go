package relationships

import (
	"context"

	"github.com/tokenized/relationship-example/internal/wallet"

	"github.com/tokenized/smart-contract/pkg/bitcoin"

	"github.com/pkg/errors"
)

var (
	ErrKeyNotFound = errors.New("Key not found")
)

func (r *Relationship) FindKey(baseKey bitcoin.PublicKey, publicKey bitcoin.PublicKey) (bitcoin.Hash32, uint64, error) {
	hp, _ := bitcoin.NewHash32(bitcoin.Sha256(r.Seed))
	h := *hp

	for i := uint64(1); i < r.NextIndex+10; i++ {
		npk, err := bitcoin.NextPublicKey(baseKey, h)
		if err != nil {
			return bitcoin.Hash32{}, 0, errors.Wrap(err, "next public key")
		}
		if npk.Equal(publicKey) {
			return h, i, nil
		}

		h = bitcoin.NextHash(h)
	}

	return bitcoin.Hash32{}, 0, ErrKeyNotFound
}

func (r *Relationship) IncrementHash(ctx context.Context, wallet *wallet.Wallet) error {
	r.NextHash = bitcoin.NextHash(r.NextHash)
	r.NextIndex++

	baseKey, err := wallet.GetKey(ctx, r.KeyType, r.KeyIndex)
	if err != nil {
		return errors.Wrap(err, "get key")
	}

	r.NextKey, err = bitcoin.NextPublicKey(baseKey.PublicKey(), r.NextHash)
	if err != nil {
		return errors.Wrap(err, "get key")
	}

	if err := wallet.AddIndependentKey(ctx, r.NextKey, r.KeyType, r.KeyIndex, r.NextHash); err != nil {
		return errors.Wrap(err, "add independent key")
	}

	return nil
}
