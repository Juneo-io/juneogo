// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"golang.org/x/exp/maps"

	"github.com/Juneo-io/juneogo/chains"
	"github.com/Juneo-io/juneogo/chains/atomic"
	"github.com/Juneo-io/juneogo/database/manager"
	"github.com/Juneo-io/juneogo/database/prefixdb"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow"
	"github.com/Juneo-io/juneogo/snow/consensus/snowman"
	"github.com/Juneo-io/juneogo/snow/engine/common"
	"github.com/Juneo-io/juneogo/snow/uptime"
	"github.com/Juneo-io/juneogo/snow/validators"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/crypto/bls"
	"github.com/Juneo-io/juneogo/utils/crypto/secp256k1"
	"github.com/Juneo-io/juneogo/utils/formatting"
	"github.com/Juneo-io/juneogo/utils/formatting/address"
	"github.com/Juneo-io/juneogo/utils/json"
	"github.com/Juneo-io/juneogo/utils/timer/mockable"
	"github.com/Juneo-io/juneogo/utils/units"
	"github.com/Juneo-io/juneogo/version"
	"github.com/Juneo-io/juneogo/vms/components/avax"
	"github.com/Juneo-io/juneogo/vms/platformvm/api"
	"github.com/Juneo-io/juneogo/vms/platformvm/block"
	"github.com/Juneo-io/juneogo/vms/platformvm/config"
	"github.com/Juneo-io/juneogo/vms/platformvm/reward"
	"github.com/Juneo-io/juneogo/vms/platformvm/signer"
	"github.com/Juneo-io/juneogo/vms/platformvm/state"
	"github.com/Juneo-io/juneogo/vms/platformvm/txs"
	"github.com/Juneo-io/juneogo/vms/platformvm/utxo"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"

	blockexecutor "github.com/Juneo-io/juneogo/vms/platformvm/block/executor"
	txexecutor "github.com/Juneo-io/juneogo/vms/platformvm/txs/executor"
)

const (
	startPrimaryWithBLS uint8 = iota
	startPrimaryWithoutBLS
	startSupernetValidator
)

var errEmptyEventsList = errors.New("empty events list")

