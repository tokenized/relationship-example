package relationships

import "github.com/tokenized/smart-contract/pkg/bitcoin"

func (m *Member) IncrementHash() {
	m.NextHash = bitcoin.NextHash(m.NextHash)
	m.NextIndex++

	m.NextKey, _ = bitcoin.NextPublicKey(m.BaseKey, m.NextHash)
}
