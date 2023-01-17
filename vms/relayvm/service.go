// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package relayvm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	stdmath "math"

	"go.uber.org/zap"

	"github.com/Juneo-io/juneogo/api"
	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/crypto"
	"github.com/Juneo-io/juneogo/utils/formatting"
	"github.com/Juneo-io/juneogo/utils/json"
	"github.com/Juneo-io/juneogo/utils/logging"
	"github.com/Juneo-io/juneogo/utils/math"
	"github.com/Juneo-io/juneogo/utils/set"
	"github.com/Juneo-io/juneogo/utils/wrappers"
	"github.com/Juneo-io/juneogo/vms/components/june"
	"github.com/Juneo-io/juneogo/vms/components/keystore"
	"github.com/Juneo-io/juneogo/vms/relayvm/reward"
	"github.com/Juneo-io/juneogo/vms/relayvm/signer"
	"github.com/Juneo-io/juneogo/vms/relayvm/stakeable"
	"github.com/Juneo-io/juneogo/vms/relayvm/state"
	"github.com/Juneo-io/juneogo/vms/relayvm/status"
	"github.com/Juneo-io/juneogo/vms/relayvm/txs"
	"github.com/Juneo-io/juneogo/vms/relayvm/txs/builder"
	"github.com/Juneo-io/juneogo/vms/relayvm/txs/executor"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"

	relayapi "github.com/Juneo-io/juneogo/vms/relayvm/api"
)

const (
	// Max number of addresses that can be passed in as argument to GetUTXOs
	maxGetUTXOsAddrs = 1024

	// Max number of addresses that can be passed in as argument to GetStake
	maxGetStakeAddrs = 256

	// Minimum amount of delay to allow a transaction to be issued through the
	// API
	minAddStakerDelay = 2 * executor.SyncBound
)

var (
	errMissingDecisionBlock       = errors.New("should have a decision block within the past two blocks")
	errNoSupernetID               = errors.New("argument 'supernetID' not provided")
	errNoRewardAddress            = errors.New("argument 'rewardAddress' not provided")
	errInvalidDelegationRate      = errors.New("argument 'delegationFeeRate' must be between 0 and 100, inclusive")
	errNoAddresses                = errors.New("no addresses provided")
	errNoKeys                     = errors.New("user has no keys or funds")
	errStartTimeTooSoon           = fmt.Errorf("start time must be at least %s in the future", minAddStakerDelay)
	errStartTimeTooLate           = errors.New("start time is too far in the future")
	errNamedSupernetCantBePrimary = errors.New("supernet validator attempts to validate primary network")
	errNoAmount                   = errors.New("argument 'amount' must be > 0")
	errMissingName                = errors.New("argument 'name' not given")
	errMissingVMID                = errors.New("argument 'vmID' not given")
	errMissingBlockchainID        = errors.New("argument 'blockchainID' not given")
	errMissingPrivateKey          = errors.New("argument 'privateKey' not given")
	errStartAfterEndTime          = errors.New("start time must be before end time")
	errStartTimeInThePast         = errors.New("start time in the past")
)

// Service defines the API calls that can be made to the platform chain
type Service struct {
	vm          *VM
	addrManager june.AddressManager
}

type GetHeightResponse struct {
	Height json.Uint64 `json:"height"`
}

// GetHeight returns the height of the last accepted block
func (s *Service) GetHeight(r *http.Request, _ *struct{}, response *GetHeightResponse) error {
	ctx := r.Context()
	lastAcceptedID, err := s.vm.LastAccepted(ctx)
	if err != nil {
		return fmt.Errorf("couldn't get last accepted block ID: %w", err)
	}
	lastAccepted, err := s.vm.GetBlock(ctx, lastAcceptedID)
	if err != nil {
		return fmt.Errorf("couldn't get last accepted block: %w", err)
	}
	response.Height = json.Uint64(lastAccepted.Height())
	return nil
}

// ExportKeyArgs are arguments for ExportKey
type ExportKeyArgs struct {
	api.UserPass
	Address string `json:"address"`
}

// ExportKeyReply is the response for ExportKey
type ExportKeyReply struct {
	// The decrypted PrivateKey for the Address provided in the arguments
	PrivateKey *crypto.PrivateKeySECP256K1R `json:"privateKey"`
}

// ExportKey returns a private key from the provided user
func (s *Service) ExportKey(_ *http.Request, args *ExportKeyArgs, reply *ExportKeyReply) error {
	s.vm.ctx.Log.Debug("Platform: ExportKey called")

	address, err := june.ParseServiceAddress(s.addrManager, args.Address)
	if err != nil {
		return fmt.Errorf("couldn't parse %s to address: %w", args.Address, err)
	}

	user, err := keystore.NewUserFromKeystore(s.vm.ctx.Keystore, args.Username, args.Password)
	if err != nil {
		return err
	}

	reply.PrivateKey, err = user.GetKey(address)
	if err != nil {
		// Drop any potential error closing the user to report the original
		// error
		_ = user.Close()
		return fmt.Errorf("problem retrieving private key: %w", err)
	}
	return user.Close()
}

// ImportKeyArgs are arguments for ImportKey
type ImportKeyArgs struct {
	api.UserPass
	PrivateKey *crypto.PrivateKeySECP256K1R `json:"privateKey"`
}

// ImportKey adds a private key to the provided user
func (s *Service) ImportKey(_ *http.Request, args *ImportKeyArgs, reply *api.JSONAddress) error {
	s.vm.ctx.Log.Debug("Platform: ImportKey called",
		logging.UserString("username", args.Username),
	)

	if args.PrivateKey == nil {
		return errMissingPrivateKey
	}

	var err error
	reply.Address, err = s.addrManager.FormatLocalAddress(args.PrivateKey.PublicKey().Address())
	if err != nil {
		return fmt.Errorf("problem formatting address: %w", err)
	}

	user, err := keystore.NewUserFromKeystore(s.vm.ctx.Keystore, args.Username, args.Password)
	if err != nil {
		return err
	}
	defer user.Close()

	if err := user.PutKeys(args.PrivateKey); err != nil {
		return fmt.Errorf("problem saving key %w", err)
	}
	return user.Close()
}

/*
 ******************************************************
 *************  Balances / Addresses ******************
 ******************************************************
 */

type GetBalanceRequest struct {
	// TODO: remove Address
	Address   *string  `json:"address,omitempty"`
	Addresses []string `json:"addresses"`
}

// Note: We explicitly duplicate JUNE out of the maps to ensure backwards
// compatibility.
type GetBalanceResponse struct {
	// Balance, in nJune, of the address
	Balance             json.Uint64            `json:"balance"`
	Unlocked            json.Uint64            `json:"unlocked"`
	LockedStakeable     json.Uint64            `json:"lockedStakeable"`
	LockedNotStakeable  json.Uint64            `json:"lockedNotStakeable"`
	Balances            map[ids.ID]json.Uint64 `json:"balances"`
	Unlockeds           map[ids.ID]json.Uint64 `json:"unlockeds"`
	LockedStakeables    map[ids.ID]json.Uint64 `json:"lockedStakeables"`
	LockedNotStakeables map[ids.ID]json.Uint64 `json:"lockedNotStakeables"`
	UTXOIDs             []*june.UTXOID         `json:"utxoIDs"`
}

// GetBalance gets the balance of an address
func (s *Service) GetBalance(_ *http.Request, args *GetBalanceRequest, response *GetBalanceResponse) error {
	if args.Address != nil {
		args.Addresses = append(args.Addresses, *args.Address)
	}

	s.vm.ctx.Log.Debug("Platform: GetBalance called",
		logging.UserStrings("addresses", args.Addresses),
	)

	// Parse to address
	addrs, err := june.ParseServiceAddresses(s.addrManager, args.Addresses)
	if err != nil {
		return err
	}

	utxos, err := june.GetAllUTXOs(s.vm.state, addrs)
	if err != nil {
		return fmt.Errorf("couldn't get UTXO set of %v: %w", args.Addresses, err)
	}

	currentTime := s.vm.clock.Unix()

	unlockeds := map[ids.ID]uint64{}
	lockedStakeables := map[ids.ID]uint64{}
	lockedNotStakeables := map[ids.ID]uint64{}

utxoFor:
	for _, utxo := range utxos {
		assetID := utxo.AssetID()
		switch out := utxo.Out.(type) {
		case *secp256k1fx.TransferOutput:
			if out.Locktime <= currentTime {
				newBalance, err := math.Add64(unlockeds[assetID], out.Amount())
				if err != nil {
					unlockeds[assetID] = stdmath.MaxUint64
				} else {
					unlockeds[assetID] = newBalance
				}
			} else {
				newBalance, err := math.Add64(lockedNotStakeables[assetID], out.Amount())
				if err != nil {
					lockedNotStakeables[assetID] = stdmath.MaxUint64
				} else {
					lockedNotStakeables[assetID] = newBalance
				}
			}
		case *stakeable.LockOut:
			innerOut, ok := out.TransferableOut.(*secp256k1fx.TransferOutput)
			switch {
			case !ok:
				s.vm.ctx.Log.Warn("unexpected output type in UTXO",
					zap.String("type", fmt.Sprintf("%T", out.TransferableOut)),
				)
				continue utxoFor
			case innerOut.Locktime > currentTime:
				newBalance, err := math.Add64(lockedNotStakeables[assetID], out.Amount())
				if err != nil {
					lockedNotStakeables[assetID] = stdmath.MaxUint64
				} else {
					lockedNotStakeables[assetID] = newBalance
				}
			case out.Locktime <= currentTime:
				newBalance, err := math.Add64(unlockeds[assetID], out.Amount())
				if err != nil {
					unlockeds[assetID] = stdmath.MaxUint64
				} else {
					unlockeds[assetID] = newBalance
				}
			default:
				newBalance, err := math.Add64(lockedStakeables[assetID], out.Amount())
				if err != nil {
					lockedStakeables[assetID] = stdmath.MaxUint64
				} else {
					lockedStakeables[assetID] = newBalance
				}
			}
		default:
			continue utxoFor
		}

		response.UTXOIDs = append(response.UTXOIDs, &utxo.UTXOID)
	}

	balances := map[ids.ID]uint64{}
	for assetID, amount := range lockedStakeables {
		balances[assetID] = amount
	}
	for assetID, amount := range lockedNotStakeables {
		newBalance, err := math.Add64(balances[assetID], amount)
		if err != nil {
			balances[assetID] = stdmath.MaxUint64
		} else {
			balances[assetID] = newBalance
		}
	}
	for assetID, amount := range unlockeds {
		newBalance, err := math.Add64(balances[assetID], amount)
		if err != nil {
			balances[assetID] = stdmath.MaxUint64
		} else {
			balances[assetID] = newBalance
		}
	}

	response.Balances = newJSONBalanceMap(balances)
	response.Unlockeds = newJSONBalanceMap(unlockeds)
	response.LockedStakeables = newJSONBalanceMap(lockedStakeables)
	response.LockedNotStakeables = newJSONBalanceMap(lockedNotStakeables)
	response.Balance = response.Balances[s.vm.ctx.JuneAssetID]
	response.Unlocked = response.Unlockeds[s.vm.ctx.JuneAssetID]
	response.LockedStakeable = response.LockedStakeables[s.vm.ctx.JuneAssetID]
	response.LockedNotStakeable = response.LockedNotStakeables[s.vm.ctx.JuneAssetID]
	return nil
}

