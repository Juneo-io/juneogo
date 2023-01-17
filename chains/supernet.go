// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chains

import (
	"sync"
	"time"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow/consensus/avalanche"
	"github.com/Juneo-io/juneogo/snow/engine/common"
	"github.com/Juneo-io/juneogo/snow/networking/sender"
	"github.com/Juneo-io/juneogo/utils/set"
)

var _ Supernet = (*supernet)(nil)

// Supernet keeps track of the currently bootstrapping chains in a supernet. If no
// chains in the supernet are currently bootstrapping, the supernet is considered
// bootstrapped.
type Supernet interface {
	common.Supernet

	afterBootstrapped() chan struct{}

	addChain(chainID ids.ID) bool
}

type SupernetConfig struct {
	sender.GossipConfig

	// ValidatorOnly indicates that this Supernet's Chains are available to only supernet validators.
	ValidatorOnly       bool                 `json:"validatorOnly" yaml:"validatorOnly"`
	ConsensusParameters avalanche.Parameters `json:"consensusParameters" yaml:"consensusParameters"`

	// ProposerMinBlockDelay is the minimum delay this node will enforce when
	// building a snowman++ block.
	// TODO: Remove this flag once all VMs throttle their own block production.
	ProposerMinBlockDelay time.Duration `json:"proposerMinBlockDelay" yaml:"proposerMinBlockDelay"`
}

type supernet struct {
	lock             sync.RWMutex
	bootstrapping    set.Set[ids.ID]
	bootstrapped     set.Set[ids.ID]
	once             sync.Once
	bootstrappedSema chan struct{}
}

func newSupernet() Supernet {
	return &supernet{
		bootstrappedSema: make(chan struct{}),
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

func (s *supernet) afterBootstrapped() chan struct{} {
	return s.bootstrappedSema
}

func (s *supernet) addChain(chainID ids.ID) bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.bootstrapping.Contains(chainID) || s.bootstrapped.Contains(chainID) {
		return false
	}

	s.bootstrapping.Add(chainID)
	return true
}