// for a given (permissioned) supernet, the test stakes and restakes multiple
// times a node as a primary and supernet validator. The BLS key of the node is
// changed across staking periods, and it can even be nil. We test that
// GetValidatorSet returns the correct primary and supernet validators data, with
// the right BLS key version at all relevant heights.
func TestGetValidatorsSetProperty(t *testing.T) {
	properties := gopter.NewProperties(nil)

	// to reproduce a given scenario do something like this:
	// parameters := gopter.DefaultTestParametersWithSeed(1685887576153675816)
	// properties := gopter.NewProperties(parameters)

	properties.Property("check GetValidatorSet", prop.ForAll(
		func(events []uint8) string {
			vm, supernetID, err := buildVM(t)
			if err != nil {
				return fmt.Sprintf("failed building vm: %s", err.Error())
			}
			vm.ctx.Lock.Lock()
			defer func() {
				_ = vm.Shutdown(context.Background())
				vm.ctx.Lock.Unlock()
			}()
			nodeID := ids.GenerateTestNodeID()

			currentTime := defaultGenesisTime
			vm.clock.Set(currentTime)
			vm.state.SetTimestamp(currentTime)

			// build a valid sequence of validators start/end times, given the
			// random events sequence received as test input
			validatorsTimes, err := buildTimestampsList(events, currentTime, nodeID)
			if err != nil {
				return fmt.Sprintf("failed building events sequence: %s", err.Error())
			}

			validatorSetByHeightAndSupernet := make(map[uint64]map[ids.ID]map[ids.NodeID]*validators.GetValidatorOutput)
			if err := takeValidatorsSnapshotAtCurrentHeight(vm, validatorSetByHeightAndSupernet); err != nil {
				return fmt.Sprintf("could not take validators snapshot: %s", err.Error())
			}

			// insert validator sequence
			var (
				currentPrimaryValidator = (*state.Staker)(nil)
				currentSupernetValidator  = (*state.Staker)(nil)
			)
			for _, ev := range validatorsTimes {
				// at each step we remove at least a supernet validator
				if currentSupernetValidator != nil {
					err := terminateSupernetValidator(vm, currentSupernetValidator)
					if err != nil {
						return fmt.Sprintf("could not terminate current supernet validator: %s", err.Error())
					}
					currentSupernetValidator = nil

					if err := takeValidatorsSnapshotAtCurrentHeight(vm, validatorSetByHeightAndSupernet); err != nil {
						return fmt.Sprintf("could not take validators snapshot: %s", err.Error())
					}
				}

				switch ev.eventType {
				case startSupernetValidator:
					currentSupernetValidator, err = addSupernetValidator(vm, ev, supernetID)
					if err != nil {
						return fmt.Sprintf("could not add supernet validator: %s", err.Error())
					}
					if err := takeValidatorsSnapshotAtCurrentHeight(vm, validatorSetByHeightAndSupernet); err != nil {
						return fmt.Sprintf("could not take validators snapshot: %s", err.Error())
					}

				case startPrimaryWithoutBLS:
					// when adding a primary validator, also remove the current
					// primary one
					if currentPrimaryValidator != nil {
						err := terminatePrimaryValidator(vm, currentPrimaryValidator)
						if err != nil {
							return fmt.Sprintf("could not terminate current primary validator: %s", err.Error())
						}
						// no need to nil current primary validator, we'll
						// reassign immediately

						if err := takeValidatorsSnapshotAtCurrentHeight(vm, validatorSetByHeightAndSupernet); err != nil {
							return fmt.Sprintf("could not take validators snapshot: %s", err.Error())
						}
					}
					currentPrimaryValidator, err = addPrimaryValidatorWithoutBLSKey(vm, ev)
					if err != nil {
						return fmt.Sprintf("could not add primary validator without BLS key: %s", err.Error())
					}
					if err := takeValidatorsSnapshotAtCurrentHeight(vm, validatorSetByHeightAndSupernet); err != nil {
						return fmt.Sprintf("could not take validators snapshot: %s", err.Error())
					}

				case startPrimaryWithBLS:
					// when adding a primary validator, also remove the current
					// primary one
					if currentPrimaryValidator != nil {
						err := terminatePrimaryValidator(vm, currentPrimaryValidator)
						if err != nil {
							return fmt.Sprintf("could not terminate current primary validator: %s", err.Error())
						}
						// no need to nil current primary validator, we'll
						// reassign immediately

						if err := takeValidatorsSnapshotAtCurrentHeight(vm, validatorSetByHeightAndSupernet); err != nil {
							return fmt.Sprintf("could not take validators snapshot: %s", err.Error())
						}
					}
					currentPrimaryValidator, err = addPrimaryValidatorWithBLSKey(vm, ev)
					if err != nil {
						return fmt.Sprintf("could not add primary validator with BLS key: %s", err.Error())
					}
					if err := takeValidatorsSnapshotAtCurrentHeight(vm, validatorSetByHeightAndSupernet); err != nil {
						return fmt.Sprintf("could not take validators snapshot: %s", err.Error())
					}

				default:
					return fmt.Sprintf("unexpected staker type: %v", ev.eventType)
				}
			}

			// Checks: let's look back at validator sets at previous heights and
			// make sure they match the snapshots already taken
			snapshotHeights := maps.Keys(validatorSetByHeightAndSupernet)
			sort.Slice(snapshotHeights, func(i, j int) bool { return snapshotHeights[i] < snapshotHeights[j] })
			for idx, snapShotHeight := range snapshotHeights {
				lastAcceptedHeight, err := vm.GetCurrentHeight(context.Background())
				if err != nil {
					return err.Error()
				}

				nextSnapShotHeight := lastAcceptedHeight + 1
				if idx != len(snapshotHeights)-1 {
					nextSnapShotHeight = snapshotHeights[idx+1]
				}

				// within [snapShotHeight] and [nextSnapShotHeight], the validator set
				// does not change and must be equal to snapshot at [snapShotHeight]
				for height := snapShotHeight; height < nextSnapShotHeight; height++ {
					for supernetID, validatorsSet := range validatorSetByHeightAndSupernet[snapShotHeight] {
						res, err := vm.GetValidatorSet(context.Background(), height, supernetID)
						if err != nil {
							return fmt.Sprintf("failed GetValidatorSet at height %v: %v", height, err)
						}
						if !reflect.DeepEqual(validatorsSet, res) {
							return "failed validators set comparison"
						}
					}
				}
			}

			return ""
		},
		gen.SliceOfN(
			10,
			gen.OneConstOf(
				startPrimaryWithBLS,
				startPrimaryWithoutBLS,
				startSupernetValidator,
			),
		).SuchThat(func(v interface{}) bool {
			list := v.([]uint8)
			return len(list) > 0 && (list[0] == startPrimaryWithBLS || list[0] == startPrimaryWithoutBLS)
		}),
	))

	properties.TestingRun(t)
}

