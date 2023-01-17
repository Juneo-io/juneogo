// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package constants

import (
	"math"
	"time"

	"github.com/Juneo-io/juneogo/utils/units"
)

// Const variables to be exported
const (
	// Request ID used when sending a Put message to gossip an accepted container
	// (ie not sent in response to a Get)
	GossipMsgRequestID uint32 = math.MaxUint32

	// The network must be "tcp", "tcp4", "tcp6", "unix" or "unixpacket".
	NetworkType = "tcp"

	DefaultMaxMessageSize  = 2 * units.MiB
	DefaultPingPongTimeout = 30 * time.Second
	DefaultPingFrequency   = 3 * DefaultPingPongTimeout / 4
	DefaultByteSliceCap    = 128

	MaxContainersLen = int(4 * DefaultMaxMessageSize / 5)

	// MinConnectedStakeBuffer is the safety buffer for calculation of MinConnectedStake.
	// This increases the required stake percentage above alpha/k. Must be [0-1]
	// 0 means MinConnectedStake = alpha/k, 1 means MinConnectedStake = 1 (fully connected)
	MinConnectedStakeBuffer = .2
)
