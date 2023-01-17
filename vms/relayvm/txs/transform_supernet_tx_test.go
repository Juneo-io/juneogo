// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/require"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/vms/components/june"
	"github.com/Juneo-io/juneogo/vms/components/verify"
	"github.com/Juneo-io/juneogo/vms/relayvm/reward"
)

func TestTransformSupernetTxSyntacticVerify(t *testing.T) {
	type test struct {
		name   string
		txFunc func(*gomock.Controller) *TransformSupernetTx
		err    error
	}

	var (
		networkID              = uint32(1337)
		chainID                = ids.GenerateTestID()
		errInvalidSupernetAuth = errors.New("invalid supernet auth")
	)

	ctx := &snow.Context{
		ChainID:     chainID,
		NetworkID:   networkID,
		JuneAssetID: ids.GenerateTestID(),
	}

	// A BaseTx that already passed syntactic verification.
	verifiedBaseTx := BaseTx{
		SyntacticallyVerified: true,
	}

	// A BaseTx that passes syntactic verification.
	validBaseTx := BaseTx{
		BaseTx: june.BaseTx{
			NetworkID:    networkID,
			BlockchainID: chainID,
		},
	}

	// A BaseTx that fails syntactic verification.
	invalidBaseTx := BaseTx{}

	tests := []test{
		{
			name: "nil tx",
			txFunc: func(*gomock.Controller) *TransformSupernetTx {
				return nil
			},
			err: ErrNilTx,
		},
		{
			name: "already verified",
			txFunc: func(*gomock.Controller) *TransformSupernetTx {
				return &TransformSupernetTx{
					BaseTx: verifiedBaseTx,
				}
			},
			err: nil,
		},
		{
			name: "invalid supernetID",
			txFunc: func(*gomock.Controller) *TransformSupernetTx {
				return &TransformSupernetTx{
					BaseTx:   validBaseTx,
					Supernet: constants.PrimaryNetworkID,
				}
			},
			err: errCantTransformPrimaryNetwork,
		},
		{
			name: "empty assetID",
			txFunc: func(*gomock.Controller) *TransformSupernetTx {
				return &TransformSupernetTx{
					BaseTx:   validBaseTx,
					Supernet: ids.GenerateTestID(),
					AssetID:  ids.Empty,
				}
			},
			err: errEmptyAssetID,
		},
		{
			name: "JUNE assetID",
			txFunc: func(*gomock.Controller) *TransformSupernetTx {
				return &TransformSupernetTx{
					BaseTx:   validBaseTx,
					Supernet: ids.GenerateTestID(),
					AssetID:  ctx.JuneAssetID,
				}
			},
			err: errAssetIDCantBeJUNE,
		},
		{
			name: "minValidatorStake == 0",
			txFunc: func(*gomock.Controller) *TransformSupernetTx {
				return &TransformSupernetTx{
					BaseTx:            validBaseTx,
					Supernet:          ids.GenerateTestID(),
					AssetID:           ids.GenerateTestID(),
					RewardShare:       50000,
					MinValidatorStake: 0,
				}
			},
			err: errMinValidatorStakeZero,
		},
		{
			name: "minValidatorStake > maxValidatorStake",
			txFunc: func(*gomock.Controller) *TransformSupernetTx {
				return &TransformSupernetTx{
					BaseTx:            validBaseTx,
					Supernet:          ids.GenerateTestID(),
					AssetID:           ids.GenerateTestID(),
					RewardShare:       50000,
					MinValidatorStake: 2,
					MaxValidatorStake: 1,
				}
			},
			err: errMinValidatorStakeAboveMax,
		},
		{
			name: "minStakeDuration == 0",
			txFunc: func(*gomock.Controller) *TransformSupernetTx {
				return &TransformSupernetTx{
					BaseTx:            validBaseTx,
					Supernet:          ids.GenerateTestID(),
					AssetID:           ids.GenerateTestID(),
					RewardShare:       50000,
					MinValidatorStake: 2,
					MaxValidatorStake: 10,
					MinStakeDuration:  0,
				}
			},
			err: errMinStakeDurationZero,
		},
		{
			name: "minStakeDuration > maxStakeDuration",
			txFunc: func(*gomock.Controller) *TransformSupernetTx {
				return &TransformSupernetTx{
					BaseTx:            validBaseTx,
					Supernet:          ids.GenerateTestID(),
					AssetID:           ids.GenerateTestID(),
					RewardShare:       50000,
					MinValidatorStake: 2,
					MaxValidatorStake: 10,
					MinStakeDuration:  2,
					MaxStakeDuration:  1,
				}
			},
			err: errMinStakeDurationTooLarge,
		},
		{
			name: "minDelegationFee > 100%",
			txFunc: func(*gomock.Controller) *TransformSupernetTx {
				return &TransformSupernetTx{
					BaseTx:            validBaseTx,
					Supernet:          ids.GenerateTestID(),
					AssetID:           ids.GenerateTestID(),
					RewardShare:       50000,
					MinValidatorStake: 2,
					MaxValidatorStake: 10,
					MinStakeDuration:  1,
					MaxStakeDuration:  2,
					MinDelegationFee:  reward.PercentDenominator + 1,
				}
			},
			err: errMinDelegationFeeTooLarge,
		},
		{
			name: "minDelegatorStake == 0",
			txFunc: func(*gomock.Controller) *TransformSupernetTx {
				return &TransformSupernetTx{
					BaseTx:            validBaseTx,
					Supernet:          ids.GenerateTestID(),
					AssetID:           ids.GenerateTestID(),
					RewardShare:       50000,
					MinValidatorStake: 2,
					MaxValidatorStake: 10,
					MinStakeDuration:  1,
					MaxStakeDuration:  2,
					MinDelegationFee:  reward.PercentDenominator,
					MinDelegatorStake: 0,
				}
			},
			err: errMinDelegatorStakeZero,
		},
		{
			name: "maxValidatorWeightFactor == 0",
			txFunc: func(*gomock.Controller) *TransformSupernetTx {
				return &TransformSupernetTx{
					BaseTx:                   validBaseTx,
					Supernet:                 ids.GenerateTestID(),
					AssetID:                  ids.GenerateTestID(),
					RewardShare:              50000,
					MinValidatorStake:        2,
					MaxValidatorStake:        10,
					MinStakeDuration:         1,
					MaxStakeDuration:         2,
					MinDelegationFee:         reward.PercentDenominator,
					MinDelegatorStake:        1,
					MaxValidatorWeightFactor: 0,
				}
			},
			err: errMaxValidatorWeightFactorZero,
		},
		{
			name: "uptimeRequirement > 100%",
			txFunc: func(*gomock.Controller) *TransformSupernetTx {
				return &TransformSupernetTx{
					BaseTx:                   validBaseTx,
					Supernet:                 ids.GenerateTestID(),
					AssetID:                  ids.GenerateTestID(),
					RewardShare:              50000,
					MinValidatorStake:        2,
					MaxValidatorStake:        10,
					MinStakeDuration:         1,
					MaxStakeDuration:         2,
					MinDelegationFee:         reward.PercentDenominator,
					MinDelegatorStake:        1,
					MaxValidatorWeightFactor: 1,
					UptimeRequirement:        reward.PercentDenominator + 1,
				}
			},
			err: errUptimeRequirementTooLarge,
		},
		{
			name: "invalid supernetAuth",
			txFunc: func(ctrl *gomock.Controller) *TransformSupernetTx {
				// This SupernetAuth fails verification.
				invalidSupernetAuth := verify.NewMockVerifiable(ctrl)
				invalidSupernetAuth.EXPECT().Verify().Return(errInvalidSupernetAuth)
				return &TransformSupernetTx{
					BaseTx:                   validBaseTx,
					Supernet:                 ids.GenerateTestID(),
					AssetID:                  ids.GenerateTestID(),
					RewardShare:              50000,
					MinValidatorStake:        2,
					MaxValidatorStake:        10,
					MinStakeDuration:         1,
					MaxStakeDuration:         2,
					MinDelegationFee:         reward.PercentDenominator,
					MinDelegatorStake:        1,
					MaxValidatorWeightFactor: 1,
					UptimeRequirement:        reward.PercentDenominator,
					SupernetAuth:             invalidSupernetAuth,
				}
			},
			err: errInvalidSupernetAuth,
		},
		{
			name: "passes verification",
			txFunc: func(ctrl *gomock.Controller) *TransformSupernetTx {
				// This SupernetAuth passes verification.
				validSupernetAuth := verify.NewMockVerifiable(ctrl)
				validSupernetAuth.EXPECT().Verify().Return(nil)
				return &TransformSupernetTx{
					BaseTx:                   validBaseTx,
					Supernet:                 ids.GenerateTestID(),
					AssetID:                  ids.GenerateTestID(),
					RewardShare:              50000,
					MinValidatorStake:        2,
					MaxValidatorStake:        10,
					MinStakeDuration:         1,
					MaxStakeDuration:         2,
					MinDelegationFee:         reward.PercentDenominator,
					MinDelegatorStake:        1,
					MaxValidatorWeightFactor: 1,
					UptimeRequirement:        reward.PercentDenominator,
					SupernetAuth:             validSupernetAuth,
				}
			},
			err: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			tx := tt.txFunc(ctrl)
			err := tx.SyntacticVerify(ctx)
			require.ErrorIs(err, tt.err)
		})
	}

	t.Run("invalid BaseTx", func(t *testing.T) {
		require := require.New(t)
		tx := &TransformSupernetTx{
			BaseTx:                   invalidBaseTx,
			Supernet:                 ids.GenerateTestID(),
			AssetID:                  ids.GenerateTestID(),
			RewardShare:              50000,
			MinValidatorStake:        2,
			MaxValidatorStake:        10,
			MinStakeDuration:         1,
			MaxStakeDuration:         2,
			MinDelegationFee:         reward.PercentDenominator,
			MinDelegatorStake:        1,
			MaxValidatorWeightFactor: 1,
			UptimeRequirement:        reward.PercentDenominator,
		}
		err := tx.SyntacticVerify(ctx)
		require.Error(err)
	})
}