func takeValidatorsSnapshotAtCurrentHeight(vm *VM, validatorsSetByHeightAndSupernet map[uint64]map[ids.ID]map[ids.NodeID]*validators.GetValidatorOutput) error {
	if validatorsSetByHeightAndSupernet == nil {
		validatorsSetByHeightAndSupernet = make(map[uint64]map[ids.ID]map[ids.NodeID]*validators.GetValidatorOutput)
	}

	lastBlkID := vm.state.GetLastAccepted()
	lastBlk, err := vm.state.GetStatelessBlock(lastBlkID)
	if err != nil {
		return err
	}
	height := lastBlk.Height()
	validatorsSetBySupernet, ok := validatorsSetByHeightAndSupernet[height]
	if !ok {
		validatorsSetByHeightAndSupernet[height] = make(map[ids.ID]map[ids.NodeID]*validators.GetValidatorOutput)
		validatorsSetBySupernet = validatorsSetByHeightAndSupernet[height]
	}

	stakerIt, err := vm.state.GetCurrentStakerIterator()
	if err != nil {
		return err
	}
	defer stakerIt.Release()
	for stakerIt.Next() {
		v := stakerIt.Value()
		validatorsSet, ok := validatorsSetBySupernet[v.SupernetID]
		if !ok {
			validatorsSetBySupernet[v.SupernetID] = make(map[ids.NodeID]*validators.GetValidatorOutput)
			validatorsSet = validatorsSetBySupernet[v.SupernetID]
		}

		blsKey := v.PublicKey
		if v.SupernetID != constants.PrimaryNetworkID {
			// pick bls key from primary validator
			s, err := vm.state.GetCurrentValidator(constants.PlatformChainID, v.NodeID)
			if err != nil {
				return err
			}
			blsKey = s.PublicKey
		}

		validatorsSet[v.NodeID] = &validators.GetValidatorOutput{
			NodeID:    v.NodeID,
			PublicKey: blsKey,
			Weight:    v.Weight,
		}
	}
	return nil
}

func addSupernetValidator(vm *VM, data *validatorInputData, supernetID ids.ID) (*state.Staker, error) {
	addr := keys[0].PublicKey().Address()
	signedTx, err := vm.txBuilder.NewAddSupernetValidatorTx(
		vm.Config.MinValidatorStake,
		uint64(data.startTime.Unix()),
		uint64(data.endTime.Unix()),
		data.nodeID,
		supernetID,
		[]*secp256k1.PrivateKey{keys[0], keys[1]},
		addr,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create AddSupernetValidatorTx: %w", err)
	}
	return internalAddValidator(vm, signedTx)
}

