package relationships

import (
	"github.com/pkg/errors"
	"github.com/tokenized/smart-contract/pkg/bitcoin"
)

func (m *Member) IncrementHash() {
	m.NextHash = bitcoin.NextHash(m.NextHash)
	m.NextIndex++

	m.NextKey, _ = bitcoin.NextPublicKey(m.BaseKey, m.NextHash)
}

func (m *Member) FindKey(publicKey bitcoin.PublicKey, seed []byte) (bitcoin.Hash32, uint64, error) {
	hp, _ := bitcoin.NewHash32(bitcoin.Sha256(seed))
	h := *hp

	for i := uint64(1); i < m.NextIndex+10; i++ {
		npk, err := bitcoin.NextPublicKey(m.BaseKey, h)
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
