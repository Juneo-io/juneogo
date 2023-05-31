// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/btree"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/cache/metercacher"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/linkeddb"
	"github.com/ava-labs/avalanchego/database/prefixdb"
	"github.com/ava-labs/avalanchego/database/versiondb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/snow/uptime"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm/blocks"
	"github.com/ava-labs/avalanchego/vms/platformvm/config"
	"github.com/ava-labs/avalanchego/vms/platformvm/genesis"
	"github.com/ava-labs/avalanchego/vms/platformvm/metrics"
	"github.com/ava-labs/avalanchego/vms/platformvm/reward"
	"github.com/ava-labs/avalanchego/vms/platformvm/status"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
)

const (
	validatorDiffsCacheSize = 2048
	blockCacheSize          = 2048
	txCacheSize             = 2048
	rewardUTXOsCacheSize    = 2048
	chainCacheSize          = 2048
	chainDBCacheSize        = 2048
)

var (
	_ State = (*state)(nil)

	ErrDelegatorSubset              = errors.New("delegator's time range must be a subset of the validator's time range")
	errMissingValidatorSet          = errors.New("missing validator set")
	errValidatorSetAlreadyPopulated = errors.New("validator set already populated")
	errDuplicateValidatorSet        = errors.New("duplicate validator set")

	blockPrefix                   = []byte("block")
	validatorsPrefix              = []byte("validators")
	currentPrefix                 = []byte("current")
	pendingPrefix                 = []byte("pending")
	validatorPrefix               = []byte("validator")
	delegatorPrefix               = []byte("delegator")
	supernetValidatorPrefix       = []byte("supernetValidator")
	supernetDelegatorPrefix       = []byte("supernetDelegator")
	validatorWeightDiffsPrefix    = []byte("validatorDiffs")
	validatorPublicKeyDiffsPrefix = []byte("publicKeyDiffs")
	txPrefix                      = []byte("tx")
	rewardUTXOsPrefix             = []byte("rewardUTXOs")
	utxoPrefix                    = []byte("utxo")
	supernetPrefix                = []byte("supernet")
	transformedSupernetPrefix     = []byte("transformedSupernet")
	supplyPrefix                  = []byte("supply")
	rewardsSupplyPrefix           = []byte("rewardsSupply")
	chainPrefix                   = []byte("chain")
	singletonPrefix               = []byte("singleton")

	timestampKey         = []byte("timestamp")
	currentSupplyKey     = []byte("current supply")
	rewardsPoolSupplyKey = []byte("rewards pool supply")
	feesPoolValueKey     = []byte("fees pool value")
	lastAcceptedKey      = []byte("last accepted")
	initializedKey       = []byte("initialized")
)

// Chain collects all methods to manage the state of the chain for block
// execution.
type Chain interface {
	Stakers
	avax.UTXOAdder
	avax.UTXOGetter
	avax.UTXODeleter

	GetTimestamp() time.Time
	SetTimestamp(tm time.Time)

	GetCurrentSupply(supernetID ids.ID) (uint64, error)
	SetCurrentSupply(supernetID ids.ID, cs uint64)

	GetRewardsPoolSupply(supernetID ids.ID) (uint64, error)
	SetRewardsPoolSupply(supernetID ids.ID, rps uint64)

	GetFeesPoolValue() uint64
	SetFeesPoolValue(fpv uint64)

	GetRewardUTXOs(txID ids.ID) ([]*avax.UTXO, error)
	AddRewardUTXO(txID ids.ID, utxo *avax.UTXO)

	GetSupernets() ([]*txs.Tx, error)
	AddSupernet(createSupernetTx *txs.Tx)

	GetSupernetTransformation(supernetID ids.ID) (*txs.Tx, error)
	AddSupernetTransformation(transformSupernetTx *txs.Tx)

	GetChains(supernetID ids.ID) ([]*txs.Tx, error)
	AddChain(createChainTx *txs.Tx)

	GetTx(txID ids.ID) (*txs.Tx, status.Status, error)
	AddTx(tx *txs.Tx, status status.Status)
}

type State interface {
	Chain
	uptime.State
	avax.UTXOReader

	GetLastAccepted() ids.ID
	SetLastAccepted(blkID ids.ID)

	GetStatelessBlock(blockID ids.ID) (blocks.Block, choices.Status, error)
	AddStatelessBlock(block blocks.Block, status choices.Status)

	// ValidatorSet adds all the validators and delegators of [supernetID] into
	// [vdrs].
	ValidatorSet(supernetID ids.ID, vdrs validators.Set) error

	GetValidatorWeightDiffs(height uint64, supernetID ids.ID) (map[ids.NodeID]*ValidatorWeightDiff, error)

	// Returns a map of node ID --> BLS Public Key for all validators
	// that left the Primary Network validator set.
	GetValidatorPublicKeyDiffs(height uint64) (map[ids.NodeID]*bls.PublicKey, error)

	SetHeight(height uint64)

	// Discard uncommitted changes to the database.
	Abort()

	// Commit changes to the base database.
	Commit() error

	// Returns a batch of unwritten changes that, when written, will commit all
	// pending changes to the base database.
	CommitBatch() (database.Batch, error)

	Close() error
}

type stateBlk struct {
	Blk    blocks.Block
	Bytes  []byte         `serialize:"true"`
	Status choices.Status `serialize:"true"`
}

/*
 * VMDB
 * |-. validators
 * | |-. current
 * | | |-. validator
 * | | | '-. list
 * | | |   '-- txID -> uptime + potential reward + potential delegatee reward
 * | | |-. delegator
 * | | | '-. list
 * | | |   '-- txID -> potential reward
 * | | |-. supernetValidator
 * | | | '-. list
 * | | |   '-- txID -> uptime + potential reward + potential delegatee reward
 * | | '-. supernetDelegator
 * | |   '-. list
 * | |     '-- txID -> potential reward
 * | |-. pending
 * | | |-. validator
 * | | | '-. list
 * | | |   '-- txID -> nil
 * | | |-. delegator
 * | | | '-. list
 * | | |   '-- txID -> nil
 * | | |-. supernetValidator
 * | | | '-. list
 * | | |   '-- txID -> nil
 * | | '-. supernetDelegator
 * | |   '-. list
 * | |     '-- txID -> nil
 * | |-. weight diffs
 * | | '-. height+supernet
 * | |   '-. list
 * | |     '-- nodeID -> weightChange
 * | '-. pub key diffs
 * |   '-. height
 * |     '-. list
 * |       '-- nodeID -> public key
 * |-. blocks
 * | '-- blockID -> block bytes
 * |-. txs
 * | '-- txID -> tx bytes + tx status
 * |- rewardUTXOs
 * | '-. txID
 * |   '-. list
 * |     '-- utxoID -> utxo bytes
 * |- utxos
 * | '-- utxoDB
 * |-. supernets
 * | '-. list
 * |   '-- txID -> nil
 * |-. chains
 * | '-. supernetID
 * |   '-. list
 * |     '-- txID -> nil
 * '-. singletons
 *   |-- initializedKey -> nil
 *   |-- timestampKey -> timestamp
 *   |-- currentSupplyKey -> currentSupply
 *   |-- rewardsPoolSupplyKey -> rewardsPoolSupply
 *   |-- feesPoolValueKey -> feesPoolValue
 *   '-- lastAcceptedKey -> lastAccepted
 */
