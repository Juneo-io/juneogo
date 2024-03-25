// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package encdb

import (
	"time"

	"github.com/Juneo-io/juneogo/codec"
	"github.com/Juneo-io/juneogo/codec/linearcodec"
)

const CodecVersion = 0

var Codec codec.Manager

func init() {
	lc := linearcodec.NewDefault(time.Time{})
	Codec = codec.NewDefaultManager()

	if err := Codec.RegisterCodec(CodecVersion, lc); err != nil {
		panic(err)
	}
}
