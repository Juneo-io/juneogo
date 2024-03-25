// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package bootstrap

import (
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/snow/engine/common"
	"github.com/Juneo-io/juneogo/snow/engine/common/queue"
	"github.com/Juneo-io/juneogo/snow/engine/common/tracker"
	"github.com/Juneo-io/juneogo/snow/engine/snowman/block"
	"github.com/Juneo-io/juneogo/snow/validators"
)

type Config struct {
	common.AllGetsServer

	Ctx     *snow.ConsensusContext
	Beacons validators.Manager

	SampleK          int
	StartupTracker   tracker.Startup
	Sender           common.Sender
	BootstrapTracker common.BootstrapTracker
	Timer            common.Timer

	// This node will only consider the first [AncestorsMaxContainersReceived]
	// containers in an ancestors message it receives.
	AncestorsMaxContainersReceived int

	// Blocked tracks operations that are blocked on blocks
	//
	// It should be guaranteed that `MissingIDs` should contain all IDs
	// referenced by the `MissingDependencies` that have not already been added
	// to the queue.
	Blocked *queue.JobsWithMissing

	VM block.ChainVM

	Bootstrapped func()
}
