package relationships

import (
	"bytes"
	"testing"

	"github.com/tokenized/relationship-example/internal/platform/tests"
	"github.com/tokenized/relationship-example/internal/wallet"

	"github.com/tokenized/specification/dist/golang/actions"
	"github.com/tokenized/specification/dist/golang/messages"

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

	originalIR, err := rs.InitiateRelationship(ctx, []bitcoin.PublicKey{receiver.PublicKey()})
	if err != nil {
		t.Fatalf("Failed to create initiate relationship : %s", err)
	}

	if len(rs.Relationships) != 1 {
		t.Fatalf("Wrong relationship count : got %d, want %d", len(rs.Relationships), 1)
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
		t.Fatalf("Wrong seed value : \n  got  %x\n  want %x", ir.SeedValue, originalIR.SeedValue)
	}

	t.Logf("Type : %d", ir.Type)

	if ir.Type != 0 {
		t.Fatalf("Wrong relationship type : got %d, want %d", ir.Type, 0)
	}
}

func TestInitiateMulti(t *testing.T) {
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

	if len(rs.Relationships) != 1 {
		t.Fatalf("Wrong relationship count : got %d, want %d", len(rs.Relationships), 1)
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
		t.Fatalf("Wrong seed value : \n  got  %x\n  want %x", ir.SeedValue, originalIR.SeedValue)
	}

	t.Logf("Type : %d", ir.Type)

	if ir.Type != 0 {
		t.Fatalf("Wrong relationship type : got %d, want %d", ir.Type, 0)
	}
}

func TestAcceptDirect(t *testing.T) {
	ctx := tests.Context()
	cfg := tests.NewMockConfig()

	sendWallet, err := tests.NewMockWallet(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create mock wallet : %s", err)
	}

	sendBroadcastTx := tests.NewMockBroadcaster(cfg)

	sendRS, err := NewRelationships(cfg, sendWallet, sendBroadcastTx)
	if err != nil {
		t.Fatalf("Failed to create relationships : %s", err)
	}

	receiveWallet, err := tests.NewMockWallet(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create mock wallet : %s", err)
	}

	receiveBroadcastTx := tests.NewMockBroadcaster(cfg)

	receiveRS, err := NewRelationships(cfg, receiveWallet, receiveBroadcastTx)
	if err != nil {
		t.Fatalf("Failed to create relationships : %s", err)
	}

	receiver, err := receiveWallet.GetUnusedAddress(ctx, wallet.KeyTypeRelateIn)
	if err != nil {
		t.Fatalf("Failed to get relationship address : %s", err)
	}

	if receiver.Address.Type() != bitcoin.ScriptTypePK {
		t.Fatalf("Wrong receiver address type : got %d, want %d", receiver.Address.Type(),
			bitcoin.ScriptTypePK)
	}

	originalIR, err := sendRS.InitiateRelationship(ctx, []bitcoin.PublicKey{receiver.PublicKey})
	if err != nil {
		t.Fatalf("Failed to create initiate relationship : %s", err)
	}

	t.Logf("Seed : %x", originalIR.SeedValue)

	if len(sendRS.Relationships) != 1 {
		t.Fatalf("Wrong send relationship count : got %d, want %d", len(sendRS.Relationships), 1)
	}

	if len(sendBroadcastTx.Msgs) != 2 {
		t.Fatalf("Failed to create funding and relationship txs : %d", len(sendBroadcastTx.Msgs))
	}

	tx := sendBroadcastTx.Msgs[1]

	messageIndex := 0xffffffff
	var message *actions.Message
	var encryptionKey bitcoin.Hash32
	for index, _ := range tx.TxOut {
		var action actions.Action
		action, encryptionKey, err = receiveWallet.DecryptActionDirect(ctx, tx, index)
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
		t.Fatalf("Wrong seed value : \n  got  %x\n  want %x", ir.SeedValue, originalIR.SeedValue)
	}

	// Mock up inspector transaction
	itx, err := tests.CreateInspector(ctx, cfg, tx, nil)
	if err != nil {
		t.Fatalf("Failed to create transaction : %s", err)
	}

	if err := receiveRS.ProcessInitiateRelationship(ctx, itx, message, ir, encryptionKey); err != nil {
		t.Fatalf("Failed to process initiate : %s", err)
	}

	if len(receiveRS.Relationships) != 1 {
		t.Fatalf("Wrong receive relationship count : got %d, want %d", len(receiveRS.Relationships), 1)
	}

	originalAR, err := receiveRS.AcceptRelationship(ctx, receiveRS.Relationships[0])
	if err != nil {
		t.Fatalf("Failed to create accept relationship : %s", err)
	}

	tx = receiveBroadcastTx.Msgs[len(receiveBroadcastTx.Msgs)-1]

	t.Logf("Accept Tx : \n%s\n", tx.StringWithAddresses(cfg.Net))

	messageIndex = 0xffffffff
	for index, _ := range tx.TxOut {
		var action actions.Action
		action, encryptionKey, err = receiveWallet.DecryptActionDirect(ctx, tx, index)
		if err != nil {
			continue
		}

		switch a := action.(type) {
		case *actions.Message:
			messageIndex = index
			message = a
		default:
		}
	}

	if messageIndex == 0xffffffff {
		t.Fatalf("Message not found in tx")
	}

	t.Logf("Message code : %d", message.MessageCode)

	if message.MessageCode != messages.CodeAcceptRelationship {
		t.Fatalf("Wrong message code : got %d, want %d", message.MessageCode,
			messages.CodeAcceptRelationship)
	}

	p, err = messages.Deserialize(message.MessageCode, message.MessagePayload)
	if err != nil {
		t.Fatalf("Failed to deserialize message payload : %s", err)
	}

	ar, ok := p.(*messages.AcceptRelationship)
	if !ok {
		t.Fatalf("Wrong message type")
	}

	if originalAR.ProofOfIdentityType != ar.ProofOfIdentityType {
		t.Fatalf("Wrong POI type : got  %d want %d", ar.ProofOfIdentityType, originalAR.ProofOfIdentityType)
	}

	if !bytes.Equal(originalAR.ProofOfIdentity, ar.ProofOfIdentity) {
		t.Fatalf("Wrong POI : \n  got  %x\n  want %x", ar.ProofOfIdentity, originalAR.ProofOfIdentity)
	}

	// Mock up inspector transaction
	itx, err = tests.CreateInspector(ctx, cfg, tx, nil)
	if err != nil {
		t.Fatalf("Failed to create transaction : %s", err)
	}
}

