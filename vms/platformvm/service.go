// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/Juneo-io/juneogo/api"
	"github.com/Juneo-io/juneogo/cache"
	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow/validators"
	"github.com/Juneo-io/juneogo/utils"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/crypto/bls"
	"github.com/Juneo-io/juneogo/utils/crypto/secp256k1"
	"github.com/Juneo-io/juneogo/utils/formatting"
	"github.com/Juneo-io/juneogo/utils/logging"
	"github.com/Juneo-io/juneogo/utils/set"
	"github.com/Juneo-io/juneogo/vms/components/avax"
	"github.com/Juneo-io/juneogo/vms/components/keystore"
	"github.com/Juneo-io/juneogo/vms/platformvm/fx"
	"github.com/Juneo-io/juneogo/vms/platformvm/reward"
	"github.com/Juneo-io/juneogo/vms/platformvm/signer"
	"github.com/Juneo-io/juneogo/vms/platformvm/stakeable"
	"github.com/Juneo-io/juneogo/vms/platformvm/state"
	"github.com/Juneo-io/juneogo/vms/platformvm/status"
	"github.com/Juneo-io/juneogo/vms/platformvm/txs"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"

	avajson "github.com/Juneo-io/juneogo/utils/json"
	safemath "github.com/Juneo-io/juneogo/utils/math"
	platformapi "github.com/Juneo-io/juneogo/vms/platformvm/api"
)

const (
	// Max number of addresses that can be passed in as argument to GetUTXOs
	maxGetUTXOsAddrs = 1024

	// Max number of addresses that can be passed in as argument to GetStake
	maxGetStakeAddrs = 256

	// Max number of items allowed in a page
	maxPageSize = 1024

	// Note: Staker attributes cache should be large enough so that no evictions
	// happen when the API loops through all stakers.
	stakerAttributesCacheSize = 100_000
)

var (
	errMissingDecisionBlock       = errors.New("should have a decision block within the past two blocks")
	errPrimaryNetworkIsNotASupernet = errors.New("the primary network isn't a supernet")
	errNoAddresses                = errors.New("no addresses provided")
	errMissingBlockchainID        = errors.New("argument 'blockchainID' not given")
)

// Service defines the API calls that can be made to the platform chain
type Service struct {
	vm                    *VM
	addrManager           avax.AddressManager
	stakerAttributesCache *cache.LRU[ids.ID, *stakerAttributes]
}

// All attributes are optional and may not be filled for each stakerTx.
type stakerAttributes struct {
	shares                 uint32
	rewardsOwner           fx.Owner
	validationRewardsOwner fx.Owner
	delegationRewardsOwner fx.Owner
	proofOfPossession      *signer.ProofOfPossession
}

// GetHeight returns the height of the last accepted block
func (s *Service) GetHeight(r *http.Request, _ *struct{}, response *api.GetHeightResponse) error {
	s.vm.ctx.Log.Debug("API called",
		zap.String("service", "platform"),
		zap.String("method", "getHeight"),
	)

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	ctx := r.Context()
	height, err := s.vm.GetCurrentHeight(ctx)
	response.Height = avajson.Uint64(height)
	return err
}

// ExportKeyArgs are arguments for ExportKey
type ExportKeyArgs struct {
	api.UserPass
	Address string `json:"address"`
}

// ExportKeyReply is the response for ExportKey
type ExportKeyReply struct {
	// The decrypted PrivateKey for the Address provided in the arguments
	PrivateKey *secp256k1.PrivateKey `json:"privateKey"`
}

