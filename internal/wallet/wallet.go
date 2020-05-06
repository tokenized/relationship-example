package wallet

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/tokenized/relationship-example/internal/platform/config"
	"github.com/tokenized/relationship-example/internal/platform/db"

	"github.com/tokenized/smart-contract/pkg/bitcoin"
	"github.com/tokenized/smart-contract/pkg/logger"

	"github.com/pkg/errors"
)

const (
	walletKey = "wallet"
)

var (
	ErrNotFound = errors.New("Not found")
)

type Wallet struct {
	cfg *config.Config

	baseKey   bitcoin.ExtendedKey
	walletKey bitcoin.ExtendedKey

	// Hashes for tx filtering
	hashes   map[bitcoin.Hash20]bitcoin.RawAddress
	hashLock sync.Mutex

	// UTXOs
	isModified bool
	utxos      map[bitcoin.Hash32][]*UTXO
	utxoLock   sync.Mutex

	// Addresses
	addressesMap  map[bitcoin.Hash20]*Address
	addressesList [][]*Address
	addressLock   sync.Mutex

	// Transactions
	txs    map[bitcoin.Hash32]*Transaction
	txLock sync.Mutex
}

func NewWallet(cfg *config.Config, keyText string) (*Wallet, error) {
	result := &Wallet{
		cfg:           cfg,
		hashes:        make(map[bitcoin.Hash20]bitcoin.RawAddress),
		utxos:         make(map[bitcoin.Hash32][]*UTXO),
		addressesMap:  make(map[bitcoin.Hash20]*Address),
		addressesList: make([][]*Address, KeyTypeCount, KeyTypeCount),
	}

	var err error
	result.baseKey, err = bitcoin.ExtendedKeyFromStr(keyText)
	if err != nil {
		return nil, errors.Wrap(err, "parse key")
	}

	return result, nil
}

func (w *Wallet) Load(ctx context.Context, dbConn *db.DB) error {
	b, err := dbConn.Fetch(ctx, walletKey)
	if err == nil {
		if err := w.Deserialize(bytes.NewReader(b)); err != nil {
			return errors.Wrap(err, "deserialize wallet")
		}
	} else if err != db.ErrNotFound {
		return errors.Wrap(err, "fetch wallet")
	}

	return w.Prepare(ctx)
}

func (w *Wallet) Prepare(ctx context.Context) error {
	w.hashLock.Lock()

	path, err := bitcoin.PathFromString(w.cfg.WalletPath)
	if err != nil {
		w.hashLock.Unlock()
		return errors.Wrap(err, "wallet path")
	}
	w.walletKey, err = w.baseKey.ChildKeyForPath(path)
	if err != nil {
		w.hashLock.Unlock()
		return errors.Wrap(err, "wallet parent")
	}

	w.hashLock.Unlock()

	w.addressLock.Lock()

	// Build initial address gap
	for t := uint32(0); t < KeyTypeCount; t++ {
		if err := w.forwardScan(ctx, t); err != nil {
			w.addressLock.Unlock()
			return errors.Wrap(err, "forward scan")
		}
	}

	w.addressLock.Unlock()

	w.utxoLock.Lock()

	balance := uint64(0)
	for _, utxos := range w.utxos {
		for _, utxo := range utxos {
			balance += utxo.UTXO.Value
		}
	}

	w.utxoLock.Unlock()

	logger.Info(ctx, "Bitcoin balance : %0.8f", float64(balance)/100000000.0)
	return nil
}

func (w *Wallet) Save(ctx context.Context, dbConn *db.DB) error {
	var buf bytes.Buffer
	if err := w.Serialize(&buf); err != nil {
		return errors.Wrap(err, "serialize wallet")
	}

	if err := dbConn.Put(ctx, walletKey, buf.Bytes()); err != nil {
		return errors.Wrap(err, "put wallet")
	}

	return nil
}

func (w *Wallet) AddMonitoredAddress(ctx context.Context, ra bitcoin.RawAddress) error {
	hashes, err := ra.Hashes()
	if err != nil {
		return errors.Wrap(err, "new address hashes")
	}

	w.hashLock.Lock()
	for _, hash := range hashes {
		w.hashes[hash] = ra
	}
	w.hashLock.Unlock()
	return nil
}

