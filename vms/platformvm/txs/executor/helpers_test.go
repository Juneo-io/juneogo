// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/chains"
	"github.com/Juneo-io/juneogo/chains/atomic"
	"github.com/Juneo-io/juneogo/codec"
	"github.com/Juneo-io/juneogo/codec/linearcodec"
	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/database/memdb"
	"github.com/Juneo-io/juneogo/database/versiondb"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/snow/snowtest"
	"github.com/Juneo-io/juneogo/snow/uptime"
	"github.com/Juneo-io/juneogo/snow/validators"
	"github.com/Juneo-io/juneogo/utils"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/crypto/secp256k1"
	"github.com/Juneo-io/juneogo/utils/formatting"
	"github.com/Juneo-io/juneogo/utils/formatting/address"
	"github.com/Juneo-io/juneogo/utils/json"
	"github.com/Juneo-io/juneogo/utils/logging"
	"github.com/Juneo-io/juneogo/utils/timer/mockable"
	"github.com/Juneo-io/juneogo/utils/units"
	"github.com/Juneo-io/juneogo/vms/platformvm/api"
	"github.com/Juneo-io/juneogo/vms/platformvm/config"
	"github.com/Juneo-io/juneogo/vms/platformvm/fx"
	"github.com/Juneo-io/juneogo/vms/platformvm/metrics"
	"github.com/Juneo-io/juneogo/vms/platformvm/reward"
	"github.com/Juneo-io/juneogo/vms/platformvm/state"
	"github.com/Juneo-io/juneogo/vms/platformvm/status"
	"github.com/Juneo-io/juneogo/vms/platformvm/txs"
	"github.com/Juneo-io/juneogo/vms/platformvm/txs/txstest"
	"github.com/Juneo-io/juneogo/vms/platformvm/utxo"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
	"github.com/Juneo-io/juneogo/wallet/supernet/primary/common"
)

const (
	defaultWeight = 5 * units.MilliAvax
	trackChecksum = false

	apricotPhase3 fork = iota
	apricotPhase5
	banff
	cortina
	durango
	eUpgrade
)

var (
	defaultMinStakingDuration = 24 * time.Hour
	defaultMaxStakingDuration = 365 * 24 * time.Hour
	defaultGenesisTime        = time.Date(1997, 1, 1, 0, 0, 0, 0, time.UTC)
	defaultValidateStartTime  = defaultGenesisTime
	defaultValidateEndTime    = defaultValidateStartTime.Add(20 * defaultMinStakingDuration)
	defaultMinValidatorStake  = 5 * units.MilliAvax
	defaultBalance            = 100 * defaultMinValidatorStake
	preFundedKeys             = secp256k1.TestKeys()
	defaultTxFee              = uint64(100)
	lastAcceptedID            = ids.GenerateTestID()

	testSupernet1            *txs.Tx
	testSupernet1ControlKeys = preFundedKeys[0:3]

	// Node IDs of genesis validators. Initialized in init function
	genesisNodeIDs []ids.NodeID
)

func init() {
	genesisNodeIDs = make([]ids.NodeID, len(preFundedKeys))
	for i := range preFundedKeys {
		genesisNodeIDs[i] = ids.GenerateTestNodeID()
	}
}

type fork uint8

type mutableSharedMemory struct {
	atomic.SharedMemory
}

type environment struct {
	isBootstrapped *utils.Atomic[bool]
	config         *config.Config
	clk            *mockable.Clock
	baseDB         *versiondb.Database
	ctx            *snow.Context
	msm            *mutableSharedMemory
	fx             fx.Fx
	state          state.State
	states         map[ids.ID]state.Chain
	uptimes        uptime.Manager
	utxosHandler   utxo.Verifier
	txBuilder      *txstest.Builder
	backend        Backend
}

func (e *environment) GetState(blkID ids.ID) (state.Chain, bool) {
	if blkID == lastAcceptedID {
		return e.state, true
	}
	chainState, ok := e.states[blkID]
	return chainState, ok
}

func (e *environment) SetState(blkID ids.ID, chainState state.Chain) {
	e.states[blkID] = chainState
}