// ExportKey returns a private key from the provided user
func (s *Service) ExportKey(_ *http.Request, args *ExportKeyArgs, reply *ExportKeyReply) error {
	s.vm.ctx.Log.Warn("deprecated API called",
		zap.String("service", "platform"),
		zap.String("method", "exportKey"),
		logging.UserString("username", args.Username),
	)

	address, err := avax.ParseServiceAddress(s.addrManager, args.Address)
	if err != nil {
		return fmt.Errorf("couldn't parse %s to address: %w", args.Address, err)
	}

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

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

type GetBalanceRequest struct {
	Addresses []string `json:"addresses"`
}

// Note: We explicitly duplicate AVAX out of the maps to ensure backwards
// compatibility.
type GetBalanceResponse struct {
	// Balance, in nAVAX, of the address
	Balance             avajson.Uint64            `json:"balance"`
	Unlocked            avajson.Uint64            `json:"unlocked"`
	LockedStakeable     avajson.Uint64            `json:"lockedStakeable"`
	LockedNotStakeable  avajson.Uint64            `json:"lockedNotStakeable"`
	Balances            map[ids.ID]avajson.Uint64 `json:"balances"`
	Unlockeds           map[ids.ID]avajson.Uint64 `json:"unlockeds"`
	LockedStakeables    map[ids.ID]avajson.Uint64 `json:"lockedStakeables"`
	LockedNotStakeables map[ids.ID]avajson.Uint64 `json:"lockedNotStakeables"`
	UTXOIDs             []*avax.UTXOID            `json:"utxoIDs"`
}

// GetBalance gets the balance of an address
func (s *Service) GetBalance(_ *http.Request, args *GetBalanceRequest, response *GetBalanceResponse) error {
	s.vm.ctx.Log.Debug("deprecated API called",
		zap.String("service", "platform"),
		zap.String("method", "getBalance"),
		logging.UserStrings("addresses", args.Addresses),
	)

	addrs, err := avax.ParseServiceAddresses(s.addrManager, args.Addresses)
	if err != nil {
		return err
	}

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	utxos, err := avax.GetAllUTXOs(s.vm.state, addrs)
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
				newBalance, err := safemath.Add64(unlockeds[assetID], out.Amount())
				if err != nil {
					unlockeds[assetID] = math.MaxUint64
				} else {
					unlockeds[assetID] = newBalance
				}
			} else {
				newBalance, err := safemath.Add64(lockedNotStakeables[assetID], out.Amount())
				if err != nil {
					lockedNotStakeables[assetID] = math.MaxUint64
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
				newBalance, err := safemath.Add64(lockedNotStakeables[assetID], out.Amount())
				if err != nil {
					lockedNotStakeables[assetID] = math.MaxUint64
				} else {
					lockedNotStakeables[assetID] = newBalance
				}
			case out.Locktime <= currentTime:
				newBalance, err := safemath.Add64(unlockeds[assetID], out.Amount())
				if err != nil {
					unlockeds[assetID] = math.MaxUint64
				} else {
					unlockeds[assetID] = newBalance
				}
			default:
				newBalance, err := safemath.Add64(lockedStakeables[assetID], out.Amount())
				if err != nil {
					lockedStakeables[assetID] = math.MaxUint64
				} else {
					lockedStakeables[assetID] = newBalance
				}
			}
		default:
			continue utxoFor
		}

		response.UTXOIDs = append(response.UTXOIDs, &utxo.UTXOID)
	}

	balances := maps.Clone(lockedStakeables)
	for assetID, amount := range lockedNotStakeables {
		newBalance, err := safemath.Add64(balances[assetID], amount)
		if err != nil {
			balances[assetID] = math.MaxUint64
		} else {
			balances[assetID] = newBalance
		}
	}
	for assetID, amount := range unlockeds {
		newBalance, err := safemath.Add64(balances[assetID], amount)
		if err != nil {
			balances[assetID] = math.MaxUint64
		} else {
			balances[assetID] = newBalance
		}
	}

	response.Balances = newJSONBalanceMap(balances)
	response.Unlockeds = newJSONBalanceMap(unlockeds)
	response.LockedStakeables = newJSONBalanceMap(lockedStakeables)
	response.LockedNotStakeables = newJSONBalanceMap(lockedNotStakeables)
	response.Balance = response.Balances[s.vm.ctx.JUNEAssetID]
	response.Unlocked = response.Unlockeds[s.vm.ctx.JUNEAssetID]
	response.LockedStakeable = response.LockedStakeables[s.vm.ctx.JUNEAssetID]
	response.LockedNotStakeable = response.LockedNotStakeables[s.vm.ctx.JUNEAssetID]
	return nil
}

func newJSONBalanceMap(balanceMap map[ids.ID]uint64) map[ids.ID]avajson.Uint64 {
	jsonBalanceMap := make(map[ids.ID]avajson.Uint64, len(balanceMap))
	for assetID, amount := range balanceMap {
		jsonBalanceMap[assetID] = avajson.Uint64(amount)
	}
	return jsonBalanceMap
}

