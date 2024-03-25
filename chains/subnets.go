// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chains

import (
	"errors"
	"sync"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/supernets"
	"github.com/Juneo-io/juneogo/utils/constants"
)

var ErrNoPrimaryNetworkConfig = errors.New("no supernet config for primary network found")

// Supernets holds the currently running supernets on this node
type Supernets struct {
	nodeID  ids.NodeID
	configs map[ids.ID]supernets.Config

	lock    sync.RWMutex
	supernets map[ids.ID]supernets.Supernet
}

// GetOrCreate returns a supernet running on this node, or creates one if it was
// not running before. Returns the supernet and if the supernet was created.
func (s *Supernets) GetOrCreate(supernetID ids.ID) (supernets.Supernet, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if supernet, ok := s.supernets[supernetID]; ok {
		return supernet, false
	}

	// Default to the primary network config if a supernet config was not
	// specified
	config, ok := s.configs[supernetID]
	if !ok {
		config = s.configs[constants.PrimaryNetworkID]
	}

	supernet := supernets.New(s.nodeID, config)
	s.supernets[supernetID] = supernet

	return supernet, true
}

// Bootstrapping returns the supernetIDs of any chains that are still
// bootstrapping.
func (s *Supernets) Bootstrapping() []ids.ID {
	s.lock.RLock()
	defer s.lock.RUnlock()

	supernetsBootstrapping := make([]ids.ID, 0, len(s.supernets))
	for supernetID, supernet := range s.supernets {
		if !supernet.IsBootstrapped() {
			supernetsBootstrapping = append(supernetsBootstrapping, supernetID)
		}
	}

	return supernetsBootstrapping
}

// NewSupernets returns an instance of Supernets
func NewSupernets(
	nodeID ids.NodeID,
	configs map[ids.ID]supernets.Config,
) (*Supernets, error) {
	if _, ok := configs[constants.PrimaryNetworkID]; !ok {
		return nil, ErrNoPrimaryNetworkConfig
	}

	s := &Supernets{
		nodeID:  nodeID,
		configs: configs,
		supernets: make(map[ids.ID]supernets.Supernet),
	}

	_, _ = s.GetOrCreate(constants.PrimaryNetworkID)
	return s, nil
}
