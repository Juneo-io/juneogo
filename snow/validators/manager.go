// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validators

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/exp/maps"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils"
	"github.com/Juneo-io/juneogo/utils/crypto/bls"
	"github.com/Juneo-io/juneogo/utils/set"
)

var (
	_ Manager = (*manager)(nil)

	ErrZeroWeight        = errors.New("weight must be non-zero")
	ErrMissingValidators = errors.New("missing validators")
)

type SetCallbackListener interface {
	OnValidatorAdded(nodeID ids.NodeID, pk *bls.PublicKey, txID ids.ID, weight uint64)
	OnValidatorRemoved(nodeID ids.NodeID, weight uint64)
	OnValidatorWeightChanged(nodeID ids.NodeID, oldWeight, newWeight uint64)
}

// Manager holds the validator set of each supernet
type Manager interface {
	fmt.Stringer

	// Add a new staker to the supernet.
	// Returns an error if:
	// - [weight] is 0
	// - [nodeID] is already in the validator set
	// If an error is returned, the set will be unmodified.
	AddStaker(supernetID ids.ID, nodeID ids.NodeID, pk *bls.PublicKey, txID ids.ID, weight uint64) error

	// AddWeight to an existing staker to the supernet.
	// Returns an error if:
	// - [weight] is 0
	// - [nodeID] is not already in the validator set
	// If an error is returned, the set will be unmodified.
	// AddWeight can result in a total weight that overflows uint64.
	// In this case no error will be returned for this call.
	// However, the next TotalWeight call will return an error.
	AddWeight(supernetID ids.ID, nodeID ids.NodeID, weight uint64) error

	// GetWeight retrieves the validator weight from the supernet.
	GetWeight(supernetID ids.ID, nodeID ids.NodeID) uint64

	// GetValidator returns the validator tied to the specified ID in supernet.
	// If the validator doesn't exist, returns false.
	GetValidator(supernetID ids.ID, nodeID ids.NodeID) (*Validator, bool)

	// GetValidatoIDs returns the validator IDs in the supernet.
	GetValidatorIDs(supernetID ids.ID) []ids.NodeID

	// SubsetWeight returns the sum of the weights of the validators in the supernet.
	// Returns err if subset weight overflows uint64.
	SubsetWeight(supernetID ids.ID, validatorIDs set.Set[ids.NodeID]) (uint64, error)

	// RemoveWeight from a staker in the supernet. If the staker's weight becomes 0, the staker
	// will be removed from the supernet set.
	// Returns an error if:
	// - [weight] is 0
	// - [nodeID] is not already in the supernet set
	// - the weight of the validator would become negative
	// If an error is returned, the set will be unmodified.
	RemoveWeight(supernetID ids.ID, nodeID ids.NodeID, weight uint64) error

	// Count returns the number of validators currently in the supernet.
	Count(supernetID ids.ID) int

	// TotalWeight returns the cumulative weight of all validators in the supernet.
	// Returns err if total weight overflows uint64.
	TotalWeight(supernetID ids.ID) (uint64, error)

	// Sample returns a collection of validatorIDs in the supernet, potentially with duplicates.
	// If sampling the requested size isn't possible, an error will be returned.
	Sample(supernetID ids.ID, size int) ([]ids.NodeID, error)

	// Map of the validators in this supernet
	GetMap(supernetID ids.ID) map[ids.NodeID]*GetValidatorOutput

	// When a validator's weight changes, or a validator is added/removed,
	// this listener is called.
	RegisterCallbackListener(supernetID ids.ID, listener SetCallbackListener)
}

// NewManager returns a new, empty manager
func NewManager() Manager {
	return &manager{
		supernetToVdrs: make(map[ids.ID]*vdrSet),
	}
}

type manager struct {
	lock sync.RWMutex

	// Key: Supernet ID
	// Value: The validators that validate the supernet
	supernetToVdrs map[ids.ID]*vdrSet
}

