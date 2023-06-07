// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package uptime

import (
	"time"

	"github.com/Juneo-io/juneogo/ids"
)

type State interface {
	// GetUptime returns [upDuration] and [lastUpdated] of [nodeID] on
	// [supernetID].
	// Returns [database.ErrNotFound] if [nodeID] isn't currently a validator of
	// the supernet.
	GetUptime(
		nodeID ids.NodeID,
		supernetID ids.ID,
	) (upDuration time.Duration, lastUpdated time.Time, err error)

	// SetUptime updates [upDuration] and [lastUpdated] of [nodeID] on
	// [supernetID].
	// Returns [database.ErrNotFound] if [nodeID] isn't currently a validator of
	// the supernet.
	// Invariant: expects [lastUpdated] to be truncated (floored) to the nearest
	//            second.
	SetUptime(
		nodeID ids.NodeID,
		supernetID ids.ID,
		upDuration time.Duration,
		lastUpdated time.Time,
	) error

	// GetStartTime returns the time that [nodeID] started validating
	// [supernetID].
	// Returns [database.ErrNotFound] if [nodeID] isn't currently a validator of
	// the supernet.
	GetStartTime(
		nodeID ids.NodeID,
		supernetID ids.ID,
	) (startTime time.Time, err error)
}
