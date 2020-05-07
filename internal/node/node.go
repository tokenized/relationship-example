package node

import (
	"bytes"
	"context"
	"net"
	"sync"
	"sync/atomic"

	"github.com/tokenized/envelope/pkg/golang/envelope"

	"github.com/tokenized/relationship-example/internal/platform/config"
	"github.com/tokenized/relationship-example/internal/platform/db"
	"github.com/tokenized/relationship-example/internal/relationships"
	"github.com/tokenized/relationship-example/internal/wallet"

	"github.com/tokenized/specification/dist/golang/actions"
	"github.com/tokenized/specification/dist/golang/protocol"

	"github.com/tokenized/smart-contract/pkg/logger"
	"github.com/tokenized/smart-contract/pkg/rpcnode"
	"github.com/tokenized/smart-contract/pkg/spynode"
	"github.com/tokenized/smart-contract/pkg/wire"

	"github.com/pkg/errors"
)

type Node struct {
	cfg         *config.Config
	masterDB    *db.DB
	wallet      *wallet.Wallet
	rs          *relationships.Relationships
	rpc         *rpcnode.RPCNode
	spy         *spynode.Node
	lock        sync.Mutex
	processLock sync.Mutex
	stop        atomic.Value
	isInSync    atomic.Value

	blockHeight  int
	refeedNeeded atomic.Value

	netListener net.Listener
	netConns    []net.Conn
	netLock     sync.Mutex
}

func NewNode(cfg *config.Config, masterDB *db.DB, wallet *wallet.Wallet, rpc *rpcnode.RPCNode,
	spy *spynode.Node) (*Node, error) {
	result := &Node{
		cfg:      cfg,
		masterDB: masterDB,
		wallet:   wallet,
		rpc:      rpc,
		spy:      spy,
	}

	var err error
	result.rs, err = relationships.NewRelationships(cfg, wallet, result)
	if err != nil {
		return nil, err
	}

	result.stop.Store(false)
	result.refeedNeeded.Store(false)
	spy.RegisterListener(result)

	return result, nil
}

func (n *Node) Run(ctx context.Context) error {
	// for {
	// 	time.Sleep(100 * time.Millisecond)
	// 	val := n.stop.Load()
	// 	s, ok := val.(bool)
	// 	if !ok || s {
	// 		break
	// 	}
	// }

	// return nil

	commandErr := n.RunCommandServer(ctx)
	if commandErr != nil {
		logger.Error(ctx, "Command server returned in error : %s", commandErr)
	}

	saveErr := n.Save(ctx)
	if saveErr != nil {
		logger.Error(ctx, "Failed to save node : %s", saveErr)
	}

	if commandErr != nil {
		return commandErr
	}
	return saveErr
}

func (n *Node) Stop(ctx context.Context) error {
	n.stop.Store(true)

	n.netLock.Lock()
	n.netListener.Close()
	for _, conn := range n.netConns {
		conn.Close()
	}
	n.netLock.Unlock()

	return nil
}

func (n *Node) IsInSync() bool {
	val := n.isInSync.Load()
	result, ok := val.(bool)
	return ok && result
}

func (n *Node) PreprocessTx(ctx context.Context, tx *wire.MsgTx) error {
	n.processLock.Lock()
	defer n.processLock.Unlock()

	logger.Info(ctx, "Pre-processing tx : %s", tx.TxHash().String())
	logger.Info(ctx, "Full tx : \n%s\n", tx.StringWithAddresses(n.cfg.Net))

	if err := n.wallet.ProcessUTXOs(ctx, tx, true); err != nil {
		return errors.Wrap(err, "process utxos")
	}

	return nil
}

func (n *Node) ProcessTx(ctx context.Context, t *wallet.Transaction) error {
	if t.Itx.IsPromoted(ctx) {
		return nil // already processed this tx
	}

	logger.Info(ctx, "Processing tx : %s", t.Itx.Hash.String())

	if err := t.Itx.Promote(ctx, n.rpc); err != nil {
		return errors.Wrap(err, "promote inspector tx")
	}

	n.processLock.Lock()
	defer n.processLock.Unlock()

	if err := n.wallet.FinalizeUTXOs(ctx, t.Itx.MsgTx); err != nil {
		return errors.Wrap(err, "finalize utxos")
	}

	// Check for a flag value
	var flag []byte
	for _, output := range t.Itx.MsgTx.TxOut {
		f, err := protocol.DeserializeFlagOutputScript(output.PkScript)
		if err == nil {
			flag = f
			break
		}
	}

	// Process any tokenized actions
	for index, _ := range t.Itx.MsgTx.TxOut {
		action, encryptionKey, err := n.rs.DecryptAction(ctx, t.Itx, index, flag)
		if err != nil {
			if errors.Cause(err) != envelope.ErrNotEnvelope {
				logger.Info(ctx, "Failed to decrypt output : %s", err)
			}
			continue
		}

		switch message := action.(type) {
		case *actions.Message:
			refeed, err := n.ProcessMessage(ctx, t.Itx, index, encryptionKey, message, flag)
			if err != nil {
				return errors.Wrap(err, "process message")
			}
			if refeed && !n.IsInSync() {
				n.refeedNeeded.Store(true)
			}
		default:
			logger.Info(ctx, "%s actions not supported", action.Code())
		}
	}

	return nil
}

func (n *Node) RevertTx(ctx context.Context, t *wallet.Transaction) error {
	n.processLock.Lock()
	defer n.processLock.Unlock()

	logger.Info(ctx, "Reverting tx : %s", t.Itx.Hash.String())

	if err := n.wallet.RevertUTXOs(ctx, t.Itx.MsgTx, true); err != nil {
		return errors.Wrap(err, "revert utxos")
	}

	return nil
}

func (n *Node) BroadcastTx(ctx context.Context, tx *wire.MsgTx) error {
	logger.Info(ctx, "Broadcasting Tx : \n%s\n", tx.StringWithAddresses(n.cfg.Net))

	var txBuf bytes.Buffer
	if err := tx.Serialize(&txBuf); err == nil {
		logger.Info(ctx, "Raw Tx : \n%x\n", txBuf.Bytes())
	}

	if err := n.spy.BroadcastTx(ctx, tx); err != nil {
		return errors.Wrap(err, "broadcast tx")
	}

	if err := n.spy.HandleTx(ctx, tx); err != nil {
		return errors.Wrap(err, "handle tx")
	}

	if err := n.rpc.SaveTX(ctx, tx); err != nil {
		return errors.Wrap(err, "save rpc tx")
	}

	return nil
}

func (n *Node) Load(ctx context.Context) error {
	if err := n.wallet.Load(ctx, n.masterDB); err != nil {
		return errors.Wrap(err, "load wallet")
	}
	if err := n.rs.Load(ctx, n.masterDB); err != nil {
		return errors.Wrap(err, "load relationships")
	}

	return nil
}

func (n *Node) Save(ctx context.Context) error {
	if err := n.wallet.Save(ctx, n.masterDB); err != nil {
		return errors.Wrap(err, "save wallet")
	}

	if err := n.rs.Save(ctx, n.masterDB); err != nil {
		return errors.Wrap(err, "save relationships")
	}

	return nil
}