func newJSONBalanceMap(balanceMap map[ids.ID]uint64) map[ids.ID]json.Uint64 {
	jsonBalanceMap := make(map[ids.ID]json.Uint64, len(balanceMap))
	for assetID, amount := range balanceMap {
		jsonBalanceMap[assetID] = json.Uint64(amount)
	}
	return jsonBalanceMap
}

// CreateAddress creates an address controlled by [args.Username]
// Returns the newly created address
func (s *Service) CreateAddress(_ *http.Request, args *api.UserPass, response *api.JSONAddress) error {
	s.vm.ctx.Log.Debug("Platform: CreateAddress called")

	user, err := keystore.NewUserFromKeystore(s.vm.ctx.Keystore, args.Username, args.Password)
	if err != nil {
		return err
	}
	defer user.Close()

	key, err := keystore.NewKey(user)
	if err != nil {
		return err
	}

	response.Address, err = s.addrManager.FormatLocalAddress(key.PublicKey().Address())
	if err != nil {
		return fmt.Errorf("problem formatting address: %w", err)
	}
	return user.Close()
}

// ListAddresses returns the addresses controlled by [args.Username]
func (s *Service) ListAddresses(_ *http.Request, args *api.UserPass, response *api.JSONAddresses) error {
	s.vm.ctx.Log.Debug("Platform: ListAddresses called")

	user, err := keystore.NewUserFromKeystore(s.vm.ctx.Keystore, args.Username, args.Password)
	if err != nil {
		return err
	}
	defer user.Close()

	addresses, err := user.GetAddresses()
	if err != nil {
		return fmt.Errorf("couldn't get addresses: %w", err)
	}
	response.Addresses = make([]string, len(addresses))
	for i, addr := range addresses {
		response.Addresses[i], err = s.addrManager.FormatLocalAddress(addr)
		if err != nil {
			return fmt.Errorf("problem formatting address: %w", err)
		}
	}
	return user.Close()
}

// Index is an address and an associated UTXO.
// Marks a starting or stopping point when fetching UTXOs. Used for pagination.
type Index struct {
	Address string `json:"address"` // The address as a string
	UTXO    string `json:"utxo"`    // The UTXO ID as a string
}

// GetUTXOs returns the UTXOs controlled by the given addresses
func (s *Service) GetUTXOs(_ *http.Request, args *api.GetUTXOsArgs, response *api.GetUTXOsReply) error {
	s.vm.ctx.Log.Debug("Platform: GetUTXOs called")

	if len(args.Addresses) == 0 {
		return errNoAddresses
	}
	if len(args.Addresses) > maxGetUTXOsAddrs {
		return fmt.Errorf("number of addresses given, %d, exceeds maximum, %d", len(args.Addresses), maxGetUTXOsAddrs)
	}

	var sourceChain ids.ID
	if args.SourceChain == "" {
		sourceChain = s.vm.ctx.ChainID
	} else {
		chainID, err := s.vm.ctx.BCLookup.Lookup(args.SourceChain)
		if err != nil {
			return fmt.Errorf("problem parsing source chainID %q: %w", args.SourceChain, err)
		}
		sourceChain = chainID
	}

	addrSet, err := june.ParseServiceAddresses(s.addrManager, args.Addresses)
	if err != nil {
		return err
	}

	startAddr := ids.ShortEmpty
	startUTXO := ids.Empty
	if args.StartIndex.Address != "" || args.StartIndex.UTXO != "" {
		startAddr, err = june.ParseServiceAddress(s.addrManager, args.StartIndex.Address)
		if err != nil {
			return fmt.Errorf("couldn't parse start index address %q: %w", args.StartIndex.Address, err)
		}
		startUTXO, err = ids.FromString(args.StartIndex.UTXO)
		if err != nil {
			return fmt.Errorf("couldn't parse start index utxo: %w", err)
		}
	}

	var (
		utxos     []*june.UTXO
		endAddr   ids.ShortID
		endUTXOID ids.ID
	)
	limit := int(args.Limit)
	if limit <= 0 || builder.MaxPageSize < limit {
		limit = builder.MaxPageSize
	}
	if sourceChain == s.vm.ctx.ChainID {
		utxos, endAddr, endUTXOID, err = june.GetPaginatedUTXOs(
			s.vm.state,
			addrSet,
			startAddr,
			startUTXO,
			limit,
		)
	} else {
		utxos, endAddr, endUTXOID, err = s.vm.atomicUtxosManager.GetAtomicUTXOs(
			sourceChain,
			addrSet,
			startAddr,
			startUTXO,
			limit,
		)
	}
	if err != nil {
		return fmt.Errorf("problem retrieving UTXOs: %w", err)
	}

	response.UTXOs = make([]string, len(utxos))
	for i, utxo := range utxos {
		bytes, err := txs.Codec.Marshal(txs.Version, utxo)
		if err != nil {
			return fmt.Errorf("couldn't serialize UTXO %q: %w", utxo.InputID(), err)
		}
		response.UTXOs[i], err = formatting.Encode(args.Encoding, bytes)
		if err != nil {
			return fmt.Errorf("couldn't encode UTXO %s as string: %w", utxo.InputID(), err)
		}
	}

	endAddress, err := s.addrManager.FormatLocalAddress(endAddr)
	if err != nil {
		return fmt.Errorf("problem formatting address: %w", err)
	}

	response.EndIndex.Address = endAddress
	response.EndIndex.UTXO = endUTXOID.String()
	response.NumFetched = json.Uint64(len(utxos))
	response.Encoding = args.Encoding
	return nil
}

/*
 ******************************************************
 ******************* Get Supernets **********************
 ******************************************************
 */

// APISupernet is a representation of a supernet used in API calls
type APISupernet struct {
	// ID of the supernet
	ID ids.ID `json:"id"`

	// Each element of [ControlKeys] the address of a public key.
	// A transaction to add a validator to this supernet requires
	// signatures from [Threshold] of these keys to be valid.
	ControlKeys []string    `json:"controlKeys"`
	Threshold   json.Uint32 `json:"threshold"`
}

// GetSupernetsArgs are the arguments to GetSupernet
type GetSupernetsArgs struct {
	// IDs of the supernets to retrieve information about
	// If omitted, gets all supernets
	IDs []ids.ID `json:"ids"`
}

// GetSupernetsResponse is the response from calling GetSupernets
type GetSupernetsResponse struct {
	// Each element is a supernet that exists
	// Null if there are no supernets other than the primary network
	Supernets []APISupernet `json:"supernets"`
}

// GetSupernets returns the supernets whose ID are in [args.IDs]
// The response will include the primary network
func (s *Service) GetSupernets(_ *http.Request, args *GetSupernetsArgs, response *GetSupernetsResponse) error {
	s.vm.ctx.Log.Debug("Platform: GetSupernets called")

	getAll := len(args.IDs) == 0
	if getAll {
		supernets, err := s.vm.state.GetSupernets() // all supernets
		if err != nil {
			return fmt.Errorf("error getting supernets from database: %w", err)
		}

		response.Supernets = make([]APISupernet, len(supernets)+1)
		for i, supernet := range supernets {
			supernetID := supernet.ID()
			if _, err := s.vm.state.GetSupernetTransformation(supernetID); err == nil {
				response.Supernets[i] = APISupernet{
					ID:          supernetID,
					ControlKeys: []string{},
					Threshold:   json.Uint32(0),
				}
				continue
			}

			unsignedTx := supernet.Unsigned.(*txs.CreateSupernetTx)
			owner := unsignedTx.Owner.(*secp256k1fx.OutputOwners)
			controlAddrs := []string{}
			for _, controlKeyID := range owner.Addrs {
				addr, err := s.addrManager.FormatLocalAddress(controlKeyID)
				if err != nil {
					return fmt.Errorf("problem formatting address: %w", err)
				}
				controlAddrs = append(controlAddrs, addr)
			}
			response.Supernets[i] = APISupernet{
				ID:          supernetID,
				ControlKeys: controlAddrs,
				Threshold:   json.Uint32(owner.Threshold),
			}
		}
		// Include primary network
		response.Supernets[len(supernets)] = APISupernet{
			ID:          constants.PrimaryNetworkID,
			ControlKeys: []string{},
			Threshold:   json.Uint32(0),
		}
		return nil
	}

	supernetSet := set.NewSet[ids.ID](len(args.IDs))
	for _, supernetID := range args.IDs {
		if supernetSet.Contains(supernetID) {
			continue
		}
		supernetSet.Add(supernetID)

		if supernetID == constants.PrimaryNetworkID {
			response.Supernets = append(response.Supernets,
				APISupernet{
					ID:          constants.PrimaryNetworkID,
					ControlKeys: []string{},
					Threshold:   json.Uint32(0),
				},
			)
			continue
		}

		if _, err := s.vm.state.GetSupernetTransformation(supernetID); err == nil {
			response.Supernets = append(response.Supernets, APISupernet{
				ID:          supernetID,
				ControlKeys: []string{},
				Threshold:   json.Uint32(0),
			})
			continue
		}

		supernetTx, _, err := s.vm.state.GetTx(supernetID)
		if err == database.ErrNotFound {
			continue
		}
		if err != nil {
			return err
		}

		supernet, ok := supernetTx.Unsigned.(*txs.CreateSupernetTx)
		if !ok {
			return fmt.Errorf("expected tx type *txs.CreateSupernetTx but got %T", supernetTx.Unsigned)
		}
		owner, ok := supernet.Owner.(*secp256k1fx.OutputOwners)
		if !ok {
			return fmt.Errorf("expected *secp256k1fx.OutputOwners but got %T", supernet.Owner)
		}

		controlAddrs := make([]string, len(owner.Addrs))
		for i, controlKeyID := range owner.Addrs {
			addr, err := s.addrManager.FormatLocalAddress(controlKeyID)
			if err != nil {
				return fmt.Errorf("problem formatting address: %w", err)
			}
			controlAddrs[i] = addr
		}

		response.Supernets = append(response.Supernets, APISupernet{
			ID:          supernetID,
			ControlKeys: controlAddrs,
			Threshold:   json.Uint32(owner.Threshold),
		})
	}
	return nil
}

// GetStakingAssetIDArgs are the arguments to GetStakingAssetID
type GetStakingAssetIDArgs struct {
	SupernetID ids.ID `json:"supernetID"`
}

