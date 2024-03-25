// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package block

import (
	"math"
	"time"

	"github.com/Juneo-io/juneogo/codec"
	"github.com/Juneo-io/juneogo/codec/linearcodec"
	"github.com/Juneo-io/juneogo/utils"
)

const CodecVersion = 0

var Codec codec.Manager

func init() {
	lc := linearcodec.NewDefault(time.Time{})
	// The maximum block size is enforced by the p2p message size limit.
	// See: [constants.DefaultMaxMessageSize]
	Codec = codec.NewManager(math.MaxInt)

	err := utils.Err(
		lc.RegisterType(&statelessBlock{}),
		lc.RegisterType(&option{}),
		Codec.RegisterCodec(CodecVersion, lc),
	)
	if err != nil {
		panic(err)
	}
}
