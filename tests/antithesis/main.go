// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"crypto/rand"
	"log"
	"math/big"
	"os"
	"time"

	"github.com/Juneo-io/juneogo/api/health"
	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/genesis"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow/choices"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/crypto/secp256k1"
	"github.com/Juneo-io/juneogo/utils/set"
	"github.com/Juneo-io/juneogo/utils/units"
	"github.com/Juneo-io/juneogo/vms/avm"
	"github.com/Juneo-io/juneogo/vms/components/avax"
	"github.com/Juneo-io/juneogo/vms/components/verify"
	"github.com/Juneo-io/juneogo/vms/platformvm"
	"github.com/Juneo-io/juneogo/vms/platformvm/status"
	"github.com/Juneo-io/juneogo/vms/propertyfx"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
	"github.com/Juneo-io/juneogo/wallet/supernet/primary"
	"github.com/Juneo-io/juneogo/wallet/supernet/primary/common"

	xtxs "github.com/Juneo-io/juneogo/vms/avm/txs"
	ptxs "github.com/Juneo-io/juneogo/vms/platformvm/txs"
	xbuilder "github.com/Juneo-io/juneogo/wallet/chain/x/builder"
)

const NumKeys = 5

func main() {
	c, err := NewConfig(os.Args)
	if err != nil {
		log.Fatalf("invalid config: %s", err)
	}

	ctx := context.Background()
	awaitHealthyNodes(ctx, c.URIs)

	kc := secp256k1fx.NewKeychain(genesis.EWOQKey)
	walletSyncStartTime := time.Now()
	wallet, err := primary.MakeWallet(ctx, &primary.WalletConfig{
		URI:          c.URIs[0],
		AVAXKeychain: kc,
		EthKeychain:  kc,
	})
	if err != nil {
		log.Fatalf("failed to initialize wallet: %s", err)
	}
	log.Printf("synced wallet in %s", time.Since(walletSyncStartTime))

	genesisWorkload := &workload{
		id:     0,
		wallet: wallet,
		addrs:  set.Of(genesis.EWOQKey.Address()),
		uris:   c.URIs,
	}

	workloads := make([]*workload, NumKeys)
	workloads[0] = genesisWorkload

	var (
		genesisXWallet  = wallet.X()
		genesisXBuilder = genesisXWallet.Builder()
		genesisXContext = genesisXBuilder.Context()
		avaxAssetID     = genesisXContext.AVAXAssetID
	)
	for i := 1; i < NumKeys; i++ {
		key, err := secp256k1.NewPrivateKey()
		if err != nil {
			log.Fatalf("failed to generate key: %s", err)
		}

		var (
			addr          = key.Address()
			baseStartTime = time.Now()
		)
		baseTx, err := genesisXWallet.IssueBaseTx([]*avax.TransferableOutput{{
			Asset: avax.Asset{
				ID: avaxAssetID,
			},
			Out: &secp256k1fx.TransferOutput{
				Amt: 100 * units.KiloAvax,
				OutputOwners: secp256k1fx.OutputOwners{
					Threshold: 1,
					Addrs: []ids.ShortID{
						addr,
					},
				},
			},
		}})
		if err != nil {
			log.Printf("failed to issue initial funding X-chain baseTx: %s", err)
			return
		}
		log.Printf("issued initial funding X-chain baseTx %s in %s", baseTx.ID(), time.Since(baseStartTime))

		genesisWorkload.confirmXChainTx(ctx, baseTx)

		uri := c.URIs[i%len(c.URIs)]
		kc := secp256k1fx.NewKeychain(key)
		walletSyncStartTime := time.Now()
		wallet, err := primary.MakeWallet(ctx, &primary.WalletConfig{
			URI:          uri,
			AVAXKeychain: kc,
			EthKeychain:  kc,
		})
		if err != nil {
			log.Fatalf("failed to initialize wallet: %s", err)
		}
		log.Printf("synced wallet in %s", time.Since(walletSyncStartTime))

		workloads[i] = &workload{
			id:     i,
			wallet: wallet,
			addrs:  set.Of(addr),
			uris:   c.URIs,
		}
	}

	for _, w := range workloads[1:] {
		go w.run(ctx)
	}
	genesisWorkload.run(ctx)
}

