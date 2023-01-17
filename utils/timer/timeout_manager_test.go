// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package timer

import (
	"sync"
	"testing"
	"time"

	"github.com/Juneo-io/juneogo/ids"
)

func TestTimeoutManager(*testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(2)
	defer wg.Wait()

	tm := TimeoutManager{}
	tm.Initialize(time.Millisecond)
	go tm.Dispatch()

	tm.Put(ids.ID{}, wg.Done)
	tm.Put(ids.ID{1}, wg.Done)
}