func newEnvironment(t *testing.T, f fork) *environment {
	var isBootstrapped utils.Atomic[bool]
	isBootstrapped.Set(true)

	config := defaultConfig(t, f)
	clk := defaultClock(f)

	baseDB := versiondb.New(memdb.New())
	ctx := snowtest.Context(t, snowtest.PChainID)
	m := atomic.NewMemory(baseDB)
	msm := &mutableSharedMemory{
		SharedMemory: m.NewSharedMemory(ctx.ChainID),
	}
	ctx.SharedMemory = msm

	fx := defaultFx(clk, ctx.Log, isBootstrapped.Get())

	rewards := reward.NewCalculator(config.RewardConfig)
	baseState := defaultState(config, ctx, baseDB, rewards)

	uptimes := uptime.NewManager(baseState, clk)
	utxosVerifier := utxo.NewVerifier(ctx, clk, fx)

	txBuilder := txstest.NewBuilder(
		ctx,
		config,
		baseState,
	)

	backend := Backend{
		Config:       config,
		Ctx:          ctx,
		Clk:          clk,
		Bootstrapped: &isBootstrapped,
		Fx:           fx,
		FlowChecker:  utxosVerifier,
		Uptimes:      uptimes,
		Rewards:      rewards,
	}

	env := &environment{
		isBootstrapped: &isBootstrapped,
		config:         config,
		clk:            clk,
		baseDB:         baseDB,
		ctx:            ctx,
		msm:            msm,
		fx:             fx,
		state:          baseState,
		states:         make(map[ids.ID]state.Chain),
		uptimes:        uptimes,
		utxosHandler:   utxosVerifier,
		txBuilder:      txBuilder,
		backend:        backend,
	}

	addSupernet(t, env)

	t.Cleanup(func() {
		env.ctx.Lock.Lock()
		defer env.ctx.Lock.Unlock()

		require := require.New(t)

		if env.isBootstrapped.Get() {
			validatorIDs := env.config.Validators.GetValidatorIDs(constants.PrimaryNetworkID)

			require.NoError(env.uptimes.StopTracking(validatorIDs, constants.PrimaryNetworkID))

			for supernetID := range env.config.TrackedSupernets {
				validatorIDs := env.config.Validators.GetValidatorIDs(supernetID)

				require.NoError(env.uptimes.StopTracking(validatorIDs, supernetID))
			}
			env.state.SetHeight(math.MaxUint64)
			require.NoError(env.state.Commit())
		}

		require.NoError(env.state.Close())
		require.NoError(env.baseDB.Close())
	})

	return env
}

func addSupernet(t *testing.T, env *environment) {
	require := require.New(t)

	// Create a supernet
	var err error
	testSupernet1, err = env.txBuilder.NewCreateSupernetTx(
		&secp256k1fx.OutputOwners{
			Threshold: 2,
			Addrs: []ids.ShortID{
				preFundedKeys[0].PublicKey().Address(),
				preFundedKeys[1].PublicKey().Address(),
				preFundedKeys[2].PublicKey().Address(),
			},
		},
		[]*secp256k1.PrivateKey{preFundedKeys[0]},
		common.WithChangeOwner(&secp256k1fx.OutputOwners{
			Threshold: 1,
			Addrs:     []ids.ShortID{preFundedKeys[0].PublicKey().Address()},
		}),
	)
	require.NoError(err)

	// store it
	stateDiff, err := state.NewDiff(lastAcceptedID, env)
	require.NoError(err)

	executor := StandardTxExecutor{
		Backend: &env.backend,
		State:   stateDiff,
		Tx:      testSupernet1,
	}
	require.NoError(testSupernet1.Unsigned.Visit(&executor))

	stateDiff.AddTx(testSupernet1, status.Committed)
	require.NoError(stateDiff.Apply(env.state))
	require.NoError(env.state.Commit())
}

func defaultState(
	cfg *config.Config,
	ctx *snow.Context,
	db database.Database,
	rewards reward.Calculator,
) state.State {
	genesisBytes := buildGenesisTest(ctx)
	execCfg, _ := config.GetExecutionConfig(nil)
	state, err := state.New(
		db,
		genesisBytes,
		prometheus.NewRegistry(),
		cfg,
		execCfg,
		ctx,
		metrics.Noop,
		rewards,
	)
	if err != nil {
		panic(err)
	}

	// persist and reload to init a bunch of in-memory stuff
	state.SetHeight(0)
	if err := state.Commit(); err != nil {
		panic(err)
	}
	lastAcceptedID = state.GetLastAccepted()
	return state
}

