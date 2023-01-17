// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package common

import (
	"github.com/Juneo-io/juneogo/ids"
)

// Supernet describes the standard interface of a supernet description
type Supernet interface {
	// Returns true iff the supernet is done bootstrapping
	IsBootstrapped() bool

	// Bootstrapped marks the named chain as being bootstrapped
	Bootstrapped(chainID ids.ID)
}
