// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package peer

import (
	"go.uber.org/zap"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow/validators"
	"github.com/Juneo-io/juneogo/utils/crypto/bls"
	"github.com/Juneo-io/juneogo/utils/logging"
)

var _ validators.SetCallbackListener = (*GossipTrackerCallback)(nil)

// GossipTrackerCallback synchronizes GossipTracker's validator state with the
// validator set it's registered to.
type GossipTrackerCallback struct {
	Log           logging.Logger
	GossipTracker GossipTracker
}

// OnValidatorAdded adds [validatorID] to the set of validators that can be
// gossiped about
func (g *GossipTrackerCallback) OnValidatorAdded(
	nodeID ids.NodeID,
	_ *bls.PublicKey,
	txID ids.ID,
	_ uint64,
) {
	vdr := ValidatorID{
		NodeID: nodeID,
		TxID:   txID,
	}
	if !g.GossipTracker.AddValidator(vdr) {
		g.Log.Error("failed to add a validator",
			zap.Stringer("nodeID", nodeID),
			zap.Stringer("txID", txID),
		)
	}
}

// OnValidatorRemoved removes [validatorID] from the set of validators that can
// be gossiped about.
func (g *GossipTrackerCallback) OnValidatorRemoved(nodeID ids.NodeID, _ uint64) {
	if !g.GossipTracker.RemoveValidator(nodeID) {
		g.Log.Error("failed to remove a validator",
			zap.Stringer("nodeID", nodeID),
		)
	}
}

// OnValidatorWeightChanged does nothing because PeerList gossip doesn't care
// about validator weights.
func (*GossipTrackerCallback) OnValidatorWeightChanged(ids.NodeID, uint64, uint64) {}