type state struct {
	validatorState

	cfg          *config.Config
	ctx          *snow.Context
	metrics      metrics.Metrics
	rewards      reward.Calculator
	bootstrapped *utils.Atomic[bool]

	baseDB *versiondb.Database

	currentStakers *baseStakers
	pendingStakers *baseStakers

	currentHeight uint64

	addedBlocks map[ids.ID]stateBlk // map of blockID -> Block
	// cache of blockID -> Block
	// If the block isn't known, nil is cached.
	blockCache cache.Cacher[ids.ID, *stateBlk]
	blockDB    database.Database

	validatorsDB                   database.Database
	currentValidatorsDB            database.Database
	currentValidatorBaseDB         database.Database
	currentValidatorList           linkeddb.LinkedDB
	currentDelegatorBaseDB         database.Database
	currentDelegatorList           linkeddb.LinkedDB
	currentSupernetValidatorBaseDB database.Database
	currentSupernetValidatorList   linkeddb.LinkedDB
	currentSupernetDelegatorBaseDB database.Database
	currentSupernetDelegatorList   linkeddb.LinkedDB
	pendingValidatorsDB            database.Database
	pendingValidatorBaseDB         database.Database
	pendingValidatorList           linkeddb.LinkedDB
	pendingDelegatorBaseDB         database.Database
	pendingDelegatorList           linkeddb.LinkedDB
	pendingSupernetValidatorBaseDB database.Database
	pendingSupernetValidatorList   linkeddb.LinkedDB
	pendingSupernetDelegatorBaseDB database.Database
	pendingSupernetDelegatorList   linkeddb.LinkedDB

	validatorWeightDiffsCache cache.Cacher[string, map[ids.NodeID]*ValidatorWeightDiff] // cache of heightWithSupernet -> map[ids.NodeID]*ValidatorWeightDiff
	validatorWeightDiffsDB    database.Database

	validatorPublicKeyDiffsCache cache.Cacher[uint64, map[ids.NodeID]*bls.PublicKey] // cache of height -> map[ids.NodeID]*bls.PublicKey
	validatorPublicKeyDiffsDB    database.Database

	addedTxs map[ids.ID]*txAndStatus            // map of txID -> {*txs.Tx, Status}
	txCache  cache.Cacher[ids.ID, *txAndStatus] // txID -> {*txs.Tx, Status}. If the entry is nil, it isn't in the database
	txDB     database.Database

	addedRewardUTXOs map[ids.ID][]*avax.UTXO            // map of txID -> []*UTXO
	rewardUTXOsCache cache.Cacher[ids.ID, []*avax.UTXO] // txID -> []*UTXO
	rewardUTXODB     database.Database

	modifiedUTXOs map[ids.ID]*avax.UTXO // map of modified UTXOID -> *UTXO if the UTXO is nil, it has been removed
	utxoDB        database.Database
	utxoState     avax.UTXOState

	cachedSupernets []*txs.Tx // nil if the supernets haven't been loaded
	addedSupernets  []*txs.Tx
	supernetBaseDB  database.Database
	supernetDB      linkeddb.LinkedDB

	transformedSupernets     map[ids.ID]*txs.Tx            // map of supernetID -> transformSupernetTx
	transformedSupernetCache cache.Cacher[ids.ID, *txs.Tx] // cache of supernetID -> transformSupernetTx if the entry is nil, it is not in the database
	transformedSupernetDB    database.Database

	modifiedSupplies map[ids.ID]uint64             // map of supernetID -> current supply
	supplyCache      cache.Cacher[ids.ID, *uint64] // cache of supernetID -> current supply if the entry is nil, it is not in the database
	supplyDB         database.Database

	modifiedRewardsSupplies map[ids.ID]uint64             // map of supernetID -> rewards pool supply
	rewardsSupplyCache      cache.Cacher[ids.ID, *uint64] // cache of supernetID -> rewards pool supply if the entry is nil, it is not in the database
	rewardsSupplyDB         database.Database

	addedChains  map[ids.ID][]*txs.Tx                    // maps supernetID -> the newly added chains to the supernet
	chainCache   cache.Cacher[ids.ID, []*txs.Tx]         // cache of supernetID -> the chains after all local modifications []*txs.Tx
	chainDBCache cache.Cacher[ids.ID, linkeddb.LinkedDB] // cache of supernetID -> linkedDB
	chainDB      database.Database

	// The persisted fields represent the current database value
	timestamp, persistedTimestamp                 time.Time
	currentSupply, persistedCurrentSupply         uint64
	rewardsPoolSupply, persistedRewardsPoolSupply uint64
	feesPoolValue, persistedFeesPoolValue         uint64
	// [lastAccepted] is the most recently accepted block.
	lastAccepted, persistedLastAccepted ids.ID
	singletonDB                         database.Database
}

type ValidatorWeightDiff struct {
	Decrease bool   `serialize:"true"`
	Amount   uint64 `serialize:"true"`
}

func (v *ValidatorWeightDiff) Add(negative bool, amount uint64) error {
	if v.Decrease == negative {
		var err error
		v.Amount, err = math.Add64(v.Amount, amount)
		return err
	}

	if v.Amount > amount {
		v.Amount -= amount
	} else {
		v.Amount = math.AbsDiff(v.Amount, amount)
		v.Decrease = negative
	}
	return nil
}

type heightWithSupernet struct {
	Height     uint64 `serialize:"true"`
	SupernetID ids.ID `serialize:"true"`
}

type txBytesAndStatus struct {
	Tx     []byte        `serialize:"true"`
	Status status.Status `serialize:"true"`
}

type txAndStatus struct {
	tx     *txs.Tx
	status status.Status
}

func New(
	db database.Database,
	genesisBytes []byte,
	metricsReg prometheus.Registerer,
	cfg *config.Config,
	ctx *snow.Context,
	metrics metrics.Metrics,
	rewards reward.Calculator,
	bootstrapped *utils.Atomic[bool],
) (State, error) {
	s, err := new(
		db,
		metrics,
		cfg,
		ctx,
		metricsReg,
		rewards,
		bootstrapped,
	)
	if err != nil {
		return nil, err
	}

	if err := s.sync(genesisBytes); err != nil {
		// Drop any errors on close to return the first error
		_ = s.Close()

		return nil, err
	}

	return s, nil
}

func new(
	db database.Database,
	metrics metrics.Metrics,
	cfg *config.Config,
	ctx *snow.Context,
	metricsReg prometheus.Registerer,
	rewards reward.Calculator,
	bootstrapped *utils.Atomic[bool],
) (*state, error) {
	blockCache, err := metercacher.New[ids.ID, *stateBlk](
		"block_cache",
		metricsReg,
		&cache.LRU[ids.ID, *stateBlk]{Size: blockCacheSize},
	)
	if err != nil {
		return nil, err
	}

	baseDB := versiondb.New(db)

	validatorsDB := prefixdb.New(validatorsPrefix, baseDB)

	currentValidatorsDB := prefixdb.New(currentPrefix, validatorsDB)
	currentValidatorBaseDB := prefixdb.New(validatorPrefix, currentValidatorsDB)
	currentDelegatorBaseDB := prefixdb.New(delegatorPrefix, currentValidatorsDB)
	currentSupernetValidatorBaseDB := prefixdb.New(supernetValidatorPrefix, currentValidatorsDB)
	currentSupernetDelegatorBaseDB := prefixdb.New(supernetDelegatorPrefix, currentValidatorsDB)

	pendingValidatorsDB := prefixdb.New(pendingPrefix, validatorsDB)
	pendingValidatorBaseDB := prefixdb.New(validatorPrefix, pendingValidatorsDB)
	pendingDelegatorBaseDB := prefixdb.New(delegatorPrefix, pendingValidatorsDB)
	pendingSupernetValidatorBaseDB := prefixdb.New(supernetValidatorPrefix, pendingValidatorsDB)
	pendingSupernetDelegatorBaseDB := prefixdb.New(supernetDelegatorPrefix, pendingValidatorsDB)

	validatorWeightDiffsDB := prefixdb.New(validatorWeightDiffsPrefix, validatorsDB)
	validatorWeightDiffsCache, err := metercacher.New[string, map[ids.NodeID]*ValidatorWeightDiff](
		"validator_weight_diffs_cache",
		metricsReg,
		&cache.LRU[string, map[ids.NodeID]*ValidatorWeightDiff]{Size: validatorDiffsCacheSize},
	)
	if err != nil {
		return nil, err
	}

	validatorPublicKeyDiffsDB := prefixdb.New(validatorPublicKeyDiffsPrefix, validatorsDB)
	validatorPublicKeyDiffsCache, err := metercacher.New[uint64, map[ids.NodeID]*bls.PublicKey](
		"validator_pub_key_diffs_cache",
		metricsReg,
		&cache.LRU[uint64, map[ids.NodeID]*bls.PublicKey]{Size: validatorDiffsCacheSize},
	)
	if err != nil {
		return nil, err
	}

	txCache, err := metercacher.New[ids.ID, *txAndStatus](
		"tx_cache",
		metricsReg,
		&cache.LRU[ids.ID, *txAndStatus]{Size: txCacheSize},
	)
	if err != nil {
		return nil, err
	}

	rewardUTXODB := prefixdb.New(rewardUTXOsPrefix, baseDB)
	rewardUTXOsCache, err := metercacher.New[ids.ID, []*avax.UTXO](
		"reward_utxos_cache",
		metricsReg,
		&cache.LRU[ids.ID, []*avax.UTXO]{Size: rewardUTXOsCacheSize},
	)
	if err != nil {
		return nil, err
	}

	utxoDB := prefixdb.New(utxoPrefix, baseDB)
	utxoState, err := avax.NewMeteredUTXOState(utxoDB, txs.GenesisCodec, metricsReg)
	if err != nil {
		return nil, err
	}

	supernetBaseDB := prefixdb.New(supernetPrefix, baseDB)

	transformedSupernetCache, err := metercacher.New[ids.ID, *txs.Tx](
		"transformed_supernet_cache",
		metricsReg,
		&cache.LRU[ids.ID, *txs.Tx]{Size: chainCacheSize},
	)
	if err != nil {
		return nil, err
	}

	supplyCache, err := metercacher.New[ids.ID, *uint64](
		"supply_cache",
		metricsReg,
		&cache.LRU[ids.ID, *uint64]{Size: chainCacheSize},
	)
	if err != nil {
		return nil, err
	}

	rewardsSupplyCache, err := metercacher.New[ids.ID, *uint64](
		"rewards_supply_cache",
		metricsReg,
		&cache.LRU[ids.ID, *uint64]{Size: chainCacheSize},
	)
	if err != nil {
		return nil, err
	}

	chainCache, err := metercacher.New[ids.ID, []*txs.Tx](
		"chain_cache",
		metricsReg,
		&cache.LRU[ids.ID, []*txs.Tx]{Size: chainCacheSize},
	)
	if err != nil {
		return nil, err
	}

	chainDBCache, err := metercacher.New[ids.ID, linkeddb.LinkedDB](
		"chain_db_cache",
		metricsReg,
		&cache.LRU[ids.ID, linkeddb.LinkedDB]{Size: chainDBCacheSize},
	)
	if err != nil {
		return nil, err
	}

	return &state{
		validatorState: newValidatorState(),

		cfg:          cfg,
		ctx:          ctx,
		metrics:      metrics,
		rewards:      rewards,
		bootstrapped: bootstrapped,
		baseDB:       baseDB,

		addedBlocks: make(map[ids.ID]stateBlk),
		blockCache:  blockCache,
		blockDB:     prefixdb.New(blockPrefix, baseDB),

		currentStakers: newBaseStakers(),
		pendingStakers: newBaseStakers(),

		validatorsDB:                   validatorsDB,
		currentValidatorsDB:            currentValidatorsDB,
		currentValidatorBaseDB:         currentValidatorBaseDB,
		currentValidatorList:           linkeddb.NewDefault(currentValidatorBaseDB),
		currentDelegatorBaseDB:         currentDelegatorBaseDB,
		currentDelegatorList:           linkeddb.NewDefault(currentDelegatorBaseDB),
		currentSupernetValidatorBaseDB: currentSupernetValidatorBaseDB,
		currentSupernetValidatorList:   linkeddb.NewDefault(currentSupernetValidatorBaseDB),
		currentSupernetDelegatorBaseDB: currentSupernetDelegatorBaseDB,
		currentSupernetDelegatorList:   linkeddb.NewDefault(currentSupernetDelegatorBaseDB),
		pendingValidatorsDB:            pendingValidatorsDB,
		pendingValidatorBaseDB:         pendingValidatorBaseDB,
		pendingValidatorList:           linkeddb.NewDefault(pendingValidatorBaseDB),
		pendingDelegatorBaseDB:         pendingDelegatorBaseDB,
		pendingDelegatorList:           linkeddb.NewDefault(pendingDelegatorBaseDB),
		pendingSupernetValidatorBaseDB: pendingSupernetValidatorBaseDB,
		pendingSupernetValidatorList:   linkeddb.NewDefault(pendingSupernetValidatorBaseDB),
		pendingSupernetDelegatorBaseDB: pendingSupernetDelegatorBaseDB,
		pendingSupernetDelegatorList:   linkeddb.NewDefault(pendingSupernetDelegatorBaseDB),
		validatorWeightDiffsDB:         validatorWeightDiffsDB,
		validatorWeightDiffsCache:      validatorWeightDiffsCache,
		validatorPublicKeyDiffsCache:   validatorPublicKeyDiffsCache,
		validatorPublicKeyDiffsDB:      validatorPublicKeyDiffsDB,

		addedTxs: make(map[ids.ID]*txAndStatus),
		txDB:     prefixdb.New(txPrefix, baseDB),
		txCache:  txCache,

		addedRewardUTXOs: make(map[ids.ID][]*avax.UTXO),
		rewardUTXODB:     rewardUTXODB,
		rewardUTXOsCache: rewardUTXOsCache,

		modifiedUTXOs: make(map[ids.ID]*avax.UTXO),
		utxoDB:        utxoDB,
		utxoState:     utxoState,

		supernetBaseDB: supernetBaseDB,
		supernetDB:     linkeddb.NewDefault(supernetBaseDB),

		transformedSupernets:     make(map[ids.ID]*txs.Tx),
		transformedSupernetCache: transformedSupernetCache,
		transformedSupernetDB:    prefixdb.New(transformedSupernetPrefix, baseDB),

		modifiedSupplies: make(map[ids.ID]uint64),
		supplyCache:      supplyCache,
		supplyDB:         prefixdb.New(supplyPrefix, baseDB),

		modifiedRewardsSupplies: make(map[ids.ID]uint64),
		rewardsSupplyCache:      rewardsSupplyCache,
		rewardsSupplyDB:         prefixdb.New(rewardsSupplyPrefix, baseDB),

		addedChains:  make(map[ids.ID][]*txs.Tx),
		chainDB:      prefixdb.New(chainPrefix, baseDB),
		chainCache:   chainCache,
		chainDBCache: chainDBCache,

		singletonDB: prefixdb.New(singletonPrefix, baseDB),
	}, nil
}