func (m *manager) AddStaker(supernetID ids.ID, nodeID ids.NodeID, pk *bls.PublicKey, txID ids.ID, weight uint64) error {
	if weight == 0 {
		return ErrZeroWeight
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	set, exists := m.supernetToVdrs[supernetID]
	if !exists {
		set = newSet()
		m.supernetToVdrs[supernetID] = set
	}

	return set.Add(nodeID, pk, txID, weight)
}

func (m *manager) AddWeight(supernetID ids.ID, nodeID ids.NodeID, weight uint64) error {
	if weight == 0 {
		return ErrZeroWeight
	}

	// We do not need to grab a write lock here because we never modify the
	// supernetToVdrs map. However, we must hold the read lock during the entirity
	// of this function to ensure that errors are returned consistently.
	//
	// Consider the case that:
	//	AddStaker(supernetID, nodeID, 1)
	//	go func() {
	//		AddWeight(supernetID, nodeID, 1)
	//	}
	//	go func() {
	//		RemoveWeight(supernetID, nodeID, 1)
	//	}
	//
	// In this case, after both goroutines have finished, either AddWeight
	// should have errored, or the weight of the node should equal 1. It would
	// be unexpected to not have received an error from AddWeight but for the
	// node to no longer be tracked as a validator.
	m.lock.RLock()
	defer m.lock.RUnlock()

	set, exists := m.supernetToVdrs[supernetID]
	if !exists {
		return errMissingValidator
	}

	return set.AddWeight(nodeID, weight)
}

func (m *manager) GetWeight(supernetID ids.ID, nodeID ids.NodeID) uint64 {
	m.lock.RLock()
	set, exists := m.supernetToVdrs[supernetID]
	m.lock.RUnlock()
	if !exists {
		return 0
	}

	return set.GetWeight(nodeID)
}

func (m *manager) GetValidator(supernetID ids.ID, nodeID ids.NodeID) (*Validator, bool) {
	m.lock.RLock()
	set, exists := m.supernetToVdrs[supernetID]
	m.lock.RUnlock()
	if !exists {
		return nil, false
	}

	return set.Get(nodeID)
}

func (m *manager) SubsetWeight(supernetID ids.ID, validatorIDs set.Set[ids.NodeID]) (uint64, error) {
	m.lock.RLock()
	set, exists := m.supernetToVdrs[supernetID]
	m.lock.RUnlock()
	if !exists {
		return 0, nil
	}

	return set.SubsetWeight(validatorIDs)
}

func (m *manager) RemoveWeight(supernetID ids.ID, nodeID ids.NodeID, weight uint64) error {
	if weight == 0 {
		return ErrZeroWeight
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	set, exists := m.supernetToVdrs[supernetID]
	if !exists {
		return errMissingValidator
	}

	if err := set.RemoveWeight(nodeID, weight); err != nil {
		return err
	}
	// If this was the last validator in the supernet and no callback listeners
	// are registered, remove the supernet
	if set.Len() == 0 && !set.HasCallbackRegistered() {
		delete(m.supernetToVdrs, supernetID)
	}

	return nil
}

func (m *manager) Count(supernetID ids.ID) int {
	m.lock.RLock()
	set, exists := m.supernetToVdrs[supernetID]
	m.lock.RUnlock()
	if !exists {
		return 0
	}

	return set.Len()
}

func (m *manager) TotalWeight(supernetID ids.ID) (uint64, error) {
	m.lock.RLock()
	set, exists := m.supernetToVdrs[supernetID]
	m.lock.RUnlock()
	if !exists {
		return 0, nil
	}

	return set.TotalWeight()
}

func (m *manager) Sample(supernetID ids.ID, size int) ([]ids.NodeID, error) {
	if size == 0 {
		return nil, nil
	}

	m.lock.RLock()
	set, exists := m.supernetToVdrs[supernetID]
	m.lock.RUnlock()
	if !exists {
		return nil, ErrMissingValidators
	}

	return set.Sample(size)
}

func (m *manager) GetMap(supernetID ids.ID) map[ids.NodeID]*GetValidatorOutput {
	m.lock.RLock()
	set, exists := m.supernetToVdrs[supernetID]
	m.lock.RUnlock()
	if !exists {
		return make(map[ids.NodeID]*GetValidatorOutput)
	}

	return set.Map()
}

func (m *manager) RegisterCallbackListener(supernetID ids.ID, listener SetCallbackListener) {
	m.lock.Lock()
	defer m.lock.Unlock()

	set, exists := m.supernetToVdrs[supernetID]
	if !exists {
		set = newSet()
		m.supernetToVdrs[supernetID] = set
	}

	set.RegisterCallbackListener(listener)
}

func (m *manager) String() string {
	m.lock.RLock()
	defer m.lock.RUnlock()

	supernets := maps.Keys(m.supernetToVdrs)
	utils.Sort(supernets)

	sb := strings.Builder{}

	sb.WriteString(fmt.Sprintf("Validator Manager: (Size = %d)",
		len(supernets),
	))
	for _, supernetID := range supernets {
		vdrs := m.supernetToVdrs[supernetID]
		sb.WriteString(fmt.Sprintf(
			"\n    Supernet[%s]: %s",
			supernetID,
			vdrs.PrefixedString("    "),
		))
	}

	return sb.String()
}

func (m *manager) GetValidatorIDs(supernetID ids.ID) []ids.NodeID {
	m.lock.RLock()
	vdrs, exist := m.supernetToVdrs[supernetID]
	m.lock.RUnlock()
	if !exist {
		return nil
	}

	return vdrs.GetValidatorIDs()
}
