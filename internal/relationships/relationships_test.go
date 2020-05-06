package relationships

import (
	"bytes"
	"testing"

	"github.com/tokenized/envelope/pkg/golang/envelope"
	"github.com/tokenized/relationship-example/internal/platform/tests"
	"github.com/tokenized/relationship-example/internal/wallet"

	"github.com/tokenized/specification/dist/golang/actions"
	"github.com/tokenized/specification/dist/golang/messages"
	"github.com/tokenized/specification/dist/golang/protocol"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/logger"
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

	poi := &messages.IdentityOracleProofField{}

	_, originalIR, err := rs.InitiateRelationship(ctx, []bitcoin.PublicKey{receiver.PublicKey()}, poi)
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
	for index, output := range tx.TxOut {
		env, err := envelope.Deserialize(bytes.NewReader(output.PkScript))
		if err != nil {
			continue
		}

		if !bytes.Equal(env.PayloadProtocol(), protocol.GetProtocolID(cfg.IsTest)) {
			continue
		}

		action, _, err := wallet.DecryptActionDirect(ctx, tx, index, env)
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

	if !bytes.Equal(originalIR.Seed, ir.Seed) {
		t.Fatalf("Wrong seed value : \n  got  %x\n  want %x", ir.Seed, originalIR.Seed)
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

	poi := &messages.IdentityOracleProofField{}

	_, originalIR, err := rs.InitiateRelationship(ctx, []bitcoin.PublicKey{
		receiver1.PublicKey(),
		receiver2.PublicKey(),
	}, poi)
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
	for index, output := range tx.TxOut {
		env, err := envelope.Deserialize(bytes.NewReader(output.PkScript))
		if err != nil {
			continue
		}

		if !bytes.Equal(env.PayloadProtocol(), protocol.GetProtocolID(cfg.IsTest)) {
			continue
		}

		action, _, err := wallet.DecryptActionDirect(ctx, tx, index, env)
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

	if !bytes.Equal(originalIR.Seed, ir.Seed) {
		t.Fatalf("Wrong seed value : \n  got  %x\n  want %x", ir.Seed, originalIR.Seed)
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

	poi := &messages.IdentityOracleProofField{}

	_, originalIR, err := sendRS.InitiateRelationship(ctx, []bitcoin.PublicKey{receiver.PublicKey}, poi)
	if err != nil {
		t.Fatalf("Failed to create initiate relationship : %s", err)
	}

	t.Logf("Seed : %x", originalIR.Seed)

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
	for index, output := range tx.TxOut {
		env, err := envelope.Deserialize(bytes.NewReader(output.PkScript))
		if err != nil {
			continue
		}

		if !bytes.Equal(env.PayloadProtocol(), protocol.GetProtocolID(cfg.IsTest)) {
			continue
		}

		var action actions.Action
		action, encryptionKey, err = receiveWallet.DecryptActionDirect(ctx, tx, index, env)
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

	if !bytes.Equal(originalIR.Seed, ir.Seed) {
		t.Fatalf("Wrong seed value : \n  got  %x\n  want %x", ir.Seed, originalIR.Seed)
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

	poi = &messages.IdentityOracleProofField{}

	originalAR, err := receiveRS.AcceptRelationship(ctx, receiveRS.Relationships[0], poi)
	if err != nil {
		t.Fatalf("Failed to create accept relationship : %s", err)
	}

	tx = receiveBroadcastTx.Msgs[len(receiveBroadcastTx.Msgs)-1]

	t.Logf("Accept Tx : \n%s\n", tx.StringWithAddresses(cfg.Net))

	messageIndex = 0xffffffff
	for index, output := range tx.TxOut {
		env, err := envelope.Deserialize(bytes.NewReader(output.PkScript))
		if err != nil {
			continue
		}

		if !bytes.Equal(env.PayloadProtocol(), protocol.GetProtocolID(cfg.IsTest)) {
			continue
		}

		var action actions.Action
		action, encryptionKey, err = receiveWallet.DecryptActionDirect(ctx, tx, index, env)
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

	poi := &messages.IdentityOracleProofField{}

	_, originalIR, err := sendRS.InitiateRelationship(ctx, []bitcoin.PublicKey{
		receiver1.PublicKey,
		receiver2.PublicKey,
	}, poi)
	if err != nil {
		t.Fatalf("Failed to create initiate relationship : %s", err)
	}

	t.Logf("Seed : %x", originalIR.Seed)
	t.Logf("Flag : %x", originalIR.Flag)

	if len(originalIR.Flag) == 0 {
		t.Fatalf("No flag specified in initiate relationship")
	}

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
	for index, output := range tx.TxOut {
		env, err := envelope.Deserialize(bytes.NewReader(output.PkScript))
		if err != nil {
			continue
		}

		if !bytes.Equal(env.PayloadProtocol(), protocol.GetProtocolID(cfg.IsTest)) {
			continue
		}

		var action actions.Action
		action, encryptionKey, err = receiveWallet.DecryptActionDirect(ctx, tx, index, env)
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

	if !bytes.Equal(originalIR.Seed, ir.Seed) {
		t.Fatalf("Wrong seed value : \n  got  %x\n  want %x", ir.Seed, originalIR.Seed)
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

	poi = &messages.IdentityOracleProofField{}

	originalAR, err := receiveRS.AcceptRelationship(ctx, receiveRS.Relationships[0], poi)
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

	encryptionKey, err = sendRS.FindEncryptionKey(ctx, sendRS.Relationships[0], publicKey)
	if err != nil {
		t.Fatalf("Failed to get encryption key : %s", err)
	}

	messageIndex = 0xffffffff
	var flag []byte
	for index, output := range tx.TxOut {
		f, err := protocol.DeserializeFlagOutputScript(output.PkScript)
		if err == nil {
			flag = f
			continue
		}

		env, err := envelope.Deserialize(bytes.NewReader(output.PkScript))
		if err != nil {
			continue
		}

		if !bytes.Equal(env.PayloadProtocol(), protocol.GetProtocolID(cfg.IsTest)) {
			continue
		}

		var action actions.Action
		action, err = sendWallet.DecryptActionIndirect(ctx, env, encryptionKey)
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

	if len(flag) == 0 {
		t.Fatalf("No flag found in tx")
	}

	if !bytes.Equal(flag, originalIR.Flag) {
		t.Fatalf("Wrong flag value : \n  got %x, \n  want %x", flag, originalIR.Flag)
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

func TestMessageDirect(t *testing.T) {
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

	createRelationship(t, ctx, cfg, sendWallet, sendRS, sendBroadcastTx, receiveWallet, receiveRS,
		receiveBroadcastTx, nil)

	logger.Info(ctx, "Sending private message ****************************************************")

	sendPrivateMessage := &messages.PrivateMessage{
		Subject: "Sample encrypted message",
	}

	err = sendRS.SendMessage(ctx, sendRS.Relationships[0], sendPrivateMessage)
	if err != nil {
		t.Fatalf("Failed to send message : %s", err)
	}

	_, message, _, _ := decryptMessage(t, ctx, cfg, receiveRS, sendBroadcastTx)

	if message.MessageCode != messages.CodePrivateMessage {
		t.Fatalf("Not a private message : %d", message.MessageCode)
	}

	p, err := messages.Deserialize(message.MessageCode, message.MessagePayload)
	if err != nil {
		t.Fatalf("Failed to deserialize message payload : %s", err)
	}

	privateMessage, ok := p.(*messages.PrivateMessage)
	if !ok {
		t.Fatalf("Failed to convert accept : %s", err)
	}

	if privateMessage.Subject != "Sample encrypted message" {
		t.Fatalf("Wrong private message subject : got \"%s\", want \"%s\"", privateMessage.Subject,
			"Sample encrypted message")
	}
}

func TestMessageIndirect(t *testing.T) {
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

	otherKey, err := bitcoin.GenerateKey(bitcoin.MainNet)
	if err != nil {
		t.Fatalf("Failed to generate other key : %s", err)
	}

	otherPublicKey := otherKey.PublicKey()

	createRelationship(t, ctx, cfg, sendWallet, sendRS, sendBroadcastTx, receiveWallet, receiveRS,
		receiveBroadcastTx, &otherPublicKey)

	logger.Info(ctx, "Sending private message ****************************************************")

	sendPrivateMessage := &messages.PrivateMessage{
		Subject: "Sample encrypted message",
	}

	err = sendRS.SendMessage(ctx, sendRS.Relationships[0], sendPrivateMessage)
	if err != nil {
		t.Fatalf("Failed to send message : %s", err)
	}

	_, message, _, _ := decryptMessage(t, ctx, cfg, receiveRS, sendBroadcastTx)

	if message.MessageCode != messages.CodePrivateMessage {
		t.Fatalf("Not a private message : %d", message.MessageCode)
	}

	p, err := messages.Deserialize(message.MessageCode, message.MessagePayload)
	if err != nil {
		t.Fatalf("Failed to deserialize message payload : %s", err)
	}

	privateMessage, ok := p.(*messages.PrivateMessage)
	if !ok {
		t.Fatalf("Failed to convert accept : %s", err)
	}

	if privateMessage.Subject != "Sample encrypted message" {
		t.Fatalf("Wrong private message subject : got \"%s\", want \"%s\"", privateMessage.Subject,
			"Sample encrypted message")
	}
}
