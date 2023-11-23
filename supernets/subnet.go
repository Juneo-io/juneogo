// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package supernets

import (
	"sync"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow/engine/common"
	"github.com/Juneo-io/juneogo/utils/set"
)

var _ Supernet = (*supernet)(nil)

type Allower interface {
	// IsAllowed filters out nodes that are not allowed to connect to this supernet
	IsAllowed(nodeID ids.NodeID, isValidator bool) bool
}

// Supernet keeps track of the currently bootstrapping chains in a supernet. If no
// chains in the supernet are currently bootstrapping, the supernet is considered
// bootstrapped.
type Supernet interface {
	common.BootstrapTracker

	// AddChain adds a chain to this Supernet
	AddChain(chainID ids.ID) bool

	// Config returns config of this Supernet
	Config() Config

	Allower
}

type supernet struct {
	lock             sync.RWMutex
	bootstrapping    set.Set[ids.ID]
	bootstrapped     set.Set[ids.ID]
	once             sync.Once
	bootstrappedSema chan struct{}
	config           Config
	myNodeID         ids.NodeID
}

func New(myNodeID ids.NodeID, config Config) Supernet {
	return &supernet{
		bootstrappedSema: make(chan struct{}),
		config:           config,
		myNodeID:         myNodeID,
	}
}

func (s *supernet) IsBootstrapped() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.bootstrapping.Len() == 0
}

func (s *supernet) Bootstrapped(chainID ids.ID) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.bootstrapping.Remove(chainID)
	s.bootstrapped.Add(chainID)
	if s.bootstrapping.Len() > 0 {
		return
	}

	s.once.Do(func() {
		close(s.bootstrappedSema)
	})
}

func (s *supernet) OnBootstrapCompleted() chan struct{} {
	return s.bootstrappedSema
}

func (s *supernet) AddChain(chainID ids.ID) bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.bootstrapping.Contains(chainID) || s.bootstrapped.Contains(chainID) {
		return false
	}

	s.bootstrapping.Add(chainID)
	return true
}

func (s *supernet) Config() Config {
	return s.config
}

func (s *supernet) IsAllowed(nodeID ids.NodeID, isValidator bool) bool {
	// Case 1: NodeID is this node
	// Case 2: This supernet is not validator-only supernet
	// Case 3: NodeID is a validator for this chain
	// Case 4: NodeID is explicitly allowed whether it's supernet validator or not
	return nodeID == s.myNodeID ||
		!s.config.ValidatorOnly ||
		isValidator ||
		s.config.AllowedNodes.Contains(nodeID)
}
