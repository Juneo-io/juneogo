// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
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
	"github.com/Juneo-io/juneogo/vms/components/june"
	"github.com/Juneo-io/juneogo/vms/components/verify"
	"github.com/Juneo-io/juneogo/vms/relayvm/reward"
	"github.com/Juneo-io/juneogo/vms/relayvm/signer"
	"github.com/Juneo-io/juneogo/vms/relayvm/validator"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
)

func ExampleWallet() {
	ctx := context.Background()
	kc := secp256k1fx.NewKeychain(genesis.EWOQKey)

	// NewWalletFromURI fetches the available UTXOs owned by [kc] on the network
	// that [LocalAPIURI] is hosting.
	walletSyncStartTime := time.Now()
	wallet, err := NewWalletFromURI(ctx, LocalAPIURI, kc)
	if err != nil {
		log.Fatalf("failed to initialize wallet with: %s\n", err)
		return
	}
	log.Printf("synced wallet in %s\n", time.Since(walletSyncStartTime))

	// Get the Relay-chain and the Asset-chain wallets
	relayWallet := wallet.Relay()
	assetWallet := wallet.Asset()

	// Pull out useful constants to use when issuing transactions.
	assetChainID := assetWallet.BlockchainID()
	owner := &secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs: []ids.ShortID{
			genesis.EWOQKey.PublicKey().Address(),
		},
	}

	// Create a custom asset to send to the Relay-chain.
	createAssetStartTime := time.Now()
	createAssetTxID, err := assetWallet.IssueCreateAssetTx(
		"RnM",
		"RNM",
		9,
		map[uint32][]verify.State{
			0: {
				&secp256k1fx.TransferOutput{
					Amt:          100 * units.MegaJune,
					OutputOwners: *owner,
				},
			},
		},
	)
	if err != nil {
		log.Fatalf("failed to create new X-chain asset with: %s\n", err)
		return
	}
	log.Printf("created Asset-chain asset %s in %s\n", createAssetTxID, time.Since(createAssetStartTime))

	// Send 100 MegaJune to the Relay-chain.
	exportStartTime := time.Now()
	exportTxID, err := assetWallet.IssueExportTx(
		constants.RelayChainID,
		[]*june.TransferableOutput{
			{
				Asset: june.Asset{
					ID: createAssetTxID,
				},
				Out: &secp256k1fx.TransferOutput{
					Amt:          100 * units.MegaJune,
					OutputOwners: *owner,
				},
			},
		},
	)
	if err != nil {
		log.Fatalf("failed to issue X->P export transaction with: %s\n", err)
		return
	}
	log.Printf("issued X->P export %s in %s\n", exportTxID, time.Since(exportStartTime))

	// Import the 100 MegaJune from the X-chain into the P-chain.
	importStartTime := time.Now()
	importTxID, err := relayWallet.IssueImportTx(assetChainID, owner)
	if err != nil {
		log.Fatalf("failed to issue X->P import transaction with: %s\n", err)
		return
	}
	log.Printf("issued X->P import %s in %s\n", importTxID, time.Since(importStartTime))

	createSupernetStartTime := time.Now()
	createSupernetTxID, err := relayWallet.IssueCreateSupernetTx(owner)
	if err != nil {
		log.Fatalf("failed to issue create supernet transaction with: %s\n", err)
		return
	}
	log.Printf("issued create supernet transaction %s in %s\n", createSupernetTxID, time.Since(createSupernetStartTime))

	transformSupernetStartTime := time.Now()
	transformSupernetTxID, err := relayWallet.IssueTransformSupernetTx(
		createSupernetTxID,
		createAssetTxID,
		50*units.MegaJune,
		50000,
		1,
		100*units.MegaJune,
		time.Second,
		365*24*time.Hour,
		0,
		1,
		5,
		.80*reward.PercentDenominator,
	)
	if err != nil {
		log.Fatalf("failed to issue transform supernet transaction with: %s\n", err)
		return
	}
	log.Printf("issued transform supernet transaction %s in %s\n", transformSupernetTxID, time.Since(transformSupernetStartTime))

	addPermissionlessValidatorStartTime := time.Now()
	startTime := time.Now().Add(time.Minute)
	addSupernetValidatorTxID, err := relayWallet.IssueAddPermissionlessValidatorTx(
		&validator.SupernetValidator{
			Validator: validator.Validator{
				NodeID: genesis.LocalConfig.InitialStakers[0].NodeID,
				Start:  uint64(startTime.Unix()),
				End:    uint64(startTime.Add(5 * time.Second).Unix()),
				Wght:   25 * units.MegaJune,
			},
			Supernet: createSupernetTxID,
		},
		&signer.Empty{},
		createAssetTxID,
		&secp256k1fx.OutputOwners{},
		&secp256k1fx.OutputOwners{},
		reward.PercentDenominator,
	)
	if err != nil {
		log.Fatalf("failed to issue add supernet validator with: %s\n", err)
		return
	}
	log.Printf("issued add supernet validator transaction %s in %s\n", addSupernetValidatorTxID, time.Since(addPermissionlessValidatorStartTime))

	addPermissionlessDelegatorStartTime := time.Now()
	addSupernetDelegatorTxID, err := relayWallet.IssueAddPermissionlessDelegatorTx(
		&validator.SupernetValidator{
			Validator: validator.Validator{
				NodeID: genesis.LocalConfig.InitialStakers[0].NodeID,
				Start:  uint64(startTime.Unix()),
				End:    uint64(startTime.Add(5 * time.Second).Unix()),
				Wght:   25 * units.MegaJune,
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
	log.Printf("issued add supernet validator delegator %s in %s\n", addSupernetDelegatorTxID, time.Since(addPermissionlessDelegatorStartTime))
}
