// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package sender

import (
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/message"
	"github.com/Juneo-io/juneogo/snow/engine/common"
	"github.com/Juneo-io/juneogo/supernets"
	"github.com/Juneo-io/juneogo/utils/set"
)

// ExternalSender sends consensus messages to other validators
// Right now this is implemented in the networking package
type ExternalSender interface {
	Send(
		msg message.OutboundMessage,
		config common.SendConfig,
		supernetID ids.ID,
		allower supernets.Allower,
	) set.Set[ids.NodeID]
}