// ListAddresses returns the addresses controlled by [args.Username]
func (s *Service) ListAddresses(_ *http.Request, args *api.UserPass, response *api.JSONAddresses) error {
	s.vm.ctx.Log.Warn("deprecated API called",
		zap.String("service", "platform"),
		zap.String("method", "listAddresses"),
		logging.UserString("username", args.Username),
	)

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

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
	s.vm.ctx.Log.Debug("API called",
		zap.String("service", "platform"),
		zap.String("method", "getUTXOs"),
	)

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

	addrSet, err := avax.ParseServiceAddresses(s.addrManager, args.Addresses)
	if err != nil {
		return err
	}

	startAddr := ids.ShortEmpty
	startUTXO := ids.Empty
	if args.StartIndex.Address != "" || args.StartIndex.UTXO != "" {
		startAddr, err = avax.ParseServiceAddress(s.addrManager, args.StartIndex.Address)
		if err != nil {
			return fmt.Errorf("couldn't parse start index address %q: %w", args.StartIndex.Address, err)
		}
		startUTXO, err = ids.FromString(args.StartIndex.UTXO)
		if err != nil {
			return fmt.Errorf("couldn't parse start index utxo: %w", err)
		}
	}

	var (
		utxos     []*avax.UTXO
		endAddr   ids.ShortID
		endUTXOID ids.ID
	)
	limit := int(args.Limit)
	if limit <= 0 || maxPageSize < limit {
		limit = maxPageSize
	}

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	if sourceChain == s.vm.ctx.ChainID {
		utxos, endAddr, endUTXOID, err = avax.GetPaginatedUTXOs(
			s.vm.state,
			addrSet,
			startAddr,
			startUTXO,
			limit,
		)
	} else {
		utxos, endAddr, endUTXOID, err = avax.GetAtomicUTXOs(
			s.vm.ctx.SharedMemory,
			txs.Codec,
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
		bytes, err := txs.Codec.Marshal(txs.CodecVersion, utxo)
		if err != nil {
			return fmt.Errorf("couldn't serialize UTXO %q: %w", utxo.InputID(), err)
		}
		response.UTXOs[i], err = formatting.Encode(args.Encoding, bytes)
		if err != nil {
			return fmt.Errorf("couldn't encode UTXO %s as %s: %w", utxo.InputID(), args.Encoding, err)
		}
	}

	endAddress, err := s.addrManager.FormatLocalAddress(endAddr)
	if err != nil {
		return fmt.Errorf("problem formatting address: %w", err)
	}

	response.EndIndex.Address = endAddress
	response.EndIndex.UTXO = endUTXOID.String()
	response.NumFetched = avajson.Uint64(len(utxos))
	response.Encoding = args.Encoding
	return nil
}

// GetSupernetArgs are the arguments to GetSupernet
type GetSupernetArgs struct {
	// ID of the supernet to retrieve information about
	SupernetID ids.ID `json:"supernetID"`
}

// GetSupernetResponse is the response from calling GetSupernet
type GetSupernetResponse struct {
	// whether it is permissioned or not
	IsPermissioned bool `json:"isPermissioned"`
	// supernet auth information for a permissioned supernet
	ControlKeys []string       `json:"controlKeys"`
	Threshold   avajson.Uint32 `json:"threshold"`
	Locktime    avajson.Uint64 `json:"locktime"`
	// supernet transformation tx ID for a permissionless supernet
	SupernetTransformationTxID ids.ID `json:"supernetTransformationTxID"`
}

func (s *Service) GetSupernet(_ *http.Request, args *GetSupernetArgs, response *GetSupernetResponse) error {
	s.vm.ctx.Log.Debug("API called",
		zap.String("service", "platform"),
		zap.String("method", "getSupernet"),
		zap.Stringer("supernetID", args.SupernetID),
	)

	if args.SupernetID == constants.PrimaryNetworkID {
		return errPrimaryNetworkIsNotASupernet
	}

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	supernetOwner, err := s.vm.state.GetSupernetOwner(args.SupernetID)
	if err != nil {
		return err
	}
	owner, ok := supernetOwner.(*secp256k1fx.OutputOwners)
	if !ok {
		return fmt.Errorf("expected *secp256k1fx.OutputOwners but got %T", supernetOwner)
	}
	controlAddrs := make([]string, len(owner.Addrs))
	for i, controlKeyID := range owner.Addrs {
		addr, err := s.addrManager.FormatLocalAddress(controlKeyID)
		if err != nil {
			return fmt.Errorf("problem formatting address: %w", err)
		}
		controlAddrs[i] = addr
	}

	response.ControlKeys = controlAddrs
	response.Threshold = avajson.Uint32(owner.Threshold)
	response.Locktime = avajson.Uint64(owner.Locktime)

	switch supernetTransformationTx, err := s.vm.state.GetSupernetTransformation(args.SupernetID); err {
	case nil:
		response.IsPermissioned = false
		response.SupernetTransformationTxID = supernetTransformationTx.ID()
	case database.ErrNotFound:
		response.IsPermissioned = true
		response.SupernetTransformationTxID = ids.Empty
	default:
		return err
	}

	return nil
}

// APISupernet is a representation of a supernet used in API calls
type APISupernet struct {
	// ID of the supernet
	ID ids.ID `json:"id"`

	// Each element of [ControlKeys] the address of a public key.
	// A transaction to add a validator to this supernet requires
	// signatures from [Threshold] of these keys to be valid.
	ControlKeys []string       `json:"controlKeys"`
	Threshold   avajson.Uint32 `json:"threshold"`
}

// GetSupernetsArgs are the arguments to GetSupernets
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
	s.vm.ctx.Log.Debug("deprecated API called",
		zap.String("service", "platform"),
		zap.String("method", "getSupernets"),
	)

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

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
					Threshold:   avajson.Uint32(0),
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
				Threshold:   avajson.Uint32(owner.Threshold),
			}
		}
		// Include primary network
		response.Supernets[len(supernets)] = APISupernet{
			ID:          constants.PrimaryNetworkID,
			ControlKeys: []string{},
			Threshold:   avajson.Uint32(0),
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
					Threshold:   avajson.Uint32(0),
				},
			)
			continue
		}

		if _, err := s.vm.state.GetSupernetTransformation(supernetID); err == nil {
			response.Supernets = append(response.Supernets, APISupernet{
				ID:          supernetID,
				ControlKeys: []string{},
				Threshold:   avajson.Uint32(0),
			})
			continue
		}

		supernetOwner, err := s.vm.state.GetSupernetOwner(supernetID)
		if err == database.ErrNotFound {
			continue
		}
		if err != nil {
			return err
		}

		owner, ok := supernetOwner.(*secp256k1fx.OutputOwners)
		if !ok {
			return fmt.Errorf("expected *secp256k1fx.OutputOwners but got %T", supernetOwner)
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
			Threshold:   avajson.Uint32(owner.Threshold),
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
	s.vm.ctx.Log.Debug("API called",
		zap.String("service", "platform"),
		zap.String("method", "getStakingAssetID"),
	)

	if args.SupernetID == constants.PrimaryNetworkID {
		response.AssetID = s.vm.ctx.JUNEAssetID
		return nil
	}

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

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

func (s *Service) loadStakerTxAttributes(txID ids.ID) (*stakerAttributes, error) {
	// Lookup tx from the cache first.
	attr, found := s.stakerAttributesCache.Get(txID)
	if found {
		return attr, nil
	}

	// Tx not available in cache; pull it from disk and populate the cache.
	tx, _, err := s.vm.state.GetTx(txID)
	if err != nil {
		return nil, err
	}

	switch stakerTx := tx.Unsigned.(type) {
	case txs.ValidatorTx:
		var pop *signer.ProofOfPossession
		if staker, ok := stakerTx.(*txs.AddPermissionlessValidatorTx); ok {
			if s, ok := staker.Signer.(*signer.ProofOfPossession); ok {
				pop = s
			}
		}

		attr = &stakerAttributes{
			shares:                 stakerTx.Shares(),
			validationRewardsOwner: stakerTx.ValidationRewardsOwner(),
			delegationRewardsOwner: stakerTx.DelegationRewardsOwner(),
			proofOfPossession:      pop,
		}

	case txs.DelegatorTx:
		attr = &stakerAttributes{
			rewardsOwner: stakerTx.RewardsOwner(),
		}

	default:
		return nil, fmt.Errorf("unexpected staker tx type %T", tx.Unsigned)
	}

	s.stakerAttributesCache.Put(txID, attr)
	return attr, nil
}

// GetCurrentValidators returns the current validators. If a single nodeID
// is provided, full delegators information is also returned. Otherwise only
// delegators' number and total weight is returned.
func (s *Service) GetCurrentValidators(_ *http.Request, args *GetCurrentValidatorsArgs, reply *GetCurrentValidatorsReply) error {
	s.vm.ctx.Log.Debug("API called",
		zap.String("service", "platform"),
		zap.String("method", "getCurrentValidators"),
	)

	reply.Validators = []interface{}{}

	// Validator's node ID as string --> Delegators to them
	vdrToDelegators := map[ids.NodeID][]platformapi.PrimaryDelegator{}

	// Create set of nodeIDs
	nodeIDs := set.Of(args.NodeIDs...)

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	numNodeIDs := nodeIDs.Len()
	targetStakers := make([]*state.Staker, 0, numNodeIDs)
	if numNodeIDs == 0 { // Include all nodes
		currentStakerIterator, err := s.vm.state.GetCurrentStakerIterator()
		if err != nil {
			return err
		}
		// TODO: avoid iterating over delegators here.
		for currentStakerIterator.Next() {
			staker := currentStakerIterator.Value()
			if args.SupernetID != staker.SupernetID {
				continue
			}
			targetStakers = append(targetStakers, staker)
		}
		currentStakerIterator.Release()
	} else {
		for nodeID := range nodeIDs {
			staker, err := s.vm.state.GetCurrentValidator(args.SupernetID, nodeID)
			switch err {
			case nil:
			case database.ErrNotFound:
				// nothing to do, continue
				continue
			default:
				return err
			}
			targetStakers = append(targetStakers, staker)

			// TODO: avoid iterating over delegators when numNodeIDs > 1.
			delegatorsIt, err := s.vm.state.GetCurrentDelegatorIterator(args.SupernetID, nodeID)
			if err != nil {
				return err
			}
			for delegatorsIt.Next() {
				staker := delegatorsIt.Value()
				targetStakers = append(targetStakers, staker)
			}
			delegatorsIt.Release()
		}
	}

	for _, currentStaker := range targetStakers {
		nodeID := currentStaker.NodeID
		weight := avajson.Uint64(currentStaker.Weight)
		apiStaker := platformapi.Staker{
			TxID:        currentStaker.TxID,
			StartTime:   avajson.Uint64(currentStaker.StartTime.Unix()),
			EndTime:     avajson.Uint64(currentStaker.EndTime.Unix()),
			Weight:      weight,
			StakeAmount: &weight,
			NodeID:      nodeID,
		}
		potentialReward := avajson.Uint64(currentStaker.PotentialReward)

		delegateeReward, err := s.vm.state.GetDelegateeReward(currentStaker.SupernetID, currentStaker.NodeID)
		if err != nil {
			return err
		}
		jsonDelegateeReward := avajson.Uint64(delegateeReward)

		switch currentStaker.Priority {
		case txs.PrimaryNetworkValidatorCurrentPriority, txs.SupernetPermissionlessValidatorCurrentPriority:
			attr, err := s.loadStakerTxAttributes(currentStaker.TxID)
			if err != nil {
				return err
			}

			shares := attr.shares
			delegationFee := avajson.Float32(100 * float32(shares) / float32(reward.PercentDenominator))

			uptime, err := s.getAPIUptime(currentStaker)
			if err != nil {
				return err
			}

			connected := s.vm.uptimeManager.IsConnected(nodeID, args.SupernetID)
			var (
				validationRewardOwner *platformapi.Owner
				delegationRewardOwner *platformapi.Owner
			)
			validationOwner, ok := attr.validationRewardsOwner.(*secp256k1fx.OutputOwners)
			if ok {
				validationRewardOwner, err = s.getAPIOwner(validationOwner)
				if err != nil {
					return err
				}
			}
			delegationOwner, ok := attr.delegationRewardsOwner.(*secp256k1fx.OutputOwners)
			if ok {
				delegationRewardOwner, err = s.getAPIOwner(delegationOwner)
				if err != nil {
					return err
				}
			}

			vdr := platformapi.PermissionlessValidator{
				Staker:                 apiStaker,
				Uptime:                 uptime,
				Connected:              connected,
				PotentialReward:        &potentialReward,
				AccruedDelegateeReward: &jsonDelegateeReward,
				RewardOwner:            validationRewardOwner,
				ValidationRewardOwner:  validationRewardOwner,
				DelegationRewardOwner:  delegationRewardOwner,
				DelegationFee:          delegationFee,
				Signer:                 attr.proofOfPossession,
			}
			reply.Validators = append(reply.Validators, vdr)

		case txs.PrimaryNetworkDelegatorCurrentPriority, txs.SupernetPermissionlessDelegatorCurrentPriority:
			var rewardOwner *platformapi.Owner
			// If we are handling multiple nodeIDs, we don't return the
			// delegator information.
			if numNodeIDs == 1 {
				attr, err := s.loadStakerTxAttributes(currentStaker.TxID)
				if err != nil {
					return err
				}
				owner, ok := attr.rewardsOwner.(*secp256k1fx.OutputOwners)
				if ok {
					rewardOwner, err = s.getAPIOwner(owner)
					if err != nil {
						return err
					}
				}
			}

			delegator := platformapi.PrimaryDelegator{
				Staker:          apiStaker,
				RewardOwner:     rewardOwner,
				PotentialReward: &potentialReward,
			}
			vdrToDelegators[delegator.NodeID] = append(vdrToDelegators[delegator.NodeID], delegator)

		case txs.SupernetPermissionedValidatorCurrentPriority:
			uptime, err := s.getAPIUptime(currentStaker)
			if err != nil {
				return err
			}
			connected := s.vm.uptimeManager.IsConnected(nodeID, args.SupernetID)
			reply.Validators = append(reply.Validators, platformapi.PermissionedValidator{
				Staker:    apiStaker,
				Connected: connected,
				Uptime:    uptime,
			})

		default:
			return fmt.Errorf("unexpected staker priority %d", currentStaker.Priority)
		}
	}

	// handle delegators' information
	for i, vdrIntf := range reply.Validators {
		vdr, ok := vdrIntf.(platformapi.PermissionlessValidator)
		if !ok {
			continue
		}
		delegators, ok := vdrToDelegators[vdr.NodeID]
		if !ok {
			// If we are expected to populate the delegators field, we should
			// always return a non-nil value.
			delegators = []platformapi.PrimaryDelegator{}
		}
		delegatorCount := avajson.Uint64(len(delegators))
		delegatorWeight := avajson.Uint64(0)
		for _, d := range delegators {
			delegatorWeight += d.Weight
		}

		vdr.DelegatorCount = &delegatorCount
		vdr.DelegatorWeight = &delegatorWeight

		if numNodeIDs == 1 {
			// queried a specific validator, load all of its delegators
			vdr.Delegators = &delegators
		}
		reply.Validators[i] = vdr
	}

	return nil
}

// GetCurrentSupplyArgs are the arguments for calling GetCurrentSupply
type GetCurrentSupplyArgs struct {
	SupernetID ids.ID `json:"supernetID"`
}

// GetCurrentSupplyReply are the results from calling GetCurrentSupply
type GetCurrentSupplyReply struct {
	Supply avajson.Uint64 `json:"supply"`
	Height avajson.Uint64 `json:"height"`
}

// GetCurrentSupply returns an upper bound on the supply of AVAX in the system
func (s *Service) GetCurrentSupply(r *http.Request, args *GetCurrentSupplyArgs, reply *GetCurrentSupplyReply) error {
	s.vm.ctx.Log.Debug("API called",
		zap.String("service", "platform"),
		zap.String("method", "getCurrentSupply"),
	)

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	supply, err := s.vm.state.GetCurrentSupply(args.SupernetID)
	if err != nil {
		return fmt.Errorf("fetching current supply failed: %w", err)
	}
	reply.Supply = avajson.Uint64(supply)

	ctx := r.Context()
	height, err := s.vm.GetCurrentHeight(ctx)
	if err != nil {
		return fmt.Errorf("fetching current height failed: %w", err)
	}
	reply.Height = avajson.Uint64(height)

	return nil
}

// GetRewardPoolSupplyArgs are the arguments for calling GetRewardPoolSupply
type GetRewardPoolSupplyArgs struct {
	SupernetID ids.ID `json:"supernetID"`
}

// GetRewardPoolSupplyReply are the results from calling GetRewardPoolSupply
type GetRewardPoolSupplyReply struct {
	RewardPoolSupply avajson.Uint64 `json:"rewardPoolSupply"`
}

// GetRewardPoolSupply returns an upper bound on the supply of AVAX in the system
func (s *Service) GetRewardPoolSupply(_ *http.Request, args *GetRewardPoolSupplyArgs, reply *GetRewardPoolSupplyReply) error {
	s.vm.ctx.Log.Debug("API called",
		zap.String("service", "platform"),
		zap.String("method", "getRewardPoolSupply"),
	)

	rewardPoolSupply, err := s.vm.state.GetRewardPoolSupply(args.SupernetID)
	reply.RewardPoolSupply = avajson.Uint64(rewardPoolSupply)
	return err
}

// GetFeePoolValueReply are the results from calling GetFeePoolValue
type GetFeePoolValueReply struct {
	FeePoolValue avajson.Uint64 `json:"feePoolValue"`
}

// GetFeePoolValue returns the current value in the fee pool
func (s *Service) GetFeePoolValue(_ *http.Request, _ *struct{}, reply *GetFeePoolValueReply) error {
	s.vm.ctx.Log.Debug("Platform: GetFeePoolValue called")

	reply.FeePoolValue = avajson.Uint64(s.vm.state.GetFeePoolValue())
	return nil
}

// SampleValidatorsArgs are the arguments for calling SampleValidators
type SampleValidatorsArgs struct {
	// Number of validators in the sample
	Size avajson.Uint16 `json:"size"`

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
	s.vm.ctx.Log.Debug("API called",
		zap.String("service", "platform"),
		zap.String("method", "sampleValidators"),
		zap.Uint16("size", uint16(args.Size)),
	)

	sample, err := s.vm.Validators.Sample(args.SupernetID, int(args.Size))
	if err != nil {
		return fmt.Errorf("sampling %s errored with %w", args.SupernetID, err)
	}

	if sample == nil {
		reply.Validators = []ids.NodeID{}
	} else {
		utils.Sort(sample)
		reply.Validators = sample
	}
	return nil
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
	s.vm.ctx.Log.Debug("API called",
		zap.String("service", "platform"),
		zap.String("method", "getBlockchainStatus"),
	)

	if args.BlockchainID == "" {
		return errMissingBlockchainID
	}

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

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

	preferredBlkID := s.vm.manager.Preferred()
	preferred, err := s.chainExists(ctx, preferredBlkID, blockchainID)
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

	_, isValidator := s.vm.Validators.GetValidator(chain.SupernetID, s.vm.ctx.NodeID)
	return isValidator
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
	s.vm.ctx.Log.Debug("API called",
		zap.String("service", "platform"),
		zap.String("method", "validatedBy"),
	)

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

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
	s.vm.ctx.Log.Debug("API called",
		zap.String("service", "platform"),
		zap.String("method", "validates"),
	)

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

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

	// The main asset used by this chain to pay the fees
	ChainAssetID ids.ID `json:"chainAssetID"`
}

// GetBlockchainsResponse is the response from a call to GetBlockchains
type GetBlockchainsResponse struct {
	// blockchains that exist
	Blockchains []APIBlockchain `json:"blockchains"`
}

// GetBlockchains returns all of the blockchains that exist
func (s *Service) GetBlockchains(_ *http.Request, _ *struct{}, response *GetBlockchainsResponse) error {
	s.vm.ctx.Log.Debug("deprecated API called",
		zap.String("service", "platform"),
		zap.String("method", "getBlockchains"),
	)

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

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
				SupernetID:     supernetID,
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
			SupernetID:     constants.PrimaryNetworkID,
			VMID:         chain.VMID,
			ChainAssetID: chain.ChainAssetID,
		})
	}

	return nil
}