func awaitHealthyNodes(ctx context.Context, uris []string) {
	for _, uri := range uris {
		awaitHealthyNode(ctx, uri)
	}
	log.Println("all nodes reported healthy")
}

func awaitHealthyNode(ctx context.Context, uri string) {
	client := health.NewClient(uri)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	log.Printf("awaiting node health at %s", uri)
	for {
		res, err := client.Health(ctx, nil)
		switch {
		case err != nil:
			log.Printf("node couldn't be reached at %s", uri)
		case res.Healthy:
			log.Printf("node reported healthy at %s", uri)
			return
		default:
			log.Printf("node reported unhealthy at %s", uri)
		}

		select {
		case <-ticker.C:
		case <-ctx.Done():
			log.Printf("node health check cancelled at %s", uri)
		}
	}
}

type workload struct {
	id     int
	wallet primary.Wallet
	addrs  set.Set[ids.ShortID]
	uris   []string
}

func (w *workload) run(ctx context.Context) {
	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}

	var (
		xWallet  = w.wallet.X()
		xBuilder = xWallet.Builder()
		pWallet  = w.wallet.P()
		pBuilder = pWallet.Builder()
	)
	xBalances, err := xBuilder.GetFTBalance()
	if err != nil {
		log.Fatalf("failed to fetch X-chain balances: %s", err)
	}
	pBalances, err := pBuilder.GetBalance()
	if err != nil {
		log.Fatalf("failed to fetch P-chain balances: %s", err)
	}
	var (
		xContext    = xBuilder.Context()
		avaxAssetID = xContext.AVAXAssetID
		xAVAX       = xBalances[avaxAssetID]
		pAVAX       = pBalances[avaxAssetID]
	)
	log.Printf("wallet starting with %d X-chain nAVAX and %d P-chain nAVAX", xAVAX, pAVAX)

	for {
		val, err := rand.Int(rand.Reader, big.NewInt(5))
		if err != nil {
			log.Fatalf("failed to read randomness: %s", err)
		}

		flowID := val.Int64()
		log.Printf("wallet %d executing flow %d", w.id, flowID)
		switch flowID {
		case 0:
			w.issueXChainBaseTx(ctx)
		case 1:
			w.issueXChainCreateAssetTx(ctx)
		case 2:
			w.issueXChainOperationTx(ctx)
		case 3:
			w.issueXToPTransfer(ctx)
		case 4:
			w.issuePToXTransfer(ctx)
		}

		val, err = rand.Int(rand.Reader, big.NewInt(int64(time.Second)))
		if err != nil {
			log.Fatalf("failed to read randomness: %s", err)
		}

		timer.Reset(time.Duration(val.Int64()))
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
	}
}

func (w *workload) issueXChainBaseTx(ctx context.Context) {
	var (
		xWallet  = w.wallet.X()
		xBuilder = xWallet.Builder()
	)
	balances, err := xBuilder.GetFTBalance()
	if err != nil {
		log.Printf("failed to fetch X-chain balances: %s", err)
		return
	}

	var (
		xContext      = xBuilder.Context()
		avaxAssetID   = xContext.AVAXAssetID
		avaxBalance   = balances[avaxAssetID]
		baseTxFee     = xContext.BaseTxFee
		neededBalance = baseTxFee + units.Schmeckle
	)
	if avaxBalance < neededBalance {
		log.Printf("skipping X-chain tx issuance due to insufficient balance: %d < %d", avaxBalance, neededBalance)
		return
	}

	var (
		owner         = w.makeOwner()
		baseStartTime = time.Now()
	)
	baseTx, err := xWallet.IssueBaseTx(
		[]*avax.TransferableOutput{
			{
				Asset: avax.Asset{
					ID: avaxAssetID,
				},
				Out: &secp256k1fx.TransferOutput{
					Amt:          units.Schmeckle,
					OutputOwners: owner,
				},
			},
		},
	)
	if err != nil {
		log.Printf("failed to issue X-chain baseTx: %s", err)
		return
	}
	log.Printf("issued new X-chain baseTx %s in %s", baseTx.ID(), time.Since(baseStartTime))

	w.confirmXChainTx(ctx, baseTx)
	w.verifyXChainTxConsumedUTXOs(ctx, baseTx)
}

