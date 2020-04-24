package wallet

import (
	"bytes"
	"context"
	"encoding/binary"
	"sync"

	"github.com/tokenized/relationship-test/internal/platform/config"

	"github.com/tokenized/smart-contract/pkg/bitcoin"

	"github.com/pkg/errors"
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
	utxos      map[bitcoin.Hash32][]UTXO
	utxoLock   sync.Mutex

	// Addresses
	addressesMap  map[bitcoin.Hash20]*Address
	addressesList [][]*Address
	addressLock   sync.Mutex
}

func NewWallet(cfg *config.Config, keyText string) (*Wallet, error) {
	result := &Wallet{
		cfg:           cfg,
		hashes:        make(map[bitcoin.Hash20]bitcoin.RawAddress),
		utxos:         make(map[bitcoin.Hash32][]UTXO),
		addressesMap:  make(map[bitcoin.Hash20]*Address),
		addressesList: make([][]*Address, 3, 3),
	}

	var err error
	result.baseKey, err = bitcoin.ExtendedKeyFromStr(keyText)
	if err != nil {
		return nil, errors.Wrap(err, "parse key")
	}

	return result, nil
}

func (w *Wallet) Load(ctx context.Context) error {
	w.hashLock.Lock()
	defer w.hashLock.Unlock()

	path, err := bitcoin.PathFromString(w.cfg.WalletPath)
	if err != nil {
		return errors.Wrap(err, "wallet path")
	}
	w.walletKey, err = w.baseKey.ChildKeyForPath(path)
	if err != nil {
		return errors.Wrap(err, "wallet parent")
	}

	// Build initial address gap
	for t := uint32(0); t < 3; t++ {
		if err := w.ForwardScan(ctx, t); err != nil {
			return errors.Wrap(err, "forward scan")
		}
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

func (w *Wallet) Serialize(buf *bytes.Buffer) error {
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

	return nil
}

func (w *Wallet) Deserialize(buf *bytes.Reader) error {
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
			return errors.Wrap(err, "read address")
		}

		w.hashes[hash] = ra
	}

	w.utxoLock.Lock()
	defer w.utxoLock.Unlock()

	if err := binary.Read(buf, binary.LittleEndian, &count); err != nil {
		return errors.Wrap(err, "utxos size")
	}
	w.utxos = make(map[bitcoin.Hash32][]UTXO)
	for i := uint64(0); i < count; i++ {
		var subCount uint32
		if err := binary.Read(buf, binary.LittleEndian, &subCount); err != nil {
			return errors.Wrap(err, "utxos sub size")
		}

		if subCount == 0 {
			continue
		}

		utxos := make([]UTXO, subCount, 0)
		for i := uint32(0); i < subCount; i++ {
			var utxo UTXO
			if err := utxo.Deserialize(buf); err != nil {
				return errors.Wrap(err, "read utxo")
			}

			utxos = append(utxos, utxo)
		}

		w.utxos[utxos[0].UTXO.Hash] = utxos
	}

	return nil
}