func addPrimaryValidatorWithBLSKey(vm *VM, data *validatorInputData) (*state.Staker, error) {
	addr := keys[0].PublicKey().Address()
	utxoHandler := utxo.NewHandler(vm.ctx, &vm.clock, vm.fx)
	ins, unstakedOuts, stakedOuts, signers, err := utxoHandler.Spend(
		vm.state,
		keys,
		vm.MinValidatorStake,
		vm.Config.AddPrimaryNetworkValidatorFee,
		addr, // change Addresss
	)
	if err != nil {
		return nil, fmt.Errorf("could not create inputs/outputs for permissionless validator: %w", err)
	}
	sk, err := bls.NewSecretKey()
	if err != nil {
		return nil, fmt.Errorf("could not create secret key: %w", err)
	}

	uPrimaryTx := &txs.AddPermissionlessValidatorTx{
		BaseTx: txs.BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    vm.ctx.NetworkID,
			BlockchainID: vm.ctx.ChainID,
			Ins:          ins,
			Outs:         unstakedOuts,
		}},
		Validator: txs.Validator{
			NodeID: data.nodeID,
			Start:  uint64(data.startTime.Unix()),
			End:    uint64(data.endTime.Unix()),
			Wght:   vm.MinValidatorStake,
		},
		Supernet:    constants.PrimaryNetworkID,
		Signer:    signer.NewProofOfPossession(sk),
		StakeOuts: stakedOuts,
		ValidatorRewardsOwner: &secp256k1fx.OutputOwners{
			Locktime:  0,
			Threshold: 1,
			Addrs: []ids.ShortID{
				addr,
			},
		},
		DelegatorRewardsOwner: &secp256k1fx.OutputOwners{
			Locktime:  0,
			Threshold: 1,
			Addrs: []ids.ShortID{
				addr,
			},
		},
		DelegationShares: reward.PercentDenominator,
	}
	signedTx, err := txs.NewSigned(uPrimaryTx, txs.Codec, signers)
	if err != nil {
		return nil, fmt.Errorf("could not create AddPermissionlessValidatorTx with BLS key: %w", err)
	}
	if err := signedTx.SyntacticVerify(vm.ctx); err != nil {
		return nil, fmt.Errorf("failed syntax verification of AddPermissionlessValidatorTx: %w", err)
	}
	return internalAddValidator(vm, signedTx)
}

func addPrimaryValidatorWithoutBLSKey(vm *VM, data *validatorInputData) (*state.Staker, error) {
	addr := keys[0].PublicKey().Address()
	signedTx, err := vm.txBuilder.NewAddValidatorTx(
		vm.Config.MinValidatorStake,
		uint64(data.startTime.Unix()),
		uint64(data.endTime.Unix()),
		data.nodeID,
		addr,
		reward.PercentDenominator,
		[]*secp256k1.PrivateKey{keys[0], keys[1]},
		addr,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create AddValidatorTx: %w", err)
	}
	return internalAddValidator(vm, signedTx)
}

func internalAddValidator(vm *VM, signedTx *txs.Tx) (*state.Staker, error) {
	stakerTx := signedTx.Unsigned.(txs.StakerTx)
	if err := vm.Builder.AddUnverifiedTx(signedTx); err != nil {
		return nil, fmt.Errorf("could not add tx to mempool: %w", err)
	}

	blk, err := vm.Builder.BuildBlock(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed building block: %w", err)
	}
	if err := blk.Verify(context.Background()); err != nil {
		return nil, fmt.Errorf("failed verifying block: %w", err)
	}
	if err := blk.Accept(context.Background()); err != nil {
		return nil, fmt.Errorf("failed accepting block: %w", err)
	}
	if err := vm.SetPreference(context.Background(), vm.manager.LastAccepted()); err != nil {
		return nil, fmt.Errorf("failed setting preference: %w", err)
	}

	// move time ahead, promoting the validator to current
	currentTime := stakerTx.StartTime()
	vm.clock.Set(currentTime)
	vm.state.SetTimestamp(currentTime)

	blk, err = vm.Builder.BuildBlock(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed building block: %w", err)
	}
	if err := blk.Verify(context.Background()); err != nil {
		return nil, fmt.Errorf("failed verifying block: %w", err)
	}
	if err := blk.Accept(context.Background()); err != nil {
		return nil, fmt.Errorf("failed accepting block: %w", err)
	}
	if err := vm.SetPreference(context.Background(), vm.manager.LastAccepted()); err != nil {
		return nil, fmt.Errorf("failed setting preference: %w", err)
	}

	return vm.state.GetCurrentValidator(stakerTx.SupernetID(), stakerTx.NodeID())
}

