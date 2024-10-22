// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package supernets

import "github.com/Juneo-io/juneogo/ids"

// NoOpAllower is an Allower that always returns true
var NoOpAllower Allower = noOpAllower{}

type noOpAllower struct{}

func (noOpAllower) IsAllowed(ids.NodeID, bool) bool {
	return true
}
