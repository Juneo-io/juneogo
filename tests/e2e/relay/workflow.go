// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package relay

import (
	"context"
	"errors"
	"fmt"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"

	"github.com/onsi/gomega"

	"github.com/Juneo-io/juneogo/api/info"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow/choices"
	"github.com/Juneo-io/juneogo/tests"
	"github.com/Juneo-io/juneogo/tests/e2e"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/units"
	"github.com/Juneo-io/juneogo/vms/components/june"
	"github.com/Juneo-io/juneogo/vms/jvm"
	"github.com/Juneo-io/juneogo/vms/relayvm"
	"github.com/Juneo-io/juneogo/vms/relayvm/status"
	"github.com/Juneo-io/juneogo/vms/relayvm/validator"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
	"github.com/Juneo-io/juneogo/wallet/supernet/primary"
	"github.com/Juneo-io/juneogo/wallet/supernet/primary/common"
)

// PChainWorkflow is an integration test for normal P-Chain operations
// - Issues an Add Validator and an Add Delegator using the funding address
// - Exports JUNE from the P-Chain funding address to the X-Chain created address
// - Exports JUNE from the X-Chain created address to the P-Chain created address
// - Checks the expected value of the funding address

var _ = e2e.DescribePChain("[Workflow]", func() {
	ginkgo.It("P-chain main operations",
		// use this for filtering tests by labels
		// ref. https://onsi.github.io/ginkgo/#spec-labels
		ginkgo.Label(
			"require-network-runner",
			"xp",
			"workflow",
		),
		ginkgo.FlakeAttempts(2),
		func() {
			rpcEps := e2e.Env.GetURIs()
			gomega.Expect(rpcEps).ShouldNot(gomega.BeEmpty())
			nodeURI := rpcEps[0]

			tests.Outf("{{blue}} setting up keys {{/}}\n")
			_, testKeyAddrs, keyChain := e2e.Env.GetTestKeys()

			tests.Outf("{{blue}} setting up wallet {{/}}\n")
			ctx, cancel := context.WithTimeout(context.Background(), e2e.DefaultWalletCreationTimeout)
			baseWallet, err := primary.NewWalletFromURI(ctx, nodeURI, keyChain)
			cancel()
			gomega.Expect(err).Should(gomega.BeNil())

			pWallet := baseWallet.Relay()
			juneAssetID := baseWallet.Relay().JuneAssetID()
			xWallet := baseWallet.Asset()
			pChainClient := relayvm.NewClient(nodeURI)
			assetChainClient := jvm.NewClient(nodeURI, xWallet.BlockchainID().String())

			tests.Outf("{{blue}} fetching minimal stake amounts {{/}}\n")
			ctx, cancel = context.WithTimeout(context.Background(), e2e.DefaultWalletCreationTimeout)
			minValStake, minDelStake, err := pChainClient.GetMinStake(ctx, constants.RelayChainID)
			cancel()
			gomega.Expect(err).Should(gomega.BeNil())
			tests.Outf("{{green}} minimal validator stake: %d {{/}}\n", minValStake)
			tests.Outf("{{green}} minimal delegator stake: %d {{/}}\n", minDelStake)

			tests.Outf("{{blue}} fetching tx fee {{/}}\n")
			infoClient := info.NewClient(nodeURI)
			ctx, cancel = context.WithTimeout(context.Background(), e2e.DefaultWalletCreationTimeout)
			fees, err := infoClient.GetTxFee(ctx)
			cancel()
			gomega.Expect(err).Should(gomega.BeNil())
			txFees := uint64(fees.TxFee)
			tests.Outf("{{green}} txFee: %d {{/}}\n", txFees)

			// amount to transfer from P to X chain
			toTransfer := 1 * units.June

			pShortAddr := testKeyAddrs[0]
			xTargetAddr := testKeyAddrs[1]
			ginkgo.By("check selected keys have sufficient funds", func() {
				relayChainBalances, err := pWallet.Builder().GetBalance()
				pBalance := relayChainBalances[juneAssetID]
				minBalance := minValStake + txFees + minDelStake + txFees + toTransfer + txFees
				gomega.Expect(pBalance, err).To(gomega.BeNumerically(">=", minBalance))
			})
			// create validator data
			validatorStartTimeDiff := 30 * time.Second
			vdrStartTime := time.Now().Add(validatorStartTimeDiff)

			vdr := &validator.Validator{
				NodeID: ids.GenerateTestNodeID(),
				Start:  uint64(vdrStartTime.Unix()),
				End:    uint64(vdrStartTime.Add(72 * time.Hour).Unix()),
				Wght:   minValStake,
			}
			rewardOwner := &secp256k1fx.OutputOwners{
				Threshold: 1,
				Addrs:     []ids.ShortID{pShortAddr},
			}
			shares := uint32(20000) // TODO: retrieve programmatically

			ginkgo.By("issue add validator tx", func() {
				ctx, cancel := context.WithTimeout(context.Background(), e2e.DefaultConfirmTxTimeout)
				addValidatorTxID, err := pWallet.IssueAddValidatorTx(
					vdr,
					rewardOwner,
					shares,
					common.WithContext(ctx),
				)
				cancel()
				gomega.Expect(err).Should(gomega.BeNil())

				ctx, cancel = context.WithTimeout(context.Background(), e2e.DefaultConfirmTxTimeout)
				txStatus, err := pChainClient.GetTxStatus(ctx, addValidatorTxID)
				cancel()
				gomega.Expect(txStatus.Status, err).To(gomega.Equal(status.Committed))
			})

			ginkgo.By("issue add delegator tx", func() {
				ctx, cancel := context.WithTimeout(context.Background(), e2e.DefaultConfirmTxTimeout)
				addDelegatorTxID, err := pWallet.IssueAddDelegatorTx(
					vdr,
					rewardOwner,
					common.WithContext(ctx),
				)
				cancel()
				gomega.Expect(err).Should(gomega.BeNil())

				ctx, cancel = context.WithTimeout(context.Background(), e2e.DefaultConfirmTxTimeout)
				txStatus, err := pChainClient.GetTxStatus(ctx, addDelegatorTxID)
				cancel()
				gomega.Expect(txStatus.Status, err).To(gomega.Equal(status.Committed))
			})

			// retrieve initial balances
			relayChainBalances, err := pWallet.Builder().GetBalance()
			gomega.Expect(err).Should(gomega.BeNil())
			pStartBalance := relayChainBalances[juneAssetID]
			tests.Outf("{{blue}} P-chain balance before P->X export: %d {{/}}\n", pStartBalance)

			assetChainBalances, err := xWallet.Builder().GetFTBalance()
			gomega.Expect(err).Should(gomega.BeNil())
			xStartBalance := assetChainBalances[juneAssetID]
			tests.Outf("{{blue}} X-chain balance before P->X export: %d {{/}}\n", xStartBalance)

			outputOwner := secp256k1fx.OutputOwners{
				Threshold: 1,
				Addrs: []ids.ShortID{
					xTargetAddr,
				},
			}
			output := &secp256k1fx.TransferOutput{
				Amt:          toTransfer,
				OutputOwners: outputOwner,
			}

			ginkgo.By("export june from P to X chain", func() {
				ctx, cancel := context.WithTimeout(context.Background(), e2e.DefaultConfirmTxTimeout)
				exportTxID, err := pWallet.IssueExportTx(
					xWallet.BlockchainID(),
					[]*june.TransferableOutput{
						{
							Asset: june.Asset{
								ID: juneAssetID,
							},
							Out: output,
						},
					},
					common.WithContext(ctx),
				)
				cancel()
				gomega.Expect(err).Should(gomega.BeNil())

				ctx, cancel = context.WithTimeout(context.Background(), e2e.DefaultConfirmTxTimeout)
				txStatus, err := pChainClient.GetTxStatus(ctx, exportTxID)
				cancel()
				gomega.Expect(txStatus.Status, err).To(gomega.Equal(status.Committed))
			})

			// check balances post export
			relayChainBalances, err = pWallet.Builder().GetBalance()
			gomega.Expect(err).Should(gomega.BeNil())
			pPreImportBalance := relayChainBalances[juneAssetID]
			tests.Outf("{{blue}} P-chain balance after P->X export: %d {{/}}\n", pPreImportBalance)

			assetChainBalances, err = xWallet.Builder().GetFTBalance()
			gomega.Expect(err).Should(gomega.BeNil())
			xPreImportBalance := assetChainBalances[juneAssetID]
			tests.Outf("{{blue}} X-chain balance after P->X export: %d {{/}}\n", xPreImportBalance)

			gomega.Expect(xPreImportBalance).To(gomega.Equal(xStartBalance)) // import not performed yet
			gomega.Expect(pPreImportBalance).To(gomega.Equal(pStartBalance - toTransfer - txFees))

			ginkgo.By("import june from P into X chain", func() {
				ctx, cancel := context.WithTimeout(context.Background(), e2e.DefaultConfirmTxTimeout)
				importTxID, err := xWallet.IssueImportTx(
					constants.RelayChainID,
					&outputOwner,
					common.WithContext(ctx),
				)
				cancel()
				gomega.Expect(err).Should(gomega.BeNil(), fmt.Errorf("error timeout: %v", errors.Is(err, context.DeadlineExceeded)))

				ctx, cancel = context.WithTimeout(context.Background(), e2e.DefaultConfirmTxTimeout)
				txStatus, err := assetChainClient.GetTxStatus(ctx, importTxID)
				cancel()
				gomega.Expect(txStatus, err).To(gomega.Equal(choices.Accepted))
			})

			// check balances post import
			relayChainBalances, err = pWallet.Builder().GetBalance()
			gomega.Expect(err).Should(gomega.BeNil())
			pFinalBalance := relayChainBalances[juneAssetID]
			tests.Outf("{{blue}} P-chain balance after P->X import: %d {{/}}\n", pFinalBalance)

			assetChainBalances, err = xWallet.Builder().GetFTBalance()
			gomega.Expect(err).Should(gomega.BeNil())
			xFinalBalance := assetChainBalances[juneAssetID]
			tests.Outf("{{blue}} X-chain balance after P->X import: %d {{/}}\n", xFinalBalance)

			gomega.Expect(xFinalBalance).To(gomega.Equal(xPreImportBalance + toTransfer - txFees)) // import not performed yet
			gomega.Expect(pFinalBalance).To(gomega.Equal(pPreImportBalance))
		})
})
