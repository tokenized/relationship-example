package relationships

import (
	"github.com/tokenized/relationship-example/internal/platform/config"
	"github.com/tokenized/relationship-example/internal/wallet"
	"github.com/tokenized/smart-contract/pkg/bitcoin"
)

// Relationships is a manager for all of the relationships associated with a wallet.
type Relationships struct {
	cfg         *config.Config
	wallet      *wallet.Wallet
	broadcastTx wallet.BroadcastTx

	Relationships []*Relationship
}

// Relationship represents a relationship, a private communication channel between two or more
//   parties.
type Relationship struct {
	KeyIndex      uint32
	NextHash      bitcoin.Hash32
	NextIndex     uint64
	Seed          []byte
	Flag          []byte
	EncryptionKey bitcoin.Hash32
	Members       []*Member
}

// Member represents a member of a relationship.
type Member struct {
	// The base public key used for deriving keys
	BaseKey bitcoin.PublicKey

	// Next expected hash to be used in a message
	NextHash  bitcoin.Hash32
	NextIndex uint64
}
