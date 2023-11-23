// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package sender

import (
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/message"
	"github.com/Juneo-io/juneogo/supernets"
	"github.com/Juneo-io/juneogo/utils/set"
)

// ExternalSender sends consensus messages to other validators
// Right now this is implemented in the networking package
type ExternalSender interface {
	// Send a message to a specific set of nodes
	Send(
		msg message.OutboundMessage,
		nodeIDs set.Set[ids.NodeID],
		supernetID ids.ID,
		allower supernets.Allower,
	) set.Set[ids.NodeID]

	// Send a message to a random group of nodes in a supernet.
	// Nodes are sampled based on their validator status.
	Gossip(
		msg message.OutboundMessage,
		supernetID ids.ID,
		numValidatorsToSend int,
		numNonValidatorsToSend int,
		numPeersToSend int,
		allower supernets.Allower,
	) set.Set[ids.NodeID]
}