func (w *Wallet) addMonitoredAddress(ctx context.Context, ra bitcoin.RawAddress) error {
	hashes, err := ra.Hashes()
	if err != nil {
		return errors.Wrap(err, "new address hashes")
	}

	for _, hash := range hashes {
		w.hashes[hash] = ra
	}
	return nil
}

func (w *Wallet) AddMonitoredHash(ctx context.Context, hash bitcoin.Hash20) error {
	w.hashLock.Lock()
	w.hashes[hash] = bitcoin.RawAddress{}
	w.hashLock.Unlock()
	return nil
}

func (w *Wallet) addMonitoredHash(ctx context.Context, hash bitcoin.Hash20) error {
	w.hashes[hash] = bitcoin.RawAddress{}
	return nil
}

func (w *Wallet) AreHashesMonitored(hashes []bitcoin.Hash20) (bool, bitcoin.RawAddress) {
	w.hashLock.Lock()
	defer w.hashLock.Unlock()
	for _, hash := range hashes {
		ra, exists := w.hashes[hash]
		if exists {
			return true, ra
		}
	}
	return false, bitcoin.RawAddress{}
}

func (w Wallet) Serialize(buf *bytes.Buffer) error {
	// Version
	if err := binary.Write(buf, binary.LittleEndian, uint8(0)); err != nil {
		return errors.Wrap(err, "version")
	}

	w.hashLock.Lock()
	defer w.hashLock.Unlock()

	if err := binary.Write(buf, binary.LittleEndian, uint64(len(w.hashes))); err != nil {
		return errors.Wrap(err, "hashes size")
	}
	for hash, ra := range w.hashes {
		if _, err := buf.Write(hash[:]); err != nil {
			return errors.Wrap(err, "write hash")
		}

		if err := ra.Serialize(buf); err != nil {
			return errors.Wrap(err, "write raw address")
		}
	}

	w.addressLock.Lock()
	defer w.addressLock.Unlock()

	// Write standard addresses
	for t := 0; t < KeyTypeCount; t++ {
		if err := binary.Write(buf, binary.LittleEndian, uint64(len(w.addressesList[t]))); err != nil {
			return errors.Wrap(err, "addresses size")
		}
		for _, address := range w.addressesList[t] {
			if err := address.Serialize(buf); err != nil {
				return errors.Wrap(err, "write address")
			}
		}
	}

	// Write hash derived addresses since they aren't put into the address lists.
	hashAddresses := make([]*Address, 0)
	for _, address := range w.addressesMap {
		if address.KeyHash != nil {
			hashAddresses = append(hashAddresses, address)
		}
	}

	if err := binary.Write(buf, binary.LittleEndian, uint64(len(hashAddresses))); err != nil {
		return errors.Wrap(err, "hash addresses size")
	}
	for _, address := range hashAddresses {
		if err := address.Serialize(buf); err != nil {
			return errors.Wrap(err, "write address")
		}
	}

	w.utxoLock.Lock()
	defer w.utxoLock.Unlock()

	if err := binary.Write(buf, binary.LittleEndian, uint64(len(w.utxos))); err != nil {
		return errors.Wrap(err, "utxos size")
	}
	for _, utxos := range w.utxos {
		if err := binary.Write(buf, binary.LittleEndian, uint32(len(utxos))); err != nil {
			return errors.Wrap(err, "utxos sub size")
		}

		for _, utxo := range utxos {
			if err := utxo.Serialize(buf); err != nil {
				return errors.Wrap(err, "write utxo")
			}
		}
	}

	w.txLock.Lock()
	defer w.txLock.Unlock()

	if err := binary.Write(buf, binary.LittleEndian, uint64(len(w.txs))); err != nil {
		return errors.Wrap(err, "txs size")
	}
	for _, tx := range w.txs {
		if err := tx.Serialize(buf); err != nil {
			return errors.Wrap(err, "serialize tx")
		}
	}

	return nil
}

