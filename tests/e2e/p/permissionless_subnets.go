// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package p

import (
	"fmt"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/tests/fixture/e2e"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/units"
	"github.com/Juneo-io/juneogo/vms/components/avax"
	"github.com/Juneo-io/juneogo/vms/components/verify"
	"github.com/Juneo-io/juneogo/vms/platformvm"
	"github.com/Juneo-io/juneogo/vms/platformvm/reward"
	"github.com/Juneo-io/juneogo/vms/platformvm/signer"
	"github.com/Juneo-io/juneogo/vms/platformvm/txs"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"

	ginkgo "github.com/onsi/ginkgo/v2"
)

var _ = e2e.DescribePChain("[Permissionless Supernets]", func() {
	require := require.New(ginkgo.GinkgoT())

	ginkgo.It("supernets operations",
		func() {
			nodeURI := e2e.Env.GetRandomNodeURI()

			keychain := e2e.Env.NewKeychain(1)
			baseWallet := e2e.NewWallet(keychain, nodeURI)

			pWallet := baseWallet.P()
			xWallet := baseWallet.X()
			xBuilder := xWallet.Builder()
			xContext := xBuilder.Context()
			xChainID := xContext.BlockchainID

			var validatorID ids.NodeID
			ginkgo.By("retrieving the node ID of a primary network validator", func() {
				pChainClient := platformvm.NewClient(nodeURI.URI)
				validatorIDs, err := pChainClient.SampleValidators(e2e.DefaultContext(), constants.PrimaryNetworkID, 1)
				require.NoError(err)
				validatorID = validatorIDs[0]
			})

			owner := &secp256k1fx.OutputOwners{
				Threshold: 1,
				Addrs: []ids.ShortID{
					keychain.Keys[0].Address(),
				},
			}

			var supernetID ids.ID
			ginkgo.By("create a permissioned supernet", func() {
				supernetTx, err := pWallet.IssueCreateSupernetTx(
					owner,
					e2e.WithDefaultContext(),
				)

				supernetID = supernetTx.ID()
				require.NoError(err)
				require.NotEqual(supernetID, constants.PrimaryNetworkID)
			})

			var supernetAssetID ids.ID
			ginkgo.By("create a custom asset for the permissionless supernet", func() {
				supernetAssetTx, err := xWallet.IssueCreateAssetTx(
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
					e2e.WithDefaultContext(),
				)
				require.NoError(err)
				supernetAssetID = supernetAssetTx.ID()
			})

			ginkgo.By(fmt.Sprintf("Send 100 MegaAvax of asset %s to the P-chain", supernetAssetID), func() {
				_, err := xWallet.IssueExportTx(
					constants.PlatformChainID,
					[]*avax.TransferableOutput{
						{
							Asset: avax.Asset{
								ID: supernetAssetID,
							},
							Out: &secp256k1fx.TransferOutput{
								Amt:          100 * units.MegaAvax,
								OutputOwners: *owner,
							},
						},
					},
					e2e.WithDefaultContext(),
				)
				require.NoError(err)
			})

			ginkgo.By(fmt.Sprintf("Import the 100 MegaAvax of asset %s from the X-chain into the P-chain", supernetAssetID), func() {
				_, err := pWallet.IssueImportTx(
					xChainID,
					owner,
					e2e.WithDefaultContext(),
				)
				require.NoError(err)
			})

			ginkgo.By("make supernet permissionless", func() {
				_, err := pWallet.IssueTransformSupernetTx(
					supernetID,
					supernetAssetID,
					50*units.MegaAvax,
					100*units.MegaAvax,
					reward.PercentDenominator,
					reward.PercentDenominator,
					1,
					100*units.MegaAvax,
					time.Second,
					365*24*time.Hour,
					0,
					1,
					5,
					.80*reward.PercentDenominator,
					e2e.WithDefaultContext(),
				)
				require.NoError(err)
			})

			endTime := time.Now().Add(time.Minute)
			ginkgo.By("add permissionless validator", func() {
				_, err := pWallet.IssueAddPermissionlessValidatorTx(
					&txs.SupernetValidator{
						Validator: txs.Validator{
							NodeID: validatorID,
							End:    uint64(endTime.Unix()),
							Wght:   25 * units.MegaAvax,
						},
						Supernet: supernetID,
					},
					&signer.Empty{},
					supernetAssetID,
					&secp256k1fx.OutputOwners{},
					&secp256k1fx.OutputOwners{},
					reward.PercentDenominator,
					e2e.WithDefaultContext(),
				)
				require.NoError(err)
			})

			ginkgo.By("add permissionless delegator", func() {
				_, err := pWallet.IssueAddPermissionlessDelegatorTx(
					&txs.SupernetValidator{
						Validator: txs.Validator{
							NodeID: validatorID,
							End:    uint64(endTime.Unix()),
							Wght:   25 * units.MegaAvax,
						},
						Supernet: supernetID,
					},
					supernetAssetID,
					&secp256k1fx.OutputOwners{},
					e2e.WithDefaultContext(),
				)
				require.NoError(err)
			})
		})
})
