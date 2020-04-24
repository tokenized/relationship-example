package wallet

import (
	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/inspector"
)

type Address struct {
	Address   bitcoin.RawAddress
	PublicKey bitcoin.PublicKey
	KeyType   uint32
	KeyIndex  uint32
	Used      bool
}

type UTXO struct {
	UTXO     bitcoin.UTXO
	KeyType  uint32
	KeyIndex uint32
	Reserved bool
}

type Transaction struct {
	Itx *inspector.Transaction
}
