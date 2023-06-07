// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validators

import (
	"context"

	"github.com/Juneo-io/juneogo/ids"
)

// SupernetConnector represents a handler that is called when a connection is
// marked as connected to a supernet
type SupernetConnector interface {
	ConnectedSupernet(ctx context.Context, nodeID ids.NodeID, supernetID ids.ID) error
}
