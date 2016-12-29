package spvwallet

import (
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/peer"
	"github.com/btcsuite/btcd/txscript"
	btc "github.com/btcsuite/btcutil"
	hd "github.com/btcsuite/btcutil/hdkeychain"
	"github.com/op/go-logging"
	b39 "github.com/tyler-smith/go-bip39"
	"net"
	"os"
	"path"
	"sync"
)

type SPVWallet struct {
	params *chaincfg.Params

	masterPrivateKey *hd.ExtendedKey
	masterPublicKey  *hd.ExtendedKey

	maxFee      uint64
	priorityFee uint64
	normalFee   uint64
	economicFee uint64
	feeAPI      string

	repoPath string

	blockchain  *Blockchain
	txstore     *TxStore
	PeerManager *PeerManager

	fPositives    chan *peer.Peer
	stopChan      chan int
	fpAccumulator map[int32]int32
	blockQueue    chan chainhash.Hash
	toDownload    map[chainhash.Hash]int32
	mutex         *sync.RWMutex

	running bool

	config *Config
}

var log = logging.MustGetLogger("bitcoin")

const WALLET_VERSION = "0.1.0"

func NewSPVWallet(mnemonic string, params *chaincfg.Params, maxFee uint64, lowFee uint64, mediumFee uint64, highFee uint64, feeApi,
	repoPath string, db Datastore, userAgent string, trustedPeer string, logger logging.LeveledBackend) (*SPVWallet, error) {

	log.SetBackend(logger)

	seed := b39.NewSeed(mnemonic, "")

	mPrivKey, err := hd.NewMaster(seed, params)
	if err != nil {
		return nil, err
	}
	mPubKey, err := mPrivKey.Neuter()
	if err != nil {
		return nil, err
	}

	w := &SPVWallet{
		repoPath:         repoPath,
		masterPrivateKey: mPrivKey,
		masterPublicKey:  mPubKey,
		params:           params,
		maxFee:           maxFee,
		priorityFee:      highFee,
		normalFee:        mediumFee,
		economicFee:      lowFee,
		feeAPI:           feeApi,
		fPositives:       make(chan *peer.Peer),
		stopChan:         make(chan int),
		fpAccumulator:    make(map[int32]int32),
		blockQueue:       make(chan chainhash.Hash, 32),
		toDownload:       make(map[chainhash.Hash]int32),
		mutex:            new(sync.RWMutex),
	}

	w.txstore, err = NewTxStore(w.params, db, w.masterPrivateKey)
	if err != nil {
		return nil, err
	}
	w.blockchain, err = NewBlockchain(w.repoPath, w.params)
	if err != nil {
		return nil, err
	}

	listeners := &peer.MessageListeners{
		OnMerkleBlock: w.onMerkleBlock,
		OnInv:         w.onInv,
		OnTx:          w.onTx,
		OnGetData:     w.onGetData,
	}

	getNewestBlock := func() (*chainhash.Hash, int32, error) {
		storedHeader, err := w.blockchain.db.GetBestHeader()
		if err != nil {
			return nil, 0, err
		}
		height, err := w.blockchain.db.Height()
		if err != nil {
			return nil, 0, err
		}
		hash := storedHeader.header.BlockHash()
		return &hash, int32(height), nil
	}

	w.config = &Config{
		UserAgentName:      userAgent,
		UserAgentVersion:   WALLET_VERSION,
		Params:             w.params,
		AddressCacheDir:    repoPath,
		GetFilter:          w.txstore.GimmeFilter,
		StartChainDownload: w.startChainDownload,
		GetNewestBlock:     getNewestBlock,
		Listeners:          listeners,
	}

	if trustedPeer != "" {
		addr, err := net.ResolveTCPAddr("tcp", trustedPeer)
		if err != nil {
			return nil, err
		}
		w.config.TrustedPeer = addr
	}

	w.PeerManager, err = NewPeerManager(w.config)
	if err != nil {
		return nil, err
	}

	return w, nil
}