func (w *workload) issueXChainCreateAssetTx(ctx context.Context) {
	var (
		xWallet  = w.wallet.X()
		xBuilder = xWallet.Builder()
	)
	balances, err := xBuilder.GetFTBalance()
	if err != nil {
		log.Printf("failed to fetch X-chain balances: %s", err)
		return
	}

	var (
		xContext      = xBuilder.Context()
		avaxAssetID   = xContext.AVAXAssetID
		avaxBalance   = balances[avaxAssetID]
		neededBalance = xContext.CreateAssetTxFee
	)
	if avaxBalance < neededBalance {
		log.Printf("skipping X-chain tx issuance due to insufficient balance: %d < %d", avaxBalance, neededBalance)
		return
	}

	var (
		owner                = w.makeOwner()
		createAssetStartTime = time.Now()
	)
	createAssetTx, err := xWallet.IssueCreateAssetTx(
		"HI",
		"HI",
		1,
		map[uint32][]verify.State{
			0: {
				&secp256k1fx.TransferOutput{
					Amt:          units.Schmeckle,
					OutputOwners: owner,
				},
			},
		},
	)
	if err != nil {
		log.Printf("failed to issue X-chain create asset transaction: %s", err)
		return
	}
	log.Printf("created new X-chain asset %s in %s", createAssetTx.ID(), time.Since(createAssetStartTime))

	w.confirmXChainTx(ctx, createAssetTx)
	w.verifyXChainTxConsumedUTXOs(ctx, createAssetTx)
}

func (w *workload) issueXChainOperationTx(ctx context.Context) {
	var (
		xWallet  = w.wallet.X()
		xBuilder = xWallet.Builder()
	)
	balances, err := xBuilder.GetFTBalance()
	if err != nil {
		log.Printf("failed to fetch X-chain balances: %s", err)
		return
	}

	var (
		xContext         = xBuilder.Context()
		avaxAssetID      = xContext.AVAXAssetID
		avaxBalance      = balances[avaxAssetID]
		createAssetTxFee = xContext.CreateAssetTxFee
		baseTxFee        = xContext.BaseTxFee
		neededBalance    = createAssetTxFee + baseTxFee
	)
	if avaxBalance < neededBalance {
		log.Printf("skipping X-chain tx issuance due to insufficient balance: %d < %d", avaxBalance, neededBalance)
		return
	}

	var (
		owner                = w.makeOwner()
		createAssetStartTime = time.Now()
	)
	createAssetTx, err := xWallet.IssueCreateAssetTx(
		"HI",
		"HI",
		1,
		map[uint32][]verify.State{
			2: {
				&propertyfx.MintOutput{
					OutputOwners: owner,
				},
			},
		},
	)
	if err != nil {
		log.Printf("failed to issue X-chain create asset transaction: %s", err)
		return
	}
	log.Printf("created new X-chain asset %s in %s", createAssetTx.ID(), time.Since(createAssetStartTime))

	operationStartTime := time.Now()
	operationTx, err := xWallet.IssueOperationTxMintProperty(
		createAssetTx.ID(),
		&owner,
	)
	if err != nil {
		log.Printf("failed to issue X-chain operation transaction: %s", err)
		return
	}
	log.Printf("issued X-chain operation tx %s in %s", operationTx.ID(), time.Since(operationStartTime))

	w.confirmXChainTx(ctx, createAssetTx)
	w.verifyXChainTxConsumedUTXOs(ctx, createAssetTx)
	w.confirmXChainTx(ctx, operationTx)
	w.verifyXChainTxConsumedUTXOs(ctx, operationTx)
}

