// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"log"
	"time"

	"github.com/Juneo-io/juneogo/genesis"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/formatting/address"
	"github.com/Juneo-io/juneogo/utils/units"
	"github.com/Juneo-io/juneogo/vms/components/avax"
	"github.com/Juneo-io/juneogo/vms/platformvm/stakeable"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
	"github.com/Juneo-io/juneogo/wallet/supernet/primary"
)

func main() {
	key := genesis.EWOQKey
	uri := primary.LocalAPIURI
	kc := secp256k1fx.NewKeychain(key)
	amount := 500 * units.MilliAvax
	locktime := uint64(time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC).Unix())
	destAddrStr := "P-local18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u"

	destAddr, err := address.ParseToID(destAddrStr)
	if err != nil {
		log.Fatalf("failed to parse address: %s\n", err)
	}

	ctx := context.Background()

	// MakeWallet fetches the available UTXOs owned by [kc] on the network that
	// [uri] is hosting.
	walletSyncStartTime := time.Now()
	wallet, err := primary.MakeWallet(ctx, &primary.WalletConfig{
		URI:          uri,
		AVAXKeychain: kc,
		EthKeychain:  kc,
	})
	if err != nil {
		log.Fatalf("failed to initialize wallet: %s\n", err)
	}
	log.Printf("synced wallet in %s\n", time.Since(walletSyncStartTime))

	// Get the P-chain wallet
	pWallet := wallet.P()
	pBuilder := pWallet.Builder()
	pContext := pBuilder.Context()
	juneAssetID := pContext.JUNEAssetID

	issueTxStartTime := time.Now()
	tx, err := pWallet.IssueBaseTx([]*avax.TransferableOutput{
		{
			Asset: avax.Asset{
				ID: juneAssetID,
			},
			Out: &stakeable.LockOut{
				Locktime: locktime,
				TransferableOut: &secp256k1fx.TransferOutput{
					Amt: amount,
					OutputOwners: secp256k1fx.OutputOwners{
						Threshold: 1,
						Addrs: []ids.ShortID{
							destAddr,
						},
					},
				},
			},
		},
	})
	if err != nil {
		log.Fatalf("failed to issue transaction: %s\n", err)
	}
	log.Printf("issued %s in %s\n", tx.ID(), time.Since(issueTxStartTime))
}