// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package relay

import (
	stdcontext "context"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/crypto/keychain"
	"github.com/Juneo-io/juneogo/vms/components/june"
	"github.com/Juneo-io/juneogo/vms/relayvm/txs"
)

var _ Signer = (*txSigner)(nil)

type Signer interface {
	SignUnsigned(ctx stdcontext.Context, tx txs.UnsignedTx) (*txs.Tx, error)
	Sign(ctx stdcontext.Context, tx *txs.Tx) error
}

type SignerBackend interface {
	GetUTXO(ctx stdcontext.Context, chainID, utxoID ids.ID) (*june.UTXO, error)
	GetTx(ctx stdcontext.Context, txID ids.ID) (*txs.Tx, error)
}

type txSigner struct {
	kc      keychain.Keychain
	backend SignerBackend
}

func NewSigner(kc keychain.Keychain, backend SignerBackend) Signer {
	return &txSigner{
		kc:      kc,
		backend: backend,
	}
}

func (s *txSigner) SignUnsigned(ctx stdcontext.Context, utx txs.UnsignedTx) (*txs.Tx, error) {
	tx := &txs.Tx{Unsigned: utx}
	return tx, s.Sign(ctx, tx)
}

func (s *txSigner) Sign(ctx stdcontext.Context, tx *txs.Tx) error {
	return tx.Unsigned.Visit(&signerVisitor{
		kc:      s.kc,
		backend: s.backend,
		ctx:     ctx,
		tx:      tx,
	})
}
