package relationship

import (
	"fmt"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/inspector"

	"github.com/tokenized/specification/dist/golang/actions"
	"github.com/tokenized/specification/dist/golang/messages"

	"github.com/pkg/errors"
)

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

func ParseRelationshipInitiation(keyIndex uint32, secret []byte, itx *inspector.Transaction,
	message *actions.Message, initiate *messages.InitiateRelationship) (Relationship, error) {

	result := Relationship{
		KeyIndex: keyIndex,
	}

	result.Seed = initiate.SeedValue
	result.Flag = initiate.FlagValue

	// TODO Other Fields --ce
	// initiate.Type
	// initiate.ProofOfIdentityType
	// initiate.ProofOfIdentity
	// initiate.ChannelParties

	for _, receiverIndex := range message.ReceiverIndexes {
		if int(receiverIndex) >= len(itx.Outputs) {
			return result, fmt.Errorf("Receiver index out of range : %d/%d", receiverIndex,
				len(itx.Outputs))
		}

		if itx.Outputs[receiverIndex].Address.Type() != bitcoin.ScriptTypePK {
			return result, errors.New("Receiver locking script not P2PK")
		}

		pk, err := itx.Outputs[receiverIndex].Address.GetPublicKey()
		if err != nil {
			return result, errors.Wrap(err, "get public key")
		}

		result.Members = append(result.Members, Member{
			BaseKey: pk,
		})
	}

	return result, nil
}