// GetStakingAssetIDResponse is the response from calling GetStakingAssetID
type GetStakingAssetIDResponse struct {
	AssetID ids.ID `json:"assetID"`
}

// GetStakingAssetID returns the assetID of the token used to stake on the
// provided supernet
func (s *Service) GetStakingAssetID(_ *http.Request, args *GetStakingAssetIDArgs, response *GetStakingAssetIDResponse) error {
	s.vm.ctx.Log.Debug("Platform: GetStakingAssetID called")

	if args.SupernetID == constants.PrimaryNetworkID {
		response.AssetID = s.vm.ctx.JuneAssetID
		return nil
	}

	transformSupernetIntf, err := s.vm.state.GetSupernetTransformation(args.SupernetID)
	if err != nil {
		return fmt.Errorf(
			"failed fetching supernet transformation for %s: %w",
			args.SupernetID,
			err,
		)
	}
	transformSupernet, ok := transformSupernetIntf.Unsigned.(*txs.TransformSupernetTx)
	if !ok {
		return fmt.Errorf(
			"unexpected supernet transformation tx type fetched %T",
			transformSupernetIntf.Unsigned,
		)
	}

	response.AssetID = transformSupernet.AssetID
	return nil
}

/*
 ******************************************************
 **************** Get/Sample Validators ***************
 ******************************************************
 */

// GetCurrentValidatorsArgs are the arguments for calling GetCurrentValidators
type GetCurrentValidatorsArgs struct {
	// Supernet we're listing the validators of
	// If omitted, defaults to primary network
	SupernetID ids.ID `json:"supernetID"`
	// NodeIDs of validators to request. If [NodeIDs]
	// is empty, it fetches all current validators. If
	// some nodeIDs are not currently validators, they
	// will be omitted from the response.
	NodeIDs []ids.NodeID `json:"nodeIDs"`
}

// GetCurrentValidatorsReply are the results from calling GetCurrentValidators.
// Each validator contains a list of delegators to itself.
type GetCurrentValidatorsReply struct {
	Validators []interface{} `json:"validators"`
}

// GetCurrentValidators returns current validators and delegators
func (s *Service) GetCurrentValidators(_ *http.Request, args *GetCurrentValidatorsArgs, reply *GetCurrentValidatorsReply) error {
	s.vm.ctx.Log.Debug("Platform: GetCurrentValidators called")

	reply.Validators = []interface{}{}

	// Validator's node ID as string --> Delegators to them
	vdrToDelegators := map[ids.NodeID][]relayapi.PrimaryDelegator{}

	// Create set of nodeIDs
	nodeIDs := set.Set[ids.NodeID]{}
	nodeIDs.Add(args.NodeIDs...)
	includeAllNodes := nodeIDs.Len() == 0

	currentStakerIterator, err := s.vm.state.GetCurrentStakerIterator()
	if err != nil {
		return err
	}
	defer currentStakerIterator.Release()

	// TODO: do not iterate over all stakers when nodeIDs given. Use currentValidators.ValidatorSet for iteration
	for currentStakerIterator.Next() { // Iterates in order of increasing stop time
		currentStaker := currentStakerIterator.Value()
		if args.SupernetID != currentStaker.SupernetID {
			continue
		}
		if !includeAllNodes && !nodeIDs.Contains(currentStaker.NodeID) {
			continue
		}

		tx, _, err := s.vm.state.GetTx(currentStaker.TxID)
		if err != nil {
			return err
		}

		nodeID := currentStaker.NodeID
		weight := json.Uint64(currentStaker.Weight)
		apiStaker := relayapi.Staker{
			TxID:        currentStaker.TxID,
			StartTime:   json.Uint64(currentStaker.StartTime.Unix()),
			EndTime:     json.Uint64(currentStaker.EndTime.Unix()),
			StakeAmount: &weight,
			NodeID:      nodeID,
		}

		potentialReward := json.Uint64(currentStaker.PotentialReward)
		switch staker := tx.Unsigned.(type) {
		case txs.ValidatorTx:
			shares := staker.Shares()
			delegationFee := json.Float32(100 * float32(shares) / float32(reward.PercentDenominator))

			uptime, err := s.getAPIUptime(currentStaker)
			if err != nil {
				return err
			}

			connected := s.vm.uptimeManager.IsConnected(nodeID, args.SupernetID)
			var (
				validationRewardOwner *relayapi.Owner
				delegationRewardOwner *relayapi.Owner
			)
			validationOwner, ok := staker.ValidationRewardsOwner().(*secp256k1fx.OutputOwners)
			if ok {
				validationRewardOwner, err = s.getAPIOwner(validationOwner)
				if err != nil {
					return err
				}
			}
			delegationOwner, ok := staker.DelegationRewardsOwner().(*secp256k1fx.OutputOwners)
			if ok {
				delegationRewardOwner, err = s.getAPIOwner(delegationOwner)
				if err != nil {
					return err
				}
			}

			vdr := relayapi.PermissionlessValidator{
				Staker:                apiStaker,
				Uptime:                uptime,
				Connected:             connected,
				PotentialReward:       &potentialReward,
				RewardOwner:           validationRewardOwner,
				ValidationRewardOwner: validationRewardOwner,
				DelegationRewardOwner: delegationRewardOwner,
				DelegationFee:         delegationFee,
			}

			if staker, ok := staker.(*txs.AddPermissionlessValidatorTx); ok {
				if signer, ok := staker.Signer.(*signer.ProofOfPossession); ok {
					vdr.Signer = signer
				}
			}

			reply.Validators = append(reply.Validators, vdr)

		case txs.DelegatorTx:
			var rewardOwner *relayapi.Owner
			owner, ok := staker.RewardsOwner().(*secp256k1fx.OutputOwners)
			if ok {
				rewardOwner, err = s.getAPIOwner(owner)
				if err != nil {
					return err
				}
			}

			delegator := relayapi.PrimaryDelegator{
				Staker:          apiStaker,
				RewardOwner:     rewardOwner,
				PotentialReward: &potentialReward,
			}
			vdrToDelegators[delegator.NodeID] = append(vdrToDelegators[delegator.NodeID], delegator)
		case *txs.AddSupernetValidatorTx:
			uptime, err := s.getAPIUptime(currentStaker)
			if err != nil {
				return err
			}
			connected := s.vm.uptimeManager.IsConnected(nodeID, args.SupernetID)
			reply.Validators = append(reply.Validators, relayapi.PermissionedValidator{
				Staker:    apiStaker,
				Connected: connected,
				Uptime:    uptime,
			})
		default:
			return fmt.Errorf("expected validator but got %T", tx.Unsigned)
		}
	}

	for i, vdrIntf := range reply.Validators {
		vdr, ok := vdrIntf.(relayapi.PermissionlessValidator)
		if !ok {
			continue
		}
		vdr.Delegators = vdrToDelegators[vdr.NodeID]
		reply.Validators[i] = vdr
	}

	return nil
}

// GetPendingValidatorsArgs are the arguments for calling GetPendingValidators
type GetPendingValidatorsArgs struct {
	// Supernet we're getting the pending validators of
	// If omitted, defaults to primary network
	SupernetID ids.ID `json:"supernetID"`
	// NodeIDs of validators to request. If [NodeIDs]
	// is empty, it fetches all pending validators. If
	// some requested nodeIDs are not pending validators,
	// they are omitted from the response.
	NodeIDs []ids.NodeID `json:"nodeIDs"`
}

// GetPendingValidatorsReply are the results from calling GetPendingValidators.
// Unlike GetCurrentValidatorsReply, each validator has a null delegator list.
type GetPendingValidatorsReply struct {
	Validators []interface{} `json:"validators"`
	Delegators []interface{} `json:"delegators"`
}

// GetPendingValidators returns the list of pending validators
func (s *Service) GetPendingValidators(_ *http.Request, args *GetPendingValidatorsArgs, reply *GetPendingValidatorsReply) error {
	s.vm.ctx.Log.Debug("Platform: GetPendingValidators called")

	reply.Validators = []interface{}{}
	reply.Delegators = []interface{}{}

	// Create set of nodeIDs
	nodeIDs := set.Set[ids.NodeID]{}
	nodeIDs.Add(args.NodeIDs...)
	includeAllNodes := nodeIDs.Len() == 0

	pendingStakerIterator, err := s.vm.state.GetPendingStakerIterator()
	if err != nil {
		return err
	}
	defer pendingStakerIterator.Release()

	for pendingStakerIterator.Next() { // Iterates in order of increasing start time
		pendingStaker := pendingStakerIterator.Value()
		if args.SupernetID != pendingStaker.SupernetID {
			continue
		}
		if !includeAllNodes && !nodeIDs.Contains(pendingStaker.NodeID) {
			continue
		}

		tx, _, err := s.vm.state.GetTx(pendingStaker.TxID)
		if err != nil {
			return err
		}

		nodeID := pendingStaker.NodeID
		weight := json.Uint64(pendingStaker.Weight)
		apiStaker := relayapi.Staker{
			TxID:        pendingStaker.TxID,
			NodeID:      nodeID,
			StartTime:   json.Uint64(pendingStaker.StartTime.Unix()),
			EndTime:     json.Uint64(pendingStaker.EndTime.Unix()),
			StakeAmount: &weight,
		}

		switch staker := tx.Unsigned.(type) {
		case txs.ValidatorTx:
			shares := staker.Shares()
			delegationFee := json.Float32(100 * float32(shares) / float32(reward.PercentDenominator))

			connected := s.vm.uptimeManager.IsConnected(nodeID, args.SupernetID)
			vdr := relayapi.PermissionlessValidator{
				Staker:        apiStaker,
				DelegationFee: delegationFee,
				Connected:     connected,
			}

			if staker, ok := staker.(*txs.AddPermissionlessValidatorTx); ok {
				if signer, ok := staker.Signer.(*signer.ProofOfPossession); ok {
					vdr.Signer = signer
				}
			}

			reply.Validators = append(reply.Validators, vdr)

		case txs.DelegatorTx:
			reply.Delegators = append(reply.Delegators, apiStaker)

		case *txs.AddSupernetValidatorTx:
			connected := s.vm.uptimeManager.IsConnected(nodeID, args.SupernetID)
			reply.Validators = append(reply.Validators, relayapi.PermissionedValidator{
				Staker:    apiStaker,
				Connected: connected,
			})
		default:
			return fmt.Errorf("expected validator but got %T", tx.Unsigned)
		}
	}
	return nil
}

// GetCurrentSupplyArgs are the arguments for calling GetCurrentSupply
type GetCurrentSupplyArgs struct {
	SupernetID ids.ID `json:"supernetID"`
}

// GetCurrentSupplyReply are the results from calling GetCurrentSupply
type GetCurrentSupplyReply struct {
	Supply json.Uint64 `json:"supply"`
}

