// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package snow

import (
	"crypto"
	"crypto/x509"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/Juneo-io/juneogo/api/keystore"
	"github.com/Juneo-io/juneogo/api/metrics"
	"github.com/Juneo-io/juneogo/chains/atomic"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow/validators"
	"github.com/Juneo-io/juneogo/utils"
	"github.com/Juneo-io/juneogo/utils/logging"
	"github.com/Juneo-io/juneogo/vms/relayvm/teleporter"
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
	NetworkID  uint32
	SupernetID ids.ID
	ChainID    ids.ID
	NodeID     ids.NodeID

	AssetChainID ids.ID
	JuneChainID  ids.ID
	JuneAssetID  ids.ID
	ChainAssetID ids.ID

	Log          logging.Logger
	Lock         sync.RWMutex
	Keystore     keystore.BlockchainKeystore
	SharedMemory atomic.SharedMemory
	BCLookup     ids.AliaserReader
	Metrics      metrics.OptionalGatherer

	TeleporterSigner teleporter.Signer

	// snowman++ attributes
	ValidatorState    validators.State  // interface for P-Chain validators
	StakingLeafSigner crypto.Signer     // block signer
	StakingCertLeaf   *x509.Certificate // block certificate

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

	Registerer Registerer

	// DecisionAcceptor is the callback that will be fired whenever a VM is
	// notified that their object, either a block in snowman or a transaction
	// in avalanche, was accepted.
	DecisionAcceptor Acceptor

	// ConsensusAcceptor is the callback that will be fired whenever a
	// container, either a block in snowman or a vertex in avalanche, was
	// accepted.
	ConsensusAcceptor Acceptor

	// Non-zero iff this chain bootstrapped.
	state utils.AtomicInterface

	// Non-zero iff this chain is executing transactions.
	executing utils.AtomicBool

	// Indicates this chain is available to only validators.
	validatorOnly utils.AtomicBool
}

func (ctx *ConsensusContext) SetState(newState State) {
	ctx.state.SetValue(newState)
}

func (ctx *ConsensusContext) GetState() State {
	stateInf := ctx.state.GetValue()
	return stateInf.(State)
}

// IsExecuting returns true iff this chain is still executing transactions.
func (ctx *ConsensusContext) IsExecuting() bool {
	return ctx.executing.GetValue()
}

// Executing marks this chain as executing or not.
// Set to "true" if there's an ongoing transaction.
func (ctx *ConsensusContext) Executing(b bool) {
	ctx.executing.SetValue(b)
}

// IsValidatorOnly returns true iff this chain is available only to validators
func (ctx *ConsensusContext) IsValidatorOnly() bool {
	return ctx.validatorOnly.GetValue()
}

// SetValidatorOnly  marks this chain as available only to validators
func (ctx *ConsensusContext) SetValidatorOnly() {
	ctx.validatorOnly.SetValue(true)
}

func DefaultContextTest() *Context {
	return &Context{
		NetworkID:    0,
		SupernetID:   ids.Empty,
		ChainID:      ids.Empty,
		NodeID:       ids.EmptyNodeID,
		Log:          logging.NoLog{},
		BCLookup:     ids.NewAliaser(),
		Metrics:      metrics.NewOptionalGatherer(),
		ChainDataDir: "",
	}
}

func DefaultConsensusContextTest() *ConsensusContext {
	return &ConsensusContext{
		Context:           DefaultContextTest(),
		Registerer:        prometheus.NewRegistry(),
		DecisionAcceptor:  noOpAcceptor{},
		ConsensusAcceptor: noOpAcceptor{},
	}
}
