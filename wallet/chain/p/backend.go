// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package p

import (
	"context"
	"sync"

	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/set"
	"github.com/Juneo-io/juneogo/vms/components/avax"
	"github.com/Juneo-io/juneogo/vms/platformvm/fx"
	"github.com/Juneo-io/juneogo/vms/platformvm/txs"
	"github.com/Juneo-io/juneogo/wallet/chain/p/builder"
	"github.com/Juneo-io/juneogo/wallet/chain/p/signer"
	"github.com/Juneo-io/juneogo/wallet/supernet/primary/common"
)

var _ Backend = (*backend)(nil)

// Backend defines the full interface required to support a P-chain wallet.
type Backend interface {
	builder.Backend
	signer.Backend

	AcceptTx(ctx context.Context, tx *txs.Tx) error
}

type backend struct {
	common.ChainUTXOs

	context *builder.Context

	supernetOwnerLock sync.RWMutex
	supernetOwner     map[ids.ID]fx.Owner // supernetID -> owner
}

func NewBackend(context *builder.Context, utxos common.ChainUTXOs, supernetTxs map[ids.ID]*txs.Tx) Backend {
	supernetOwner := make(map[ids.ID]fx.Owner)
	for txID, tx := range supernetTxs { // first get owners from the CreateSupernetTx
		createSupernetTx, ok := tx.Unsigned.(*txs.CreateSupernetTx)
		if !ok {
			continue
		}
		supernetOwner[txID] = createSupernetTx.Owner
	}
	for _, tx := range supernetTxs { // then check for TransferSupernetOwnershipTx
		transferSupernetOwnershipTx, ok := tx.Unsigned.(*txs.TransferSupernetOwnershipTx)
		if !ok {
			continue
		}
		supernetOwner[transferSupernetOwnershipTx.Supernet] = transferSupernetOwnershipTx.Owner
	}
	return &backend{
		ChainUTXOs:  utxos,
		context:     context,
		supernetOwner: supernetOwner,
	}
}

func (b *backend) AcceptTx(ctx context.Context, tx *txs.Tx) error {
	txID := tx.ID()
	err := tx.Unsigned.Visit(&backendVisitor{
		b:    b,
		ctx:  ctx,
		txID: txID,
	})
	if err != nil {
		return err
	}

	producedUTXOSlice := tx.UTXOs()
	return b.addUTXOs(ctx, constants.PlatformChainID, producedUTXOSlice)
}

func (b *backend) addUTXOs(ctx context.Context, destinationChainID ids.ID, utxos []*avax.UTXO) error {
	for _, utxo := range utxos {
		if err := b.AddUTXO(ctx, destinationChainID, utxo); err != nil {
			return err
		}
	}
	return nil
}

func (b *backend) removeUTXOs(ctx context.Context, sourceChain ids.ID, utxoIDs set.Set[ids.ID]) error {
	for utxoID := range utxoIDs {
		if err := b.RemoveUTXO(ctx, sourceChain, utxoID); err != nil {
			return err
		}
	}
	return nil
}

func (b *backend) GetSupernetOwner(_ context.Context, supernetID ids.ID) (fx.Owner, error) {
	b.supernetOwnerLock.RLock()
	defer b.supernetOwnerLock.RUnlock()

	owner, exists := b.supernetOwner[supernetID]
	if !exists {
		return nil, database.ErrNotFound
	}
	return owner, nil
}

func (b *backend) setSupernetOwner(supernetID ids.ID, owner fx.Owner) {
	b.supernetOwnerLock.Lock()
	defer b.supernetOwnerLock.Unlock()

	b.supernetOwner[supernetID] = owner
}