// GetCurrentSupply returns an upper bound on the supply of JUNE in the system
func (s *Service) GetCurrentSupply(_ *http.Request, args *GetCurrentSupplyArgs, reply *GetCurrentSupplyReply) error {
	s.vm.ctx.Log.Debug("Platform: GetCurrentSupply called")

	supply, err := s.vm.state.GetCurrentSupply(args.SupernetID)
	reply.Supply = json.Uint64(supply)
	return err
}

// GetRewardsPoolSupplyArgs are the arguments for calling GetRewardsPoolSupply
type GetRewardsPoolSupplyArgs struct {
	SupernetID ids.ID `json:"supernetID"`
}

// GetRewardsPoolSupplyReply are the results from calling GetRewardsPoolSupply
type GetRewardsPoolSupplyReply struct {
	RewardsPoolSupply json.Uint64 `json:"rewardsPoolSupply"`
}

// GetRewardsPoolSupply returns the supply of the rewards pool
func (s *Service) GetRewardsPoolSupply(_ *http.Request, args *GetRewardsPoolSupplyArgs, reply *GetRewardsPoolSupplyReply) error {
	s.vm.ctx.Log.Debug("Platform: GetCurrentSupply called")

	supply, err := s.vm.state.GetCurrentSupply(args.SupernetID)
	reply.RewardsPoolSupply = json.Uint64(supply)
	return err
}

// GetFeesPoolValueReply are the results from calling GetFeesPoolValue
type GetFeesPoolValueReply struct {
	FeesPoolValue json.Uint64 `json:"feesPoolValue"`
}

// GetFeesPoolValue returns the current value in the fees pool
func (s *Service) GetFeesPoolValue(_ *http.Request, _ *struct{}, reply *GetFeesPoolValueReply) error {
	s.vm.ctx.Log.Debug("Platform: GetFeesPoolValue called")

	reply.FeesPoolValue = json.Uint64(s.vm.state.GetFeesPoolValue())
	return nil
}

// SampleValidatorsArgs are the arguments for calling SampleValidators
type SampleValidatorsArgs struct {
	// Number of validators in the sample
	Size json.Uint16 `json:"size"`

	// ID of supernet to sample validators from
	// If omitted, defaults to the primary network
	SupernetID ids.ID `json:"supernetID"`
}

// SampleValidatorsReply are the results from calling Sample
type SampleValidatorsReply struct {
	Validators []ids.NodeID `json:"validators"`
}

// SampleValidators returns a sampling of the list of current validators
func (s *Service) SampleValidators(_ *http.Request, args *SampleValidatorsArgs, reply *SampleValidatorsReply) error {
	s.vm.ctx.Log.Debug("Platform: SampleValidators called",
		zap.Uint16("size", uint16(args.Size)),
	)

	validators, ok := s.vm.Validators.Get(args.SupernetID)
	if !ok {
		return fmt.Errorf(
			"couldn't get validators of supernet %q. Is it being validated?",
			args.SupernetID,
		)
	}

	sample, err := validators.Sample(int(args.Size))
	if err != nil {
		return fmt.Errorf("sampling errored with %w", err)
	}

	if sample == nil {
		reply.Validators = []ids.NodeID{}
	} else {
		utils.Sort(sample)
		reply.Validators = sample
	}
	return nil
}

/*
 ******************************************************
 ************ Add Validators to Supernets ***************
 ******************************************************
 */

// AddValidatorArgs are the arguments to AddValidator
type AddValidatorArgs struct {
	// User, password, from addrs, change addr
	api.JSONSpendHeader
	relayapi.Staker
	// The address the staking reward, if applicable, will go to
	RewardAddress     string       `json:"rewardAddress"`
	DelegationFeeRate json.Float32 `json:"delegationFeeRate"`
}

// AddValidator creates and signs and issues a transaction to add a validator to
// the primary network
func (s *Service) AddValidator(_ *http.Request, args *AddValidatorArgs, reply *api.JSONTxIDChangeAddr) error {
	s.vm.ctx.Log.Debug("Platform: AddValidator called")

	now := s.vm.clock.Time()
	minAddStakerTime := now.Add(minAddStakerDelay)
	minAddStakerUnix := json.Uint64(minAddStakerTime.Unix())
	maxAddStakerTime := now.Add(executor.MaxFutureStartTime)
	maxAddStakerUnix := json.Uint64(maxAddStakerTime.Unix())

	if args.StartTime == 0 {
		args.StartTime = minAddStakerUnix
	}

	switch {
	case args.RewardAddress == "":
		return errNoRewardAddress
	case args.StartTime < minAddStakerUnix:
		return errStartTimeTooSoon
	case args.StartTime > maxAddStakerUnix:
		return errStartTimeTooLate
	case args.DelegationFeeRate < 0 || args.DelegationFeeRate > 100:
		return errInvalidDelegationRate
	}

	// Parse the node ID
	var nodeID ids.NodeID
	if args.NodeID == ids.EmptyNodeID { // If ID unspecified, use this node's ID
		nodeID = s.vm.ctx.NodeID
	} else {
		nodeID = args.NodeID
	}

	// Parse the from addresses
	fromAddrs, err := june.ParseServiceAddresses(s.addrManager, args.From)
	if err != nil {
		return err
	}

	// Parse the reward address
	rewardAddress, err := june.ParseServiceAddress(s.addrManager, args.RewardAddress)
	if err != nil {
		return fmt.Errorf("problem while parsing reward address: %w", err)
	}

	user, err := keystore.NewUserFromKeystore(s.vm.ctx.Keystore, args.Username, args.Password)
	if err != nil {
		return err
	}
	defer user.Close()

	// Get the user's keys
	privKeys, err := keystore.GetKeychain(user, fromAddrs)
	if err != nil {
		return fmt.Errorf("couldn't get addresses controlled by the user: %w", err)
	}

	// Parse the change address.
	if len(privKeys.Keys) == 0 {
		return errNoKeys
	}
	changeAddr := privKeys.Keys[0].PublicKey().Address() // By default, use a key controlled by the user
	if args.ChangeAddr != "" {
		changeAddr, err = june.ParseServiceAddress(s.addrManager, args.ChangeAddr)
		if err != nil {
			return fmt.Errorf("couldn't parse changeAddr: %w", err)
		}
	}

	// Create the transaction
	tx, err := s.vm.txBuilder.NewAddValidatorTx(
		args.GetWeight(),                     // Stake amount
		uint64(args.StartTime),               // Start time
		uint64(args.EndTime),                 // End time
		nodeID,                               // Node ID
		rewardAddress,                        // Reward Address
		uint32(10000*args.DelegationFeeRate), // Shares
		privKeys.Keys,                        // Keys providing the staked tokens
		changeAddr,
	)
	if err != nil {
		return fmt.Errorf("couldn't create tx: %w", err)
	}

	reply.TxID = tx.ID()
	reply.ChangeAddr, err = s.addrManager.FormatLocalAddress(changeAddr)

	errs := wrappers.Errs{}
	errs.Add(
		err,
		s.vm.Builder.AddUnverifiedTx(tx),
		user.Close(),
	)
	return errs.Err
}

// AddDelegatorArgs are the arguments to AddDelegator
type AddDelegatorArgs struct {
	// User, password, from addrs, change addr
	api.JSONSpendHeader
	relayapi.Staker
	RewardAddress string `json:"rewardAddress"`
}

// AddDelegator creates and signs and issues a transaction to add a delegator to
// the primary network
func (s *Service) AddDelegator(_ *http.Request, args *AddDelegatorArgs, reply *api.JSONTxIDChangeAddr) error {
	s.vm.ctx.Log.Debug("Platform: AddDelegator called")

	now := s.vm.clock.Time()
	minAddStakerTime := now.Add(minAddStakerDelay)
	minAddStakerUnix := json.Uint64(minAddStakerTime.Unix())
	maxAddStakerTime := now.Add(executor.MaxFutureStartTime)
	maxAddStakerUnix := json.Uint64(maxAddStakerTime.Unix())

	if args.StartTime == 0 {
		args.StartTime = minAddStakerUnix
	}

	switch {
	case args.RewardAddress == "":
		return errNoRewardAddress
	case args.StartTime < minAddStakerUnix:
		return errStartTimeTooSoon
	case args.StartTime > maxAddStakerUnix:
		return errStartTimeTooLate
	}

	var nodeID ids.NodeID
	if args.NodeID == ids.EmptyNodeID { // If ID unspecified, use this node's ID
		nodeID = s.vm.ctx.NodeID
	} else {
		nodeID = args.NodeID
	}

	// Parse the reward address
	rewardAddress, err := june.ParseServiceAddress(s.addrManager, args.RewardAddress)
	if err != nil {
		return fmt.Errorf("problem parsing 'rewardAddress': %w", err)
	}

	// Parse the from addresses
	fromAddrs, err := june.ParseServiceAddresses(s.addrManager, args.From)
	if err != nil {
		return err
	}

	user, err := keystore.NewUserFromKeystore(s.vm.ctx.Keystore, args.Username, args.Password)
	if err != nil {
		return err
	}
	defer user.Close()

	privKeys, err := keystore.GetKeychain(user, fromAddrs)
	if err != nil {
		return fmt.Errorf("couldn't get addresses controlled by the user: %w", err)
	}

	// Parse the change address. Assumes that if the user has no keys,
	// this operation will fail so the change address can be anything.
	if len(privKeys.Keys) == 0 {
		return errNoKeys
	}
	changeAddr := privKeys.Keys[0].PublicKey().Address() // By default, use a key controlled by the user
	if args.ChangeAddr != "" {
		changeAddr, err = june.ParseServiceAddress(s.addrManager, args.ChangeAddr)
		if err != nil {
			return fmt.Errorf("couldn't parse changeAddr: %w", err)
		}
	}

	// Create the transaction
	tx, err := s.vm.txBuilder.NewAddDelegatorTx(
		args.GetWeight(),       // Stake amount
		uint64(args.StartTime), // Start time
		uint64(args.EndTime),   // End time
		nodeID,                 // Node ID
		rewardAddress,          // Reward Address
		privKeys.Keys,          // Private keys
		changeAddr,             // Change address
	)
	if err != nil {
		return fmt.Errorf("couldn't create tx: %w", err)
	}

	reply.TxID = tx.ID()
	reply.ChangeAddr, err = s.addrManager.FormatLocalAddress(changeAddr)

	errs := wrappers.Errs{}
	errs.Add(
		err,
		s.vm.Builder.AddUnverifiedTx(tx),
		user.Close(),
	)
	return errs.Err
}

// AddSupernetValidatorArgs are the arguments to AddSupernetValidator
type AddSupernetValidatorArgs struct {
	// User, password, from addrs, change addr
	api.JSONSpendHeader
	relayapi.Staker
	// ID of supernet to validate
	SupernetID string `json:"supernetID"`
}

