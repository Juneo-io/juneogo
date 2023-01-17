// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/vms/components/june"
)

type UTXOGetter interface {
	GetUTXO(utxoID ids.ID) (*june.UTXO, error)
}

type UTXOAdder interface {
	AddUTXO(utxo *june.UTXO)
}

type UTXODeleter interface {
	DeleteUTXO(utxoID ids.ID)
}