func terminateSupernetValidator(vm *VM, validator *state.Staker) error {
	currentTime := validator.EndTime
	vm.clock.Set(currentTime)
	vm.state.SetTimestamp(currentTime)

	blk, err := vm.Builder.BuildBlock(context.Background())
	if err != nil {
		return fmt.Errorf("failed building block: %w", err)
	}
	if err := blk.Verify(context.Background()); err != nil {
		return fmt.Errorf("failed verifying block: %w", err)
	}
	if err := blk.Accept(context.Background()); err != nil {
		return fmt.Errorf("failed accepting block: %w", err)
	}
	if err := vm.SetPreference(context.Background(), vm.manager.LastAccepted()); err != nil {
		return fmt.Errorf("failed setting preference: %w", err)
	}

	return nil
}

func terminatePrimaryValidator(vm *VM, validator *state.Staker) error {
	currentTime := validator.EndTime
	vm.clock.Set(currentTime)
	vm.state.SetTimestamp(currentTime)

	blk, err := vm.Builder.BuildBlock(context.Background())
	if err != nil {
		return fmt.Errorf("failed building block: %w", err)
	}
	if err := blk.Verify(context.Background()); err != nil {
		return fmt.Errorf("failed verifying block: %w", err)
	}

	proposalBlk := blk.(snowman.OracleBlock)
	options, err := proposalBlk.Options(context.Background())
	if err != nil {
		return fmt.Errorf("failed retrieving options: %w", err)
	}

	commit := options[0].(*blockexecutor.Block)
	_, ok := commit.Block.(*block.BanffCommitBlock)
	if !ok {
		return fmt.Errorf("failed retrieving commit option: %w", err)
	}
	if err := blk.Accept(context.Background()); err != nil {
		return fmt.Errorf("failed accepting block: %w", err)
	}

	if err := commit.Verify(context.Background()); err != nil {
		return fmt.Errorf("failed verifying commit block: %w", err)
	}
	if err := commit.Accept(context.Background()); err != nil {
		return fmt.Errorf("failed accepting commit block: %w", err)
	}

	if err := vm.SetPreference(context.Background(), vm.manager.LastAccepted()); err != nil {
		return fmt.Errorf("failed setting preference: %w", err)
	}

	return nil
}

type validatorInputData struct {
	eventType uint8
	startTime time.Time
	endTime   time.Time
	nodeID    ids.NodeID
	publicKey *bls.PublicKey
}