func defaultConfig(t *testing.T, f fork) *config.Config {
	c := &config.Config{
		Chains:                 chains.TestManager,
		UptimeLockedCalculator: uptime.NewLockedCalculator(),
		Validators:             validators.NewManager(),
		TxFee:                  defaultTxFee,
		CreateSupernetTxFee:      100 * defaultTxFee,
		CreateBlockchainTxFee:  100 * defaultTxFee,
		MinValidatorStake:      5 * units.MilliAvax,
		MaxValidatorStake:      500 * units.MilliAvax,
		MinDelegatorStake:      1 * units.MilliAvax,
		MaxDelegationFee:       reward.PercentDenominator,
		MinStakeDuration:       defaultMinStakingDuration,
		MaxStakeDuration:       defaultMaxStakingDuration,
		RewardConfig: reward.Config{
			MinStakePeriod:         defaultMinStakingDuration,
			MaxStakePeriod:         defaultMaxStakingDuration,
			StakePeriodRewardShare: 2_0000,
			StartRewardShare:       12_0000,
			StartRewardTime:        uint64(time.Date(2024, time.April, 1, 0, 0, 0, 0, time.UTC).Unix()),
			DiminishingRewardShare: 8_0000,
			DiminishingRewardTime:  uint64(time.Date(2029, time.April, 1, 0, 0, 0, 0, time.UTC).Unix()),
			TargetRewardShare:      6_0000,
			TargetRewardTime:       uint64(time.Date(2030, time.April, 1, 0, 0, 0, 0, time.UTC).Unix()),
		},
		ApricotPhase3Time: mockable.MaxTime,
		ApricotPhase5Time: mockable.MaxTime,
		BanffTime:         mockable.MaxTime,
		CortinaTime:       mockable.MaxTime,
		DurangoTime:       mockable.MaxTime,
		EUpgradeTime:      mockable.MaxTime,
	}

	switch f {
	case eUpgrade:
		c.EUpgradeTime = defaultValidateStartTime.Add(-2 * time.Second)
		fallthrough
	case durango:
		c.DurangoTime = defaultValidateStartTime.Add(-2 * time.Second)
		fallthrough
	case cortina:
		c.CortinaTime = defaultValidateStartTime.Add(-2 * time.Second)
		fallthrough
	case banff:
		c.BanffTime = defaultValidateStartTime.Add(-2 * time.Second)
		fallthrough
	case apricotPhase5:
		c.ApricotPhase5Time = defaultValidateEndTime
		fallthrough
	case apricotPhase3:
		c.ApricotPhase3Time = defaultValidateEndTime
	default:
		require.FailNow(t, "unhandled fork", f)
	}

	return c
}

func defaultClock(f fork) *mockable.Clock {
	now := defaultGenesisTime
	if f >= banff {
		// 1 second after active fork
		now = defaultValidateEndTime.Add(-2 * time.Second)
	}
	clk := &mockable.Clock{}
	clk.Set(now)
	return clk
}

type fxVMInt struct {
	registry codec.Registry
	clk      *mockable.Clock
	log      logging.Logger
}

func (fvi *fxVMInt) CodecRegistry() codec.Registry {
	return fvi.registry
}

func (fvi *fxVMInt) Clock() *mockable.Clock {
	return fvi.clk
}

func (fvi *fxVMInt) Logger() logging.Logger {
	return fvi.log
}

func defaultFx(clk *mockable.Clock, log logging.Logger, isBootstrapped bool) fx.Fx {
	fxVMInt := &fxVMInt{
		registry: linearcodec.NewDefault(),
		clk:      clk,
		log:      log,
	}
	res := &secp256k1fx.Fx{}
	if err := res.Initialize(fxVMInt); err != nil {
		panic(err)
	}
	if isBootstrapped {
		if err := res.Bootstrapped(); err != nil {
			panic(err)
		}
	}
	return res
}

func buildGenesisTest(ctx *snow.Context) []byte {
	genesisUTXOs := make([]api.UTXO, len(preFundedKeys))
	for i, key := range preFundedKeys {
		id := key.PublicKey().Address()
		addr, err := address.FormatBech32(constants.UnitTestHRP, id.Bytes())
		if err != nil {
			panic(err)
		}
		genesisUTXOs[i] = api.UTXO{
			Amount:  json.Uint64(defaultBalance),
			Address: addr,
		}
	}

	genesisValidators := make([]api.GenesisPermissionlessValidator, len(genesisNodeIDs))
	for i, nodeID := range genesisNodeIDs {
		addr, err := address.FormatBech32(constants.UnitTestHRP, nodeID.Bytes())
		if err != nil {
			panic(err)
		}
		genesisValidators[i] = api.GenesisPermissionlessValidator{
			GenesisValidator: api.GenesisValidator{
				StartTime: json.Uint64(defaultValidateStartTime.Unix()),
				EndTime:   json.Uint64(defaultValidateEndTime.Unix()),
				NodeID:    nodeID,
			},
			RewardOwner: &api.Owner{
				Threshold: 1,
				Addresses: []string{addr},
			},
			Staked: []api.UTXO{{
				Amount:  json.Uint64(defaultWeight),
				Address: addr,
			}},
			DelegationFee: reward.PercentDenominator,
		}
	}

	buildGenesisArgs := api.BuildGenesisArgs{
		NetworkID:     json.Uint32(constants.UnitTestID),
		AvaxAssetID:   ctx.JUNEAssetID,
		UTXOs:         genesisUTXOs,
		Validators:    genesisValidators,
		Chains:        nil,
		Time:          json.Uint64(defaultGenesisTime.Unix()),
		InitialSupply: json.Uint64(360 * units.MegaAvax),
		Encoding:      formatting.Hex,
	}

	buildGenesisResponse := api.BuildGenesisReply{}
	platformvmSS := api.StaticService{}
	if err := platformvmSS.BuildGenesis(nil, &buildGenesisArgs, &buildGenesisResponse); err != nil {
		panic(fmt.Errorf("problem while building platform chain's genesis state: %w", err))
	}

	genesisBytes, err := formatting.Decode(buildGenesisResponse.Encoding, buildGenesisResponse.Bytes)
	if err != nil {
		panic(err)
	}

	return genesisBytes
}
