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

func (r *Relationship) FindEncryptionKey(publicKey bitcoin.PublicKey) (bitcoin.Hash32, error) {
	for _, m := range r.Members {
		if m.NextKey.Equal(publicKey) {
			return bitcoin.AddHashes(r.EncryptionKey, m.NextHash), nil
		}
	}

	return bitcoin.Hash32{}, ErrKeyNotFound
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

	return nil
}