// buildTimestampsList creates validators start and end time, given the event list.
// output is returned as a list of validatorInputData
func buildTimestampsList(events []uint8, currentTime time.Time, nodeID ids.NodeID) ([]*validatorInputData, error) {
	res := make([]*validatorInputData, 0, len(events))

	currentTime = currentTime.Add(txexecutor.SyncBound)
	switch endTime := currentTime.Add(defaultMinStakingDuration); events[0] {
	case startPrimaryWithBLS:
		sk, err := bls.NewSecretKey()
		if err != nil {
			return nil, fmt.Errorf("could not make private key: %w", err)
		}

		res = append(res, &validatorInputData{
			eventType: startPrimaryWithBLS,
			startTime: currentTime,
			endTime:   endTime,
			nodeID:    nodeID,
			publicKey: bls.PublicFromSecretKey(sk),
		})
	case startPrimaryWithoutBLS:
		res = append(res, &validatorInputData{
			eventType: startPrimaryWithoutBLS,
			startTime: currentTime,
			endTime:   endTime,
			nodeID:    nodeID,
			publicKey: nil,
		})
	default:
		return nil, fmt.Errorf("unexpected initial event %d", events[0])
	}

	// track current primary validator to make sure its staking period
	// covers all of its supernet validators
	currentPrimaryVal := res[0]
	for i := 1; i < len(events); i++ {
		currentTime = currentTime.Add(txexecutor.SyncBound)

		switch currentEvent := events[i]; currentEvent {
		case startSupernetValidator:
			endTime := currentTime.Add(defaultMinStakingDuration)
			res = append(res, &validatorInputData{
				eventType: startSupernetValidator,
				startTime: currentTime,
				endTime:   endTime,
				nodeID:    nodeID,
				publicKey: nil,
			})

			currentPrimaryVal.endTime = endTime.Add(time.Second)
			currentTime = endTime.Add(time.Second)

		case startPrimaryWithBLS:
			currentTime = currentPrimaryVal.endTime.Add(txexecutor.SyncBound)
			sk, err := bls.NewSecretKey()
			if err != nil {
				return nil, fmt.Errorf("could not make private key: %w", err)
			}

			endTime := currentTime.Add(defaultMinStakingDuration)
			val := &validatorInputData{
				eventType: startPrimaryWithBLS,
				startTime: currentTime,
				endTime:   endTime,
				nodeID:    nodeID,
				publicKey: bls.PublicFromSecretKey(sk),
			}
			res = append(res, val)
			currentPrimaryVal = val

		case startPrimaryWithoutBLS:
			currentTime = currentPrimaryVal.endTime.Add(txexecutor.SyncBound)
			endTime := currentTime.Add(defaultMinStakingDuration)
			val := &validatorInputData{
				eventType: startPrimaryWithoutBLS,
				startTime: currentTime,
				endTime:   endTime,
				nodeID:    nodeID,
				publicKey: nil,
			}
			res = append(res, val)
			currentPrimaryVal = val
		}
	}
	return res, nil
}

func TestTimestampListGenerator(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("primary validators are returned in sequence", prop.ForAll(
		func(events []uint8) string {
			currentTime := time.Now()
			nodeID := ids.GenerateTestNodeID()
			validatorsTimes, err := buildTimestampsList(events, currentTime, nodeID)
			if err != nil {
				return fmt.Sprintf("failed building events sequence: %s", err.Error())
			}

			if len(validatorsTimes) == 0 {
				return errEmptyEventsList.Error()
			}

			// nil out non supernet validators
			supernetIndexes := make([]int, 0)
			for idx, ev := range validatorsTimes {
				if ev.eventType == startSupernetValidator {
					supernetIndexes = append(supernetIndexes, idx)
				}
			}
			for _, idx := range supernetIndexes {
				validatorsTimes[idx] = nil
			}

			currentEventTime := currentTime
			for i, ev := range validatorsTimes {
				if ev == nil {
					continue // a supernet validator
				}
				if currentEventTime.After(ev.startTime) {
					return fmt.Sprintf("validator %d start time larger than current event time", i)
				}

				if ev.startTime.After(ev.endTime) {
					return fmt.Sprintf("validator %d start time larger than its end time", i)
				}

				currentEventTime = ev.endTime
			}

			return ""
		},
		gen.SliceOf(gen.OneConstOf(
			startPrimaryWithBLS,
			startPrimaryWithoutBLS,
			startSupernetValidator,
		)).SuchThat(func(v interface{}) bool {
			list := v.([]uint8)
			return len(list) > 0 && (list[0] == startPrimaryWithBLS || list[0] == startPrimaryWithoutBLS)
		}),
	))

	properties.Property("supernet validators are returned in sequence", prop.ForAll(
		func(events []uint8) string {
			currentTime := time.Now()
			nodeID := ids.GenerateTestNodeID()
			validatorsTimes, err := buildTimestampsList(events, currentTime, nodeID)
			if err != nil {
				return fmt.Sprintf("failed building events sequence: %s", err.Error())
			}

			if len(validatorsTimes) == 0 {
				return errEmptyEventsList.Error()
			}

			// nil out non supernet validators
			nonSupernetIndexes := make([]int, 0)
			for idx, ev := range validatorsTimes {
				if ev.eventType != startSupernetValidator {
					nonSupernetIndexes = append(nonSupernetIndexes, idx)
				}
			}
			for _, idx := range nonSupernetIndexes {
				validatorsTimes[idx] = nil
			}

			currentEventTime := currentTime
			for i, ev := range validatorsTimes {
				if ev == nil {
					continue // a non-supernet validator
				}
				if currentEventTime.After(ev.startTime) {
					return fmt.Sprintf("validator %d start time larger than current event time", i)
				}

				if ev.startTime.After(ev.endTime) {
					return fmt.Sprintf("validator %d start time larger than its end time", i)
				}

				currentEventTime = ev.endTime
			}

			return ""
		},
		gen.SliceOf(gen.OneConstOf(
			startPrimaryWithBLS,
			startPrimaryWithoutBLS,
			startSupernetValidator,
		)).SuchThat(func(v interface{}) bool {
			list := v.([]uint8)
			return len(list) > 0 && (list[0] == startPrimaryWithBLS || list[0] == startPrimaryWithoutBLS)
		}),
	))

	properties.Property("supernet validators' times are bound by a primary validator's times", prop.ForAll(
		func(events []uint8) string {
			currentTime := time.Now()
			nodeID := ids.GenerateTestNodeID()
			validatorsTimes, err := buildTimestampsList(events, currentTime, nodeID)
			if err != nil {
				return fmt.Sprintf("failed building events sequence: %s", err.Error())
			}

			if len(validatorsTimes) == 0 {
				return errEmptyEventsList.Error()
			}

			currentPrimaryValidator := validatorsTimes[0]
			for i := 1; i < len(validatorsTimes); i++ {
				if validatorsTimes[i].eventType != startSupernetValidator {
					currentPrimaryValidator = validatorsTimes[i]
					continue
				}

				supernetVal := validatorsTimes[i]
				if currentPrimaryValidator.startTime.After(supernetVal.startTime) ||
					supernetVal.endTime.After(currentPrimaryValidator.endTime) {
					return "supernet validator not bounded by primary network ones"
				}
			}
			return ""
		},
		gen.SliceOf(gen.OneConstOf(
			startPrimaryWithBLS,
			startPrimaryWithoutBLS,
			startSupernetValidator,
		)).SuchThat(func(v interface{}) bool {
			list := v.([]uint8)
			return len(list) > 0 && (list[0] == startPrimaryWithBLS || list[0] == startPrimaryWithoutBLS)
		}),
	))

	properties.TestingRun(t)
}

