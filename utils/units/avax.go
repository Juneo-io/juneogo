// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package units

// Denominations of value
const (
	NanoJune  uint64 = 1
	MicroJune uint64 = 1000 * NanoJune
	Schmeckle uint64 = 49*MicroJune + 463*NanoJune
	MilliJune uint64 = 1000 * MicroJune
	June      uint64 = 1000 * MilliJune
	KiloJune  uint64 = 1000 * June
	MegaJune  uint64 = 1000 * KiloJune
)
