package relationships

import (
	"github.com/tokenized/smart-contract/pkg/bitcoin"

	"github.com/pkg/errors"
)

var (
	ErrKeyNotFound = errors.New("Key not found")
)

func (r *Relationship) NextKey(baseKey bitcoin.Key) (bitcoin.Key, error) {
	return bitcoin.NextKey(baseKey, r.NextHash)
}

func (r *Relationship) FindEncryptionKey(publicKey bitcoin.PublicKey) (bitcoin.Hash32, error) {
	for _, m := range r.Members {
		memberKey, err := m.NextKey()
		if err != nil {
			return bitcoin.Hash32{}, errors.Wrap(err, "member next key")
		}

		if memberKey.Equal(publicKey) {
			return bitcoin.AddHashes(r.EncryptionKey, m.NextHash), nil
		}
	}

	return bitcoin.Hash32{}, ErrKeyNotFound
}

func (r *Relationship) IncrementHash() {
	r.NextHash = bitcoin.NextHash(r.NextHash)
	r.NextIndex++
}
