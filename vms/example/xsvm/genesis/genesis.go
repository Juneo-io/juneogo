// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/hashing"
	"github.com/Juneo-io/juneogo/vms/example/xsvm/block"
)

type Genesis struct {
	Timestamp   int64        `serialize:"true" json:"timestamp"`
	Allocations []Allocation `serialize:"true" json:"allocations"`
}

type Allocation struct {
	Address ids.ShortID `serialize:"true" json:"address"`
	Balance uint64      `serialize:"true" json:"balance"`
}

func Parse(bytes []byte) (*Genesis, error) {
	genesis := &Genesis{}
	_, err := Codec.Unmarshal(bytes, genesis)
	return genesis, err
}

func Block(genesis *Genesis) (*block.Stateless, error) {
	bytes, err := Codec.Marshal(Version, genesis)
	if err != nil {
		return nil, err
	}
	return &block.Stateless{
		ParentID:  hashing.ComputeHash256Array(bytes),
		Timestamp: genesis.Timestamp,
	}, nil
}
