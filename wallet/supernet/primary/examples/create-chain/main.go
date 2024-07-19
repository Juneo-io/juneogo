// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"log"
	"math"
	"time"

	"github.com/Juneo-io/juneogo/genesis"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/set"
	"github.com/Juneo-io/juneogo/vms/example/xsvm"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
	"github.com/Juneo-io/juneogo/wallet/supernet/primary"

	xsgenesis "github.com/Juneo-io/juneogo/vms/example/xsvm/genesis"
)

func main() {
	key := genesis.EWOQKey
	uri := primary.LocalAPIURI
	kc := secp256k1fx.NewKeychain(key)
	supernetIDStr := "29uVeLPJB1eQJkzRemU8g8wZDw5uJRqpab5U2mX9euieVwiEbL"
	genesis := &xsgenesis.Genesis{
		Timestamp: time.Now().Unix(),
		Allocations: []xsgenesis.Allocation{
			{
				Address: genesis.EWOQKey.Address(),
				Balance: math.MaxUint64,
			},
		},
	}
	vmID := xsvm.ID
	name := "let there"
	chainAssetID := ids.Empty

	supernetID, err := ids.FromString(supernetIDStr)
	if err != nil {
		log.Fatalf("failed to parse supernet ID: %s\n", err)
	}

	genesisBytes, err := xsgenesis.Codec.Marshal(xsgenesis.CodecVersion, genesis)
	if err != nil {
		log.Fatalf("failed to create genesis bytes: %s\n", err)
	}

	ctx := context.Background()

	// MakeWallet fetches the available UTXOs owned by [kc] on the network that
	// [uri] is hosting and registers [supernetID].
	walletSyncStartTime := time.Now()
	wallet, err := primary.MakeWallet(ctx, &primary.WalletConfig{
		URI:              uri,
		AVAXKeychain:     kc,
		EthKeychain:      kc,
		PChainTxsToFetch: set.Of(supernetID),
	})
	if err != nil {
		log.Fatalf("failed to initialize wallet: %s\n", err)
	}
	log.Printf("synced wallet in %s\n", time.Since(walletSyncStartTime))

	// Get the P-chain wallet
	pWallet := wallet.P()

	createChainStartTime := time.Now()
	createChainTx, err := pWallet.IssueCreateChainTx(
		supernetID,
		genesisBytes,
		vmID,
		nil,
		name,
		chainAssetID,
	)
	if err != nil {
		log.Fatalf("failed to issue create chain transaction: %s\n", err)
	}
	log.Printf("created new chain %s in %s\n", createChainTx.ID(), time.Since(createChainStartTime))
}
