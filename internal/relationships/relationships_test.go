package relationships

import (
	"bytes"
	"context"
	"testing"

	"github.com/tokenized/relationship-example/internal/platform/config"
	"github.com/tokenized/relationship-example/internal/platform/tests"
	"github.com/tokenized/relationship-example/internal/wallet"

	"github.com/tokenized/specification/dist/golang/actions"
	"github.com/tokenized/specification/dist/golang/messages"
	"github.com/tokenized/specification/dist/golang/protocol"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/inspector"
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

	originalIR, err := rs.InitiateRelationship(ctx, []bitcoin.PublicKey{receiver.PublicKey()}, poi)
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

	originalIR, err := rs.InitiateRelationship(ctx, []bitcoin.PublicKey{
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

	originalIR, err := sendRS.InitiateRelationship(ctx, []bitcoin.PublicKey{receiver.PublicKey}, poi)
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

	poi := &messages.IdentityOracleProofField{}

	originalIR, err := sendRS.InitiateRelationship(ctx, []bitcoin.PublicKey{
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

	encryptionKey, err = sendRS.Relationships[0].FindEncryptionKey(publicKey)
	if err != nil {
		t.Fatalf("Failed to get encryption key : %s", err)
	}

	messageIndex = 0xffffffff
	var flag []byte
	for index, output := range tx.TxOut {
		f, err := protocol.DeserializeFlagOutputScript(output.PkScript)
		if err == nil {
			flag = f
		}

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

func decryptMessage(t *testing.T, ctx context.Context, cfg *config.Config, rs *Relationships,
	broadcastTx *tests.MockBroadcaster) (*inspector.Transaction, *actions.Message, bitcoin.Hash32, []byte) {

	if len(broadcastTx.Msgs) == 0 {
		t.Fatalf("No txs broadcast")
	}

	tx := broadcastTx.Msgs[len(broadcastTx.Msgs)-1]
	broadcastTx.Msgs = nil

	itx, err := tests.CreateInspector(ctx, cfg, tx, nil)
	if err != nil {
		t.Fatalf("Failed to create transaction : %s", err)
	}

	var flag []byte
	for _, output := range tx.TxOut {
		f, err := protocol.DeserializeFlagOutputScript(output.PkScript)
		if err == nil {
			flag = f
			break
		}
	}

	for index, _ := range tx.TxOut {
		action, encryptionKey, err := rs.DecryptAction(ctx, itx, index, flag)
		if err != nil {
			continue
		}

		message, ok := action.(*actions.Message)
		if !ok {
			continue
		}

		return itx, message, encryptionKey, flag
	}

	t.Fatalf("Didn't find initiate message")
	return nil, nil, bitcoin.Hash32{}, nil
}

func createRelationship(t *testing.T, ctx context.Context, cfg *config.Config,
	sendWallet *wallet.Wallet, sendRS *Relationships, sendBroadcastTx *tests.MockBroadcaster,
	receiveWallet *wallet.Wallet, receiveRS *Relationships, receiveBroadcastTx *tests.MockBroadcaster) {

	// Initiate Relationship ***********************************************************************
	receiveAddress, err := receiveWallet.GetUnusedAddress(ctx, wallet.KeyTypeRelateIn)
	if err != nil {
		t.Fatalf("Failed to get relationships address : %s", err)
	}

	logger.Info(ctx, "Send initiate **************************************************************")

	poi := &messages.IdentityOracleProofField{}

	_, err = sendRS.InitiateRelationship(ctx, []bitcoin.PublicKey{receiveAddress.PublicKey}, poi)
	if err != nil {
		t.Fatalf("Failed to initiate relationship : %s", err)
	}

	itx, message, encryptionKey, flag := decryptMessage(t, ctx, cfg, receiveRS, sendBroadcastTx)

	if message.MessageCode != messages.CodeInitiateRelationship {
		t.Fatalf("Not an initiate message : %d", message.MessageCode)
	}

	p, err := messages.Deserialize(message.MessageCode, message.MessagePayload)
	if err != nil {
		t.Fatalf("Failed to deserialize message payload : %s", err)
	}

	initiate, ok := p.(*messages.InitiateRelationship)
	if !ok {
		t.Fatalf("Failed to convert initiate : %s", err)
	}

	logger.Info(ctx, "Process initiate ***********************************************************")
	err = receiveRS.ProcessInitiateRelationship(ctx, itx, message, initiate, encryptionKey)
	if err != nil {
		t.Fatalf("Failed to process initiate : %s", err)
	}

	// Accept Relationship *************************************************************************
	logger.Info(ctx, "Send accept ****************************************************************")

	poi = &messages.IdentityOracleProofField{}

	_, err = receiveRS.AcceptRelationship(ctx, receiveRS.Relationships[0], poi)
	if err != nil {
		t.Fatalf("Failed to accept relationship : %s", err)
	}

	itx, message, encryptionKey, flag = decryptMessage(t, ctx, cfg, sendRS, receiveBroadcastTx)

	if message.MessageCode != messages.CodeAcceptRelationship {
		t.Fatalf("Not an accept message : %d", message.MessageCode)
	}

	p, err = messages.Deserialize(message.MessageCode, message.MessagePayload)
	if err != nil {
		t.Fatalf("Failed to deserialize message payload : %s", err)
	}

	accept, ok := p.(*messages.AcceptRelationship)
	if !ok {
		t.Fatalf("Failed to convert accept : %s", err)
	}

	logger.Info(ctx, "Process accept *************************************************************")
	err = sendRS.ProcessAcceptRelationship(ctx, itx, message, accept, flag)
	if err != nil {
		t.Fatalf("Failed to process accept : %s", err)
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
		receiveBroadcastTx)

	logger.Info(ctx, "Sending private message ****************************************************")

	sra, err := sendRS.Relationships[0].NextKey.RawAddress()
	if err != nil {
		t.Fatalf("Failed to generate sender next address : %s", err)
	}
	logger.Info(ctx, "Sender next address : %s", bitcoin.NewAddressFromRawAddress(sra, cfg.Net).String())

	rra, err := receiveRS.Relationships[0].NextKey.RawAddress()
	if err != nil {
		t.Fatalf("Failed to generate sender next address : %s", err)
	}
	logger.Info(ctx, "Receiver next address : %s", bitcoin.NewAddressFromRawAddress(rra, cfg.Net).String())

	sendPrivateMessage := messages.PrivateMessage{
		Subject: "Sample encrypted message",
	}

	var pmBuf bytes.Buffer
	if err := sendPrivateMessage.Serialize(&pmBuf); err != nil {
		t.Fatalf("Failed to serialize private message : %s", err)
	}

	sendMessage := &actions.Message{
		MessageCode:    messages.CodePrivateMessage,
		MessagePayload: pmBuf.Bytes(),
	}

	err = sendRS.SendMessage(ctx, sendRS.Relationships[0], sendMessage)
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
