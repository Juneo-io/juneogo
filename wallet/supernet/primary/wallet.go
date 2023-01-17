// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package primary

import (
	"context"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/crypto/keychain"
	"github.com/Juneo-io/juneogo/vms/jvm"
	"github.com/Juneo-io/juneogo/vms/relayvm"
	"github.com/Juneo-io/juneogo/vms/relayvm/txs"
	"github.com/Juneo-io/juneogo/wallet/chain/asset"
	"github.com/Juneo-io/juneogo/wallet/chain/relay"
	"github.com/Juneo-io/juneogo/wallet/supernet/primary/common"
)

var _ Wallet = (*wallet)(nil)

// Wallet provides chain wallets for the primary network.
type Wallet interface {
	Relay() relay.Wallet
	Asset() asset.Wallet
}

type wallet struct {
	relay relay.Wallet
	asset asset.Wallet
}

func (w *wallet) Relay() relay.Wallet {
	return w.relay
}

func (w *wallet) Asset() asset.Wallet {
	return w.asset
}

// NewWalletFromURI returns a wallet that supports issuing transactions to the
// chains living in the primary network to a provided [uri].
//
// On creation, the wallet attaches to the provided [uri] and fetches all UTXOs
// that reference any of the keys contained in [kc]. If the UTXOs are modified
// through an external issuance process, such as another instance of the wallet,
// the UTXOs may become out of sync.
//
// The wallet manages all UTXOs locally, and performs all tx signing locally.
func NewWalletFromURI(ctx context.Context, uri string, kc keychain.Keychain) (Wallet, error) {
	relayCTX, assetCTX, utxos, err := FetchState(ctx, uri, kc.Addresses())
	if err != nil {
		return nil, err
	}
	return NewWalletWithState(uri, relayCTX, assetCTX, utxos, kc), nil
}

// Creates a wallet with pre-loaded/cached P-chain transactions.
func NewWalletWithTxs(ctx context.Context, uri string, kc keychain.Keychain, preloadTXs ...ids.ID) (Wallet, error) {
	relayCTX, assetCTX, utxos, err := FetchState(ctx, uri, kc.Addresses())
	if err != nil {
		return nil, err
	}
	relayTXs := make(map[ids.ID]*txs.Tx)
	relayChainClient := relayvm.NewClient(uri)
	for _, id := range preloadTXs {
		txBytes, err := relayChainClient.GetTx(ctx, id)
		if err != nil {
			return nil, err
		}
		tx, err := txs.Parse(txs.Codec, txBytes)
		if err != nil {
			return nil, err
		}
		relayTXs[id] = tx
	}
	return NewWalletWithTxsAndState(uri, relayCTX, assetCTX, utxos, kc, relayTXs), nil
}

// Creates a wallet with pre-loaded/cached P-chain transactions and state.
func NewWalletWithTxsAndState(
	uri string,
	relayCTX relay.Context,
	assetCTX asset.Context,
	utxos UTXOs,
	kc keychain.Keychain,
	relayTXs map[ids.ID]*txs.Tx,
) Wallet {
	addrs := kc.Addresses()
	pUTXOs := NewChainUTXOs(constants.RelayChainID, utxos)
	pBackend := relay.NewBackend(relayCTX, pUTXOs, relayTXs)
	pBuilder := relay.NewBuilder(addrs, pBackend)
	pSigner := relay.NewSigner(kc, pBackend)
	relayChainClient := relayvm.NewClient(uri)

	assetChainID := assetCTX.BlockchainID()
	xUTXOs := NewChainUTXOs(assetChainID, utxos)
	xBackend := asset.NewBackend(assetCTX, assetChainID, xUTXOs)
	xBuilder := asset.NewBuilder(addrs, xBackend)
	xSigner := asset.NewSigner(kc, xBackend)
	assetChainClient := jvm.NewClient(uri, "X")

	return NewWallet(
		relay.NewWallet(pBuilder, pSigner, relayChainClient, pBackend),
		asset.NewWallet(xBuilder, xSigner, assetChainClient, xBackend),
	)
}

// Creates a wallet with pre-fetched state.
func NewWalletWithState(
	uri string,
	relayCTX relay.Context,
	assetCTX asset.Context,
	utxos UTXOs,
	kc keychain.Keychain,
) Wallet {
	relayTXs := make(map[ids.ID]*txs.Tx)
	return NewWalletWithTxsAndState(uri, relayCTX, assetCTX, utxos, kc, relayTXs)
}

// Creates a Wallet with the given set of options
func NewWalletWithOptions(w Wallet, options ...common.Option) Wallet {
	return NewWallet(
		relay.NewWalletWithOptions(w.Relay(), options...),
		asset.NewWalletWithOptions(w.Asset(), options...),
	)
}

// Creates a new default wallet
func NewWallet(relay relay.Wallet, asset asset.Wallet) Wallet {
	return &wallet{
		relay: relay,
		asset: asset,
	}
}