func TestAcceptIndirect(t *testing.T) {
	ctx := tests.Context()
	cfg := tests.NewMockConfig()

	sendWallet, err := tests.NewMockWallet(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create mock wallet : %s", err)
	}

	sendBroadcastTx := tests.NewMockBroadcaster(cfg)

	sendRS, err := NewRelationships(cfg, sendWallet, sendBroadcastTx)
	if err != nil {
		t.Fatalf("Failed to create relationships : %s", err)
	}

	receiveWallet, err := tests.NewMockWallet(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create mock wallet : %s", err)
	}

	receiveBroadcastTx := tests.NewMockBroadcaster(cfg)

	receiveRS, err := NewRelationships(cfg, receiveWallet, receiveBroadcastTx)
	if err != nil {
		t.Fatalf("Failed to create relationships : %s", err)
	}

	receiver1, err := receiveWallet.GetUnusedAddress(ctx, wallet.KeyTypeRelateIn)
	if err != nil {
		t.Fatalf("Failed to get relationship address : %s", err)
	}

	if receiver1.Address.Type() != bitcoin.ScriptTypePK {
		t.Fatalf("Wrong receiver address type : got %d, want %d", receiver1.Address.Type(),
			bitcoin.ScriptTypePK)
	}

	thirdWallet, err := tests.NewMockWallet(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create mock wallet : %s", err)
	}

	receiver2, err := thirdWallet.GetUnusedAddress(ctx, wallet.KeyTypeRelateIn)
	if err != nil {
		t.Fatalf("Failed to get relationship address : %s", err)
	}

	if receiver2.Address.Type() != bitcoin.ScriptTypePK {
		t.Fatalf("Wrong receiver address type : got %d, want %d", receiver2.Address.Type(),
			bitcoin.ScriptTypePK)
	}

	originalIR, err := sendRS.InitiateRelationship(ctx, []bitcoin.PublicKey{
		receiver1.PublicKey,
		receiver2.PublicKey,
	})
	if err != nil {
		t.Fatalf("Failed to create initiate relationship : %s", err)
	}

	t.Logf("Seed : %x", originalIR.SeedValue)

	if len(sendRS.Relationships) != 1 {
		t.Fatalf("Wrong send relationship count : got %d, want %d", len(sendRS.Relationships), 1)
	}

	if len(sendBroadcastTx.Msgs) != 2 {
		t.Fatalf("Failed to create funding and relationship txs : %d", len(sendBroadcastTx.Msgs))
	}

	tx := sendBroadcastTx.Msgs[1]

	messageIndex := 0xffffffff
	var message *actions.Message
	var encryptionKey bitcoin.Hash32
	for index, _ := range tx.TxOut {
		var action actions.Action
		action, encryptionKey, err = receiveWallet.DecryptActionDirect(ctx, tx, index)
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
		t.Fatalf("Wrong seed value : \n  got  %x\n  want %x", ir.SeedValue, originalIR.SeedValue)
	}

	// Mock up inspector transaction
	itx, err := tests.CreateInspector(ctx, cfg, tx, nil)
	if err != nil {
		t.Fatalf("Failed to create transaction : %s", err)
	}

	if err := receiveRS.ProcessInitiateRelationship(ctx, itx, message, ir, encryptionKey); err != nil {
		t.Fatalf("Failed to process initiate : %s", err)
	}

	if len(receiveRS.Relationships) != 1 {
		t.Fatalf("Wrong receive relationship count : got %d, want %d", len(receiveRS.Relationships), 1)
	}

	originalAR, err := receiveRS.AcceptRelationship(ctx, receiveRS.Relationships[0])
	if err != nil {
		t.Fatalf("Failed to create accept relationship : %s", err)
	}

	tx = receiveBroadcastTx.Msgs[len(receiveBroadcastTx.Msgs)-1]

	t.Logf("Accept Tx : \n%s\n", tx.StringWithAddresses(cfg.Net))

	pk, err := bitcoin.PublicKeyFromUnlockingScript(tx.TxIn[0].SignatureScript)
	if err != nil {
		t.Fatalf("Failed to get sender public key : %s", err)
	}

	publicKey, err := bitcoin.PublicKeyFromBytes(pk)
	if err != nil {
		t.Fatalf("Failed to invalid sender public key : %s", err)
	}

	encryptionKey, err = sendRS.Relationships[0].FindEncryptionKey(publicKey)
	if err != nil {
		t.Fatalf("Failed to get encryption key : %s", err)
	}

	messageIndex = 0xffffffff
	for index, output := range tx.TxOut {
		var action actions.Action
		action, err = sendWallet.DecryptActionIndirect(ctx, output.PkScript, encryptionKey)
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

	if message.MessageCode != messages.CodeAcceptRelationship {
		t.Fatalf("Wrong message code : got %d, want %d", message.MessageCode,
			messages.CodeAcceptRelationship)
	}

	p, err = messages.Deserialize(message.MessageCode, message.MessagePayload)
	if err != nil {
		t.Fatalf("Failed to deserialize message payload : %s", err)
	}

	ar, ok := p.(*messages.AcceptRelationship)
	if !ok {
		t.Fatalf("Wrong message type")
	}

	if originalAR.ProofOfIdentityType != ar.ProofOfIdentityType {
		t.Fatalf("Wrong POI type : got  %d want %d", ar.ProofOfIdentityType, originalAR.ProofOfIdentityType)
	}

	if !bytes.Equal(originalAR.ProofOfIdentity, ar.ProofOfIdentity) {
		t.Fatalf("Wrong POI : \n  got  %x\n  want %x", ar.ProofOfIdentity, originalAR.ProofOfIdentity)
	}

	// Mock up inspector transaction
	itx, err = tests.CreateInspector(ctx, cfg, tx, nil)
	if err != nil {
		t.Fatalf("Failed to create transaction : %s", err)
	}
}