func (w *workload) issueXToPTransfer(ctx context.Context) {
	var (
		xWallet  = w.wallet.X()
		pWallet  = w.wallet.P()
		xBuilder = xWallet.Builder()
	)
	balances, err := xBuilder.GetFTBalance()
	if err != nil {
		log.Printf("failed to fetch X-chain balances: %s", err)
		return
	}

	var (
		xContext      = xBuilder.Context()
		avaxAssetID   = xContext.AVAXAssetID
		avaxBalance   = balances[avaxAssetID]
		xBaseTxFee    = xContext.BaseTxFee
		pBuilder      = pWallet.Builder()
		pContext      = pBuilder.Context()
		pBaseTxFee    = pContext.BaseTxFee
		txFees        = xBaseTxFee + pBaseTxFee
		neededBalance = txFees + units.Avax
	)
	if avaxBalance < neededBalance {
		log.Printf("skipping X-chain tx issuance due to insufficient balance: %d < %d", avaxBalance, neededBalance)
		return
	}

	var (
		owner           = w.makeOwner()
		exportStartTime = time.Now()
	)
	exportTx, err := xWallet.IssueExportTx(
		constants.PlatformChainID,
		[]*avax.TransferableOutput{{
			Asset: avax.Asset{
				ID: avaxAssetID,
			},
			Out: &secp256k1fx.TransferOutput{
				Amt: units.Avax,
			},
		}},
	)
	if err != nil {
		log.Printf("failed to issue X-chain export transaction: %s", err)
		return
	}
	log.Printf("created X-chain export transaction %s in %s", exportTx.ID(), time.Since(exportStartTime))

	var (
		xChainID        = xContext.BlockchainID
		importStartTime = time.Now()
	)
	importTx, err := pWallet.IssueImportTx(
		xChainID,
		&owner,
	)
	if err != nil {
		log.Printf("failed to issue P-chain import transaction: %s", err)
		return
	}
	log.Printf("created P-chain import transaction %s in %s", importTx.ID(), time.Since(importStartTime))

	w.confirmXChainTx(ctx, exportTx)
	w.verifyXChainTxConsumedUTXOs(ctx, exportTx)
	w.confirmPChainTx(ctx, importTx)
	w.verifyPChainTxConsumedUTXOs(ctx, importTx)
}

func (w *workload) issuePToXTransfer(ctx context.Context) {
	var (
		xWallet  = w.wallet.X()
		pWallet  = w.wallet.P()
		xBuilder = xWallet.Builder()
		pBuilder = pWallet.Builder()
	)
	balances, err := pBuilder.GetBalance()
	if err != nil {
		log.Printf("failed to fetch P-chain balances: %s", err)
		return
	}

	var (
		xContext      = xBuilder.Context()
		pContext      = pBuilder.Context()
		avaxAssetID   = pContext.AVAXAssetID
		avaxBalance   = balances[avaxAssetID]
		pBaseTxFee    = pContext.BaseTxFee
		xBaseTxFee    = xContext.BaseTxFee
		txFees        = pBaseTxFee + xBaseTxFee
		neededBalance = txFees + units.Schmeckle
	)
	if avaxBalance < neededBalance {
		log.Printf("skipping P-chain tx issuance due to insufficient balance: %d < %d", avaxBalance, neededBalance)
		return
	}

	var (
		xChainID        = xContext.BlockchainID
		owner           = w.makeOwner()
		exportStartTime = time.Now()
	)
	exportTx, err := pWallet.IssueExportTx(
		xChainID,
		[]*avax.TransferableOutput{{
			Asset: avax.Asset{
				ID: avaxAssetID,
			},
			Out: &secp256k1fx.TransferOutput{
				Amt: units.Schmeckle,
			},
		}},
	)
	if err != nil {
		log.Printf("failed to issue P-chain export transaction: %s", err)
		return
	}
	log.Printf("created P-chain export transaction %s in %s", exportTx.ID(), time.Since(exportStartTime))

	importStartTime := time.Now()
	importTx, err := xWallet.IssueImportTx(
		constants.PlatformChainID,
		&owner,
	)
	if err != nil {
		log.Printf("failed to issue X-chain import transaction: %s", err)
		return
	}
	log.Printf("created X-chain import transaction %s in %s", importTx.ID(), time.Since(importStartTime))

	w.confirmPChainTx(ctx, exportTx)
	w.verifyPChainTxConsumedUTXOs(ctx, exportTx)
	w.confirmXChainTx(ctx, importTx)
	w.verifyXChainTxConsumedUTXOs(ctx, importTx)
}

