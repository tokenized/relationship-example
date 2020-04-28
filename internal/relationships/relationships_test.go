package relationships

import (
	"bytes"
	"testing"

	"github.com/tokenized/relationship-example/internal/platform/tests"
	"github.com/tokenized/specification/dist/golang/actions"
	"github.com/tokenized/specification/dist/golang/messages"
	"github.com/tokenized/specification/dist/golang/protocol"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
)

func TestInitiate(t *testing.T) {
	ctx := tests.Context()
	cfg := tests.NewMockConfig()

	wallet, err := tests.NewMockWallet(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create mock wallet : %s", err)
	}

	broadcastTx := tests.NewMockBroadcaster(cfg)

	rs, err := NewRelationships(cfg, wallet, broadcastTx)
	if err != nil {
		t.Fatalf("Failed to create relationships : %s", err)
	}

	receiver, err := bitcoin.GenerateKey(bitcoin.MainNet)
	if err != nil {
		t.Fatalf("Failed to generate receiver key : %s", err)
	}

	originalIR, err := rs.CreateInitiateRelationship(ctx, receiver.PublicKey())
	if err != nil {
		t.Fatalf("Failed to create initiate relationship : %s", err)
	}

	if len(broadcastTx.Msgs) != 2 {
		t.Fatalf("Failed to create funding and relationship txs : %d", len(broadcastTx.Msgs))
	}

	// Check encryption
	tx := broadcastTx.Msgs[1]

	messageIndex := 0xffffffff
	var message *actions.Message
	for index, output := range tx.TxOut {
		a, err := protocol.Deserialize(output.PkScript, cfg.IsTest)
		if err != nil {
			continue
		}

		switch action := a.(type) {
		case *actions.Message:
			messageIndex = index
			message = action
			break
		}
	}

	if messageIndex == 0xffffffff {
		t.Fatalf("Message not found in tx")
	}

	unencrypted, err := wallet.DecryptScript(ctx, tx, tx.TxOut[messageIndex].PkScript)
	if err != nil {
		t.Fatalf("Failed to decrypt message : %s", err)
	}

	t.Logf("Decrypted : %x", unencrypted)

	a, err := actions.Deserialize([]byte(message.Code()), unencrypted)
	if err != nil {
		t.Fatalf("Failed to deserialize action : %s", err)
	}

	m, ok := a.(*actions.Message)
	if !ok {
		t.Fatalf("Action not a message")
	}

	p, err := messages.Deserialize(m.MessageCode, m.MessagePayload)
	if err != nil {
		t.Fatalf("Failed to deserialize message payload : %s", err)
	}

	ir, ok := p.(*messages.InitiateRelationship)
	if !ok {
		t.Fatalf("Wrong message type")
	}

	t.Logf("Seed Value : %x", ir.SeedValue)

	if !bytes.Equal(originalIR.SeedValue, ir.SeedValue) {
		t.Fatalf("Wrong seed value : \n  got  %x\n  want %x", originalIR.SeedValue, ir.SeedValue)
	}

	t.Logf("Type : %d", ir.Type)

	if ir.Type != 0 {
		t.Fatalf("Wrong relationship type : got %d, want %d", ir.Type, 0)
	}
}
