// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package primary

import (
	"context"
	"log"
	"time"

	"github.com/Juneo-io/juneogo/genesis"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/units"
	"github.com/Juneo-io/juneogo/vms/components/avax"
	"github.com/Juneo-io/juneogo/vms/components/verify"
	"github.com/Juneo-io/juneogo/vms/platformvm/reward"
	"github.com/Juneo-io/juneogo/vms/platformvm/signer"
	"github.com/Juneo-io/juneogo/vms/platformvm/txs"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
)

func ExampleWallet() {
	ctx := context.Background()
	kc := secp256k1fx.NewKeychain(genesis.EWOQKey)

	// MakeWallet fetches the available UTXOs owned by [kc] on the network that
	// [LocalAPIURI] is hosting.
	walletSyncStartTime := time.Now()
	wallet, err := MakeWallet(ctx, &WalletConfig{
		URI:          LocalAPIURI,
		AVAXKeychain: kc,
		EthKeychain:  kc,
	})
	if err != nil {
		log.Fatalf("failed to initialize wallet with: %s\n", err)
		return
	}
	log.Printf("synced wallet in %s\n", time.Since(walletSyncStartTime))

	// Get the P-chain and the X-chain wallets
	pWallet := wallet.P()
	xWallet := wallet.X()
	xBuilder := xWallet.Builder()
	xContext := xBuilder.Context()

	// Pull out useful constants to use when issuing transactions.
	xChainID := xContext.BlockchainID
	owner := &secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs: []ids.ShortID{
			genesis.EWOQKey.PublicKey().Address(),
		},
	}

	// Create a custom asset to send to the P-chain.
	createAssetStartTime := time.Now()
	createAssetTx, err := xWallet.IssueCreateAssetTx(
		"RnM",
		"RNM",
		9,
		map[uint32][]verify.State{
			0: {
				&secp256k1fx.TransferOutput{
					Amt:          100 * units.MegaAvax,
					OutputOwners: *owner,
				},
			},
		},
	)
	if err != nil {
		log.Fatalf("failed to create new X-chain asset with: %s\n", err)
		return
	}
	createAssetTxID := createAssetTx.ID()
	log.Printf("created X-chain asset %s in %s\n", createAssetTxID, time.Since(createAssetStartTime))

	// Send 100 MegaAvax to the P-chain.
	exportStartTime := time.Now()
	exportTx, err := xWallet.IssueExportTx(
		constants.PlatformChainID,
		[]*avax.TransferableOutput{
			{
				Asset: avax.Asset{
					ID: createAssetTxID,
				},
				Out: &secp256k1fx.TransferOutput{
					Amt:          100 * units.MegaAvax,
					OutputOwners: *owner,
				},
			},
		},
	)
	if err != nil {
		log.Fatalf("failed to issue X->P export transaction with: %s\n", err)
		return
	}
	exportTxID := exportTx.ID()
	log.Printf("issued X->P export %s in %s\n", exportTxID, time.Since(exportStartTime))

	// Import the 100 MegaAvax from the X-chain into the P-chain.
	importStartTime := time.Now()
	importTx, err := pWallet.IssueImportTx(xChainID, owner)
	if err != nil {
		log.Fatalf("failed to issue X->P import transaction with: %s\n", err)
		return
	}
	importTxID := importTx.ID()
	log.Printf("issued X->P import %s in %s\n", importTxID, time.Since(importStartTime))

	createSupernetStartTime := time.Now()
	createSupernetTx, err := pWallet.IssueCreateSupernetTx(owner)
	if err != nil {
		log.Fatalf("failed to issue create supernet transaction with: %s\n", err)
		return
	}
	createSupernetTxID := createSupernetTx.ID()
	log.Printf("issued create supernet transaction %s in %s\n", createSupernetTxID, time.Since(createSupernetStartTime))

	transformSupernetStartTime := time.Now()
	transformSupernetTx, err := pWallet.IssueTransformSupernetTx(
		createSupernetTxID,
		createAssetTxID,
		0,
		1_0000,
		uint64(time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC).Unix()),
		8000,
		uint64(time.Date(2001, time.January, 1, 0, 0, 0, 0, time.UTC).Unix()),
		6000,
		uint64(time.Date(2002, time.January, 1, 0, 0, 0, 0, time.UTC).Unix()),
		1,
		100*units.MegaAvax,
		time.Second,
		365*24*time.Hour,
		2_0000,
		0,
		0,
		1,
		5,
		.80*reward.PercentDenominator,
	)
	if err != nil {
		log.Fatalf("failed to issue transform supernet transaction with: %s\n", err)
		return
	}
	transformSupernetTxID := transformSupernetTx.ID()
	log.Printf("issued transform supernet transaction %s in %s\n", transformSupernetTxID, time.Since(transformSupernetStartTime))

	addPermissionlessValidatorStartTime := time.Now()
	startTime := time.Now().Add(time.Minute)
	addSupernetValidatorTx, err := pWallet.IssueAddPermissionlessValidatorTx(
		&txs.SupernetValidator{
			Validator: txs.Validator{
				NodeID: genesis.LocalConfig.InitialStakers[0].NodeID,
				Start:  uint64(startTime.Unix()),
				End:    uint64(startTime.Add(5 * time.Second).Unix()),
				Wght:   25 * units.MegaAvax,
			},
			Supernet: createSupernetTxID,
		},
		&signer.Empty{},
		createAssetTx.ID(),
		&secp256k1fx.OutputOwners{},
		&secp256k1fx.OutputOwners{},
		reward.PercentDenominator,
	)
	if err != nil {
		log.Fatalf("failed to issue add supernet validator with: %s\n", err)
		return
	}
	addSupernetValidatorTxID := addSupernetValidatorTx.ID()
	log.Printf("issued add supernet validator transaction %s in %s\n", addSupernetValidatorTxID, time.Since(addPermissionlessValidatorStartTime))

	addPermissionlessDelegatorStartTime := time.Now()
	addSupernetDelegatorTx, err := pWallet.IssueAddPermissionlessDelegatorTx(
		&txs.SupernetValidator{
			Validator: txs.Validator{
				NodeID: genesis.LocalConfig.InitialStakers[0].NodeID,
				Start:  uint64(startTime.Unix()),
				End:    uint64(startTime.Add(5 * time.Second).Unix()),
				Wght:   25 * units.MegaAvax,
			},
			Supernet: createSupernetTxID,
		},
		createAssetTxID,
		&secp256k1fx.OutputOwners{},
	)
	if err != nil {
		log.Fatalf("failed to issue add supernet delegator with: %s\n", err)
		return
	}
	addSupernetDelegatorTxID := addSupernetDelegatorTx.ID()
	log.Printf("issued add supernet validator delegator %s in %s\n", addSupernetDelegatorTxID, time.Since(addPermissionlessDelegatorStartTime))
}