// AddSupernetValidator creates and signs and issues a transaction to add a
// validator to a supernet other than the primary network
func (s *Service) AddSupernetValidator(_ *http.Request, args *AddSupernetValidatorArgs, response *api.JSONTxIDChangeAddr) error {
	s.vm.ctx.Log.Debug("Platform: AddSupernetValidator called")

	now := s.vm.clock.Time()
	minAddStakerTime := now.Add(minAddStakerDelay)
	minAddStakerUnix := json.Uint64(minAddStakerTime.Unix())
	maxAddStakerTime := now.Add(executor.MaxFutureStartTime)
	maxAddStakerUnix := json.Uint64(maxAddStakerTime.Unix())

	if args.StartTime == 0 {
		args.StartTime = minAddStakerUnix
	}

	switch {
	case args.SupernetID == "":
		return errNoSupernetID
	case args.StartTime < minAddStakerUnix:
		return errStartTimeTooSoon
	case args.StartTime > maxAddStakerUnix:
		return errStartTimeTooLate
	}

	// Parse the supernet ID
	supernetID, err := ids.FromString(args.SupernetID)
	if err != nil {
		return fmt.Errorf("problem parsing supernetID %q: %w", args.SupernetID, err)
	}
	if supernetID == constants.PrimaryNetworkID {
		return errNamedSupernetCantBePrimary
	}

	// Parse the from addresses
	fromAddrs, err := june.ParseServiceAddresses(s.addrManager, args.From)
	if err != nil {
		return err
	}

	user, err := keystore.NewUserFromKeystore(s.vm.ctx.Keystore, args.Username, args.Password)
	if err != nil {
		return err
	}
	defer user.Close()

	keys, err := keystore.GetKeychain(user, fromAddrs)
	if err != nil {
		return fmt.Errorf("couldn't get addresses controlled by the user: %w", err)
	}

	// Parse the change address.
	if len(keys.Keys) == 0 {
		return errNoKeys
	}
	changeAddr := keys.Keys[0].PublicKey().Address() // By default, use a key controlled by the user
	if args.ChangeAddr != "" {
		changeAddr, err = june.ParseServiceAddress(s.addrManager, args.ChangeAddr)
		if err != nil {
			return fmt.Errorf("couldn't parse changeAddr: %w", err)
		}
	}

	// Create the transaction
	tx, err := s.vm.txBuilder.NewAddSupernetValidatorTx(
		args.GetWeight(),       // Stake amount
		uint64(args.StartTime), // Start time
		uint64(args.EndTime),   // End time
		args.NodeID,            // Node ID
		supernetID,             // Supernet ID
		keys.Keys,
		changeAddr,
	)
	if err != nil {
		return fmt.Errorf("couldn't create tx: %w", err)
	}

	response.TxID = tx.ID()
	response.ChangeAddr, err = s.addrManager.FormatLocalAddress(changeAddr)

	errs := wrappers.Errs{}
	errs.Add(
		err,
		s.vm.Builder.AddUnverifiedTx(tx),
		user.Close(),
	)
	return errs.Err
}

// CreateSupernetArgs are the arguments to CreateSupernet
type CreateSupernetArgs struct {
	// User, password, from addrs, change addr
	api.JSONSpendHeader
	// The ID member of APISupernet is ignored
	APISupernet
}

// CreateSupernet creates and signs and issues a transaction to create a new
// supernet
func (s *Service) CreateSupernet(_ *http.Request, args *CreateSupernetArgs, response *api.JSONTxIDChangeAddr) error {
	s.vm.ctx.Log.Debug("Platform: CreateSupernet called")

	// Parse the control keys
	controlKeys, err := june.ParseServiceAddresses(s.addrManager, args.ControlKeys)
	if err != nil {
		return err
	}

	// Parse the from addresses
	fromAddrs, err := june.ParseServiceAddresses(s.addrManager, args.From)
	if err != nil {
		return err
	}

	user, err := keystore.NewUserFromKeystore(s.vm.ctx.Keystore, args.Username, args.Password)
	if err != nil {
		return err
	}
	defer user.Close()

	privKeys, err := keystore.GetKeychain(user, fromAddrs)
	if err != nil {
		return fmt.Errorf("couldn't get addresses controlled by the user: %w", err)
	}

	// Parse the change address. Assumes that if the user has no keys,
	// this operation will fail so the change address can be anything.
	if len(privKeys.Keys) == 0 {
		return errNoKeys
	}
	changeAddr := privKeys.Keys[0].PublicKey().Address() // By default, use a key controlled by the user
	if args.ChangeAddr != "" {
		changeAddr, err = june.ParseServiceAddress(s.addrManager, args.ChangeAddr)
		if err != nil {
			return fmt.Errorf("couldn't parse changeAddr: %w", err)
		}
	}

	// Create the transaction
	tx, err := s.vm.txBuilder.NewCreateSupernetTx(
		uint32(args.Threshold), // Threshold
		controlKeys.List(),     // Control Addresses
		privKeys.Keys,          // Private keys
		changeAddr,
	)
	if err != nil {
		return fmt.Errorf("couldn't create tx: %w", err)
	}

	response.TxID = tx.ID()
	response.ChangeAddr, err = s.addrManager.FormatLocalAddress(changeAddr)

	errs := wrappers.Errs{}
	errs.Add(
		err,
		s.vm.Builder.AddUnverifiedTx(tx),
		user.Close(),
	)
	return errs.Err
}

// ExportJUNEArgs are the arguments to ExportJUNE
type ExportJUNEArgs struct {
	// User, password, from addrs, change addr
	api.JSONSpendHeader

	// Amount of JUNE to send
	Amount json.Uint64 `json:"amount"`

	// Chain the funds are going to. Optional. Used if To address does not include the chainID.
	TargetChain string `json:"targetChain"`

	// ID of the address that will receive the JUNE. This address may include the
	// chainID, which is used to determine what the destination chain is.
	To string `json:"to"`
}

// ExportJUNE exports JUNE from the P-Chain to the X-Chain
// It must be imported on the X-Chain to complete the transfer
func (s *Service) ExportJUNE(_ *http.Request, args *ExportJUNEArgs, response *api.JSONTxIDChangeAddr) error {
	s.vm.ctx.Log.Debug("Platform: ExportJUNE called")

	if args.Amount == 0 {
		return errNoAmount
	}

	// Get the chainID and parse the to address
	chainID, to, err := s.addrManager.ParseAddress(args.To)
	if err != nil {
		chainID, err = s.vm.ctx.BCLookup.Lookup(args.TargetChain)
		if err != nil {
			return err
		}
		to, err = ids.ShortFromString(args.To)
		if err != nil {
			return err
		}
	}

	// Parse the from addresses
	fromAddrs, err := june.ParseServiceAddresses(s.addrManager, args.From)
	if err != nil {
		return err
	}

	user, err := keystore.NewUserFromKeystore(s.vm.ctx.Keystore, args.Username, args.Password)
	if err != nil {
		return err
	}
	defer user.Close()

	privKeys, err := keystore.GetKeychain(user, fromAddrs)
	if err != nil {
		return fmt.Errorf("couldn't get addresses controlled by the user: %w", err)
	}

	// Parse the change address. Assumes that if the user has no keys,
	// this operation will fail so the change address can be anything.
	if len(privKeys.Keys) == 0 {
		return errNoKeys
	}
	changeAddr := privKeys.Keys[0].PublicKey().Address() // By default, use a key controlled by the user
	if args.ChangeAddr != "" {
		changeAddr, err = june.ParseServiceAddress(s.addrManager, args.ChangeAddr)
		if err != nil {
			return fmt.Errorf("couldn't parse changeAddr: %w", err)
		}
	}

	// Create the transaction
	tx, err := s.vm.txBuilder.NewExportTx(
		uint64(args.Amount), // Amount
		chainID,             // ID of the chain to send the funds to
		to,                  // Address
		privKeys.Keys,       // Private keys
		changeAddr,          // Change address
	)
	if err != nil {
		return fmt.Errorf("couldn't create tx: %w", err)
	}

	response.TxID = tx.ID()
	response.ChangeAddr, err = s.addrManager.FormatLocalAddress(changeAddr)

	errs := wrappers.Errs{}
	errs.Add(
		err,
		s.vm.Builder.AddUnverifiedTx(tx),
		user.Close(),
	)
	return errs.Err
}

// ImportJUNEArgs are the arguments to ImportJUNE
type ImportJUNEArgs struct {
	// User, password, from addrs, change addr
	api.JSONSpendHeader

	// Chain the funds are coming from
	SourceChain string `json:"sourceChain"`

	// The address that will receive the imported funds
	To string `json:"to"`
}

// ImportJUNE issues a transaction to import JUNE from the X-chain. The JUNE
// must have already been exported from the X-Chain.
func (s *Service) ImportJUNE(_ *http.Request, args *ImportJUNEArgs, response *api.JSONTxIDChangeAddr) error {
	s.vm.ctx.Log.Debug("Platform: ImportJUNE called")

	// Parse the sourceCHain
	chainID, err := s.vm.ctx.BCLookup.Lookup(args.SourceChain)
	if err != nil {
		return fmt.Errorf("problem parsing chainID %q: %w", args.SourceChain, err)
	}

	// Parse the to address
	to, err := june.ParseServiceAddress(s.addrManager, args.To)
	if err != nil { // Parse address
		return fmt.Errorf("couldn't parse argument 'to' to an address: %w", err)
	}

	// Parse the from addresses
	fromAddrs, err := june.ParseServiceAddresses(s.addrManager, args.From)
	if err != nil {
		return err
	}

	user, err := keystore.NewUserFromKeystore(s.vm.ctx.Keystore, args.Username, args.Password)
	if err != nil {
		return err
	}
	defer user.Close()

	privKeys, err := keystore.GetKeychain(user, fromAddrs)
	if err != nil { // Get keys
		return fmt.Errorf("couldn't get keys controlled by the user: %w", err)
	}

	// Parse the change address. Assumes that if the user has no keys,
	// this operation will fail so the change address can be anything.
	if len(privKeys.Keys) == 0 {
		return errNoKeys
	}
	changeAddr := privKeys.Keys[0].PublicKey().Address() // By default, use a key controlled by the user
	if args.ChangeAddr != "" {
		changeAddr, err = june.ParseServiceAddress(s.addrManager, args.ChangeAddr)
		if err != nil {
			return fmt.Errorf("couldn't parse changeAddr: %w", err)
		}
	}

	tx, err := s.vm.txBuilder.NewImportTx(
		chainID,
		to,
		privKeys.Keys,
		changeAddr,
	)
	if err != nil {
		return err
	}

	response.TxID = tx.ID()
	response.ChangeAddr, err = s.addrManager.FormatLocalAddress(changeAddr)

	errs := wrappers.Errs{}
	errs.Add(
		err,
		s.vm.Builder.AddUnverifiedTx(tx),
		user.Close(),
	)
	return errs.Err
}

/*
 ******************************************************
 ******** Create/get status of a blockchain ***********
 ******************************************************
 */