// add a single validator at the end of times,
// to make sure it won't pollute our tests
func buildVM(t *testing.T) (*VM, ids.ID, error) {
	forkTime := defaultGenesisTime
	vm := &VM{Config: config.Config{
		Chains:                 chains.TestManager,
		UptimeLockedCalculator: uptime.NewLockedCalculator(),
		SybilProtectionEnabled: true,
		Validators:             validators.NewManager(),
		TxFee:                  defaultTxFee,
		CreateSupernetTxFee:      100 * defaultTxFee,
		TransformSupernetTxFee:   100 * defaultTxFee,
		CreateBlockchainTxFee:  100 * defaultTxFee,
		MinValidatorStake:      defaultMinValidatorStake,
		MaxValidatorStake:      defaultMaxValidatorStake,
		MinDelegatorStake:      defaultMinDelegatorStake,
		MinStakeDuration:       defaultMinStakingDuration,
		MaxStakeDuration:       defaultMaxStakingDuration,
		RewardConfig:           defaultRewardConfig,
		ApricotPhase3Time:      forkTime,
		ApricotPhase5Time:      forkTime,
		BanffTime:              forkTime,
		CortinaTime:            forkTime,
	}}
	vm.clock.Set(forkTime.Add(time.Second))

	baseDBManager := manager.NewMemDB(version.Semantic1_0_0)
	chainDBManager := baseDBManager.NewPrefixDBManager([]byte{0})
	atomicDB := prefixdb.New([]byte{1}, baseDBManager.Current().Database)

	msgChan := make(chan common.Message, 1)
	ctx := defaultContext(t)

	m := atomic.NewMemory(atomicDB)
	ctx.SharedMemory = m.NewSharedMemory(ctx.ChainID)

	ctx.Lock.Lock()
	defer ctx.Lock.Unlock()
	appSender := &common.SenderTest{}
	appSender.CantSendAppGossip = true
	appSender.SendAppGossipF = func(context.Context, []byte) error {
		return nil
	}

	genesisBytes, err := buildCustomGenesis()
	if err != nil {
		return nil, ids.Empty, err
	}

	err = vm.Initialize(
		context.Background(),
		ctx,
		chainDBManager,
		genesisBytes,
		nil,
		nil,
		msgChan,
		nil,
		appSender,
	)
	if err != nil {
		return nil, ids.Empty, err
	}

	err = vm.SetState(context.Background(), snow.NormalOp)
	if err != nil {
		return nil, ids.Empty, err
	}

	// Create a supernet and store it in testSupernet1
	// Note: following Banff activation, block acceptance will move
	// chain time ahead
	testSupernet1, err = vm.txBuilder.NewCreateSupernetTx(
		1, // threshold
		[]ids.ShortID{keys[0].PublicKey().Address()},
		[]*secp256k1.PrivateKey{keys[len(keys)-1]}, // pays tx fee
		keys[0].PublicKey().Address(),              // change addr
	)
	if err != nil {
		return nil, ids.Empty, err
	}
	if err := vm.Builder.AddUnverifiedTx(testSupernet1); err != nil {
		return nil, ids.Empty, err
	}

	blk, err := vm.Builder.BuildBlock(context.Background())
	if err != nil {
		return nil, ids.Empty, err
	}
	if err := blk.Verify(context.Background()); err != nil {
		return nil, ids.Empty, err
	}
	if err := blk.Accept(context.Background()); err != nil {
		return nil, ids.Empty, err
	}
	if err := vm.SetPreference(context.Background(), vm.manager.LastAccepted()); err != nil {
		return nil, ids.Empty, err
	}

	return vm, testSupernet1.ID(), nil
}