func (s *state) GetCurrentValidator(supernetID ids.ID, nodeID ids.NodeID) (*Staker, error) {
	return s.currentStakers.GetValidator(supernetID, nodeID)
}

func (s *state) PutCurrentValidator(staker *Staker) {
	s.currentStakers.PutValidator(staker)
}

func (s *state) DeleteCurrentValidator(staker *Staker) {
	s.currentStakers.DeleteValidator(staker)
}

func (s *state) GetCurrentDelegatorIterator(supernetID ids.ID, nodeID ids.NodeID) (StakerIterator, error) {
	return s.currentStakers.GetDelegatorIterator(supernetID, nodeID), nil
}

func (s *state) PutCurrentDelegator(staker *Staker) {
	s.currentStakers.PutDelegator(staker)
}

func (s *state) DeleteCurrentDelegator(staker *Staker) {
	s.currentStakers.DeleteDelegator(staker)
}

func (s *state) GetCurrentStakerIterator() (StakerIterator, error) {
	return s.currentStakers.GetStakerIterator(), nil
}

func (s *state) GetPendingValidator(supernetID ids.ID, nodeID ids.NodeID) (*Staker, error) {
	return s.pendingStakers.GetValidator(supernetID, nodeID)
}

func (s *state) PutPendingValidator(staker *Staker) {
	s.pendingStakers.PutValidator(staker)
}

func (s *state) DeletePendingValidator(staker *Staker) {
	s.pendingStakers.DeleteValidator(staker)
}

func (s *state) GetPendingDelegatorIterator(supernetID ids.ID, nodeID ids.NodeID) (StakerIterator, error) {
	return s.pendingStakers.GetDelegatorIterator(supernetID, nodeID), nil
}

func (s *state) PutPendingDelegator(staker *Staker) {
	s.pendingStakers.PutDelegator(staker)
}

func (s *state) DeletePendingDelegator(staker *Staker) {
	s.pendingStakers.DeleteDelegator(staker)
}

func (s *state) GetPendingStakerIterator() (StakerIterator, error) {
	return s.pendingStakers.GetStakerIterator(), nil
}

func (s *state) shouldInit() (bool, error) {
	has, err := s.singletonDB.Has(initializedKey)
	return !has, err
}

func (s *state) doneInit() error {
	return s.singletonDB.Put(initializedKey, nil)
}

func (s *state) GetSupernets() ([]*txs.Tx, error) {
	if s.cachedSupernets != nil {
		return s.cachedSupernets, nil
	}

	supernetDBIt := s.supernetDB.NewIterator()
	defer supernetDBIt.Release()

	txs := []*txs.Tx(nil)
	for supernetDBIt.Next() {
		supernetIDBytes := supernetDBIt.Key()
		supernetID, err := ids.ToID(supernetIDBytes)
		if err != nil {
			return nil, err
		}
		supernetTx, _, err := s.GetTx(supernetID)
		if err != nil {
			return nil, err
		}
		txs = append(txs, supernetTx)
	}
	if err := supernetDBIt.Error(); err != nil {
		return nil, err
	}
	txs = append(txs, s.addedSupernets...)
	s.cachedSupernets = txs
	return txs, nil
}

func (s *state) AddSupernet(createSupernetTx *txs.Tx) {
	s.addedSupernets = append(s.addedSupernets, createSupernetTx)
	if s.cachedSupernets != nil {
		s.cachedSupernets = append(s.cachedSupernets, createSupernetTx)
	}
}

