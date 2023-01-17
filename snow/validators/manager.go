// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
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
)

var (
	_ Manager = (*manager)(nil)

	errMissingValidators = errors.New("missing validators")
)

// Manager holds the validator set of each supernet
type Manager interface {
	fmt.Stringer

	// Add a supernet's validator set to the manager.
	//
	// If the supernet had previously registered a validator set, false will be
	// returned and the manager will not be modified.
	Add(supernetID ids.ID, set Set) bool

	// Get returns the validator set for the given supernet
	// Returns false if the supernet doesn't exist
	Get(ids.ID) (Set, bool)
}

// NewManager returns a new, empty manager
func NewManager() Manager {
	return &manager{
		supernetToVdrs: make(map[ids.ID]Set),
	}
}

type manager struct {
	lock sync.RWMutex

	// Key: Supernet ID
	// Value: The validators that validate the supernet
	supernetToVdrs map[ids.ID]Set
}

func (m *manager) Add(supernetID ids.ID, set Set) bool {
	m.lock.Lock()
	defer m.lock.Unlock()

	if _, exists := m.supernetToVdrs[supernetID]; exists {
		return false
	}

	m.supernetToVdrs[supernetID] = set
	return true
}

func (m *manager) Get(supernetID ids.ID) (Set, bool) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	vdrs, ok := m.supernetToVdrs[supernetID]
	return vdrs, ok
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

// Add is a helper that fetches the validator set of [supernetID] from [m] and
// adds [nodeID] to the validator set.
// Returns an error if:
// - [supernetID] does not have a registered validator set in [m]
// - adding [nodeID] to the validator set returns an error
func Add(m Manager, supernetID ids.ID, nodeID ids.NodeID, pk *bls.PublicKey, txID ids.ID, weight uint64) error {
	vdrs, ok := m.Get(supernetID)
	if !ok {
		return fmt.Errorf("%w: %s", errMissingValidators, supernetID)
	}
	return vdrs.Add(nodeID, pk, txID, weight)
}

// AddWeight is a helper that fetches the validator set of [supernetID] from [m]
// and adds [weight] to [nodeID] in the validator set.
// Returns an error if:
// - [supernetID] does not have a registered validator set in [m]
// - adding [weight] to [nodeID] in the validator set returns an error
func AddWeight(m Manager, supernetID ids.ID, nodeID ids.NodeID, weight uint64) error {
	vdrs, ok := m.Get(supernetID)
	if !ok {
		return fmt.Errorf("%w: %s", errMissingValidators, supernetID)
	}
	return vdrs.AddWeight(nodeID, weight)
}

// RemoveWeight is a helper that fetches the validator set of [supernetID] from
// [m] and removes [weight] from [nodeID] in the validator set.
// Returns an error if:
// - [supernetID] does not have a registered validator set in [m]
// - removing [weight] from [nodeID] in the validator set returns an error
func RemoveWeight(m Manager, supernetID ids.ID, nodeID ids.NodeID, weight uint64) error {
	vdrs, ok := m.Get(supernetID)
	if !ok {
		return fmt.Errorf("%w: %s", errMissingValidators, supernetID)
	}
	return vdrs.RemoveWeight(nodeID, weight)
}

// AddWeight is a helper that fetches the validator set of [supernetID] from [m]
// and returns if the validator set contains [nodeID]. If [m] does not contain a
// validator set for [supernetID], false is returned.
func Contains(m Manager, supernetID ids.ID, nodeID ids.NodeID) bool {
	vdrs, ok := m.Get(supernetID)
	if !ok {
		return false
	}
	return vdrs.Contains(nodeID)
}