func buildCustomGenesis() ([]byte, error) {
	genesisUTXOs := make([]api.UTXO, len(keys))
	for i, key := range keys {
		id := key.PublicKey().Address()
		addr, err := address.FormatBech32(constants.UnitTestHRP, id.Bytes())
		if err != nil {
			return nil, err
		}
		genesisUTXOs[i] = api.UTXO{
			Amount:  json.Uint64(defaultBalance),
			Address: addr,
		}
	}

	// we need at least a validator, otherwise BuildBlock would fail, since it
	// won't find next staker to promote/evict from stakers set. Contrary to
	// what happens with production code we push such validator at the end of
	// times, so to avoid interference with our tests
	nodeID := ids.NodeID(keys[len(keys)-1].PublicKey().Address())
	addr, err := address.FormatBech32(constants.UnitTestHRP, nodeID.Bytes())
	if err != nil {
		return nil, err
	}

	starTime := mockable.MaxTime.Add(-1 * defaultMinStakingDuration)
	endTime := mockable.MaxTime
	genesisValidator := api.PermissionlessValidator{
		Staker: api.Staker{
			StartTime: json.Uint64(starTime.Unix()),
			EndTime:   json.Uint64(endTime.Unix()),
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

	buildGenesisArgs := api.BuildGenesisArgs{
		Encoding:      formatting.Hex,
		NetworkID:     json.Uint32(constants.UnitTestID),
		AvaxAssetID:   avaxAssetID,
		UTXOs:         genesisUTXOs,
		Validators:    []api.PermissionlessValidator{genesisValidator},
		Chains:        nil,
		Time:          json.Uint64(defaultGenesisTime.Unix()),
		InitialSupply: json.Uint64(360 * units.MegaAvax),
	}

	buildGenesisResponse := api.BuildGenesisReply{}
	platformvmSS := api.StaticService{}
	if err := platformvmSS.BuildGenesis(nil, &buildGenesisArgs, &buildGenesisResponse); err != nil {
		return nil, err
	}

	genesisBytes, err := formatting.Decode(buildGenesisResponse.Encoding, buildGenesisResponse.Bytes)
	if err != nil {
		return nil, err
	}

	return genesisBytes, nil
}