func (s *Service) IssueTx(_ *http.Request, args *api.FormattedTx, response *api.JSONTxID) error {
	s.vm.ctx.Log.Debug("API called",
		zap.String("service", "platform"),
		zap.String("method", "issueTx"),
	)

	txBytes, err := formatting.Decode(args.Encoding, args.Tx)
	if err != nil {
		return fmt.Errorf("problem decoding transaction: %w", err)
	}
	tx, err := txs.Parse(txs.Codec, txBytes)
	if err != nil {
		return fmt.Errorf("couldn't parse tx: %w", err)
	}

	if err := s.vm.issueTxFromRPC(tx); err != nil {
		return fmt.Errorf("couldn't issue tx: %w", err)
	}

	response.TxID = tx.ID()
	return nil
}

func (s *Service) GetTx(_ *http.Request, args *api.GetTxArgs, response *api.GetTxReply) error {
	s.vm.ctx.Log.Debug("API called",
		zap.String("service", "platform"),
		zap.String("method", "getTx"),
	)

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	tx, _, err := s.vm.state.GetTx(args.TxID)
	if err != nil {
		return fmt.Errorf("couldn't get tx: %w", err)
	}
	response.Encoding = args.Encoding

	var result any
	if args.Encoding == formatting.JSON {
		tx.Unsigned.InitCtx(s.vm.ctx)
		result = tx
	} else {
		result, err = formatting.Encode(args.Encoding, tx.Bytes())
		if err != nil {
			return fmt.Errorf("couldn't encode tx as %s: %w", args.Encoding, err)
		}
	}

	response.Tx, err = json.Marshal(result)
	return err
}