// CreateBlockchainArgs is the arguments for calling CreateBlockchain
type CreateBlockchainArgs struct {
	// User, password, from addrs, change addr
	api.JSONSpendHeader
	// ID of Supernet that validates the new blockchain
	SupernetID ids.ID `json:"supernetID"`
	// ID of the VM the new blockchain is running
	VMID string `json:"vmID"`
	// IDs of the FXs the VM is running
	FxIDs []string `json:"fxIDs"`
	// Human-readable name for the new blockchain, not necessarily unique
	Name string `json:"name"`
	// Main asset used by the chain
	ChainAssetID ids.ID `json:"chainAssetID"`
	// Genesis state of the blockchain being created
	GenesisData string `json:"genesisData"`
	// Encoding format to use for genesis data
	Encoding formatting.Encoding `json:"encoding"`
}

// CreateBlockchain issues a transaction to create a new blockchain
func (s *Service) CreateBlockchain(_ *http.Request, args *CreateBlockchainArgs, response *api.JSONTxIDChangeAddr) error {
	s.vm.ctx.Log.Debug("Platform: CreateBlockchain called")

	switch {
	case args.Name == "":
		return errMissingName
	case args.VMID == "":
		return errMissingVMID
	}

	genesisBytes, err := formatting.Decode(args.Encoding, args.GenesisData)
	if err != nil {
		return fmt.Errorf("problem parsing genesis data: %w", err)
	}

	vmID, err := s.vm.Chains.LookupVM(args.VMID)
	if err != nil {
		return fmt.Errorf("no VM with ID '%s' found", args.VMID)
	}

	fxIDs := []ids.ID(nil)
	for _, fxIDStr := range args.FxIDs {
		fxID, err := s.vm.Chains.LookupVM(fxIDStr)
		if err != nil {
			return fmt.Errorf("no FX with ID '%s' found", fxIDStr)
		}
		fxIDs = append(fxIDs, fxID)
	}
	// If creating JVM instance, use secp256k1fx
	// TODO: Document FXs and have user specify them in API call
	fxIDsSet := set.Set[ids.ID]{}
	fxIDsSet.Add(fxIDs...)
	if vmID == constants.JVMID && !fxIDsSet.Contains(secp256k1fx.ID) {
		fxIDs = append(fxIDs, secp256k1fx.ID)
	}

	if args.SupernetID == constants.PrimaryNetworkID {
		return txs.ErrCantValidatePrimaryNetwork
	}

	// Parse the from addresses
	fromAddrs, err := june.ParseServiceAddresses(s.addrManager, args.From)
	if err != nil {
		return err
	}

	user, err := keystore.NewUserFromKeystore(s.vm.ctx.Keystore, args.Username, args.Password)
	if err != nil {
		return err
	}
	defer user.Close()

	keys, err := keystore.GetKeychain(user, fromAddrs)
	if err != nil {
		return fmt.Errorf("couldn't get addresses controlled by the user: %w", err)
	}

	// Parse the change address. Assumes that if the user has no keys,
	// this operation will fail so the change address can be anything.
	if len(keys.Keys) == 0 {
		return errNoKeys
	}
	changeAddr := keys.Keys[0].PublicKey().Address() // By default, use a key controlled by the user
	if args.ChangeAddr != "" {
		changeAddr, err = june.ParseServiceAddress(s.addrManager, args.ChangeAddr)
		if err != nil {
			return fmt.Errorf("couldn't parse changeAddr: %w", err)
		}
	}

	// Create the transaction
	tx, err := s.vm.txBuilder.NewCreateChainTx(
		args.SupernetID,
		genesisBytes,
		vmID,
		fxIDs,
		args.Name,
		args.ChainAssetID,
		keys.Keys,
		changeAddr, // Change address
	)
	if err != nil {
		return fmt.Errorf("couldn't create tx: %w", err)
	}

	response.TxID = tx.ID()
	response.ChangeAddr, err = s.addrManager.FormatLocalAddress(changeAddr)

	errs := wrappers.Errs{}
	errs.Add(
		err,
		s.vm.Builder.AddUnverifiedTx(tx),
		user.Close(),
	)
	return errs.Err
}

// GetBlockchainStatusArgs is the arguments for calling GetBlockchainStatus
// [BlockchainID] is the ID of or an alias of the blockchain to get the status of.
type GetBlockchainStatusArgs struct {
	BlockchainID string `json:"blockchainID"`
}

// GetBlockchainStatusReply is the reply from calling GetBlockchainStatus
// [Status] is the blockchain's status.
type GetBlockchainStatusReply struct {
	Status status.BlockchainStatus `json:"status"`
}

// GetBlockchainStatus gets the status of a blockchain with the ID [args.BlockchainID].
func (s *Service) GetBlockchainStatus(r *http.Request, args *GetBlockchainStatusArgs, reply *GetBlockchainStatusReply) error {
	s.vm.ctx.Log.Debug("Platform: GetBlockchainStatus called")

	if args.BlockchainID == "" {
		return errMissingBlockchainID
	}

	// if its aliased then vm created this chain.
	if aliasedID, err := s.vm.Chains.Lookup(args.BlockchainID); err == nil {
		if s.nodeValidates(aliasedID) {
			reply.Status = status.Validating
			return nil
		}

		reply.Status = status.Syncing
		return nil
	}

	blockchainID, err := ids.FromString(args.BlockchainID)
	if err != nil {
		return fmt.Errorf("problem parsing blockchainID %q: %w", args.BlockchainID, err)
	}

	ctx := r.Context()
	lastAcceptedID, err := s.vm.LastAccepted(ctx)
	if err != nil {
		return fmt.Errorf("problem loading last accepted ID: %w", err)
	}

	exists, err := s.chainExists(ctx, lastAcceptedID, blockchainID)
	if err != nil {
		return fmt.Errorf("problem looking up blockchain: %w", err)
	}
	if exists {
		reply.Status = status.Created
		return nil
	}

	preferredBlk, err := s.vm.Preferred()
	if err != nil {
		return fmt.Errorf("could not retrieve preferred block, err %w", err)
	}
	preferred, err := s.chainExists(ctx, preferredBlk.ID(), blockchainID)
	if err != nil {
		return fmt.Errorf("problem looking up blockchain: %w", err)
	}
	if preferred {
		reply.Status = status.Preferred
	} else {
		reply.Status = status.UnknownChain
	}
	return nil
}

func (s *Service) nodeValidates(blockchainID ids.ID) bool {
	chainTx, _, err := s.vm.state.GetTx(blockchainID)
	if err != nil {
		return false
	}

	chain, ok := chainTx.Unsigned.(*txs.CreateChainTx)
	if !ok {
		return false
	}

	validators, ok := s.vm.Validators.Get(chain.SupernetID)
	if !ok {
		return false
	}

	return validators.Contains(s.vm.ctx.NodeID)
}

func (s *Service) chainExists(ctx context.Context, blockID ids.ID, chainID ids.ID) (bool, error) {
	state, ok := s.vm.manager.GetState(blockID)
	if !ok {
		block, err := s.vm.GetBlock(ctx, blockID)
		if err != nil {
			return false, err
		}
		state, ok = s.vm.manager.GetState(block.Parent())
		if !ok {
			return false, errMissingDecisionBlock
		}
	}

	tx, _, err := state.GetTx(chainID)
	if err == database.ErrNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	_, ok = tx.Unsigned.(*txs.CreateChainTx)
	return ok, nil
}

// ValidatedByArgs is the arguments for calling ValidatedBy
type ValidatedByArgs struct {
	// ValidatedBy returns the ID of the Supernet validating the blockchain with this ID
	BlockchainID ids.ID `json:"blockchainID"`
}

// ValidatedByResponse is the reply from calling ValidatedBy
type ValidatedByResponse struct {
	// ID of the Supernet validating the specified blockchain
	SupernetID ids.ID `json:"supernetID"`
}

// ValidatedBy returns the ID of the Supernet that validates [args.BlockchainID]
func (s *Service) ValidatedBy(r *http.Request, args *ValidatedByArgs, response *ValidatedByResponse) error {
	s.vm.ctx.Log.Debug("Platform: ValidatedBy called")

	var err error
	ctx := r.Context()
	response.SupernetID, err = s.vm.GetSupernetID(ctx, args.BlockchainID)
	return err
}

// ValidatesArgs are the arguments to Validates
type ValidatesArgs struct {
	SupernetID ids.ID `json:"supernetID"`
}

// ValidatesResponse is the response from calling Validates
type ValidatesResponse struct {
	BlockchainIDs []ids.ID `json:"blockchainIDs"`
}

// Validates returns the IDs of the blockchains validated by [args.SupernetID]
func (s *Service) Validates(_ *http.Request, args *ValidatesArgs, response *ValidatesResponse) error {
	s.vm.ctx.Log.Debug("Platform: Validates called")

	if args.SupernetID != constants.PrimaryNetworkID {
		supernetTx, _, err := s.vm.state.GetTx(args.SupernetID)
		if err != nil {
			return fmt.Errorf(
				"problem retrieving supernet %q: %w",
				args.SupernetID,
				err,
			)
		}
		_, ok := supernetTx.Unsigned.(*txs.CreateSupernetTx)
		if !ok {
			return fmt.Errorf("%q is not a supernet", args.SupernetID)
		}
	}

	// Get the chains that exist
	chains, err := s.vm.state.GetChains(args.SupernetID)
	if err != nil {
		return fmt.Errorf("problem retrieving chains for supernet %q: %w", args.SupernetID, err)
	}

	response.BlockchainIDs = make([]ids.ID, len(chains))
	for i, chain := range chains {
		response.BlockchainIDs[i] = chain.ID()
	}
	return nil
}

// APIBlockchain is the representation of a blockchain used in API calls
type APIBlockchain struct {
	// Blockchain's ID
	ID ids.ID `json:"id"`

	// Blockchain's (non-unique) human-readable name
	Name string `json:"name"`

	// Supernet that validates the blockchain
	SupernetID ids.ID `json:"supernetID"`

	// Virtual Machine the blockchain runs
	VMID ids.ID `json:"vmID"`

	// The chain asset used to pay fees
	ChainAssetID ids.ID `json:"chainAssetID"`
}

// GetBlockchainsResponse is the response from a call to GetBlockchains
type GetBlockchainsResponse struct {
	// blockchains that exist
	Blockchains []APIBlockchain `json:"blockchains"`
}

