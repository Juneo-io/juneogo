// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"reflect"

	"github.com/Juneo-io/juneogo/codec"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/vms/avm/config"
	"github.com/Juneo-io/juneogo/vms/avm/fxs"
)

type Backend struct {
	Ctx           *snow.Context
	Config        *config.Config
	Fxs           []*fxs.ParsedFx
	TypeToFxIndex map[reflect.Type]int
	Codec         codec.Manager
	// Note: FeeAssetID may be different than ctx.JUNEAssetID if this AVM is
	// running in a supernet.
	FeeAssetID   ids.ID
	Bootstrapped bool
}
