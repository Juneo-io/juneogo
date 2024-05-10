// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package node

import (
	"fmt"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow/validators"
	"github.com/Juneo-io/juneogo/utils/crypto/bls"
	"github.com/Juneo-io/juneogo/utils/set"
)

var _ validators.Manager = (*overriddenManager)(nil)

// newOverriddenManager returns a Manager that overrides of all calls to the
// underlying Manager to only operate on the validators in [supernetID].
func newOverriddenManager(supernetID ids.ID, manager validators.Manager) *overriddenManager {
	return &overriddenManager{
		supernetID: supernetID,
		manager:  manager,
	}
}

// overriddenManager is a wrapper around a Manager that overrides of all calls
// to the underlying Manager to only operate on the validators in [supernetID].
// supernetID here is typically the primary network ID, as it has the superset of
// all supernet validators.
type overriddenManager struct {
	manager  validators.Manager
	supernetID ids.ID
}

func (o *overriddenManager) AddStaker(_ ids.ID, nodeID ids.NodeID, pk *bls.PublicKey, txID ids.ID, weight uint64) error {
	return o.manager.AddStaker(o.supernetID, nodeID, pk, txID, weight)
}

func (o *overriddenManager) AddWeight(_ ids.ID, nodeID ids.NodeID, weight uint64) error {
	return o.manager.AddWeight(o.supernetID, nodeID, weight)
}

func (o *overriddenManager) GetWeight(_ ids.ID, nodeID ids.NodeID) uint64 {
	return o.manager.GetWeight(o.supernetID, nodeID)
}

func (o *overriddenManager) GetValidator(_ ids.ID, nodeID ids.NodeID) (*validators.Validator, bool) {
	return o.manager.GetValidator(o.supernetID, nodeID)
}

func (o *overriddenManager) SubsetWeight(_ ids.ID, nodeIDs set.Set[ids.NodeID]) (uint64, error) {
	return o.manager.SubsetWeight(o.supernetID, nodeIDs)
}

func (o *overriddenManager) RemoveWeight(_ ids.ID, nodeID ids.NodeID, weight uint64) error {
	return o.manager.RemoveWeight(o.supernetID, nodeID, weight)
}

func (o *overriddenManager) Count(ids.ID) int {
	return o.manager.Count(o.supernetID)
}

func (o *overriddenManager) TotalWeight(ids.ID) (uint64, error) {
	return o.manager.TotalWeight(o.supernetID)
}

func (o *overriddenManager) Sample(_ ids.ID, size int) ([]ids.NodeID, error) {
	return o.manager.Sample(o.supernetID, size)
}

func (o *overriddenManager) GetMap(ids.ID) map[ids.NodeID]*validators.GetValidatorOutput {
	return o.manager.GetMap(o.supernetID)
}

func (o *overriddenManager) RegisterCallbackListener(listener validators.ManagerCallbackListener) {
	o.manager.RegisterCallbackListener(listener)
}

func (o *overriddenManager) RegisterSetCallbackListener(_ ids.ID, listener validators.SetCallbackListener) {
	o.manager.RegisterSetCallbackListener(o.supernetID, listener)
}

func (o *overriddenManager) String() string {
	return fmt.Sprintf("Overridden Validator Manager (SupernetID = %s): %s", o.supernetID, o.manager)
}

func (o *overriddenManager) GetValidatorIDs(ids.ID) []ids.NodeID {
	return o.manager.GetValidatorIDs(o.supernetID)
}