func (w *Wallet) Deserialize(buf *bytes.Reader) error {
	var version uint8
	if err := binary.Read(buf, binary.LittleEndian, &version); err != nil {
		return errors.Wrap(err, "version")
	}

	if version != 0 {
		return fmt.Errorf("Unsupported version : %d", version)
	}

	w.hashLock.Lock()
	defer w.hashLock.Unlock()

	var count uint64
	if err := binary.Read(buf, binary.LittleEndian, &count); err != nil {
		return errors.Wrap(err, "hashes size")
	}
	w.hashes = make(map[bitcoin.Hash20]bitcoin.RawAddress)
	for i := uint64(0); i < count; i++ {
		var hash bitcoin.Hash20
		if _, err := buf.Read(hash[:]); err != nil {
			return errors.Wrap(err, "read hash")
		}

		var ra bitcoin.RawAddress
		if err := ra.Deserialize(buf); err != nil {
			return errors.Wrap(err, "read raw address")
		}

		w.hashes[hash] = ra
	}

	w.addressLock.Lock()
	defer w.addressLock.Unlock()

	// Read standard addresses
	w.addressesList = make([][]*Address, KeyTypeCount)
	w.addressesMap = make(map[bitcoin.Hash20]*Address)
	for t := 0; t < KeyTypeCount; t++ {
		if err := binary.Read(buf, binary.LittleEndian, &count); err != nil {
			return errors.Wrap(err, "addresses size")
		}
		w.addressesList[t] = make([]*Address, 0, count)
		for i := uint64(0); i < count; i++ {
			var address Address
			if err := address.Deserialize(buf); err != nil {
				return errors.Wrap(err, "read address")
			}

			w.addressesList[t] = append(w.addressesList[t], &address)

			hashes, err := address.Address.Hashes()
			if err != nil {
				return errors.Wrap(err, "raw address hashes")
			}
			for _, hash := range hashes {
				w.addressesMap[hash] = &address
			}
		}
	}

	// Read hash derived addresses
	if err := binary.Read(buf, binary.LittleEndian, &count); err != nil {
		return errors.Wrap(err, "hash addresses size")
	}
	for i := uint64(0); i < count; i++ {
		var address Address
		if err := address.Deserialize(buf); err != nil {
			return errors.Wrap(err, "read address")
		}

		hashes, err := address.Address.Hashes()
		if err != nil {
			return errors.Wrap(err, "raw address hashes")
		}
		for _, hash := range hashes {
			w.addressesMap[hash] = &address
		}
	}

	w.utxoLock.Lock()
	defer w.utxoLock.Unlock()

	if err := binary.Read(buf, binary.LittleEndian, &count); err != nil {
		return errors.Wrap(err, "utxos size")
	}
	w.utxos = make(map[bitcoin.Hash32][]*UTXO)
	for i := uint64(0); i < count; i++ {
		var subCount uint32
		if err := binary.Read(buf, binary.LittleEndian, &subCount); err != nil {
			return errors.Wrap(err, "utxos sub size")
		}

		if subCount == 0 {
			continue
		}

		utxos := make([]*UTXO, 0, subCount)
		for i := uint32(0); i < subCount; i++ {
			utxo := &UTXO{}
			if err := utxo.Deserialize(buf); err != nil {
				return errors.Wrap(err, "read utxo")
			}

			utxos = append(utxos, utxo)
		}

		w.utxos[utxos[0].UTXO.Hash] = utxos
	}

	w.txLock.Lock()
	defer w.txLock.Unlock()

	if err := binary.Read(buf, binary.LittleEndian, &count); err != nil {
		return errors.Wrap(err, "txs size")
	}
	w.txs = make(map[bitcoin.Hash32]*Transaction)
	for i := uint64(0); i < count; i++ {
		var tx Transaction
		if err := tx.Deserialize(buf, w.cfg.IsTest); err != nil {
			return errors.Wrap(err, "deserialize tx")
		}

		w.txs[*tx.Itx.Hash] = &tx
	}

	return nil
}
