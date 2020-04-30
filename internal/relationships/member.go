package relationships

import "github.com/tokenized/smart-contract/pkg/bitcoin"

func (m *Member) NextKey() (bitcoin.PublicKey, error) {
	return bitcoin.NextPublicKey(m.BaseKey, m.NextHash)
}

func (m *Member) IncrementHash() {
	m.NextHash = bitcoin.NextHash(m.NextHash)
	m.NextIndex++
}
