// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/Juneo-io/juneogo/api/keystore"
	"github.com/Juneo-io/juneogo/api/metrics"
	"github.com/Juneo-io/juneogo/chains/atomic"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow/validators"
	"github.com/Juneo-io/juneogo/utils"
	"github.com/Juneo-io/juneogo/utils/crypto/bls"
	"github.com/Juneo-io/juneogo/utils/logging"
	"github.com/Juneo-io/juneogo/vms/platformvm/warp"
)

// ContextInitializable represents an object that can be initialized
// given a *Context object
type ContextInitializable interface {
	// InitCtx initializes an object provided a *Context object
	InitCtx(ctx *Context)
}

// Context is information about the current execution.
// [NetworkID] is the ID of the network this context exists within.
// [ChainID] is the ID of the chain this context exists within.
// [NodeID] is the ID of this node
type Context struct {
	NetworkID uint32
	SupernetID  ids.ID
	ChainID   ids.ID
	NodeID    ids.NodeID
	PublicKey *bls.PublicKey

	JVMChainID     ids.ID
	JUNEChainID     ids.ID
	JUNEAssetID  ids.ID
	ChainAssetID ids.ID

	Log          logging.Logger
	Lock         sync.RWMutex
	Keystore     keystore.BlockchainKeystore
	SharedMemory atomic.SharedMemory
	BCLookup     ids.AliaserReader
	Metrics      metrics.OptionalGatherer

	WarpSigner warp.Signer

	// snowman++ attributes
	ValidatorState validators.State // interface for P-Chain validators
	// Chain-specific directory where arbitrary data can be written
	ChainDataDir string
}

// Expose gatherer interface for unit testing.
type Registerer interface {
	prometheus.Registerer
	prometheus.Gatherer
}

type ConsensusContext struct {
	*Context

	// Registers all common and snowman consensus metrics. Unlike the avalanche
	// consensus engine metrics, we do not prefix the name with the engine name,
	// as snowman is used for all chains by default.
	Registerer Registerer
	// Only used to register Avalanche consensus metrics. Previously, all
	// metrics were prefixed with "avalanche_{chainID}_". Now we add avalanche
	// to the prefix, "avalanche_{chainID}_avalanche_", to differentiate
	// consensus operations after the DAG linearization.
	AvalancheRegisterer Registerer

	// BlockAcceptor is the callback that will be fired whenever a VM is
	// notified that their block was accepted.
	BlockAcceptor Acceptor

	// TxAcceptor is the callback that will be fired whenever a VM is notified
	// that their transaction was accepted.
	TxAcceptor Acceptor

	// VertexAcceptor is the callback that will be fired whenever a vertex was
	// accepted.
	VertexAcceptor Acceptor

	// State indicates the current state of this consensus instance.
	State utils.Atomic[EngineState]

	// True iff this chain is executing transactions as part of bootstrapping.
	Executing utils.Atomic[bool]

	// True iff this chain is currently state-syncing
	StateSyncing utils.Atomic[bool]
}
