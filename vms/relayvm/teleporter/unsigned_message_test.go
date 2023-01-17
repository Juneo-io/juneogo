// Copyright (C) 2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package teleporter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils"
)

func TestUnsignedMessage(t *testing.T) {
	require := require.New(t)

	msg, err := NewUnsignedMessage(
		ids.GenerateTestID(),
		ids.GenerateTestID(),
		[]byte("payload"),
	)
	require.NoError(err)

	msgBytes := msg.Bytes()
	msg2, err := ParseUnsignedMessage(msgBytes)
	require.NoError(err)
	require.Equal(msg, msg2)
}

func TestParseUnsignedMessageJunk(t *testing.T) {
	require := require.New(t)

	_, err := ParseUnsignedMessage(utils.RandomBytes(1024))
	require.Error(err)
}
