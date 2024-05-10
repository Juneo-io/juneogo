// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package sender

import (
	"errors"
	"testing"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/message"
	"github.com/Juneo-io/juneogo/snow/engine/common"
	"github.com/Juneo-io/juneogo/supernets"
	"github.com/Juneo-io/juneogo/utils/set"
)

var (
	_ ExternalSender = (*ExternalSenderTest)(nil)

	errSend = errors.New("unexpectedly called Send")
)

// ExternalSenderTest is a test sender
type ExternalSenderTest struct {
	TB testing.TB

	CantSend bool

	SendF func(msg message.OutboundMessage, config common.SendConfig, supernetID ids.ID, allower supernets.Allower) set.Set[ids.NodeID]
}

// Default set the default callable value to [cant]
func (s *ExternalSenderTest) Default(cant bool) {
	s.CantSend = cant
}

func (s *ExternalSenderTest) Send(
	msg message.OutboundMessage,
	config common.SendConfig,
	supernetID ids.ID,
	allower supernets.Allower,
) set.Set[ids.NodeID] {
	if s.SendF != nil {
		return s.SendF(msg, config, supernetID, allower)
	}
	if s.CantSend {
		if s.TB != nil {
			s.TB.Helper()
			s.TB.Fatal(errSend)
		}
	}
	return nil
}
