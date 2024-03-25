// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validators

import (
	"context"
	"fmt"

	"github.com/Juneo-io/juneogo/ids"
)

var UnhandledSupernetConnector SupernetConnector = &unhandledSupernetConnector{}

type unhandledSupernetConnector struct{}

func (unhandledSupernetConnector) ConnectedSupernet(_ context.Context, nodeID ids.NodeID, supernetID ids.ID) error {
	return fmt.Errorf(
		"unhandled ConnectedSupernet with nodeID=%q and supernetID=%q",
		nodeID,
		supernetID,
	)
}