func (w *SPVWallet) Start() {
	go w.PeerManager.Start()
	go w.fPositiveHandler(w.stopChan)
	w.running = true
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////
//
// API
//
//////////////

func (w *SPVWallet) CurrencyCode() string {
	return "btc"
}

func (w *SPVWallet) MasterPrivateKey() *hd.ExtendedKey {
	return w.masterPrivateKey
}

func (w *SPVWallet) MasterPublicKey() *hd.ExtendedKey {
	return w.masterPublicKey
}

func (w *SPVWallet) CurrentAddress(purpose KeyPurpose) btc.Address {
	key := w.txstore.GetCurrentKey(purpose)
	addr, _ := key.Address(w.params)
	return btc.Address(addr)
}

func (w *SPVWallet) HasKey(addr btc.Address) bool {
	script, err := txscript.PayToAddrScript(addr)
	if err != nil {
		return false
	}
	_, err = w.txstore.GetKeyForScript(script)
	if err != nil {
		return false
	}
	return true
}

func (w *SPVWallet) Balance() (confirmed, unconfirmed int64) {
	utxos, _ := w.txstore.Utxos().GetAll()
	stxos, _ := w.txstore.Stxos().GetAll()
	for _, utxo := range utxos {
		if !utxo.Freeze {
			if utxo.AtHeight > 0 {
				confirmed += utxo.Value
			} else {
				if w.checkIfStxoIsConfirmed(utxo, stxos) {
					confirmed += utxo.Value
				} else {
					unconfirmed += utxo.Value
				}
			}
		}
	}
	return confirmed, unconfirmed
}

func (w *SPVWallet) checkIfStxoIsConfirmed(utxo Utxo, stxos []Stxo) bool {
	for _, stxo := range stxos {
		if stxo.SpendTxid.IsEqual(&utxo.Op.Hash) {
			if stxo.Utxo.AtHeight > 0 {
				return true
			} else {
				return w.checkIfStxoIsConfirmed(stxo.Utxo, stxos)
			}
		}
	}
	return false
}

func (w *SPVWallet) Params() *chaincfg.Params {
	return w.params
}

func (w *SPVWallet) AddTransactionListener(callback func(TransactionCallback)) {
	w.txstore.listeners = append(w.txstore.listeners, callback)
}

func (w *SPVWallet) ChainTip() uint32 {
	height, _ := w.blockchain.db.Height()
	return uint32(height)
}

func (w *SPVWallet) AddWatchedScript(script []byte) error {
	err := w.txstore.WatchedScripts().Put(script)
	w.txstore.PopulateAdrs()

	for _, peer := range w.PeerManager.ConnectedPeers() {
		w.updateFilterAndSend(peer)
	}
	return err
}

func (w *SPVWallet) GenerateMultisigScript(keys []hd.ExtendedKey, threshold int) (addr btc.Address, redeemScript []byte, err error) {
	var addrPubKeys []*btc.AddressPubKey
	for _, key := range keys {
		ecKey, err := key.ECPubKey()
		if err != nil {
			return nil, nil, err
		}
		k, err := btc.NewAddressPubKey(ecKey.SerializeCompressed(), w.params)
		if err != nil {
			return nil, nil, err
		}
		addrPubKeys = append(addrPubKeys, k)
	}
	redeemScript, err = txscript.MultiSigScript(addrPubKeys, threshold)
	if err != nil {
		return nil, nil, err
	}
	addr, err = btc.NewAddressScriptHash(redeemScript, w.params)
	if err != nil {
		return nil, nil, err
	}
	return addr, redeemScript, nil
}

func (w *SPVWallet) Close() {
	log.Info("Disconnecting from peers and shutting down")
	if w.running {
		w.PeerManager.Stop()
		w.blockchain.Close()
		w.stopChan <- 1
		w.running = false
	}
}

func (w *SPVWallet) ReSyncBlockchain(fromHeight int32) {
	w.Close()
	os.Remove(path.Join(w.repoPath, "headers.bin"))
	blockchain, err := NewBlockchain(w.repoPath, w.params)
	if err != nil {
		return
	}
	w.blockchain = blockchain
	go w.Start()
}