func (w *workload) makeOwner() secp256k1fx.OutputOwners {
	addr, _ := w.addrs.Peek()
	return secp256k1fx.OutputOwners{
		Threshold: 1,
		Addrs: []ids.ShortID{
			addr,
		},
	}
}

func (w *workload) confirmXChainTx(ctx context.Context, tx *xtxs.Tx) {
	txID := tx.ID()
	for _, uri := range w.uris {
		client := avm.NewClient(uri, "X")
		status, err := client.ConfirmTx(ctx, txID, 100*time.Millisecond)
		if err != nil {
			log.Printf("failed to confirm X-chain transaction %s on %s: %s", txID, uri, err)
			return
		}
		if status != choices.Accepted {
			log.Printf("failed to confirm X-chain transaction %s on %s: status == %s", txID, uri, status)
			return
		}
		log.Printf("confirmed X-chain transaction %s on %s", txID, uri)
	}
	log.Printf("confirmed X-chain transaction %s on all nodes", txID)
}

func (w *workload) confirmPChainTx(ctx context.Context, tx *ptxs.Tx) {
	txID := tx.ID()
	for _, uri := range w.uris {
		client := platformvm.NewClient(uri)
		s, err := client.AwaitTxDecided(ctx, txID, 100*time.Millisecond)
		if err != nil {
			log.Printf("failed to confirm P-chain transaction %s on %s: %s", txID, uri, err)
			return
		}
		if s.Status != status.Committed {
			log.Printf("failed to confirm P-chain transaction %s on %s: status == %s", txID, uri, s.Status)
			return
		}
		log.Printf("confirmed P-chain transaction %s on %s", txID, uri)
	}
	log.Printf("confirmed P-chain transaction %s on all nodes", txID)
}

func (w *workload) verifyXChainTxConsumedUTXOs(ctx context.Context, tx *xtxs.Tx) {
	txID := tx.ID()
	chainID := w.wallet.X().Builder().Context().BlockchainID
	for _, uri := range w.uris {
		client := avm.NewClient(uri, "X")

		utxos := common.NewUTXOs()
		err := primary.AddAllUTXOs(
			ctx,
			utxos,
			client,
			xbuilder.Parser.Codec(),
			chainID,
			chainID,
			w.addrs.List(),
		)
		if err != nil {
			log.Printf("failed to fetch X-chain UTXOs on %s: %s", uri, err)
			return
		}

		inputs := tx.Unsigned.InputIDs()
		for input := range inputs {
			_, err := utxos.GetUTXO(ctx, chainID, chainID, input)
			if err != database.ErrNotFound {
				log.Printf("failed to verify that X-chain UTXO %s was deleted on %s after %s", input, uri, txID)
				return
			}
		}
		log.Printf("confirmed all X-chain UTXOs consumed by %s are not present on %s", txID, uri)
	}
	log.Printf("confirmed all X-chain UTXOs consumed by %s are not present on all nodes", txID)
}

func (w *workload) verifyPChainTxConsumedUTXOs(ctx context.Context, tx *ptxs.Tx) {
	txID := tx.ID()
	for _, uri := range w.uris {
		client := platformvm.NewClient(uri)

		utxos := common.NewUTXOs()
		err := primary.AddAllUTXOs(
			ctx,
			utxos,
			client,
			ptxs.Codec,
			constants.PlatformChainID,
			constants.PlatformChainID,
			w.addrs.List(),
		)
		if err != nil {
			log.Printf("failed to fetch P-chain UTXOs on %s: %s", uri, err)
			return
		}

		inputs := tx.Unsigned.InputIDs()
		for input := range inputs {
			_, err := utxos.GetUTXO(ctx, constants.PlatformChainID, constants.PlatformChainID, input)
			if err != database.ErrNotFound {
				log.Printf("failed to verify that P-chain UTXO %s was deleted on %s after %s", input, uri, txID)
				return
			}
		}
		log.Printf("confirmed all P-chain UTXOs consumed by %s are not present on %s", txID, uri)
	}
	log.Printf("confirmed all P-chain UTXOs consumed by %s are not present on all nodes", txID)
}