type GetTxStatusArgs struct {
	TxID ids.ID `json:"txID"`
}

type GetTxStatusResponse struct {
	Status status.Status `json:"status"`
	// Reason this tx was dropped.
	// Only non-empty if Status is dropped
	Reason string `json:"reason,omitempty"`
}

// GetTxStatus gets a tx's status
func (s *Service) GetTxStatus(_ *http.Request, args *GetTxStatusArgs, response *GetTxStatusResponse) error {
	s.vm.ctx.Log.Debug("API called",
		zap.String("service", "platform"),
		zap.String("method", "getTxStatus"),
	)

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

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
	preferredID := s.vm.manager.Preferred()
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

	if _, ok := s.vm.Builder.Get(args.TxID); ok {
		// Found the tx in the mempool. Report tx is processing.
		response.Status = status.Processing
		return nil
	}

	// Note: we check if tx is dropped only after having looked for it
	// in the database and the mempool, because dropped txs may be re-issued.
	reason := s.vm.Builder.GetDropReason(args.TxID)
	if reason == nil {
		// The tx isn't being tracked by the node.
		response.Status = status.Unknown
		return nil
	}

	// The tx was recently dropped because it was invalid.
	response.Status = status.Dropped
	response.Reason = reason.Error()
	return nil
}

type GetStakeArgs struct {
	api.JSONAddresses
	ValidatorsOnly bool                `json:"validatorsOnly"`
	Encoding       formatting.Encoding `json:"encoding"`
}

