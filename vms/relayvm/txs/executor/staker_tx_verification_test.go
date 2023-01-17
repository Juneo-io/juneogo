// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/utils"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/timer/mockable"
	"github.com/Juneo-io/juneogo/vms/components/june"
	"github.com/Juneo-io/juneogo/vms/components/verify"
	"github.com/Juneo-io/juneogo/vms/relayvm/config"
	"github.com/Juneo-io/juneogo/vms/relayvm/state"
	"github.com/Juneo-io/juneogo/vms/relayvm/txs"
	"github.com/Juneo-io/juneogo/vms/relayvm/utxo"
	"github.com/Juneo-io/juneogo/vms/relayvm/validator"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
)

func TestVerifyAddPermissionlessValidatorTx(t *testing.T) {
	type test struct {
		name        string
		backendF    func(*gomock.Controller) *Backend
		stateF      func(*gomock.Controller) state.Chain
		sTxF        func() *txs.Tx
		txF         func() *txs.AddPermissionlessValidatorTx
		expectedErr error
	}

	var (
		supernetID          = ids.GenerateTestID()
		customAssetID       = ids.GenerateTestID()
		unsignedTransformTx = &txs.TransformSupernetTx{
			AssetID:           customAssetID,
			MinValidatorStake: 1,
			MaxValidatorStake: 2,
			MinStakeDuration:  3,
			MaxStakeDuration:  4,
			MinDelegationFee:  5,
		}
		transformTx = txs.Tx{
			Unsigned: unsignedTransformTx,
			Creds:    []verify.Verifiable{},
		}
		// This tx already passed syntactic verification.
		verifiedTx = txs.AddPermissionlessValidatorTx{
			BaseTx: txs.BaseTx{
				SyntacticallyVerified: true,
				BaseTx: june.BaseTx{
					NetworkID:    1,
					BlockchainID: ids.GenerateTestID(),
					Outs:         []*june.TransferableOutput{},
					Ins:          []*june.TransferableInput{},
				},
			},
			Validator: validator.Validator{
				NodeID: ids.GenerateTestNodeID(),
				Start:  1,
				End:    1 + uint64(unsignedTransformTx.MinStakeDuration),
				Wght:   unsignedTransformTx.MinValidatorStake,
			},
			Supernet: supernetID,
			StakeOuts: []*june.TransferableOutput{
				{
					Asset: june.Asset{
						ID: customAssetID,
					},
				},
			},
			ValidatorRewardsOwner: &secp256k1fx.OutputOwners{
				Addrs:     []ids.ShortID{ids.GenerateTestShortID()},
				Threshold: 1,
			},
			DelegatorRewardsOwner: &secp256k1fx.OutputOwners{
				Addrs:     []ids.ShortID{ids.GenerateTestShortID()},
				Threshold: 1,
			},
			DelegationShares: 20_000,
		}
		verifiedSignedTx = txs.Tx{
			Unsigned: &verifiedTx,
			Creds:    []verify.Verifiable{},
		}
	)
	verifiedSignedTx.SetBytes([]byte{1}, []byte{2})

	tests := []test{
		{
			name: "fail syntactic verification",
			backendF: func(*gomock.Controller) *Backend {
				return &Backend{
					Ctx: snow.DefaultContextTest(),
				}
			},
			stateF: func(*gomock.Controller) state.Chain {
				return nil
			},
			sTxF: func() *txs.Tx {
				return nil
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				return nil
			},
			expectedErr: txs.ErrNilSignedTx,
		},
		{
			name: "not bootstrapped",
			backendF: func(*gomock.Controller) *Backend {
				return &Backend{
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: &utils.AtomicBool{},
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				return nil
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				return nil
			},
			expectedErr: nil,
		},
		{
			name: "start time too early",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.AtomicBool{}
				bootstrapped.SetValue(true)
				return &Backend{
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetTimestamp().Return(verifiedTx.StartTime())
				return state
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				return &verifiedTx
			},
			expectedErr: errTimestampNotBeforeStartTime,
		},
		{
			name: "weight too low",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.AtomicBool{}
				bootstrapped.SetValue(true)
				return &Backend{
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				state.EXPECT().GetSupernetTransformation(supernetID).Return(&transformTx, nil)
				return state
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				tx := verifiedTx // Note that this copies [verifiedTx]
				tx.Validator.Wght = unsignedTransformTx.MinValidatorStake - 1
				return &tx
			},
			expectedErr: errWeightTooSmall,
		},
		{
			name: "weight too high",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.AtomicBool{}
				bootstrapped.SetValue(true)
				return &Backend{
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				state.EXPECT().GetSupernetTransformation(supernetID).Return(&transformTx, nil)
				return state
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				tx := verifiedTx // Note that this copies [verifiedTx]
				tx.Validator.Wght = unsignedTransformTx.MaxValidatorStake + 1
				return &tx
			},
			expectedErr: errWeightTooLarge,
		},
		{
			name: "insufficient delegation fee",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.AtomicBool{}
				bootstrapped.SetValue(true)
				return &Backend{
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				state.EXPECT().GetSupernetTransformation(supernetID).Return(&transformTx, nil)
				return state
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				tx := verifiedTx // Note that this copies [verifiedTx]
				tx.Validator.Wght = unsignedTransformTx.MaxValidatorStake
				tx.DelegationShares = unsignedTransformTx.MinDelegationFee - 1
				return &tx
			},
			expectedErr: errInsufficientDelegationFee,
		},
		{
			name: "duration too short",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.AtomicBool{}
				bootstrapped.SetValue(true)
				return &Backend{
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				state.EXPECT().GetSupernetTransformation(supernetID).Return(&transformTx, nil)
				return state
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				tx := verifiedTx // Note that this copies [verifiedTx]
				tx.Validator.Wght = unsignedTransformTx.MaxValidatorStake
				tx.DelegationShares = unsignedTransformTx.MinDelegationFee
				// Note the duration is 1 less than the minimum
				tx.Validator.Start = 1
				tx.Validator.End = uint64(unsignedTransformTx.MinStakeDuration)
				return &tx
			},
			expectedErr: errStakeTooShort,
		},
		{
			name: "duration too long",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.AtomicBool{}
				bootstrapped.SetValue(true)
				return &Backend{
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				state.EXPECT().GetSupernetTransformation(supernetID).Return(&transformTx, nil)
				return state
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				tx := verifiedTx // Note that this copies [verifiedTx]
				tx.Validator.Wght = unsignedTransformTx.MaxValidatorStake
				tx.DelegationShares = unsignedTransformTx.MinDelegationFee
				// Note the duration is more than the maximum
				tx.Validator.Start = 1
				tx.Validator.End = 2 + uint64(unsignedTransformTx.MaxStakeDuration)
				return &tx
			},
			expectedErr: errStakeTooLong,
		},
		{
			name: "wrong assetID",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.AtomicBool{}
				bootstrapped.SetValue(true)
				return &Backend{
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				state.EXPECT().GetSupernetTransformation(supernetID).Return(&transformTx, nil)
				return state
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				tx := verifiedTx // Note that this copies [verifiedTx]
				tx.StakeOuts = []*june.TransferableOutput{
					{
						Asset: june.Asset{
							ID: ids.GenerateTestID(),
						},
					},
				}
				return &tx
			},
			expectedErr: errWrongStakedAssetID,
		},
		{
			name: "duplicate validator",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.AtomicBool{}
				bootstrapped.SetValue(true)
				return &Backend{
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				state.EXPECT().GetSupernetTransformation(supernetID).Return(&transformTx, nil)
				// State says validator exists
				state.EXPECT().GetCurrentValidator(supernetID, verifiedTx.NodeID()).Return(nil, nil)
				return state
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				return &verifiedTx
			},
			expectedErr: errDuplicateValidator,
		},
		{
			name: "validator not subset of primary network validator",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.AtomicBool{}
				bootstrapped.SetValue(true)
				return &Backend{
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				mockState := state.NewMockChain(ctrl)
				mockState.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				mockState.EXPECT().GetSupernetTransformation(supernetID).Return(&transformTx, nil)
				mockState.EXPECT().GetCurrentValidator(supernetID, verifiedTx.NodeID()).Return(nil, database.ErrNotFound)
				mockState.EXPECT().GetPendingValidator(supernetID, verifiedTx.NodeID()).Return(nil, database.ErrNotFound)
				// Validator time isn't subset of primary network validator time
				primaryNetworkVdr := &state.Staker{
					StartTime: verifiedTx.StartTime().Add(time.Second),
					EndTime:   verifiedTx.EndTime(),
				}
				mockState.EXPECT().GetCurrentValidator(constants.PrimaryNetworkID, verifiedTx.NodeID()).Return(primaryNetworkVdr, nil)
				return mockState
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				return &verifiedTx
			},
			expectedErr: errValidatorSubset,
		},
		{
			name: "flow check fails",
			backendF: func(ctrl *gomock.Controller) *Backend {
				bootstrapped := &utils.AtomicBool{}
				bootstrapped.SetValue(true)

				flowChecker := utxo.NewMockVerifier(ctrl)
				flowChecker.EXPECT().VerifySpend(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(errors.New("flow check failed"))

				return &Backend{
					FlowChecker: flowChecker,
					Config: &config.Config{
						AddSupernetValidatorFee: 1,
					},
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				mockState := state.NewMockChain(ctrl)
				mockState.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				mockState.EXPECT().GetSupernetTransformation(supernetID).Return(&transformTx, nil)
				mockState.EXPECT().GetCurrentValidator(supernetID, verifiedTx.NodeID()).Return(nil, database.ErrNotFound)
				mockState.EXPECT().GetPendingValidator(supernetID, verifiedTx.NodeID()).Return(nil, database.ErrNotFound)
				primaryNetworkVdr := &state.Staker{
					StartTime: verifiedTx.StartTime(),
					EndTime:   verifiedTx.EndTime(),
				}
				mockState.EXPECT().GetCurrentValidator(constants.PrimaryNetworkID, verifiedTx.NodeID()).Return(primaryNetworkVdr, nil)
				return mockState
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				return &verifiedTx
			},
			expectedErr: errFlowCheckFailed,
		},
		{
			name: "starts too far in the future",
			backendF: func(ctrl *gomock.Controller) *Backend {
				bootstrapped := &utils.AtomicBool{}
				bootstrapped.SetValue(true)

				flowChecker := utxo.NewMockVerifier(ctrl)
				flowChecker.EXPECT().VerifySpend(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(nil)

				return &Backend{
					FlowChecker: flowChecker,
					Config: &config.Config{
						AddSupernetValidatorFee: 1,
					},
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				mockState := state.NewMockChain(ctrl)
				mockState.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				mockState.EXPECT().GetSupernetTransformation(supernetID).Return(&transformTx, nil)
				mockState.EXPECT().GetCurrentValidator(supernetID, verifiedTx.NodeID()).Return(nil, database.ErrNotFound)
				mockState.EXPECT().GetPendingValidator(supernetID, verifiedTx.NodeID()).Return(nil, database.ErrNotFound)
				primaryNetworkVdr := &state.Staker{
					StartTime: time.Unix(0, 0),
					EndTime:   mockable.MaxTime,
				}
				mockState.EXPECT().GetCurrentValidator(constants.PrimaryNetworkID, verifiedTx.NodeID()).Return(primaryNetworkVdr, nil)
				return mockState
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				// Note this copies [verifiedTx]
				tx := verifiedTx
				tx.Validator.Start = uint64(MaxFutureStartTime.Seconds()) + 1
				tx.Validator.End = tx.Validator.Start + uint64(unsignedTransformTx.MinStakeDuration)
				return &tx
			},
			expectedErr: errFutureStakeTime,
		},
		{
			name: "success",
			backendF: func(ctrl *gomock.Controller) *Backend {
				bootstrapped := &utils.AtomicBool{}
				bootstrapped.SetValue(true)

				flowChecker := utxo.NewMockVerifier(ctrl)
				flowChecker.EXPECT().VerifySpend(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(nil)

				return &Backend{
					FlowChecker: flowChecker,
					Config: &config.Config{
						AddSupernetValidatorFee: 1,
					},
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				mockState := state.NewMockChain(ctrl)
				mockState.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				mockState.EXPECT().GetSupernetTransformation(supernetID).Return(&transformTx, nil)
				mockState.EXPECT().GetCurrentValidator(supernetID, verifiedTx.NodeID()).Return(nil, database.ErrNotFound)
				mockState.EXPECT().GetPendingValidator(supernetID, verifiedTx.NodeID()).Return(nil, database.ErrNotFound)
				primaryNetworkVdr := &state.Staker{
					StartTime: time.Unix(0, 0),
					EndTime:   mockable.MaxTime,
				}
				mockState.EXPECT().GetCurrentValidator(constants.PrimaryNetworkID, verifiedTx.NodeID()).Return(primaryNetworkVdr, nil)
				return mockState
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				return &verifiedTx
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var (
				backend = tt.backendF(ctrl)
				state   = tt.stateF(ctrl)
				sTx     = tt.sTxF()
				tx      = tt.txF()
			)

			err := verifyAddPermissionlessValidatorTx(backend, state, sTx, tx)
			require.ErrorIs(err, tt.expectedErr)
		})
	}
}

func TestGetValidatorRules(t *testing.T) {
	type test struct {
		name          string
		supernetID    ids.ID
		backend       *Backend
		chainStateF   func(*gomock.Controller) state.Chain
		expectedRules *addValidatorRules
		expectedErr   error
	}

	var (
		config = &config.Config{
			MinValidatorStake: 1,
			MaxValidatorStake: 2,
			MinStakeDuration:  time.Second,
			MaxStakeDuration:  2 * time.Second,
			MinDelegationFee:  1337,
		}
		juneAssetID   = ids.GenerateTestID()
		customAssetID = ids.GenerateTestID()
		supernetID    = ids.GenerateTestID()
		testErr       = errors.New("an error")
	)

	tests := []test{
		{
			name:       "primary network",
			supernetID: constants.PrimaryNetworkID,
			backend: &Backend{
				Config: config,
				Ctx: &snow.Context{
					JuneAssetID: juneAssetID,
				},
			},
			chainStateF: func(*gomock.Controller) state.Chain {
				return nil
			},
			expectedRules: &addValidatorRules{
				assetID:           juneAssetID,
				minValidatorStake: config.MinValidatorStake,
				maxValidatorStake: config.MaxValidatorStake,
				minStakeDuration:  config.MinStakeDuration,
				maxStakeDuration:  config.MaxStakeDuration,
				minDelegationFee:  config.MinDelegationFee,
			},
		},
		{
			name:       "can't get supernet transformation",
			supernetID: supernetID,
			backend:    nil,
			chainStateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetSupernetTransformation(supernetID).Return(nil, testErr)
				return state
			},
			expectedRules: &addValidatorRules{},
			expectedErr:   testErr,
		},
		{
			name:       "invalid transformation tx",
			supernetID: supernetID,
			backend:    nil,
			chainStateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				tx := &txs.Tx{
					Unsigned: &txs.AddDelegatorTx{},
				}
				state.EXPECT().GetSupernetTransformation(supernetID).Return(tx, nil)
				return state
			},
			expectedRules: &addValidatorRules{},
			expectedErr:   errIsNotTransformSupernetTx,
		},
		{
			name:       "supernet",
			supernetID: supernetID,
			backend:    nil,
			chainStateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				tx := &txs.Tx{
					Unsigned: &txs.TransformSupernetTx{
						AssetID:           customAssetID,
						MinValidatorStake: config.MinValidatorStake,
						MaxValidatorStake: config.MaxValidatorStake,
						MinStakeDuration:  1337,
						MaxStakeDuration:  42,
						MinDelegationFee:  config.MinDelegationFee,
					},
				}
				state.EXPECT().GetSupernetTransformation(supernetID).Return(tx, nil)
				return state
			},
			expectedRules: &addValidatorRules{
				assetID:           customAssetID,
				minValidatorStake: config.MinValidatorStake,
				maxValidatorStake: config.MaxValidatorStake,
				minStakeDuration:  time.Duration(1337) * time.Second,
				maxStakeDuration:  time.Duration(42) * time.Second,
				minDelegationFee:  config.MinDelegationFee,
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			chainState := tt.chainStateF(ctrl)
			rules, err := getValidatorRules(tt.backend, chainState, tt.supernetID)
			if tt.expectedErr != nil {
				require.ErrorIs(tt.expectedErr, err)
				return
			}
			require.NoError(err)
			require.Equal(tt.expectedRules, rules)
		})
	}
}

func TestGetDelegatorRules(t *testing.T) {
	type test struct {
		name          string
		supernetID    ids.ID
		backend       *Backend
		chainStateF   func(*gomock.Controller) state.Chain
		expectedRules *addDelegatorRules
		expectedErr   error
	}
	var (
		config = &config.Config{
			MinDelegatorStake: 1,
			MaxValidatorStake: 2,
			MinStakeDuration:  time.Second,
			MaxStakeDuration:  2 * time.Second,
		}
		juneAssetID   = ids.GenerateTestID()
		customAssetID = ids.GenerateTestID()
		supernetID    = ids.GenerateTestID()
		testErr       = errors.New("an error")
	)
	tests := []test{
		{
			name:       "primary network",
			supernetID: constants.PrimaryNetworkID,
			backend: &Backend{
				Config: config,
				Ctx: &snow.Context{
					JuneAssetID: juneAssetID,
				},
			},
			chainStateF: func(*gomock.Controller) state.Chain {
				return nil
			},
			expectedRules: &addDelegatorRules{
				assetID:                  juneAssetID,
				minDelegatorStake:        config.MinDelegatorStake,
				maxValidatorStake:        config.MaxValidatorStake,
				minStakeDuration:         config.MinStakeDuration,
				maxStakeDuration:         config.MaxStakeDuration,
				maxValidatorWeightFactor: MaxValidatorWeightFactor,
			},
		},
		{
			name:       "can't get supernet transformation",
			supernetID: supernetID,
			backend:    nil,
			chainStateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetSupernetTransformation(supernetID).Return(nil, testErr)
				return state
			},
			expectedRules: &addDelegatorRules{},
			expectedErr:   testErr,
		},
		{
			name:       "invalid transformation tx",
			supernetID: supernetID,
			backend:    nil,
			chainStateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				tx := &txs.Tx{
					Unsigned: &txs.AddDelegatorTx{},
				}
				state.EXPECT().GetSupernetTransformation(supernetID).Return(tx, nil)
				return state
			},
			expectedRules: &addDelegatorRules{},
			expectedErr:   errIsNotTransformSupernetTx,
		},
		{
			name:       "supernet",
			supernetID: supernetID,
			backend:    nil,
			chainStateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				tx := &txs.Tx{
					Unsigned: &txs.TransformSupernetTx{
						AssetID:                  customAssetID,
						MinDelegatorStake:        config.MinDelegatorStake,
						MinValidatorStake:        config.MinValidatorStake,
						MaxValidatorStake:        config.MaxValidatorStake,
						MinStakeDuration:         1337,
						MaxStakeDuration:         42,
						MinDelegationFee:         config.MinDelegationFee,
						MaxValidatorWeightFactor: 21,
					},
				}
				state.EXPECT().GetSupernetTransformation(supernetID).Return(tx, nil)
				return state
			},
			expectedRules: &addDelegatorRules{
				assetID:                  customAssetID,
				minDelegatorStake:        config.MinDelegatorStake,
				maxValidatorStake:        config.MaxValidatorStake,
				minStakeDuration:         time.Duration(1337) * time.Second,
				maxStakeDuration:         time.Duration(42) * time.Second,
				maxValidatorWeightFactor: 21,
			},
			expectedErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			chainState := tt.chainStateF(ctrl)
			rules, err := getDelegatorRules(tt.backend, chainState, tt.supernetID)
			if tt.expectedErr != nil {
				require.ErrorIs(tt.expectedErr, err)
				return
			}
			require.NoError(err)
			require.Equal(tt.expectedRules, rules)
		})
	}
}
