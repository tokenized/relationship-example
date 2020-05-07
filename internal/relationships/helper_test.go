package relationships

import (
	"context"
	"testing"

	"github.com/tokenized/relationship-example/internal/platform/config"
	"github.com/tokenized/relationship-example/internal/platform/tests"
	"github.com/tokenized/relationship-example/internal/wallet"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/inspector"
	"github.com/tokenized/smart-contract/pkg/logger"

	"github.com/tokenized/specification/dist/golang/actions"
	"github.com/tokenized/specification/dist/golang/messages"
	"github.com/tokenized/specification/dist/golang/protocol"
)

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
	receiveWallet *wallet.Wallet, receiveRS *Relationships, receiveBroadcastTx *tests.MockBroadcaster,
	otherReceiver *bitcoin.PublicKey) {

	// Initiate Relationship ***********************************************************************
	receiveAddress, err := receiveWallet.GetUnusedAddress(ctx, wallet.KeyTypeRelateIn)
	if err != nil {
		t.Fatalf("Failed to get relationships address : %s", err)
	}

	receivers := make([]bitcoin.PublicKey, 0)
	receivers = append(receivers, receiveAddress.PublicKey)

	if otherReceiver != nil {
		receivers = append(receivers, *otherReceiver)
	}

	logger.Info(ctx, "Send initiate **************************************************************")

	poi := &messages.IdentityOracleProofField{}

	_, _, err = sendRS.InitiateRelationship(ctx, receivers, poi)
	if err != nil {
		t.Fatalf("Failed to initiate relationship : %s", err)
	}

	if len(sendRS.Relationships) != 1 {
		t.Fatalf("Wrong send relationship count : %d", len(sendRS.Relationships))
	}

	if otherReceiver != nil {
		if sendRS.Relationships[0].EncryptionType != 1 {
			t.Fatalf("Wrong send relationship encryption type : got %d, want %d",
				sendRS.Relationships[0].EncryptionType, 1)
		}
	} else {
		if sendRS.Relationships[0].EncryptionType != 0 {
			t.Fatalf("Wrong send relationship encryption type : got %d, want %d",
				sendRS.Relationships[0].EncryptionType, 0)
		}
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

	if len(receiveRS.Relationships) != 1 {
		t.Fatalf("Wrong receive relationship count : %d", len(receiveRS.Relationships))
	}

	if otherReceiver != nil {
		if receiveRS.Relationships[0].EncryptionType != 1 {
			t.Fatalf("Wrong receive relationship encryption type : got %d, want %d",
				receiveRS.Relationships[0].EncryptionType, 1)
		}
	} else {
		if receiveRS.Relationships[0].EncryptionType != 0 {
			t.Fatalf("Wrong receive relationship encryption type : got %d, want %d",
				receiveRS.Relationships[0].EncryptionType, 0)
		}
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
	_, err = sendRS.ProcessAcceptRelationship(ctx, itx, message, accept, flag)
	if err != nil {
		t.Fatalf("Failed to process accept : %s", err)
	}
}