// GetStakeReply is the response from calling GetStake.
type GetStakeReply struct {
	Staked  avajson.Uint64            `json:"staked"`
	Stakeds map[ids.ID]avajson.Uint64 `json:"stakeds"`
	// String representation of staked outputs
	// Each is of type avax.TransferableOutput
	Outputs []string `json:"stakedOutputs"`
	// Encoding of [Outputs]
	Encoding formatting.Encoding `json:"encoding"`
}

// GetStake returns the amount of nAVAX that [args.Addresses] have cumulatively
// staked on the Primary Network.
//
// This method assumes that each stake output has only owner
// This method assumes only AVAX can be staked
// This method only concerns itself with the Primary Network, not supernets
// TODO: Improve the performance of this method by maintaining this data
// in a data structure rather than re-calculating it by iterating over stakers
func (s *Service) GetStake(_ *http.Request, args *GetStakeArgs, response *GetStakeReply) error {
	s.vm.ctx.Log.Debug("deprecated API called",
		zap.String("service", "platform"),
		zap.String("method", "getStake"),
	)

	if len(args.Addresses) > maxGetStakeAddrs {
		return fmt.Errorf("%d addresses provided but this method can take at most %d", len(args.Addresses), maxGetStakeAddrs)
	}

	addrs, err := avax.ParseServiceAddresses(s.addrManager, args.Addresses)
	if err != nil {
		return err
	}

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	currentStakerIterator, err := s.vm.state.GetCurrentStakerIterator()
	if err != nil {
		return err
	}
	defer currentStakerIterator.Release()

	var (
		totalAmountStaked = make(map[ids.ID]uint64)
		stakedOuts        []avax.TransferableOutput
	)
	for currentStakerIterator.Next() { // Iterates over current stakers
		staker := currentStakerIterator.Value()

		if args.ValidatorsOnly && !staker.Priority.IsValidator() {
			continue
		}

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

		if args.ValidatorsOnly && !staker.Priority.IsValidator() {
			continue
		}

		tx, _, err := s.vm.state.GetTx(staker.TxID)
		if err != nil {
			return err
		}

		stakedOuts = append(stakedOuts, getStakeHelper(tx, addrs, totalAmountStaked)...)
	}

	response.Stakeds = newJSONBalanceMap(totalAmountStaked)
	response.Staked = response.Stakeds[s.vm.ctx.JUNEAssetID]
	response.Outputs = make([]string, len(stakedOuts))
	for i, output := range stakedOuts {
		bytes, err := txs.Codec.Marshal(txs.CodecVersion, output)
		if err != nil {
			return fmt.Errorf("couldn't serialize output %s: %w", output.ID, err)
		}
		response.Outputs[i], err = formatting.Encode(args.Encoding, bytes)
		if err != nil {
			return fmt.Errorf("couldn't encode output %s as %s: %w", output.ID, args.Encoding, err)
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
	MinValidatorStake avajson.Uint64 `json:"minValidatorStake"`
	// Minimum stake, in nAVAX, that can be delegated on the primary network
	MinDelegatorStake avajson.Uint64 `json:"minDelegatorStake"`
}

// GetMinStake returns the minimum staking amount in nAVAX.
func (s *Service) GetMinStake(_ *http.Request, args *GetMinStakeArgs, reply *GetMinStakeReply) error {
	s.vm.ctx.Log.Debug("API called",
		zap.String("service", "platform"),
		zap.String("method", "getMinStake"),
	)

	if args.SupernetID == constants.PrimaryNetworkID {
		reply.MinValidatorStake = avajson.Uint64(s.vm.MinValidatorStake)
		reply.MinDelegatorStake = avajson.Uint64(s.vm.MinDelegatorStake)
		return nil
	}

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

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

	reply.MinValidatorStake = avajson.Uint64(transformSupernet.MinValidatorStake)
	reply.MinDelegatorStake = avajson.Uint64(transformSupernet.MinDelegatorStake)

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
	// Deprecated: Use Weight instead.
	Stake avajson.Uint64 `json:"stake"`

	Weight avajson.Uint64 `json:"weight"`
}

// GetTotalStake returns the total amount staked on the Primary Network
func (s *Service) GetTotalStake(_ *http.Request, args *GetTotalStakeArgs, reply *GetTotalStakeReply) error {
	s.vm.ctx.Log.Debug("API called",
		zap.String("service", "platform"),
		zap.String("method", "getTotalStake"),
	)

	totalWeight, err := s.vm.Validators.TotalWeight(args.SupernetID)
	if err != nil {
		return fmt.Errorf("couldn't get total weight: %w", err)
	}
	weight := avajson.Uint64(totalWeight)
	reply.Weight = weight
	reply.Stake = weight
	return nil
}

// GetRewardUTXOsReply defines the GetRewardUTXOs replies returned from the API
type GetRewardUTXOsReply struct {
	// Number of UTXOs returned
	NumFetched avajson.Uint64 `json:"numFetched"`
	// The UTXOs
	UTXOs []string `json:"utxos"`
	// Encoding specifies the encoding format the UTXOs are returned in
	Encoding formatting.Encoding `json:"encoding"`
}

// GetRewardUTXOs returns the UTXOs that were rewarded after the provided
// transaction's staking period ended.
func (s *Service) GetRewardUTXOs(_ *http.Request, args *api.GetTxArgs, reply *GetRewardUTXOsReply) error {
	s.vm.ctx.Log.Debug("deprecated API called",
		zap.String("service", "platform"),
		zap.String("method", "getRewardUTXOs"),
	)

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	utxos, err := s.vm.state.GetRewardUTXOs(args.TxID)
	if err != nil {
		return fmt.Errorf("couldn't get reward UTXOs: %w", err)
	}

	reply.NumFetched = avajson.Uint64(len(utxos))
	reply.UTXOs = make([]string, len(utxos))
	for i, utxo := range utxos {
		utxoBytes, err := txs.GenesisCodec.Marshal(txs.CodecVersion, utxo)
		if err != nil {
			return fmt.Errorf("couldn't encode UTXO to bytes: %w", err)
		}

		utxoStr, err := formatting.Encode(args.Encoding, utxoBytes)
		if err != nil {
			return fmt.Errorf("couldn't encode utxo as %s: %w", args.Encoding, err)
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
	s.vm.ctx.Log.Debug("API called",
		zap.String("service", "platform"),
		zap.String("method", "getTimestamp"),
	)

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	reply.Timestamp = s.vm.state.GetTimestamp()
	return nil
}

// GetValidatorsAtArgs is the response from GetValidatorsAt
type GetValidatorsAtArgs struct {
	Height   avajson.Uint64 `json:"height"`
	SupernetID ids.ID         `json:"supernetID"`
}

type jsonGetValidatorOutput struct {
	PublicKey *string        `json:"publicKey"`
	Weight    avajson.Uint64 `json:"weight"`
}

func (v *GetValidatorsAtReply) MarshalJSON() ([]byte, error) {
	m := make(map[ids.NodeID]*jsonGetValidatorOutput, len(v.Validators))
	for _, vdr := range v.Validators {
		vdrJSON := &jsonGetValidatorOutput{
			Weight: avajson.Uint64(vdr.Weight),
		}

		if vdr.PublicKey != nil {
			pk, err := formatting.Encode(formatting.HexNC, bls.PublicKeyToCompressedBytes(vdr.PublicKey))
			if err != nil {
				return nil, err
			}
			vdrJSON.PublicKey = &pk
		}

		m[vdr.NodeID] = vdrJSON
	}
	return json.Marshal(m)
}

func (v *GetValidatorsAtReply) UnmarshalJSON(b []byte) error {
	var m map[ids.NodeID]*jsonGetValidatorOutput
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	if m == nil {
		v.Validators = nil
		return nil
	}

	v.Validators = make(map[ids.NodeID]*validators.GetValidatorOutput, len(m))
	for nodeID, vdrJSON := range m {
		vdr := &validators.GetValidatorOutput{
			NodeID: nodeID,
			Weight: uint64(vdrJSON.Weight),
		}

		if vdrJSON.PublicKey != nil {
			pkBytes, err := formatting.Decode(formatting.HexNC, *vdrJSON.PublicKey)
			if err != nil {
				return err
			}
			vdr.PublicKey, err = bls.PublicKeyFromCompressedBytes(pkBytes)
			if err != nil {
				return err
			}
		}

		v.Validators[nodeID] = vdr
	}
	return nil
}

// GetValidatorsAtReply is the response from GetValidatorsAt
type GetValidatorsAtReply struct {
	Validators map[ids.NodeID]*validators.GetValidatorOutput
}

// GetValidatorsAt returns the weights of the validator set of a provided supernet
// at the specified height.
func (s *Service) GetValidatorsAt(r *http.Request, args *GetValidatorsAtArgs, reply *GetValidatorsAtReply) error {
	height := uint64(args.Height)
	s.vm.ctx.Log.Debug("API called",
		zap.String("service", "platform"),
		zap.String("method", "getValidatorsAt"),
		zap.Uint64("height", height),
		zap.Stringer("supernetID", args.SupernetID),
	)

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	ctx := r.Context()
	var err error
	reply.Validators, err = s.vm.GetValidatorSet(ctx, height, args.SupernetID)
	if err != nil {
		return fmt.Errorf("failed to get validator set: %w", err)
	}
	return nil
}

func (s *Service) GetBlock(_ *http.Request, args *api.GetBlockArgs, response *api.GetBlockResponse) error {
	s.vm.ctx.Log.Debug("API called",
		zap.String("service", "platform"),
		zap.String("method", "getBlock"),
		zap.Stringer("blkID", args.BlockID),
		zap.Stringer("encoding", args.Encoding),
	)

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	block, err := s.vm.manager.GetStatelessBlock(args.BlockID)
	if err != nil {
		return fmt.Errorf("couldn't get block with id %s: %w", args.BlockID, err)
	}
	response.Encoding = args.Encoding

	var result any
	if args.Encoding == formatting.JSON {
		block.InitCtx(s.vm.ctx)
		result = block
	} else {
		result, err = formatting.Encode(args.Encoding, block.Bytes())
		if err != nil {
			return fmt.Errorf("couldn't encode block %s as %s: %w", args.BlockID, args.Encoding, err)
		}
	}

	response.Block, err = json.Marshal(result)
	return err
}

// GetBlockByHeight returns the block at the given height.
func (s *Service) GetBlockByHeight(_ *http.Request, args *api.GetBlockByHeightArgs, response *api.GetBlockResponse) error {
	s.vm.ctx.Log.Debug("API called",
		zap.String("service", "platform"),
		zap.String("method", "getBlockByHeight"),
		zap.Uint64("height", uint64(args.Height)),
		zap.Stringer("encoding", args.Encoding),
	)

	s.vm.ctx.Lock.Lock()
	defer s.vm.ctx.Lock.Unlock()

	blockID, err := s.vm.state.GetBlockIDAtHeight(uint64(args.Height))
	if err != nil {
		return fmt.Errorf("couldn't get block at height %d: %w", args.Height, err)
	}

	block, err := s.vm.manager.GetStatelessBlock(blockID)
	if err != nil {
		s.vm.ctx.Log.Error("couldn't get accepted block",
			zap.Stringer("blkID", blockID),
			zap.Error(err),
		)
		return fmt.Errorf("couldn't get block with id %s: %w", blockID, err)
	}
	response.Encoding = args.Encoding

	var result any
	if args.Encoding == formatting.JSON {
		block.InitCtx(s.vm.ctx)
		result = block
	} else {
		result, err = formatting.Encode(args.Encoding, block.Bytes())
		if err != nil {
			return fmt.Errorf("couldn't encode block %s as %s: %w", blockID, args.Encoding, err)
		}
	}

	response.Block, err = json.Marshal(result)
	return err
}

func (s *Service) getAPIUptime(staker *state.Staker) (*avajson.Float32, error) {
	// Only report uptimes that we have been actively tracking.
	if constants.PrimaryNetworkID != staker.SupernetID && !s.vm.TrackedSupernets.Contains(staker.SupernetID) {
		return nil, nil
	}

	rawUptime, err := s.vm.uptimeManager.CalculateUptimePercentFrom(staker.NodeID, staker.SupernetID, staker.StartTime)
	if err != nil {
		return nil, err
	}
	// Transform this to a percentage (0-100) to make it consistent
	// with observedUptime in info.peers API
	uptime := avajson.Float32(rawUptime * 100)
	return &uptime, nil
}

func (s *Service) getAPIOwner(owner *secp256k1fx.OutputOwners) (*platformapi.Owner, error) {
	apiOwner := &platformapi.Owner{
		Locktime:  avajson.Uint64(owner.Locktime),
		Threshold: avajson.Uint32(owner.Threshold),
		Addresses: make([]string, 0, len(owner.Addrs)),
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
func getStakeHelper(tx *txs.Tx, addrs set.Set[ids.ShortID], totalAmountStaked map[ids.ID]uint64) []avax.TransferableOutput {
	staker, ok := tx.Unsigned.(txs.PermissionlessStaker)
	if !ok {
		return nil
	}

	stake := staker.Stake()
	stakedOuts := make([]avax.TransferableOutput, 0, len(stake))
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
		newAmount, err := safemath.Add64(totalAmountStaked[assetID], secpOut.Amt)
		if err != nil {
			newAmount = math.MaxUint64
		}
		totalAmountStaked[assetID] = newAmount

		stakedOuts = append(
			stakedOuts,
			*output,
		)
	}
	return stakedOuts
}
