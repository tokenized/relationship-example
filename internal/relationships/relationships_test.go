package relationships

import (
	"bytes"
	"testing"

	"github.com/tokenized/relationship-example/internal/platform/tests"

	"github.com/tokenized/specification/dist/golang/actions"
	"github.com/tokenized/specification/dist/golang/messages"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
)

func TestInitiateDirect(t *testing.T) {
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

	originalIR, err := rs.InitiateRelationship(ctx, []bitcoin.PublicKey{receiver.PublicKey()})
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
	for index, _ := range tx.TxOut {
		action, _, err := wallet.DecryptActionDirect(ctx, tx, index)
		if err != nil {
			continue
		}

		switch a := action.(type) {
		case *actions.Message:
			messageIndex = index
			message = a
		}
	}

	if messageIndex == 0xffffffff {
		t.Fatalf("Message not found in tx")
	}

	t.Logf("Message code : %d", message.MessageCode)

	if message.MessageCode != messages.CodeInitiateRelationship {
		t.Fatalf("Wrong message code : got %d, want %d", message.MessageCode,
			messages.CodeInitiateRelationship)
	}

	p, err := messages.Deserialize(message.MessageCode, message.MessagePayload)
	if err != nil {
		t.Fatalf("Failed to deserialize message payload : %s", err)
	}

	ir, ok := p.(*messages.InitiateRelationship)
	if !ok {
		t.Fatalf("Wrong message type")
	}

	if !bytes.Equal(originalIR.SeedValue, ir.SeedValue) {
		t.Fatalf("Wrong seed value : \n  got  %x\n  want %x", originalIR.SeedValue, ir.SeedValue)
	}

	t.Logf("Type : %d", ir.Type)

	if ir.Type != 0 {
		t.Fatalf("Wrong relationship type : got %d, want %d", ir.Type, 0)
	}
}

func TestInitiateDirectMulti(t *testing.T) {
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

	receiver1, err := bitcoin.GenerateKey(bitcoin.MainNet)
	if err != nil {
		t.Fatalf("Failed to generate receiver key : %s", err)
	}

	receiver2, err := bitcoin.GenerateKey(bitcoin.MainNet)
	if err != nil {
		t.Fatalf("Failed to generate receiver key : %s", err)
	}

	originalIR, err := rs.InitiateRelationship(ctx, []bitcoin.PublicKey{
		receiver1.PublicKey(),
		receiver2.PublicKey(),
	})
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
	for index, _ := range tx.TxOut {
		action, _, err := wallet.DecryptActionDirect(ctx, tx, index)
		if err != nil {
			continue
		}

		switch a := action.(type) {
		case *actions.Message:
			messageIndex = index
			message = a
		}
	}

	if messageIndex == 0xffffffff {
		t.Fatalf("Message not found in tx")
	}

	t.Logf("Message code : %d", message.MessageCode)

	if message.MessageCode != messages.CodeInitiateRelationship {
		t.Fatalf("Wrong message code : got %d, want %d", message.MessageCode,
			messages.CodeInitiateRelationship)
	}

	p, err := messages.Deserialize(message.MessageCode, message.MessagePayload)
	if err != nil {
		t.Fatalf("Failed to deserialize message payload : %s", err)
	}

	ir, ok := p.(*messages.InitiateRelationship)
	if !ok {
		t.Fatalf("Wrong message type")
	}

	if !bytes.Equal(originalIR.SeedValue, ir.SeedValue) {
		t.Fatalf("Wrong seed value : \n  got  %x\n  want %x", originalIR.SeedValue, ir.SeedValue)
	}

	t.Logf("Type : %d", ir.Type)

	if ir.Type != 0 {
		t.Fatalf("Wrong relationship type : got %d, want %d", ir.Type, 0)
	}
}
