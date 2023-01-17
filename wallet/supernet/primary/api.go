// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package primary

import (
	"context"

	"github.com/Juneo-io/juneogo/api/info"
	"github.com/Juneo-io/juneogo/codec"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/rpc"
	"github.com/Juneo-io/juneogo/utils/set"
	"github.com/Juneo-io/juneogo/vms/components/june"
	"github.com/Juneo-io/juneogo/vms/jvm"
	"github.com/Juneo-io/juneogo/vms/relayvm"
	"github.com/Juneo-io/juneogo/vms/relayvm/txs"
	"github.com/Juneo-io/juneogo/wallet/chain/asset"
	"github.com/Juneo-io/juneogo/wallet/chain/relay"
)

const (
	MainnetAPIURI = "https://api.avax.network"
	FujiAPIURI    = "https://api.avax-test.network"
	LocalAPIURI   = "http://localhost:9650"

	fetchLimit = 1024
)

// TODO: refactor UTXOClient definition to allow the client implementations to
//
//	perform their own assertions.
var (
	_ UTXOClient = relayvm.Client(nil)
	_ UTXOClient = jvm.Client(nil)
)

type UTXOClient interface {
	GetAtomicUTXOs(
		ctx context.Context,
		addrs []ids.ShortID,
		sourceChain string,
		limit uint32,
		startAddress ids.ShortID,
		startUTXOID ids.ID,
		options ...rpc.Option,
	) ([][]byte, ids.ShortID, ids.ID, error)
}

func FetchState(ctx context.Context, uri string, addrs set.Set[ids.ShortID]) (relay.Context, asset.Context, UTXOs, error) {
	infoClient := info.NewClient(uri)
	assetChainClient := jvm.NewClient(uri, "X")

	relayCTX, err := relay.NewContextFromClients(ctx, infoClient, assetChainClient)
	if err != nil {
		return nil, nil, nil, err
	}

	assetCTX, err := asset.NewContextFromClients(ctx, infoClient, assetChainClient)
	if err != nil {
		return nil, nil, nil, err
	}

	utxos := NewUTXOs()
	addrList := addrs.List()
	chains := []struct {
		id     ids.ID
		client UTXOClient
		codec  codec.Manager
	}{
		{
			id:     constants.RelayChainID,
			client: relayvm.NewClient(uri),
			codec:  txs.Codec,
		},
		{
			id:     assetCTX.BlockchainID(),
			client: assetChainClient,
			codec:  asset.Parser.Codec(),
		},
	}
	for _, destinationChain := range chains {
		for _, sourceChain := range chains {
			err = AddAllUTXOs(
				ctx,
				utxos,
				destinationChain.client,
				destinationChain.codec,
				sourceChain.id,
				destinationChain.id,
				addrList,
			)
			if err != nil {
				return nil, nil, nil, err
			}
		}
	}
	return relayCTX, assetCTX, utxos, nil
}

// AddAllUTXOs fetches all the UTXOs referenced by [addresses] that were sent
// from [sourceChainID] to [destinationChainID] from the [client]. It then uses
// [codec] to parse the returned UTXOs and it adds them into [utxos]. If [ctx]
// expires, then the returned error will be immediately reported.
func AddAllUTXOs(
	ctx context.Context,
	utxos UTXOs,
	client UTXOClient,
	codec codec.Manager,
	sourceChainID ids.ID,
	destinationChainID ids.ID,
	addrs []ids.ShortID,
) error {
	var (
		sourceChainIDStr = sourceChainID.String()
		startAddr        ids.ShortID
		startUTXO        ids.ID
	)
	for {
		utxosBytes, endAddr, endUTXO, err := client.GetAtomicUTXOs(
			ctx,
			addrs,
			sourceChainIDStr,
			fetchLimit,
			startAddr,
			startUTXO,
		)
		if err != nil {
			return err
		}

		for _, utxoBytes := range utxosBytes {
			var utxo june.UTXO
			_, err := codec.Unmarshal(utxoBytes, &utxo)
			if err != nil {
				return err
			}

			if err := utxos.AddUTXO(ctx, sourceChainID, destinationChainID, &utxo); err != nil {
				return err
			}
		}

		if len(utxosBytes) < fetchLimit {
			break
		}

		// Update the vars to query the next page of UTXOs.
		startAddr = endAddr
		startUTXO = endUTXO
	}
	return nil
}
