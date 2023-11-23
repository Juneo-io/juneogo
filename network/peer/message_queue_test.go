// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package peer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/message"
	"github.com/Juneo-io/juneogo/proto/pb/p2p"
	"github.com/Juneo-io/juneogo/utils/logging"
)

func TestMessageQueue(t *testing.T) {
	require := require.New(t)

	expectFail := false
	q := NewBlockingMessageQueue(
		SendFailedFunc(func(msg message.OutboundMessage) {
			require.True(expectFail)
		}),
		logging.NoLog{},
		0,
	)

	mc := newMessageCreator(t)
	msgs := []message.OutboundMessage{}
	numToSend := 10

	// Assert that the messages are popped in the same order they were pushed
	for i := 0; i < numToSend; i++ {
		testID := ids.GenerateTestID()
		testID2 := ids.GenerateTestID()
		m, err := mc.Ping(
			uint32(i),
			[]*p2p.SupernetUptime{
				{SupernetId: testID[:], Uptime: uint32(i)},
				{SupernetId: testID2[:], Uptime: uint32(i)},
			})
		require.NoError(err)
		msgs = append(msgs, m)
	}

	go func() {
		for i := 0; i < numToSend; i++ {
			q.Push(context.Background(), msgs[i])
		}
	}()

	for i := 0; i < numToSend; i++ {
		msg, ok := q.Pop()
		require.True(ok)
		require.Equal(msgs[i], msg)
	}

	// Assert that PopNow returns false when the queue is empty
	_, ok := q.PopNow()
	require.False(ok)

	// Assert that Push returns false when the context is canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	expectFail = true
	done := make(chan struct{})
	go func() {
		ok := q.Push(ctx, msgs[0])
		require.False(ok)
		close(done)
	}()
	<-done

	// Assert that Push returns false when the queue is closed
	done = make(chan struct{})
	go func() {
		ok := q.Push(context.Background(), msgs[0])
		require.False(ok)
		close(done)
	}()
	q.Close()
	<-done

	// Assert Pop returns false when the queue is closed
	_, ok = q.Pop()
	require.False(ok)
}