func (s *state) GetSupernetTransformation(supernetID ids.ID) (*txs.Tx, error) {
	if tx, exists := s.transformedSupernets[supernetID]; exists {
		return tx, nil
	}

	if tx, cached := s.transformedSupernetCache.Get(supernetID); cached {
		if tx == nil {
			return nil, database.ErrNotFound
		}
		return tx, nil
	}

	transformSupernetTxID, err := database.GetID(s.transformedSupernetDB, supernetID[:])
	if err == database.ErrNotFound {
		s.transformedSupernetCache.Put(supernetID, nil)
		return nil, database.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	transformSupernetTx, _, err := s.GetTx(transformSupernetTxID)
	if err != nil {
		return nil, err
	}
	s.transformedSupernetCache.Put(supernetID, transformSupernetTx)
	return transformSupernetTx, nil
}

func (s *state) AddSupernetTransformation(transformSupernetTxIntf *txs.Tx) {
	transformSupernetTx := transformSupernetTxIntf.Unsigned.(*txs.TransformSupernetTx)
	s.transformedSupernets[transformSupernetTx.Supernet] = transformSupernetTxIntf
}

func (s *state) GetChains(supernetID ids.ID) ([]*txs.Tx, error) {
	if chains, cached := s.chainCache.Get(supernetID); cached {
		return chains, nil
	}
	chainDB := s.getChainDB(supernetID)
	chainDBIt := chainDB.NewIterator()
	defer chainDBIt.Release()

	txs := []*txs.Tx(nil)
	for chainDBIt.Next() {
		chainIDBytes := chainDBIt.Key()
		chainID, err := ids.ToID(chainIDBytes)
		if err != nil {
			return nil, err
		}
		chainTx, _, err := s.GetTx(chainID)
		if err != nil {
			return nil, err
		}
		txs = append(txs, chainTx)
	}
	if err := chainDBIt.Error(); err != nil {
		return nil, err
	}
	txs = append(txs, s.addedChains[supernetID]...)
	s.chainCache.Put(supernetID, txs)
	return txs, nil
}

func (s *state) AddChain(createChainTxIntf *txs.Tx) {
	createChainTx := createChainTxIntf.Unsigned.(*txs.CreateChainTx)
	supernetID := createChainTx.SupernetID
	s.addedChains[supernetID] = append(s.addedChains[supernetID], createChainTxIntf)
	if chains, cached := s.chainCache.Get(supernetID); cached {
		chains = append(chains, createChainTxIntf)
		s.chainCache.Put(supernetID, chains)
	}
}

func (s *state) getChainDB(supernetID ids.ID) linkeddb.LinkedDB {
	if chainDB, cached := s.chainDBCache.Get(supernetID); cached {
		return chainDB
	}
	rawChainDB := prefixdb.New(supernetID[:], s.chainDB)
	chainDB := linkeddb.NewDefault(rawChainDB)
	s.chainDBCache.Put(supernetID, chainDB)
	return chainDB
}

func (s *state) GetTx(txID ids.ID) (*txs.Tx, status.Status, error) {
	if tx, exists := s.addedTxs[txID]; exists {
		return tx.tx, tx.status, nil
	}
	if tx, cached := s.txCache.Get(txID); cached {
		if tx == nil {
			return nil, status.Unknown, database.ErrNotFound
		}
		return tx.tx, tx.status, nil
	}
	txBytes, err := s.txDB.Get(txID[:])
	if err == database.ErrNotFound {
		s.txCache.Put(txID, nil)
		return nil, status.Unknown, database.ErrNotFound
	} else if err != nil {
		return nil, status.Unknown, err
	}

	stx := txBytesAndStatus{}
	if _, err := txs.GenesisCodec.Unmarshal(txBytes, &stx); err != nil {
		return nil, status.Unknown, err
	}

	tx, err := txs.Parse(txs.GenesisCodec, stx.Tx)
	if err != nil {
		return nil, status.Unknown, err
	}

	ptx := &txAndStatus{
		tx:     tx,
		status: stx.Status,
	}

	s.txCache.Put(txID, ptx)
	return ptx.tx, ptx.status, nil
}

func (s *state) AddTx(tx *txs.Tx, status status.Status) {
	s.addedTxs[tx.ID()] = &txAndStatus{
		tx:     tx,
		status: status,
	}
}

func (s *state) GetRewardUTXOs(txID ids.ID) ([]*avax.UTXO, error) {
	if utxos, exists := s.addedRewardUTXOs[txID]; exists {
		return utxos, nil
	}
	if utxos, exists := s.rewardUTXOsCache.Get(txID); exists {
		return utxos, nil
	}

	rawTxDB := prefixdb.New(txID[:], s.rewardUTXODB)
	txDB := linkeddb.NewDefault(rawTxDB)
	it := txDB.NewIterator()
	defer it.Release()

	utxos := []*avax.UTXO(nil)
	for it.Next() {
		utxo := &avax.UTXO{}
		if _, err := txs.Codec.Unmarshal(it.Value(), utxo); err != nil {
			return nil, err
		}
		utxos = append(utxos, utxo)
	}
	if err := it.Error(); err != nil {
		return nil, err
	}

	s.rewardUTXOsCache.Put(txID, utxos)
	return utxos, nil
}

func (s *state) AddRewardUTXO(txID ids.ID, utxo *avax.UTXO) {
	s.addedRewardUTXOs[txID] = append(s.addedRewardUTXOs[txID], utxo)
}

func (s *state) GetUTXO(utxoID ids.ID) (*avax.UTXO, error) {
	if utxo, exists := s.modifiedUTXOs[utxoID]; exists {
		if utxo == nil {
			return nil, database.ErrNotFound
		}
		return utxo, nil
	}
	return s.utxoState.GetUTXO(utxoID)
}

func (s *state) UTXOIDs(addr []byte, start ids.ID, limit int) ([]ids.ID, error) {
	return s.utxoState.UTXOIDs(addr, start, limit)
}

func (s *state) AddUTXO(utxo *avax.UTXO) {
	s.modifiedUTXOs[utxo.InputID()] = utxo
}

func (s *state) DeleteUTXO(utxoID ids.ID) {
	s.modifiedUTXOs[utxoID] = nil
}

func (s *state) GetStartTime(nodeID ids.NodeID, supernetID ids.ID) (time.Time, error) {
	staker, err := s.currentStakers.GetValidator(supernetID, nodeID)
	if err != nil {
		return time.Time{}, err
	}
	return staker.StartTime, nil
}

func (s *state) GetTimestamp() time.Time {
	return s.timestamp
}

func (s *state) SetTimestamp(tm time.Time) {
	s.timestamp = tm
}

func (s *state) GetLastAccepted() ids.ID {
	return s.lastAccepted
}

func (s *state) SetLastAccepted(lastAccepted ids.ID) {
	s.lastAccepted = lastAccepted
}

func (s *state) GetCurrentSupply(supernetID ids.ID) (uint64, error) {
	if supernetID == constants.PrimaryNetworkID {
		return s.currentSupply, nil
	}

	supply, ok := s.modifiedSupplies[supernetID]
	if ok {
		return supply, nil
	}

	cachedSupply, ok := s.supplyCache.Get(supernetID)
	if ok {
		if cachedSupply == nil {
			return 0, database.ErrNotFound
		}
		return *cachedSupply, nil
	}

	supply, err := database.GetUInt64(s.supplyDB, supernetID[:])
	if err == database.ErrNotFound {
		s.supplyCache.Put(supernetID, nil)
		return 0, database.ErrNotFound
	}
	if err != nil {
		return 0, err
	}

	s.supplyCache.Put(supernetID, &supply)
	return supply, nil
}

func (s *state) SetCurrentSupply(supernetID ids.ID, cs uint64) {
	if supernetID == constants.PrimaryNetworkID {
		s.currentSupply = cs
	} else {
		s.modifiedSupplies[supernetID] = cs
	}
}

func (s *state) GetRewardsPoolSupply(supernetID ids.ID) (uint64, error) {
	if supernetID == constants.PrimaryNetworkID {
		return s.rewardsPoolSupply, nil
	}

	rewardsSupply, ok := s.modifiedRewardsSupplies[supernetID]
	if ok {
		return rewardsSupply, nil
	}

	cachedRewardsSupply, ok := s.rewardsSupplyCache.Get(supernetID)
	if ok {
		if cachedRewardsSupply == nil {
			return 0, database.ErrNotFound
		}
		return *cachedRewardsSupply, nil
	}

	rewardsSupply, err := database.GetUInt64(s.rewardsSupplyDB, supernetID[:])
	if err == database.ErrNotFound {
		s.rewardsSupplyCache.Put(supernetID, nil)
		return 0, database.ErrNotFound
	}
	if err != nil {
		return 0, err
	}

	s.rewardsSupplyCache.Put(supernetID, &rewardsSupply)
	return rewardsSupply, nil
}

func (s *state) SetRewardsPoolSupply(supernetID ids.ID, rps uint64) {
	if supernetID == constants.PrimaryNetworkID {
		s.rewardsPoolSupply = rps
	} else {
		s.modifiedRewardsSupplies[supernetID] = rps
	}
}

func (s *state) GetFeesPoolValue() uint64 {
	return s.feesPoolValue
}

func (s *state) SetFeesPoolValue(fpv uint64) {
	s.feesPoolValue = fpv
}

func (s *state) ValidatorSet(supernetID ids.ID, vdrs validators.Set) error {
	for nodeID, validator := range s.currentStakers.validators[supernetID] {
		staker := validator.validator
		if err := vdrs.Add(nodeID, staker.PublicKey, staker.TxID, staker.Weight); err != nil {
			return err
		}

		delegatorIterator := NewTreeIterator(validator.delegators)
		for delegatorIterator.Next() {
			staker := delegatorIterator.Value()
			if err := vdrs.AddWeight(nodeID, staker.Weight); err != nil {
				delegatorIterator.Release()
				return err
			}
		}
		delegatorIterator.Release()
	}
	return nil
}

func (s *state) GetValidatorWeightDiffs(height uint64, supernetID ids.ID) (map[ids.NodeID]*ValidatorWeightDiff, error) {
	prefixStruct := heightWithSupernet{
		Height:     height,
		SupernetID: supernetID,
	}
	prefixBytes, err := blocks.GenesisCodec.Marshal(blocks.Version, prefixStruct)
	if err != nil {
		return nil, err
	}
	prefixStr := string(prefixBytes)

	if weightDiffs, ok := s.validatorWeightDiffsCache.Get(prefixStr); ok {
		return weightDiffs, nil
	}

	rawDiffDB := prefixdb.New(prefixBytes, s.validatorWeightDiffsDB)
	diffDB := linkeddb.NewDefault(rawDiffDB)
	diffIter := diffDB.NewIterator()
	defer diffIter.Release()

	weightDiffs := make(map[ids.NodeID]*ValidatorWeightDiff)
	for diffIter.Next() {
		nodeID, err := ids.ToNodeID(diffIter.Key())
		if err != nil {
			return nil, err
		}

		weightDiff := ValidatorWeightDiff{}
		_, err = blocks.GenesisCodec.Unmarshal(diffIter.Value(), &weightDiff)
		if err != nil {
			return nil, err
		}

		weightDiffs[nodeID] = &weightDiff
	}

	s.validatorWeightDiffsCache.Put(prefixStr, weightDiffs)
	return weightDiffs, diffIter.Error()
}

func (s *state) GetValidatorPublicKeyDiffs(height uint64) (map[ids.NodeID]*bls.PublicKey, error) {
	if publicKeyDiffs, ok := s.validatorPublicKeyDiffsCache.Get(height); ok {
		return publicKeyDiffs, nil
	}

	heightBytes := database.PackUInt64(height)
	rawDiffDB := prefixdb.New(heightBytes, s.validatorPublicKeyDiffsDB)
	diffDB := linkeddb.NewDefault(rawDiffDB)
	diffIter := diffDB.NewIterator()
	defer diffIter.Release()

	pkDiffs := make(map[ids.NodeID]*bls.PublicKey)
	for diffIter.Next() {
		nodeID, err := ids.ToNodeID(diffIter.Key())
		if err != nil {
			return nil, err
		}

		pkBytes := diffIter.Value()
		pk, err := bls.PublicKeyFromBytes(pkBytes)
		if err != nil {
			return nil, err
		}
		pkDiffs[nodeID] = pk
	}

	s.validatorPublicKeyDiffsCache.Put(height, pkDiffs)
	return pkDiffs, diffIter.Error()
}

func (s *state) syncGenesis(genesisBlk blocks.Block, genesis *genesis.State) error {
	genesisBlkID := genesisBlk.ID()
	s.SetLastAccepted(genesisBlkID)
	genesisTimestamp := time.Unix(int64(genesis.Timestamp), 0)
	s.SetTimestamp(genesisTimestamp)
	s.SetFeesPoolValue(uint64(0))
	s.AddStatelessBlock(genesisBlk, choices.Accepted)

	// Persist UTXOs that exist at genesis
	for _, utxo := range genesis.UTXOs {
		s.AddUTXO(utxo)
	}

	totalRewards := uint64(0)

	// Persist primary network validator set at genesis
	for _, vdrTx := range genesis.Validators {
		tx, ok := vdrTx.Unsigned.(*txs.AddValidatorTx)
		if !ok {
			return fmt.Errorf("expected tx type *txs.AddValidatorTx but got %T", vdrTx.Unsigned)
		}

		stakeAmount := tx.Validator.Wght
		stakeDuration := tx.Validator.Duration()

		potentialReward := s.rewards.CalculatePrimary(
			stakeDuration,
			genesisTimestamp,
			stakeAmount,
		)
		newTotalRewards, err := math.Add64(totalRewards, potentialReward)
		if err != nil {
			return err
		}
		totalRewards = newTotalRewards

		staker, err := NewCurrentStaker(vdrTx.ID(), tx, potentialReward)
		if err != nil {
			return err
		}

		s.PutCurrentValidator(staker)
		s.AddTx(vdrTx, status.Committed)
	}

	currentSupply := genesis.InitialSupply
	rewardsPoolSupply := uint64(0)
	if genesis.RewardsPoolSupply > totalRewards {
		rewardsPoolSupply = genesis.RewardsPoolSupply - totalRewards
	} else {
		newCurrentSupply, err := math.Add64(currentSupply, totalRewards-genesis.RewardsPoolSupply)
		if err != nil {
			return err
		}
		currentSupply = newCurrentSupply
	}
	s.SetCurrentSupply(constants.PrimaryNetworkID, currentSupply)
	s.SetRewardsPoolSupply(constants.PrimaryNetworkID, rewardsPoolSupply)

	for _, chain := range genesis.Chains {
		unsignedChain, ok := chain.Unsigned.(*txs.CreateChainTx)
		if !ok {
			return fmt.Errorf("expected tx type *txs.CreateChainTx but got %T", chain.Unsigned)
		}

		// Ensure all chains that the genesis bytes say to create have the right
		// network ID
		if unsignedChain.NetworkID != s.ctx.NetworkID {
			return avax.ErrWrongNetworkID
		}

		s.AddChain(chain)
		s.AddTx(chain, status.Committed)
	}

	// updateValidators is set to false here to maintain the invariant that the
	// primary network's validator set is empty before the validator sets are
	// initialized.
	return s.write(false /*=updateValidators*/, 0)
}

// Load pulls data previously stored on disk that is expected to be in memory.
func (s *state) load() error {
	errs := wrappers.Errs{}
	errs.Add(
		s.loadMetadata(),
		s.loadCurrentValidators(),
		s.loadPendingValidators(),
		s.initValidatorSets(),
	)
	return errs.Err
}

func (s *state) loadMetadata() error {
	timestamp, err := database.GetTimestamp(s.singletonDB, timestampKey)
	if err != nil {
		return err
	}
	s.persistedTimestamp = timestamp
	s.SetTimestamp(timestamp)

	currentSupply, err := database.GetUInt64(s.singletonDB, currentSupplyKey)
	if err != nil {
		return err
	}
	s.persistedCurrentSupply = currentSupply
	s.SetCurrentSupply(constants.PrimaryNetworkID, currentSupply)

	rewardsPoolSupply, err := database.GetUInt64(s.singletonDB, rewardsPoolSupplyKey)
	if err != nil {
		return err
	}
	s.persistedRewardsPoolSupply = rewardsPoolSupply
	s.SetRewardsPoolSupply(constants.PrimaryNetworkID, rewardsPoolSupply)

	feesPoolValue, err := database.GetUInt64(s.singletonDB, feesPoolValueKey)
	if err != nil {
		return err
	}
	s.persistedFeesPoolValue = feesPoolValue
	s.SetFeesPoolValue(feesPoolValue)

	lastAccepted, err := database.GetID(s.singletonDB, lastAcceptedKey)
	if err != nil {
		return err
	}
	s.persistedLastAccepted = lastAccepted
	s.lastAccepted = lastAccepted
	return nil
}

func (s *state) loadCurrentValidators() error {
	s.currentStakers = newBaseStakers()

	validatorIt := s.currentValidatorList.NewIterator()
	defer validatorIt.Release()
	for validatorIt.Next() {
		txIDBytes := validatorIt.Key()
		txID, err := ids.ToID(txIDBytes)
		if err != nil {
			return err
		}
		tx, _, err := s.GetTx(txID)
		if err != nil {
			return err
		}

		metadataBytes := validatorIt.Value()
		metadata := &validatorMetadata{
			txID: txID,
			// Note: we don't provide [LastUpdated] here because we expect it to
			// always be present on disk.
		}
		if err := parseValidatorMetadata(metadataBytes, metadata); err != nil {
			return err
		}

		stakerTx, ok := tx.Unsigned.(txs.Staker)
		if !ok {
			return fmt.Errorf("expected tx type txs.Staker but got %T", tx.Unsigned)
		}

		staker, err := NewCurrentStaker(txID, stakerTx, metadata.PotentialReward)
		if err != nil {
			return err
		}

		validator := s.currentStakers.getOrCreateValidator(staker.SupernetID, staker.NodeID)
		validator.validator = staker

		s.currentStakers.stakers.ReplaceOrInsert(staker)

		s.validatorState.LoadValidatorMetadata(staker.NodeID, staker.SupernetID, metadata)
	}

	supernetValidatorIt := s.currentSupernetValidatorList.NewIterator()
	defer supernetValidatorIt.Release()
	for supernetValidatorIt.Next() {
		txIDBytes := supernetValidatorIt.Key()
		txID, err := ids.ToID(txIDBytes)
		if err != nil {
			return err
		}
		tx, _, err := s.GetTx(txID)
		if err != nil {
			return err
		}

		stakerTx, ok := tx.Unsigned.(txs.Staker)
		if !ok {
			return fmt.Errorf("expected tx type txs.Staker but got %T", tx.Unsigned)
		}

		metadataBytes := supernetValidatorIt.Value()
		metadata := &validatorMetadata{
			txID: txID,
			// use the start time as the fallback value
			// in case it's not stored in the database
			LastUpdated: uint64(stakerTx.StartTime().Unix()),
		}
		if err := parseValidatorMetadata(metadataBytes, metadata); err != nil {
			return err
		}

		staker, err := NewCurrentStaker(txID, stakerTx, metadata.PotentialReward)
		if err != nil {
			return err
		}
		validator := s.currentStakers.getOrCreateValidator(staker.SupernetID, staker.NodeID)
		validator.validator = staker

		s.currentStakers.stakers.ReplaceOrInsert(staker)

		s.validatorState.LoadValidatorMetadata(staker.NodeID, staker.SupernetID, metadata)
	}

	delegatorIt := s.currentDelegatorList.NewIterator()
	defer delegatorIt.Release()

	supernetDelegatorIt := s.currentSupernetDelegatorList.NewIterator()
	defer supernetDelegatorIt.Release()

	for _, delegatorIt := range []database.Iterator{delegatorIt, supernetDelegatorIt} {
		for delegatorIt.Next() {
			txIDBytes := delegatorIt.Key()
			txID, err := ids.ToID(txIDBytes)
			if err != nil {
				return err
			}
			tx, _, err := s.GetTx(txID)
			if err != nil {
				return err
			}

			potentialRewardBytes := delegatorIt.Value()
			potentialReward, err := database.ParseUInt64(potentialRewardBytes)
			if err != nil {
				return err
			}

			stakerTx, ok := tx.Unsigned.(txs.Staker)
			if !ok {
				return fmt.Errorf("expected tx type txs.Staker but got %T", tx.Unsigned)
			}

			staker, err := NewCurrentStaker(txID, stakerTx, potentialReward)
			if err != nil {
				return err
			}

			validator := s.currentStakers.getOrCreateValidator(staker.SupernetID, staker.NodeID)
			if validator.delegators == nil {
				validator.delegators = btree.NewG(defaultTreeDegree, (*Staker).Less)
			}
			validator.delegators.ReplaceOrInsert(staker)

			s.currentStakers.stakers.ReplaceOrInsert(staker)
		}
	}

	errs := wrappers.Errs{}
	errs.Add(
		validatorIt.Error(),
		supernetValidatorIt.Error(),
		delegatorIt.Error(),
		supernetDelegatorIt.Error(),
	)
	return errs.Err
}

func (s *state) loadPendingValidators() error {
	s.pendingStakers = newBaseStakers()

	validatorIt := s.pendingValidatorList.NewIterator()
	defer validatorIt.Release()

	supernetValidatorIt := s.pendingSupernetValidatorList.NewIterator()
	defer supernetValidatorIt.Release()

	for _, validatorIt := range []database.Iterator{validatorIt, supernetValidatorIt} {
		for validatorIt.Next() {
			txIDBytes := validatorIt.Key()
			txID, err := ids.ToID(txIDBytes)
			if err != nil {
				return err
			}
			tx, _, err := s.GetTx(txID)
			if err != nil {
				return err
			}

			stakerTx, ok := tx.Unsigned.(txs.Staker)
			if !ok {
				return fmt.Errorf("expected tx type txs.Staker but got %T", tx.Unsigned)
			}

			staker, err := NewPendingStaker(txID, stakerTx)
			if err != nil {
				return err
			}

			validator := s.pendingStakers.getOrCreateValidator(staker.SupernetID, staker.NodeID)
			validator.validator = staker

			s.pendingStakers.stakers.ReplaceOrInsert(staker)
		}
	}

	delegatorIt := s.pendingDelegatorList.NewIterator()
	defer delegatorIt.Release()

	supernetDelegatorIt := s.pendingSupernetDelegatorList.NewIterator()
	defer supernetDelegatorIt.Release()

	for _, delegatorIt := range []database.Iterator{delegatorIt, supernetDelegatorIt} {
		for delegatorIt.Next() {
			txIDBytes := delegatorIt.Key()
			txID, err := ids.ToID(txIDBytes)
			if err != nil {
				return err
			}
			tx, _, err := s.GetTx(txID)
			if err != nil {
				return err
			}

			stakerTx, ok := tx.Unsigned.(txs.Staker)
			if !ok {
				return fmt.Errorf("expected tx type txs.Staker but got %T", tx.Unsigned)
			}

			staker, err := NewPendingStaker(txID, stakerTx)
			if err != nil {
				return err
			}

			validator := s.pendingStakers.getOrCreateValidator(staker.SupernetID, staker.NodeID)
			if validator.delegators == nil {
				validator.delegators = btree.NewG(defaultTreeDegree, (*Staker).Less)
			}
			validator.delegators.ReplaceOrInsert(staker)

			s.pendingStakers.stakers.ReplaceOrInsert(staker)
		}
	}

	errs := wrappers.Errs{}
	errs.Add(
		validatorIt.Error(),
		supernetValidatorIt.Error(),
		delegatorIt.Error(),
		supernetDelegatorIt.Error(),
	)
	return errs.Err
}

// Invariant: initValidatorSets requires loadCurrentValidators to have already
// been called.
func (s *state) initValidatorSets() error {
	primaryValidators, ok := s.cfg.Validators.Get(constants.PrimaryNetworkID)
	if !ok {
		return errMissingValidatorSet
	}
	if primaryValidators.Len() != 0 {
		// Enforce the invariant that the validator set is empty here.
		return errValidatorSetAlreadyPopulated
	}
	err := s.ValidatorSet(constants.PrimaryNetworkID, primaryValidators)
	if err != nil {
		return err
	}

	vl := validators.NewLogger(s.ctx.Log, s.bootstrapped, constants.PrimaryNetworkID, s.ctx.NodeID)
	primaryValidators.RegisterCallbackListener(vl)

	s.metrics.SetLocalStake(primaryValidators.GetWeight(s.ctx.NodeID))
	s.metrics.SetTotalStake(primaryValidators.Weight())

	for supernetID := range s.cfg.TrackedSupernets {
		supernetValidators := validators.NewSet()
		err := s.ValidatorSet(supernetID, supernetValidators)
		if err != nil {
			return err
		}

		if !s.cfg.Validators.Add(supernetID, supernetValidators) {
			return fmt.Errorf("%w: %s", errDuplicateValidatorSet, supernetID)
		}

		vl := validators.NewLogger(s.ctx.Log, s.bootstrapped, supernetID, s.ctx.NodeID)
		supernetValidators.RegisterCallbackListener(vl)
	}
	return nil
}

func (s *state) write(updateValidators bool, height uint64) error {
	errs := wrappers.Errs{}
	errs.Add(
		s.writeBlocks(),
		s.writeCurrentStakers(updateValidators, height),
		s.writePendingStakers(),
		s.WriteValidatorMetadata(s.currentValidatorList, s.currentSupernetValidatorList), // Must be called after writeCurrentStakers
		s.writeTXs(),
		s.writeRewardUTXOs(),
		s.writeUTXOs(),
		s.writeSupernets(),
		s.writeTransformedSupernets(),
		s.writeSupernetSupplies(),
		s.writeSupernetRewardsSupplies(),
		s.writeChains(),
	)
	var metadataErr error
	// force update at genesis height
	if height == 0 {
		metadataErr = s.forceWriteMetadata()
	} else {
		metadataErr = s.writeMetadata()
	}
	errs.Add(metadataErr)
	return errs.Err
}

func (s *state) Close() error {
	errs := wrappers.Errs{}
	errs.Add(
		s.pendingSupernetValidatorBaseDB.Close(),
		s.pendingSupernetDelegatorBaseDB.Close(),
		s.pendingDelegatorBaseDB.Close(),
		s.pendingValidatorBaseDB.Close(),
		s.pendingValidatorsDB.Close(),
		s.currentSupernetValidatorBaseDB.Close(),
		s.currentSupernetDelegatorBaseDB.Close(),
		s.currentDelegatorBaseDB.Close(),
		s.currentValidatorBaseDB.Close(),
		s.currentValidatorsDB.Close(),
		s.validatorsDB.Close(),
		s.txDB.Close(),
		s.rewardUTXODB.Close(),
		s.utxoDB.Close(),
		s.supernetBaseDB.Close(),
		s.transformedSupernetDB.Close(),
		s.supplyDB.Close(),
		s.rewardsSupplyDB.Close(),
		s.chainDB.Close(),
		s.singletonDB.Close(),
		s.blockDB.Close(),
	)
	return errs.Err
}

func (s *state) sync(genesis []byte) error {
	shouldInit, err := s.shouldInit()
	if err != nil {
		return fmt.Errorf(
			"failed to check if the database is initialized: %w",
			err,
		)
	}

	// If the database is empty, create the platform chain anew using the
	// provided genesis state
	if shouldInit {
		if err := s.init(genesis); err != nil {
			return fmt.Errorf(
				"failed to initialize the database: %w",
				err,
			)
		}
	}

	if err := s.load(); err != nil {
		return fmt.Errorf(
			"failed to load the database state: %w",
			err,
		)
	}
	return nil
}

func (s *state) init(genesisBytes []byte) error {
	// Create the genesis block and save it as being accepted (We don't do
	// genesisBlock.Accept() because then it'd look for genesisBlock's
	// non-existent parent)
	genesisID := hashing.ComputeHash256Array(genesisBytes)
	genesisBlock, err := blocks.NewApricotCommitBlock(genesisID, 0 /*height*/)
	if err != nil {
		return err
	}

	genesisState, err := genesis.ParseState(genesisBytes)
	if err != nil {
		return err
	}
	if err := s.syncGenesis(genesisBlock, genesisState); err != nil {
		return err
	}

	if err := s.doneInit(); err != nil {
		return err
	}

	return s.Commit()
}

func (s *state) AddStatelessBlock(block blocks.Block, status choices.Status) {
	s.addedBlocks[block.ID()] = stateBlk{
		Blk:    block,
		Bytes:  block.Bytes(),
		Status: status,
	}
}

func (s *state) SetHeight(height uint64) {
	s.currentHeight = height
}

func (s *state) Commit() error {
	defer s.Abort()
	batch, err := s.CommitBatch()
	if err != nil {
		return err
	}
	return batch.Write()
}

func (s *state) Abort() {
	s.baseDB.Abort()
}

func (s *state) CommitBatch() (database.Batch, error) {
	// updateValidators is set to true here so that the validator manager is
	// kept up to date with the last accepted state.
	if err := s.write(true /*=updateValidators*/, s.currentHeight); err != nil {
		return nil, err
	}
	return s.baseDB.CommitBatch()
}

func (s *state) writeBlocks() error {
	for blkID, stateBlk := range s.addedBlocks {
		var (
			blkID = blkID
			stBlk = stateBlk
		)

		// Note: blocks to be stored are verified, so it's safe to marshal them with GenesisCodec
		blockBytes, err := blocks.GenesisCodec.Marshal(blocks.Version, &stBlk)
		if err != nil {
			return fmt.Errorf("failed to marshal block %s to store: %w", blkID, err)
		}

		delete(s.addedBlocks, blkID)
		s.blockCache.Put(blkID, &stBlk)
		if err := s.blockDB.Put(blkID[:], blockBytes); err != nil {
			return fmt.Errorf("failed to write block %s: %w", blkID, err)
		}
	}
	return nil
}

func (s *state) GetStatelessBlock(blockID ids.ID) (blocks.Block, choices.Status, error) {
	if blk, ok := s.addedBlocks[blockID]; ok {
		return blk.Blk, blk.Status, nil
	}
	if blkState, ok := s.blockCache.Get(blockID); ok {
		if blkState == nil {
			return nil, choices.Processing, database.ErrNotFound
		}
		return blkState.Blk, blkState.Status, nil
	}

	blkBytes, err := s.blockDB.Get(blockID[:])
	if err == database.ErrNotFound {
		s.blockCache.Put(blockID, nil)
		return nil, choices.Processing, database.ErrNotFound // status does not matter here
	} else if err != nil {
		return nil, choices.Processing, err // status does not matter here
	}

	// Note: stored blocks are verified, so it's safe to unmarshal them with GenesisCodec
	blkState := stateBlk{}
	if _, err := blocks.GenesisCodec.Unmarshal(blkBytes, &blkState); err != nil {
		return nil, choices.Processing, err // status does not matter here
	}

	blkState.Blk, err = blocks.Parse(blocks.GenesisCodec, blkState.Bytes)
	if err != nil {
		return nil, choices.Processing, err
	}

	s.blockCache.Put(blockID, &blkState)
	return blkState.Blk, blkState.Status, nil
}

func (s *state) writeCurrentStakers(updateValidators bool, height uint64) error {
	heightBytes := database.PackUInt64(height)
	rawPublicKeyDiffDB := prefixdb.New(heightBytes, s.validatorPublicKeyDiffsDB)
	pkDiffDB := linkeddb.NewDefault(rawPublicKeyDiffDB)
	// Node ID --> BLS public key of node before it left the validator set.
	pkDiffs := make(map[ids.NodeID]*bls.PublicKey)

	for supernetID, validatorDiffs := range s.currentStakers.validatorDiffs {
		delete(s.currentStakers.validatorDiffs, supernetID)

		// Select db to write to
		validatorDB := s.currentSupernetValidatorList
		delegatorDB := s.currentSupernetDelegatorList
		if supernetID == constants.PrimaryNetworkID {
			validatorDB = s.currentValidatorList
			delegatorDB = s.currentDelegatorList
		}

		prefixStruct := heightWithSupernet{
			Height:     height,
			SupernetID: supernetID,
		}
		prefixBytes, err := blocks.GenesisCodec.Marshal(blocks.Version, prefixStruct)
		if err != nil {
			return fmt.Errorf("failed to create prefix bytes: %w", err)
		}
		rawWeightDiffDB := prefixdb.New(prefixBytes, s.validatorWeightDiffsDB)
		weightDiffDB := linkeddb.NewDefault(rawWeightDiffDB)
		weightDiffs := make(map[ids.NodeID]*ValidatorWeightDiff)

		// Record the change in weight and/or public key for each validator.
		for nodeID, validatorDiff := range validatorDiffs {
			// Copy [nodeID] so it doesn't get overwritten next iteration.
			nodeID := nodeID

			weightDiff := &ValidatorWeightDiff{
				Decrease: validatorDiff.validatorStatus == deleted,
			}
			switch validatorDiff.validatorStatus {
			case added:
				staker := validatorDiff.validator
				weightDiff.Amount = staker.Weight

				// The validator is being added.
				//
				// Invariant: It's impossible for a delegator to have been
				// rewarded in the same block that the validator was added.
				metadata := &validatorMetadata{
					txID:        staker.TxID,
					lastUpdated: staker.StartTime,

					UpDuration:               0,
					LastUpdated:              uint64(staker.StartTime.Unix()),
					PotentialReward:          staker.PotentialReward,
					PotentialDelegateeReward: 0,
				}

				metadataBytes, err := blocks.GenesisCodec.Marshal(blocks.Version, metadata)
				if err != nil {
					return fmt.Errorf("failed to serialize current validator: %w", err)
				}

				if err = validatorDB.Put(staker.TxID[:], metadataBytes); err != nil {
					return fmt.Errorf("failed to write current validator to list: %w", err)
				}

				s.validatorState.LoadValidatorMetadata(nodeID, supernetID, metadata)
			case deleted:
				staker := validatorDiff.validator
				weightDiff.Amount = staker.Weight

				// Invariant: Only the Primary Network contains non-nil
				//            public keys.
				if staker.PublicKey != nil {
					// Record the public key of the validator being removed.
					pkDiffs[nodeID] = staker.PublicKey

					pkBytes := bls.PublicKeyToBytes(staker.PublicKey)
					if err := pkDiffDB.Put(nodeID[:], pkBytes); err != nil {
						return err
					}
				}

				if err := validatorDB.Delete(staker.TxID[:]); err != nil {
					return fmt.Errorf("failed to delete current staker: %w", err)
				}

				s.validatorState.DeleteValidatorMetadata(nodeID, supernetID)
			}

			err := writeCurrentDelegatorDiff(
				delegatorDB,
				weightDiff,
				validatorDiff,
			)
			if err != nil {
				return err
			}

			if weightDiff.Amount == 0 {
				// No weight change to record; go to next validator.
				continue
			}
			weightDiffs[nodeID] = weightDiff

			weightDiffBytes, err := blocks.GenesisCodec.Marshal(blocks.Version, weightDiff)
			if err != nil {
				return fmt.Errorf("failed to serialize validator weight diff: %w", err)
			}

			if err := weightDiffDB.Put(nodeID[:], weightDiffBytes); err != nil {
				return err
			}

			// TODO: Move the validator set management out of the state package
			if !updateValidators {
				continue
			}

			// We only track the current validator set of tracked supernets.
			if supernetID != constants.PrimaryNetworkID && !s.cfg.TrackedSupernets.Contains(supernetID) {
				continue
			}

			if weightDiff.Decrease {
				err = validators.RemoveWeight(s.cfg.Validators, supernetID, nodeID, weightDiff.Amount)
			} else {
				if validatorDiff.validatorStatus == added {
					staker := validatorDiff.validator
					err = validators.Add(
						s.cfg.Validators,
						supernetID,
						nodeID,
						staker.PublicKey,
						staker.TxID,
						weightDiff.Amount,
					)
				} else {
					err = validators.AddWeight(s.cfg.Validators, supernetID, nodeID, weightDiff.Amount)
				}
			}
			if err != nil {
				return fmt.Errorf("failed to update validator weight: %w", err)
			}
		}
		s.validatorWeightDiffsCache.Put(string(prefixBytes), weightDiffs)
	}
	s.validatorPublicKeyDiffsCache.Put(height, pkDiffs)

	// TODO: Move validator set management out of the state package
	//
	// Attempt to update the stake metrics
	if !updateValidators {
		return nil
	}
	primaryValidators, ok := s.cfg.Validators.Get(constants.PrimaryNetworkID)
	if !ok {
		return nil
	}
	s.metrics.SetLocalStake(primaryValidators.GetWeight(s.ctx.NodeID))
	s.metrics.SetTotalStake(primaryValidators.Weight())
	return nil
}

func writeCurrentDelegatorDiff(
	currentDelegatorList linkeddb.LinkedDB,
	weightDiff *ValidatorWeightDiff,
	validatorDiff *diffValidator,
) error {
	addedDelegatorIterator := NewTreeIterator(validatorDiff.addedDelegators)
	defer addedDelegatorIterator.Release()
	for addedDelegatorIterator.Next() {
		staker := addedDelegatorIterator.Value()

		if err := weightDiff.Add(false, staker.Weight); err != nil {
			return fmt.Errorf("failed to increase node weight diff: %w", err)
		}

		if err := database.PutUInt64(currentDelegatorList, staker.TxID[:], staker.PotentialReward); err != nil {
			return fmt.Errorf("failed to write current delegator to list: %w", err)
		}
	}

	for _, staker := range validatorDiff.deletedDelegators {
		if err := weightDiff.Add(true, staker.Weight); err != nil {
			return fmt.Errorf("failed to decrease node weight diff: %w", err)
		}

		if err := currentDelegatorList.Delete(staker.TxID[:]); err != nil {
			return fmt.Errorf("failed to delete current staker: %w", err)
		}
	}
	return nil
}

func (s *state) writePendingStakers() error {
	for supernetID, supernetValidatorDiffs := range s.pendingStakers.validatorDiffs {
		delete(s.pendingStakers.validatorDiffs, supernetID)

		validatorDB := s.pendingSupernetValidatorList
		delegatorDB := s.pendingSupernetDelegatorList
		if supernetID == constants.PrimaryNetworkID {
			validatorDB = s.pendingValidatorList
			delegatorDB = s.pendingDelegatorList
		}

		for _, validatorDiff := range supernetValidatorDiffs {
			err := writePendingDiff(
				validatorDB,
				delegatorDB,
				validatorDiff,
			)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func writePendingDiff(
	pendingValidatorList linkeddb.LinkedDB,
	pendingDelegatorList linkeddb.LinkedDB,
	validatorDiff *diffValidator,
) error {
	switch validatorDiff.validatorStatus {
	case added:
		err := pendingValidatorList.Put(validatorDiff.validator.TxID[:], nil)
		if err != nil {
			return fmt.Errorf("failed to add pending validator: %w", err)
		}
	case deleted:
		err := pendingValidatorList.Delete(validatorDiff.validator.TxID[:])
		if err != nil {
			return fmt.Errorf("failed to delete pending validator: %w", err)
		}
	}

	addedDelegatorIterator := NewTreeIterator(validatorDiff.addedDelegators)
	defer addedDelegatorIterator.Release()
	for addedDelegatorIterator.Next() {
		staker := addedDelegatorIterator.Value()

		if err := pendingDelegatorList.Put(staker.TxID[:], nil); err != nil {
			return fmt.Errorf("failed to write pending delegator to list: %w", err)
		}
	}

	for _, staker := range validatorDiff.deletedDelegators {
		if err := pendingDelegatorList.Delete(staker.TxID[:]); err != nil {
			return fmt.Errorf("failed to delete pending delegator: %w", err)
		}
	}
	return nil
}

func (s *state) writeTXs() error {
	for txID, txStatus := range s.addedTxs {
		txID := txID

		stx := txBytesAndStatus{
			Tx:     txStatus.tx.Bytes(),
			Status: txStatus.status,
		}

		// Note that we're serializing a [txBytesAndStatus] here, not a
		// *txs.Tx, so we don't use [txs.Codec].
		txBytes, err := txs.GenesisCodec.Marshal(txs.Version, &stx)
		if err != nil {
			return fmt.Errorf("failed to serialize tx: %w", err)
		}

		delete(s.addedTxs, txID)
		s.txCache.Put(txID, txStatus)
		if err := s.txDB.Put(txID[:], txBytes); err != nil {
			return fmt.Errorf("failed to add tx: %w", err)
		}
	}
	return nil
}

func (s *state) writeRewardUTXOs() error {
	for txID, utxos := range s.addedRewardUTXOs {
		delete(s.addedRewardUTXOs, txID)
		s.rewardUTXOsCache.Put(txID, utxos)
		rawTxDB := prefixdb.New(txID[:], s.rewardUTXODB)
		txDB := linkeddb.NewDefault(rawTxDB)

		for _, utxo := range utxos {
			utxoBytes, err := txs.GenesisCodec.Marshal(txs.Version, utxo)
			if err != nil {
				return fmt.Errorf("failed to serialize reward UTXO: %w", err)
			}
			utxoID := utxo.InputID()
			if err := txDB.Put(utxoID[:], utxoBytes); err != nil {
				return fmt.Errorf("failed to add reward UTXO: %w", err)
			}
		}
	}
	return nil
}

func (s *state) writeUTXOs() error {
	for utxoID, utxo := range s.modifiedUTXOs {
		delete(s.modifiedUTXOs, utxoID)

		if utxo == nil {
			if err := s.utxoState.DeleteUTXO(utxoID); err != nil {
				return fmt.Errorf("failed to delete UTXO: %w", err)
			}
			continue
		}
		if err := s.utxoState.PutUTXO(utxo); err != nil {
			return fmt.Errorf("failed to add UTXO: %w", err)
		}
	}
	return nil
}

func (s *state) writeSupernets() error {
	for _, supernet := range s.addedSupernets {
		supernetID := supernet.ID()

		if err := s.supernetDB.Put(supernetID[:], nil); err != nil {
			return fmt.Errorf("failed to write supernet: %w", err)
		}
	}
	s.addedSupernets = nil
	return nil
}

func (s *state) writeTransformedSupernets() error {
	for supernetID, tx := range s.transformedSupernets {
		txID := tx.ID()

		delete(s.transformedSupernets, supernetID)
		s.transformedSupernetCache.Put(supernetID, tx)
		if err := database.PutID(s.transformedSupernetDB, supernetID[:], txID); err != nil {
			return fmt.Errorf("failed to write transformed supernet: %w", err)
		}
	}
	return nil
}

func (s *state) writeSupernetSupplies() error {
	for supernetID, supply := range s.modifiedSupplies {
		supply := supply
		delete(s.modifiedSupplies, supernetID)
		s.supplyCache.Put(supernetID, &supply)
		if err := database.PutUInt64(s.supplyDB, supernetID[:], supply); err != nil {
			return fmt.Errorf("failed to write supernet supply: %w", err)
		}
	}
	return nil
}

func (s *state) writeSupernetRewardsSupplies() error {
	for supernetID, rewardsSupply := range s.modifiedRewardsSupplies {
		rewardsSupply := rewardsSupply
		delete(s.modifiedRewardsSupplies, supernetID)
		s.supplyCache.Put(supernetID, &rewardsSupply)
		if err := database.PutUInt64(s.rewardsSupplyDB, supernetID[:], rewardsSupply); err != nil {
			return fmt.Errorf("failed to write supernet rewards supply: %w", err)
		}
	}
	return nil
}

func (s *state) writeChains() error {
	for supernetID, chains := range s.addedChains {
		for _, chain := range chains {
			chainDB := s.getChainDB(supernetID)

			chainID := chain.ID()
			if err := chainDB.Put(chainID[:], nil); err != nil {
				return fmt.Errorf("failed to write chain: %w", err)
			}
		}
		delete(s.addedChains, supernetID)
	}
	return nil
}

func (s *state) writeMetadata() error {
	if !s.persistedTimestamp.Equal(s.timestamp) {
		if err := database.PutTimestamp(s.singletonDB, timestampKey, s.timestamp); err != nil {
			return fmt.Errorf("failed to write timestamp: %w", err)
		}
		s.persistedTimestamp = s.timestamp
	}
	if s.persistedCurrentSupply != s.currentSupply {
		if err := database.PutUInt64(s.singletonDB, currentSupplyKey, s.currentSupply); err != nil {
			return fmt.Errorf("failed to write current supply: %w", err)
		}
		s.persistedCurrentSupply = s.currentSupply
	}
	if s.persistedRewardsPoolSupply != s.rewardsPoolSupply {
		if err := database.PutUInt64(s.singletonDB, rewardsPoolSupplyKey, s.rewardsPoolSupply); err != nil {
			return fmt.Errorf("failed to write rewards pool supply: %w", err)
		}
		s.persistedRewardsPoolSupply = s.rewardsPoolSupply
	}
	if s.persistedFeesPoolValue != s.feesPoolValue {
		if err := database.PutUInt64(s.singletonDB, feesPoolValueKey, s.feesPoolValue); err != nil {
			return fmt.Errorf("failed to write fees pool value: %w", err)
		}
		s.persistedFeesPoolValue = s.feesPoolValue
	}
	if s.persistedLastAccepted != s.lastAccepted {
		if err := database.PutID(s.singletonDB, lastAcceptedKey, s.lastAccepted); err != nil {
			return fmt.Errorf("failed to write last accepted: %w", err)
		}
		s.persistedLastAccepted = s.lastAccepted
	}
	return nil
}

func (s *state) forceWriteMetadata() error {
	if err := database.PutTimestamp(s.singletonDB, timestampKey, s.timestamp); err != nil {
		return fmt.Errorf("failed to force write timestamp: %w", err)
	}
	s.persistedTimestamp = s.timestamp
	if err := database.PutUInt64(s.singletonDB, currentSupplyKey, s.currentSupply); err != nil {
		return fmt.Errorf("failed to force write current supply: %w", err)
	}
	s.persistedCurrentSupply = s.currentSupply
	if err := database.PutUInt64(s.singletonDB, rewardsPoolSupplyKey, s.rewardsPoolSupply); err != nil {
		return fmt.Errorf("failed to force write rewards pool supply: %w", err)
	}
	s.persistedRewardsPoolSupply = s.rewardsPoolSupply
	if err := database.PutUInt64(s.singletonDB, feesPoolValueKey, s.feesPoolValue); err != nil {
		return fmt.Errorf("failed to write fees pool value: %w", err)
	}
	s.persistedFeesPoolValue = s.feesPoolValue
	if err := database.PutID(s.singletonDB, lastAcceptedKey, s.lastAccepted); err != nil {
		return fmt.Errorf("failed to force write last accepted: %w", err)
	}
	s.persistedLastAccepted = s.lastAccepted
	return nil
}