// GetBlockchains returns all of the blockchains that exist
func (s *Service) GetBlockchains(_ *http.Request, _ *struct{}, response *GetBlockchainsResponse) error {
	s.vm.ctx.Log.Debug("Platform: GetBlockchains called")

	supernets, err := s.vm.state.GetSupernets()
	if err != nil {
		return fmt.Errorf("couldn't retrieve supernets: %w", err)
	}

	response.Blockchains = []APIBlockchain{}
	for _, supernet := range supernets {
		supernetID := supernet.ID()
		chains, err := s.vm.state.GetChains(supernetID)
		if err != nil {
			return fmt.Errorf(
				"couldn't retrieve chains for supernet %q: %w",
				supernetID,
				err,
			)
		}

		for _, chainTx := range chains {
			chainID := chainTx.ID()
			chain, ok := chainTx.Unsigned.(*txs.CreateChainTx)
			if !ok {
				return fmt.Errorf("expected tx type *txs.CreateChainTx but got %T", chainTx.Unsigned)
			}
			response.Blockchains = append(response.Blockchains, APIBlockchain{
				ID:           chainID,
				Name:         chain.ChainName,
				SupernetID:   supernetID,
				VMID:         chain.VMID,
				ChainAssetID: chain.ChainAssetID,
			})
		}
	}

	chains, err := s.vm.state.GetChains(constants.PrimaryNetworkID)
	if err != nil {
		return fmt.Errorf("couldn't retrieve supernets: %w", err)
	}
	for _, chainTx := range chains {
		chainID := chainTx.ID()
		chain, ok := chainTx.Unsigned.(*txs.CreateChainTx)
		if !ok {
			return fmt.Errorf("expected tx type *txs.CreateChainTx but got %T", chainTx.Unsigned)
		}
		response.Blockchains = append(response.Blockchains, APIBlockchain{
			ID:           chainID,
			Name:         chain.ChainName,
			SupernetID:   constants.PrimaryNetworkID,
			VMID:         chain.VMID,
			ChainAssetID: chain.ChainAssetID,
		})
	}

	return nil
}

// GetBlockchainArgs is the arguments for calling GetBlockchain
// [BlockchainID] is the ID of or an alias of the blockchain to get the data of.
type GetBlockchainArgs struct {
	BlockchainID string `json:"blockchainID"`
}

// GetBlockchainResponse is the response from calling GetBlockchain
// [Blockchain] is the blockchain's data.
type GetBlockchainResponse struct {
	Blockchain APIBlockchain `json:"blockchain"`
}

// GetBlockchain gets the data of a blockchain with the ID [args.BlockchainID].
func (service *Service) GetBlockchain(_ *http.Request, args *GetBlockchainArgs, response *GetBlockchainResponse) error {
	service.vm.ctx.Log.Debug("Platform: GetBlockchain called")

	if args.BlockchainID == "" {
		return errMissingBlockchainID
	}

	bID, err := service.vm.Chains.Lookup(args.BlockchainID)
	if err != nil {
		return fmt.Errorf("problem looking up blockchain: %w", err)
	}

	supernetID, err := service.vm.Chains.SupernetID(bID)
	if err != nil {
		return fmt.Errorf("problem looking up supernet: %w", err)
	}

	chains, err := service.vm.state.GetChains(supernetID)
	if err != nil {
		return fmt.Errorf(
			"couldn't retrieve chains for supernet %q: %w",
			supernetID,
			err,
		)
	}

	for _, chainTx := range chains {
		chainID := chainTx.ID()
		chain, ok := chainTx.Unsigned.(*txs.CreateChainTx)
		if !ok {
			return fmt.Errorf("expected tx type *txs.CreateChainTx but got %T", chainTx.Unsigned)
		}
		if chain.BlockchainID == bID {
			response.Blockchain = APIBlockchain{
				ID:           chainID,
				Name:         chain.ChainName,
				SupernetID:   supernetID,
				VMID:         chain.VMID,
				ChainAssetID: chain.ChainAssetID,
			}
			return nil
		}
	}

	return fmt.Errorf("could not find data for blockchain %q", args.BlockchainID)
}

// IssueTx issues a tx
func (s *Service) IssueTx(_ *http.Request, args *api.FormattedTx, response *api.JSONTxID) error {
	s.vm.ctx.Log.Debug("Platform: IssueTx called")

	txBytes, err := formatting.Decode(args.Encoding, args.Tx)
	if err != nil {
		return fmt.Errorf("problem decoding transaction: %w", err)
	}
	tx, err := txs.Parse(txs.Codec, txBytes)
	if err != nil {
		return fmt.Errorf("couldn't parse tx: %w", err)
	}
	if err := s.vm.Builder.AddUnverifiedTx(tx); err != nil {
		return fmt.Errorf("couldn't issue tx: %w", err)
	}

	response.TxID = tx.ID()
	return nil
}

// GetTx gets a tx
func (s *Service) GetTx(_ *http.Request, args *api.GetTxArgs, response *api.GetTxReply) error {
	s.vm.ctx.Log.Debug("Platform: GetTx called")

	tx, _, err := s.vm.state.GetTx(args.TxID)
	if err != nil {
		return fmt.Errorf("couldn't get tx: %w", err)
	}
	txBytes := tx.Bytes()
	response.Encoding = args.Encoding

	if args.Encoding == formatting.JSON {
		tx.Unsigned.InitCtx(s.vm.ctx)
		response.Tx = tx
		return nil
	}

	response.Tx, err = formatting.Encode(args.Encoding, txBytes)
	if err != nil {
		return fmt.Errorf("couldn't encode tx as a string: %w", err)
	}
	return nil
}

type GetTxStatusArgs struct {
	TxID ids.ID `json:"txID"`
	// Returns a response that looks like this:
	// {
	// 	"jsonrpc": "2.0",
	// 	"result": {
	//     "status":"[Status]",
	//     "reason":"[Reason tx was dropped, if applicable]"
	//  },
	// 	"id": 1
	// }
	// "reason" is only present if the status is dropped
}

type GetTxStatusResponse struct {
	Status status.Status `json:"status"`
	// Reason this tx was dropped.
	// Only non-empty if Status is dropped
	Reason string `json:"reason,omitempty"`
}

// GetTxStatus gets a tx's status
func (s *Service) GetTxStatus(_ *http.Request, args *GetTxStatusArgs, response *GetTxStatusResponse) error {
	s.vm.ctx.Log.Debug("Platform: GetTxStatus called",
		zap.Stringer("txID", args.TxID),
	)

	_, txStatus, err := s.vm.state.GetTx(args.TxID)
	if err == nil { // Found the status. Report it.
		response.Status = txStatus
		return nil
	}
	if err != database.ErrNotFound {
		return err
	}

	// The status of this transaction is not in the database - check if the tx
	// is in the preferred block's db. If so, return that it's processing.
	prefBlk, err := s.vm.Preferred()
	if err != nil {
		return err
	}

	preferredID := prefBlk.ID()
	onAccept, ok := s.vm.manager.GetState(preferredID)
	if !ok {
		return fmt.Errorf("could not retrieve state for block %s", preferredID)
	}

	_, _, err = onAccept.GetTx(args.TxID)
	if err == nil {
		// Found the status in the preferred block's db. Report tx is processing.
		response.Status = status.Processing
		return nil
	}
	if err != database.ErrNotFound {
		return err
	}

	if s.vm.Builder.Has(args.TxID) {
		// Found the tx in the mempool. Report tx is processing.
		response.Status = status.Processing
		return nil
	}

	// Note: we check if tx is dropped only after having looked for it
	// in the database and the mempool, because dropped txs may be re-issued.
	reason, dropped := s.vm.Builder.GetDropReason(args.TxID)
	if !dropped {
		// The tx isn't being tracked by the node.
		response.Status = status.Unknown
		return nil
	}

	// The tx was recently dropped because it was invalid.
	response.Status = status.Dropped
	response.Reason = reason
	return nil
}

type GetStakeArgs struct {
	api.JSONAddresses
	Encoding formatting.Encoding `json:"encoding"`
}

// GetStakeReply is the response from calling GetStake.
type GetStakeReply struct {
	Staked  json.Uint64            `json:"staked"`
	Stakeds map[ids.ID]json.Uint64 `json:"stakeds"`
	// String representation of staked outputs
	// Each is of type june.TransferableOutput
	Outputs []string `json:"stakedOutputs"`
	// Encoding of [Outputs]
	Encoding formatting.Encoding `json:"encoding"`
}

// GetStake returns the amount of nJune that [args.Addresses] have cumulatively
// staked on the Primary Network.
//
// This method assumes that each stake output has only owner
// This method assumes only JUNE can be staked
// This method only concerns itself with the Primary Network, not supernets
// TODO: Improve the performance of this method by maintaining this data
// in a data structure rather than re-calculating it by iterating over stakers
func (s *Service) GetStake(_ *http.Request, args *GetStakeArgs, response *GetStakeReply) error {
	s.vm.ctx.Log.Debug("Platform: GetStake called")

	if len(args.Addresses) > maxGetStakeAddrs {
		return fmt.Errorf("%d addresses provided but this method can take at most %d", len(args.Addresses), maxGetStakeAddrs)
	}

	addrs, err := june.ParseServiceAddresses(s.addrManager, args.Addresses)
	if err != nil {
		return err
	}

	currentStakerIterator, err := s.vm.state.GetCurrentStakerIterator()
	if err != nil {
		return err
	}
	defer currentStakerIterator.Release()

	var (
		totalAmountStaked = make(map[ids.ID]uint64)
		stakedOuts        []june.TransferableOutput
	)
	for currentStakerIterator.Next() { // Iterates over current stakers
		staker := currentStakerIterator.Value()

		tx, _, err := s.vm.state.GetTx(staker.TxID)
		if err != nil {
			return err
		}

		stakedOuts = append(stakedOuts, getStakeHelper(tx, addrs, totalAmountStaked)...)
	}

	pendingStakerIterator, err := s.vm.state.GetPendingStakerIterator()
	if err != nil {
		return err
	}
	defer pendingStakerIterator.Release()

	for pendingStakerIterator.Next() { // Iterates over pending stakers
		staker := pendingStakerIterator.Value()

		tx, _, err := s.vm.state.GetTx(staker.TxID)
		if err != nil {
			return err
		}

		stakedOuts = append(stakedOuts, getStakeHelper(tx, addrs, totalAmountStaked)...)
	}

	response.Stakeds = newJSONBalanceMap(totalAmountStaked)
	response.Staked = response.Stakeds[s.vm.ctx.JuneAssetID]
	response.Outputs = make([]string, len(stakedOuts))
	for i, output := range stakedOuts {
		bytes, err := txs.Codec.Marshal(txs.Version, output)
		if err != nil {
			return fmt.Errorf("couldn't serialize output %s: %w", output.ID, err)
		}
		response.Outputs[i], err = formatting.Encode(args.Encoding, bytes)
		if err != nil {
			return fmt.Errorf("couldn't encode output %s as string: %w", output.ID, err)
		}
	}
	response.Encoding = args.Encoding

	return nil
}

// GetMinStakeArgs are the arguments for calling GetMinStake.
type GetMinStakeArgs struct {
	SupernetID ids.ID `json:"supernetID"`
}

