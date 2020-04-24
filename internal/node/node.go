package node

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tokenized/relationship-test/internal/platform/config"
	"github.com/tokenized/relationship-test/internal/wallet"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/inspector"
	"github.com/tokenized/smart-contract/pkg/logger"
	"github.com/tokenized/smart-contract/pkg/rpcnode"
	"github.com/tokenized/smart-contract/pkg/spynode"
	"github.com/tokenized/smart-contract/pkg/wire"

	"github.com/pkg/errors"
)

type Node struct {
	cfg         *config.Config
	wallet      *wallet.Wallet
	rpc         *rpcnode.RPCNode
	spy         *spynode.Node
	txs         map[bitcoin.Hash32]*wire.MsgTx
	lock        sync.Mutex
	processLock sync.Mutex
	stop        atomic.Value
	isInSync    atomic.Value
}

func NewNode(cfg *config.Config, wallet *wallet.Wallet, rpc *rpcnode.RPCNode, spy *spynode.Node) (*Node, error) {
	result := &Node{
		cfg:    cfg,
		wallet: wallet,
		rpc:    rpc,
		spy:    spy,
		txs:    make(map[bitcoin.Hash32]*wire.MsgTx),
	}

	result.stop.Store(false)

	return result, nil
}

func (n *Node) Run(ctx context.Context) error {
	for {
		time.Sleep(100 * time.Millisecond)
		val := n.stop.Load()
		s, ok := val.(bool)
		if !ok || s {
			break
		}
	}
	return nil
}

func (n *Node) Stop(ctx context.Context) error {
	n.stop.Store(true)
	return nil
}

func (n *Node) IsInSync() bool {
	val := n.isInSync.Load()
	result, ok := val.(bool)
	return ok && result
}

func (n *Node) GetTx(txid bitcoin.Hash32) *wire.MsgTx {
	n.lock.Lock()
	tx, ok := n.txs[txid]
	n.lock.Unlock()

	if !ok {
		tx = nil
	}

	return tx
}

func (n *Node) SetTx(tx *wire.MsgTx) {
	n.lock.Lock()
	n.txs[*tx.TxHash()] = tx
	n.lock.Unlock()
}

func (n *Node) RemoveTx(txid bitcoin.Hash32) {
	n.lock.Lock()
	delete(n.txs, txid)
	n.lock.Unlock()
}

func (n *Node) PreprocessTx(ctx context.Context, tx *wire.MsgTx) error {
	n.processLock.Lock()
	defer n.processLock.Unlock()

	logger.Info(ctx, "Pre-processing tx : %s", tx.TxHash().String())

	return nil
}

func (n *Node) ProcessTx(ctx context.Context, tx *wire.MsgTx) error {
	n.processLock.Lock()
	defer n.processLock.Unlock()

	logger.Info(ctx, "Processing tx : %s", tx.TxHash().String())

	if err := n.wallet.ProcessUTXOs(ctx, tx, true); err != nil {
		return errors.Wrap(err, "process utxos")
	}

	itx, err := inspector.NewBaseTransactionFromWire(ctx, tx)
	if err != nil {
		return errors.Wrap(err, "new inspector tx")
	}

	if err := itx.Setup(ctx, n.cfg.IsTest); err != nil {
		return errors.Wrap(err, "setup inspector tx")
	}

	if err := itx.Validate(ctx); err != nil {
		return errors.Wrap(err, "validate inspector tx")
	}

	if err := itx.Promote(ctx, n.rpc); err != nil {
		return errors.Wrap(err, "promote inspector tx")
	}

	return nil
}

func (n *Node) BroadcastTx(ctx context.Context, tx *wire.MsgTx) error {
	if err := n.spy.BroadcastTx(ctx, tx); err != nil {
		return err
	}

	if err := n.spy.HandleTx(ctx, tx); err != nil {
		return err
	}

	return nil
}
