package relationships

import "github.com/tokenized/smart-contract/pkg/bitcoin"

type Relationship struct {
	KeyIndex uint32
	Seed     []byte
	Flag     []byte
	Secret   []byte
	Members  []Member
}

type Member struct {
	BaseKey bitcoin.PublicKey
}
