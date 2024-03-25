// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"encoding/hex"
	"log"
	"time"

	"github.com/Juneo-io/juneogo/genesis"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/set"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
	"github.com/Juneo-io/juneogo/wallet/supernet/primary"
)

func main() {
	key := genesis.EWOQKey
	uri := primary.LocalAPIURI
	kc := secp256k1fx.NewKeychain(key)
	supernetIDStr := "29uVeLPJB1eQJkzRemU8g8wZDw5uJRqpab5U2mX9euieVwiEbL"
	genesisHex := "00000000000000000000000000017b5490493f8a2fff444ac8b54e27b3339d7c60dcffffffffffffffff"
	vmID := ids.ID{'x', 's', 'v', 'm'}
	name := "let there"

	supernetID, err := ids.FromString(supernetIDStr)
	if err != nil {
		log.Fatalf("failed to parse supernet ID: %s\n", err)
	}

	genesisBytes, err := hex.DecodeString(genesisHex)
	if err != nil {
		log.Fatalf("failed to parse genesis bytes: %s\n", err)
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
	)
	if err != nil {
		log.Fatalf("failed to issue create chain transaction: %s\n", err)
	}
	log.Printf("created new chain %s in %s\n", createChainTx.ID(), time.Since(createChainStartTime))
}