// GetMinStakeReply is the response from calling GetMinStake.
type GetMinStakeReply struct {
	//  The minimum amount of tokens one must bond to be a validator
	MinValidatorStake json.Uint64 `json:"minValidatorStake"`
	// Minimum stake, in nJune, that can be delegated on the primary network
	MinDelegatorStake json.Uint64 `json:"minDelegatorStake"`
}

// GetMinStake returns the minimum staking amount in nJune.
func (s *Service) GetMinStake(_ *http.Request, args *GetMinStakeArgs, reply *GetMinStakeReply) error {
	if args.SupernetID == constants.PrimaryNetworkID {
		reply.MinValidatorStake = json.Uint64(s.vm.MinValidatorStake)
		reply.MinDelegatorStake = json.Uint64(s.vm.MinDelegatorStake)
		return nil
	}

	transformSupernetIntf, err := s.vm.state.GetSupernetTransformation(args.SupernetID)
	if err != nil {
		return fmt.Errorf(
			"failed fetching supernet transformation for %s: %w",
			args.SupernetID,
			err,
		)
	}
	transformSupernet, ok := transformSupernetIntf.Unsigned.(*txs.TransformSupernetTx)
	if !ok {
		return fmt.Errorf(
			"unexpected supernet transformation tx type fetched %T",
			transformSupernetIntf.Unsigned,
		)
	}

	reply.MinValidatorStake = json.Uint64(transformSupernet.MinValidatorStake)
	reply.MinDelegatorStake = json.Uint64(transformSupernet.MinDelegatorStake)

	return nil
}

// GetTotalStakeArgs are the arguments for calling GetTotalStake
type GetTotalStakeArgs struct {
	// Supernet we're getting the total stake
	// If omitted returns Primary network weight
	SupernetID ids.ID `json:"supernetID"`
}

// GetTotalStakeReply is the response from calling GetTotalStake.
type GetTotalStakeReply struct {
	// TODO: deprecate one of these fields
	Stake  json.Uint64 `json:"stake"`
	Weight json.Uint64 `json:"weight"`
}

// GetTotalStake returns the total amount staked on the Primary Network
func (s *Service) GetTotalStake(_ *http.Request, args *GetTotalStakeArgs, reply *GetTotalStakeReply) error {
	vdrs, ok := s.vm.Validators.Get(args.SupernetID)
	if !ok {
		return errMissingValidatorSet
	}
	weight := json.Uint64(vdrs.Weight())
	reply.Weight = weight
	reply.Stake = weight
	return nil
}

// GetMaxStakeAmountArgs is the request for calling GetMaxStakeAmount.
type GetMaxStakeAmountArgs struct {
	SupernetID ids.ID      `json:"supernetID"`
	NodeID     ids.NodeID  `json:"nodeID"`
	StartTime  json.Uint64 `json:"startTime"`
	EndTime    json.Uint64 `json:"endTime"`
}

// GetMaxStakeAmountReply is the response from calling GetMaxStakeAmount.
type GetMaxStakeAmountReply struct {
	Amount json.Uint64 `json:"amount"`
}

// GetMaxStakeAmount returns the maximum amount of nJune staking to the named
// node during the time period.
func (s *Service) GetMaxStakeAmount(_ *http.Request, args *GetMaxStakeAmountArgs, reply *GetMaxStakeAmountReply) error {
	startTime := time.Unix(int64(args.StartTime), 0)
	endTime := time.Unix(int64(args.EndTime), 0)

	if startTime.After(endTime) {
		return errStartAfterEndTime
	}
	now := s.vm.state.GetTimestamp()
	if startTime.Before(now) {
		return errStartTimeInThePast
	}

	staker, err := executor.GetValidator(s.vm.state, args.SupernetID, args.NodeID)
	if err == database.ErrNotFound {
		return nil
	}
	if err != nil {
		return err
	}

	if startTime.After(staker.EndTime) {
		return nil
	}
	if endTime.Before(staker.StartTime) {
		return nil
	}

	maxStakeAmount, err := executor.GetMaxWeight(s.vm.state, staker, startTime, endTime)
	reply.Amount = json.Uint64(maxStakeAmount)
	return err
}

// GetRewardUTXOsReply defines the GetRewardUTXOs replies returned from the API
type GetRewardUTXOsReply struct {
	// Number of UTXOs returned
	NumFetched json.Uint64 `json:"numFetched"`
	// The UTXOs
	UTXOs []string `json:"utxos"`
	// Encoding specifies the encoding format the UTXOs are returned in
	Encoding formatting.Encoding `json:"encoding"`
}

// GetRewardUTXOs returns the UTXOs that were rewarded after the provided
// transaction's staking period ended.
func (s *Service) GetRewardUTXOs(_ *http.Request, args *api.GetTxArgs, reply *GetRewardUTXOsReply) error {
	s.vm.ctx.Log.Debug("Platform: GetRewardUTXOs called")

	utxos, err := s.vm.state.GetRewardUTXOs(args.TxID)
	if err != nil {
		return fmt.Errorf("couldn't get reward UTXOs: %w", err)
	}

	reply.NumFetched = json.Uint64(len(utxos))
	reply.UTXOs = make([]string, len(utxos))
	for i, utxo := range utxos {
		utxoBytes, err := txs.GenesisCodec.Marshal(txs.Version, utxo)
		if err != nil {
			return fmt.Errorf("failed to encode UTXO to bytes: %w", err)
		}

		utxoStr, err := formatting.Encode(args.Encoding, utxoBytes)
		if err != nil {
			return fmt.Errorf("couldn't encode utxo as a string: %w", err)
		}
		reply.UTXOs[i] = utxoStr
	}
	reply.Encoding = args.Encoding
	return nil
}

// GetTimestampReply is the response from GetTimestamp
type GetTimestampReply struct {
	// Current timestamp
	Timestamp time.Time `json:"timestamp"`
}

// GetTimestamp returns the current timestamp on chain.
func (s *Service) GetTimestamp(_ *http.Request, _ *struct{}, reply *GetTimestampReply) error {
	s.vm.ctx.Log.Debug("Platform: GetTimestamp called")

	reply.Timestamp = s.vm.state.GetTimestamp()
	return nil
}

// GetValidatorsAtArgs is the response from GetValidatorsAt
type GetValidatorsAtArgs struct {
	Height     json.Uint64 `json:"height"`
	SupernetID ids.ID      `json:"supernetID"`
}

// GetValidatorsAtReply is the response from GetValidatorsAt
type GetValidatorsAtReply struct {
	// TODO should we change this to map[ids.NodeID]*validators.Validator?
	// We'd have to add a MarshalJSON method to validators.Validator.
	Validators map[ids.NodeID]uint64 `json:"validators"`
}

// GetValidatorsAt returns the weights of the validator set of a provided supernet
// at the specified height.
func (s *Service) GetValidatorsAt(r *http.Request, args *GetValidatorsAtArgs, reply *GetValidatorsAtReply) error {
	height := uint64(args.Height)
	s.vm.ctx.Log.Debug("Platform: GetValidatorsAt called",
		zap.Uint64("height", height),
		zap.Stringer("supernetID", args.SupernetID),
	)

	ctx := r.Context()
	var err error
	vdrs, err := s.vm.GetValidatorSet(ctx, height, args.SupernetID)
	if err != nil {
		return fmt.Errorf("failed to get validator set: %w", err)
	}
	reply.Validators = make(map[ids.NodeID]uint64, len(vdrs))
	for _, vdr := range vdrs {
		reply.Validators[vdr.NodeID] = vdr.Weight
	}
	return nil
}

func (s *Service) GetBlock(_ *http.Request, args *api.GetBlockArgs, response *api.GetBlockResponse) error {
	s.vm.ctx.Log.Debug("Platform: GetBlock called",
		zap.Stringer("blkID", args.BlockID),
		zap.Stringer("encoding", args.Encoding),
	)

	block, err := s.vm.manager.GetStatelessBlock(args.BlockID)
	if err != nil {
		return fmt.Errorf("couldn't get block with id %s: %w", args.BlockID, err)
	}
	response.Encoding = args.Encoding

	if args.Encoding == formatting.JSON {
		block.InitCtx(s.vm.ctx)
		response.Block = block
		return nil
	}

	response.Block, err = formatting.Encode(args.Encoding, block.Bytes())
	if err != nil {
		return fmt.Errorf("couldn't encode block %s as string: %w", args.BlockID, err)
	}

	return nil
}

func (s *Service) getAPIUptime(staker *state.Staker) (*json.Float32, error) {
	// Only report uptimes that we have been actively tracking.
	if constants.PrimaryNetworkID != staker.SupernetID && !s.vm.WhitelistedSupernets.Contains(staker.SupernetID) {
		return nil, nil
	}

	rawUptime, err := s.vm.uptimeManager.CalculateUptimePercentFrom(staker.NodeID, staker.SupernetID, staker.StartTime)
	if err != nil {
		return nil, err
	}
	uptime := json.Float32(rawUptime)
	return &uptime, nil
}

func (s *Service) getAPIOwner(owner *secp256k1fx.OutputOwners) (*relayapi.Owner, error) {
	apiOwner := &relayapi.Owner{
		Locktime:  json.Uint64(owner.Locktime),
		Threshold: json.Uint32(owner.Threshold),
	}
	for _, addr := range owner.Addrs {
		addrStr, err := s.addrManager.FormatLocalAddress(addr)
		if err != nil {
			return nil, err
		}
		apiOwner.Addresses = append(apiOwner.Addresses, addrStr)
	}
	return apiOwner, nil
}

// Takes in a staker and a set of addresses
// Returns:
// 1) The total amount staked by addresses in [addrs]
// 2) The staked outputs
func getStakeHelper(tx *txs.Tx, addrs set.Set[ids.ShortID], totalAmountStaked map[ids.ID]uint64) []june.TransferableOutput {
	staker, ok := tx.Unsigned.(txs.PermissionlessStaker)
	if !ok {
		return nil
	}

	stake := staker.Stake()
	stakedOuts := make([]june.TransferableOutput, 0, len(stake))
	// Go through all of the staked outputs
	for _, output := range stake {
		out := output.Out
		if lockedOut, ok := out.(*stakeable.LockOut); ok {
			// This output can only be used for staking until [stakeOnlyUntil]
			out = lockedOut.TransferableOut
		}
		secpOut, ok := out.(*secp256k1fx.TransferOutput)
		if !ok {
			continue
		}

		// Check whether this output is owned by one of the given addresses
		contains := false
		for _, addr := range secpOut.Addrs {
			if addrs.Contains(addr) {
				contains = true
				break
			}
		}
		if !contains {
			// This output isn't owned by one of the given addresses. Ignore.
			continue
		}

		assetID := output.AssetID()
		newAmount, err := math.Add64(totalAmountStaked[assetID], secpOut.Amt)
		if err != nil {
			newAmount = stdmath.MaxUint64
		}
		totalAmountStaked[assetID] = newAmount

		stakedOuts = append(
			stakedOuts,
			*output,
		)
	}
	return stakedOuts
}
