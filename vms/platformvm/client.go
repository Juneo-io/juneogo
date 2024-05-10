// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package platformvm

import (
	"context"
	"time"

	"github.com/Juneo-io/juneogo/api"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/snow/validators"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/crypto/secp256k1"
	"github.com/Juneo-io/juneogo/utils/formatting"
	"github.com/Juneo-io/juneogo/utils/formatting/address"
	"github.com/Juneo-io/juneogo/utils/json"
	"github.com/Juneo-io/juneogo/utils/rpc"
	"github.com/Juneo-io/juneogo/vms/platformvm/status"
)

var _ Client = (*client)(nil)

// Client interface for interacting with the P Chain endpoint
type Client interface {
	// GetHeight returns the current block height of the P Chain
	GetHeight(ctx context.Context, options ...rpc.Option) (uint64, error)
	// ExportKey returns the private key corresponding to [address] from [user]'s account
	//
	// Deprecated: Keys should no longer be stored on the node.
	ExportKey(ctx context.Context, user api.UserPass, address ids.ShortID, options ...rpc.Option) (*secp256k1.PrivateKey, error)
	// GetBalance returns the balance of [addrs] on the P Chain
	//
	// Deprecated: GetUTXOs should be used instead.
	GetBalance(ctx context.Context, addrs []ids.ShortID, options ...rpc.Option) (*GetBalanceResponse, error)
	// ListAddresses returns an array of platform addresses controlled by [user]
	//
	// Deprecated: Keys should no longer be stored on the node.
	ListAddresses(ctx context.Context, user api.UserPass, options ...rpc.Option) ([]ids.ShortID, error)
	// GetUTXOs returns the byte representation of the UTXOs controlled by [addrs]
	GetUTXOs(
		ctx context.Context,
		addrs []ids.ShortID,
		limit uint32,
		startAddress ids.ShortID,
		startUTXOID ids.ID,
		options ...rpc.Option,
	) ([][]byte, ids.ShortID, ids.ID, error)
	// GetAtomicUTXOs returns the byte representation of the atomic UTXOs controlled by [addrs]
	// from [sourceChain]
	GetAtomicUTXOs(
		ctx context.Context,
		addrs []ids.ShortID,
		sourceChain string,
		limit uint32,
		startAddress ids.ShortID,
		startUTXOID ids.ID,
		options ...rpc.Option,
	) ([][]byte, ids.ShortID, ids.ID, error)
	// GetSupernet returns information about the specified supernet
	GetSupernet(ctx context.Context, supernetID ids.ID, options ...rpc.Option) (GetSupernetClientResponse, error)
	// GetSupernets returns information about the specified supernets
	//
	// Deprecated: Supernets should be fetched from a dedicated indexer.
	GetSupernets(ctx context.Context, supernetIDs []ids.ID, options ...rpc.Option) ([]ClientSupernet, error)
	// GetStakingAssetID returns the assetID of the asset used for staking on
	// supernet corresponding to [supernetID]
	GetStakingAssetID(ctx context.Context, supernetID ids.ID, options ...rpc.Option) (ids.ID, error)
	// GetCurrentValidators returns the list of current validators for supernet with ID [supernetID]
	GetCurrentValidators(ctx context.Context, supernetID ids.ID, nodeIDs []ids.NodeID, options ...rpc.Option) ([]ClientPermissionlessValidator, error)
	// GetCurrentSupply returns an upper bound on the supply of AVAX in the system along with the P-chain height
	GetCurrentSupply(ctx context.Context, supernetID ids.ID, options ...rpc.Option) (uint64, uint64, error)
	// GetRewardPoolSupply returns the current supply in the reward pool
	GetRewardPoolSupply(ctx context.Context, supernetID ids.ID, options ...rpc.Option) (uint64, error)
	// GetFeePoolValue returns the current value in the fee pool
	GetFeePoolValue(ctx context.Context, options ...rpc.Option) (uint64, error)
	// SampleValidators returns the nodeIDs of a sample of [sampleSize] validators from the current validator set for supernet with ID [supernetID]
	SampleValidators(ctx context.Context, supernetID ids.ID, sampleSize uint16, options ...rpc.Option) ([]ids.NodeID, error)
	// GetBlockchainStatus returns the current status of blockchain with ID: [blockchainID]
	GetBlockchainStatus(ctx context.Context, blockchainID string, options ...rpc.Option) (status.BlockchainStatus, error)
	// ValidatedBy returns the ID of the Supernet that validates [blockchainID]
	ValidatedBy(ctx context.Context, blockchainID ids.ID, options ...rpc.Option) (ids.ID, error)
	// Validates returns the list of blockchains that are validated by the supernet with ID [supernetID]
	Validates(ctx context.Context, supernetID ids.ID, options ...rpc.Option) ([]ids.ID, error)
	// GetBlockchains returns the list of blockchains on the platform
	//
	// Deprecated: Blockchains should be fetched from a dedicated indexer.
	GetBlockchains(ctx context.Context, options ...rpc.Option) ([]APIBlockchain, error)
	// IssueTx issues the transaction and returns its txID
	IssueTx(ctx context.Context, tx []byte, options ...rpc.Option) (ids.ID, error)
	// GetTx returns the byte representation of the transaction corresponding to [txID]
	GetTx(ctx context.Context, txID ids.ID, options ...rpc.Option) ([]byte, error)
	// GetTxStatus returns the status of the transaction corresponding to [txID]
	GetTxStatus(ctx context.Context, txID ids.ID, options ...rpc.Option) (*GetTxStatusResponse, error)
	// AwaitTxDecided polls [GetTxStatus] until a status is returned that
	// implies the tx may be decided.
	// TODO: Move this function off of the Client interface into a utility
	// function.
	AwaitTxDecided(
		ctx context.Context,
		txID ids.ID,
		freq time.Duration,
		options ...rpc.Option,
	) (*GetTxStatusResponse, error)
	// GetStake returns the amount of nAVAX that [addrs] have cumulatively
	// staked on the Primary Network.
	//
	// Deprecated: Stake should be calculated using GetTx and GetCurrentValidators.
	GetStake(
		ctx context.Context,
		addrs []ids.ShortID,
		validatorsOnly bool,
		options ...rpc.Option,
	) (map[ids.ID]uint64, [][]byte, error)
	// GetMinStake returns the minimum staking amount in nAVAX for validators
	// and delegators respectively
	GetMinStake(ctx context.Context, supernetID ids.ID, options ...rpc.Option) (uint64, uint64, error)
	// GetTotalStake returns the total amount (in nAVAX) staked on the network
	GetTotalStake(ctx context.Context, supernetID ids.ID, options ...rpc.Option) (uint64, error)
	// GetRewardUTXOs returns the reward UTXOs for a transaction
	//
	// Deprecated: GetRewardUTXOs should be fetched from a dedicated indexer.
	GetRewardUTXOs(context.Context, *api.GetTxArgs, ...rpc.Option) ([][]byte, error)
	// GetTimestamp returns the current chain timestamp
	GetTimestamp(ctx context.Context, options ...rpc.Option) (time.Time, error)
	// GetValidatorsAt returns the weights of the validator set of a provided
	// supernet at the specified height.
	GetValidatorsAt(
		ctx context.Context,
		supernetID ids.ID,
		height uint64,
		options ...rpc.Option,
	) (map[ids.NodeID]*validators.GetValidatorOutput, error)
	// GetBlock returns the block with the given id.
	GetBlock(ctx context.Context, blockID ids.ID, options ...rpc.Option) ([]byte, error)
	// GetBlockByHeight returns the block at the given [height].
	GetBlockByHeight(ctx context.Context, height uint64, options ...rpc.Option) ([]byte, error)
}

// Client implementation for interacting with the P Chain endpoint
type client struct {
	requester rpc.EndpointRequester
}

// NewClient returns a Client for interacting with the P Chain endpoint
func NewClient(uri string) Client {
	return &client{requester: rpc.NewEndpointRequester(
		uri + "/ext/P",
	)}
}

func (c *client) GetHeight(ctx context.Context, options ...rpc.Option) (uint64, error) {
	res := &api.GetHeightResponse{}
	err := c.requester.SendRequest(ctx, "platform.getHeight", struct{}{}, res, options...)
	return uint64(res.Height), err
}

func (c *client) ExportKey(ctx context.Context, user api.UserPass, address ids.ShortID, options ...rpc.Option) (*secp256k1.PrivateKey, error) {
	res := &ExportKeyReply{}
	err := c.requester.SendRequest(ctx, "platform.exportKey", &ExportKeyArgs{
		UserPass: user,
		Address:  address.String(),
	}, res, options...)
	return res.PrivateKey, err
}

func (c *client) GetBalance(ctx context.Context, addrs []ids.ShortID, options ...rpc.Option) (*GetBalanceResponse, error) {
	res := &GetBalanceResponse{}
	err := c.requester.SendRequest(ctx, "platform.getBalance", &GetBalanceRequest{
		Addresses: ids.ShortIDsToStrings(addrs),
	}, res, options...)
	return res, err
}

func (c *client) ListAddresses(ctx context.Context, user api.UserPass, options ...rpc.Option) ([]ids.ShortID, error) {
	res := &api.JSONAddresses{}
	err := c.requester.SendRequest(ctx, "platform.listAddresses", &user, res, options...)
	if err != nil {
		return nil, err
	}
	return address.ParseToIDs(res.Addresses)
}

func (c *client) GetUTXOs(
	ctx context.Context,
	addrs []ids.ShortID,
	limit uint32,
	startAddress ids.ShortID,
	startUTXOID ids.ID,
	options ...rpc.Option,
) ([][]byte, ids.ShortID, ids.ID, error) {
	return c.GetAtomicUTXOs(ctx, addrs, "", limit, startAddress, startUTXOID, options...)
}

func (c *client) GetAtomicUTXOs(
	ctx context.Context,
	addrs []ids.ShortID,
	sourceChain string,
	limit uint32,
	startAddress ids.ShortID,
	startUTXOID ids.ID,
	options ...rpc.Option,
) ([][]byte, ids.ShortID, ids.ID, error) {
	res := &api.GetUTXOsReply{}
	err := c.requester.SendRequest(ctx, "platform.getUTXOs", &api.GetUTXOsArgs{
		Addresses:   ids.ShortIDsToStrings(addrs),
		SourceChain: sourceChain,
		Limit:       json.Uint32(limit),
		StartIndex: api.Index{
			Address: startAddress.String(),
			UTXO:    startUTXOID.String(),
		},
		Encoding: formatting.Hex,
	}, res, options...)
	if err != nil {
		return nil, ids.ShortID{}, ids.Empty, err
	}

	utxos := make([][]byte, len(res.UTXOs))
	for i, utxo := range res.UTXOs {
		utxoBytes, err := formatting.Decode(res.Encoding, utxo)
		if err != nil {
			return nil, ids.ShortID{}, ids.Empty, err
		}
		utxos[i] = utxoBytes
	}
	endAddr, err := address.ParseToID(res.EndIndex.Address)
	if err != nil {
		return nil, ids.ShortID{}, ids.Empty, err
	}
	endUTXOID, err := ids.FromString(res.EndIndex.UTXO)
	return utxos, endAddr, endUTXOID, err
}

// GetSupernetClientResponse is the response from calling GetSupernet on the client
type GetSupernetClientResponse struct {
	// whether it is permissioned or not
	IsPermissioned bool
	// supernet auth information for a permissioned supernet
	ControlKeys []ids.ShortID
	Threshold   uint32
	Locktime    uint64
	// supernet transformation tx ID for a permissionless supernet
	SupernetTransformationTxID ids.ID
}

func (c *client) GetSupernet(ctx context.Context, supernetID ids.ID, options ...rpc.Option) (GetSupernetClientResponse, error) {
	res := &GetSupernetResponse{}
	err := c.requester.SendRequest(ctx, "platform.getSupernet", &GetSupernetArgs{
		SupernetID: supernetID,
	}, res, options...)
	if err != nil {
		return GetSupernetClientResponse{}, err
	}
	controlKeys, err := address.ParseToIDs(res.ControlKeys)
	if err != nil {
		return GetSupernetClientResponse{}, err
	}

	return GetSupernetClientResponse{
		IsPermissioned:           res.IsPermissioned,
		ControlKeys:              controlKeys,
		Threshold:                uint32(res.Threshold),
		Locktime:                 uint64(res.Locktime),
		SupernetTransformationTxID: res.SupernetTransformationTxID,
	}, nil
}

// ClientSupernet is a representation of a supernet used in client methods
type ClientSupernet struct {
	// ID of the supernet
	ID ids.ID
	// Each element of [ControlKeys] the address of a public key.
	// A transaction to add a validator to this supernet requires
	// signatures from [Threshold] of these keys to be valid.
	ControlKeys []ids.ShortID
	Threshold   uint32
}

func (c *client) GetSupernets(ctx context.Context, ids []ids.ID, options ...rpc.Option) ([]ClientSupernet, error) {
	res := &GetSupernetsResponse{}
	err := c.requester.SendRequest(ctx, "platform.getSupernets", &GetSupernetsArgs{
		IDs: ids,
	}, res, options...)
	if err != nil {
		return nil, err
	}
	supernets := make([]ClientSupernet, len(res.Supernets))
	for i, apiSupernet := range res.Supernets {
		controlKeys, err := address.ParseToIDs(apiSupernet.ControlKeys)
		if err != nil {
			return nil, err
		}

		supernets[i] = ClientSupernet{
			ID:          apiSupernet.ID,
			ControlKeys: controlKeys,
			Threshold:   uint32(apiSupernet.Threshold),
		}
	}
	return supernets, nil
}

func (c *client) GetStakingAssetID(ctx context.Context, supernetID ids.ID, options ...rpc.Option) (ids.ID, error) {
	res := &GetStakingAssetIDResponse{}
	err := c.requester.SendRequest(ctx, "platform.getStakingAssetID", &GetStakingAssetIDArgs{
		SupernetID: supernetID,
	}, res, options...)
	return res.AssetID, err
}

func (c *client) GetCurrentValidators(
	ctx context.Context,
	supernetID ids.ID,
	nodeIDs []ids.NodeID,
	options ...rpc.Option,
) ([]ClientPermissionlessValidator, error) {
	res := &GetCurrentValidatorsReply{}
	err := c.requester.SendRequest(ctx, "platform.getCurrentValidators", &GetCurrentValidatorsArgs{
		SupernetID: supernetID,
		NodeIDs:  nodeIDs,
	}, res, options...)
	if err != nil {
		return nil, err
	}
	return getClientPermissionlessValidators(res.Validators)
}

func (c *client) GetCurrentSupply(ctx context.Context, supernetID ids.ID, options ...rpc.Option) (uint64, uint64, error) {
	res := &GetCurrentSupplyReply{}
	err := c.requester.SendRequest(ctx, "platform.getCurrentSupply", &GetCurrentSupplyArgs{
		SupernetID: supernetID,
	}, res, options...)
	return uint64(res.Supply), uint64(res.Height), err
}

func (c *client) GetRewardPoolSupply(ctx context.Context, supernetID ids.ID, options ...rpc.Option) (uint64, error) {
	res := &GetRewardPoolSupplyReply{}
	err := c.requester.SendRequest(ctx, "platform.getRewardPoolSupply", &GetRewardPoolSupplyArgs{
		SupernetID: supernetID,
	}, res, options...)
	return uint64(res.RewardPoolSupply), err
}

func (c *client) GetFeePoolValue(ctx context.Context, options ...rpc.Option) (uint64, error) {
	res := &GetFeePoolValueReply{}
	err := c.requester.SendRequest(ctx, "platform.getFeePoolValue", struct{}{}, res, options...)
	return uint64(res.FeePoolValue), err
}

func (c *client) SampleValidators(ctx context.Context, supernetID ids.ID, sampleSize uint16, options ...rpc.Option) ([]ids.NodeID, error) {
	res := &SampleValidatorsReply{}
	err := c.requester.SendRequest(ctx, "platform.sampleValidators", &SampleValidatorsArgs{
		SupernetID: supernetID,
		Size:     json.Uint16(sampleSize),
	}, res, options...)
	return res.Validators, err
}

func (c *client) GetBlockchainStatus(ctx context.Context, blockchainID string, options ...rpc.Option) (status.BlockchainStatus, error) {
	res := &GetBlockchainStatusReply{}
	err := c.requester.SendRequest(ctx, "platform.getBlockchainStatus", &GetBlockchainStatusArgs{
		BlockchainID: blockchainID,
	}, res, options...)
	return res.Status, err
}

func (c *client) ValidatedBy(ctx context.Context, blockchainID ids.ID, options ...rpc.Option) (ids.ID, error) {
	res := &ValidatedByResponse{}
	err := c.requester.SendRequest(ctx, "platform.validatedBy", &ValidatedByArgs{
		BlockchainID: blockchainID,
	}, res, options...)
	return res.SupernetID, err
}

func (c *client) Validates(ctx context.Context, supernetID ids.ID, options ...rpc.Option) ([]ids.ID, error) {
	res := &ValidatesResponse{}
	err := c.requester.SendRequest(ctx, "platform.validates", &ValidatesArgs{
		SupernetID: supernetID,
	}, res, options...)
	return res.BlockchainIDs, err
}

func (c *client) GetBlockchains(ctx context.Context, options ...rpc.Option) ([]APIBlockchain, error) {
	res := &GetBlockchainsResponse{}
	err := c.requester.SendRequest(ctx, "platform.getBlockchains", struct{}{}, res, options...)
	return res.Blockchains, err
}

func (c *client) IssueTx(ctx context.Context, txBytes []byte, options ...rpc.Option) (ids.ID, error) {
	txStr, err := formatting.Encode(formatting.Hex, txBytes)
	if err != nil {
		return ids.ID{}, err
	}

	res := &api.JSONTxID{}
	err = c.requester.SendRequest(ctx, "platform.issueTx", &api.FormattedTx{
		Tx:       txStr,
		Encoding: formatting.Hex,
	}, res, options...)
	return res.TxID, err
}

func (c *client) GetTx(ctx context.Context, txID ids.ID, options ...rpc.Option) ([]byte, error) {
	res := &api.FormattedTx{}
	err := c.requester.SendRequest(ctx, "platform.getTx", &api.GetTxArgs{
		TxID:     txID,
		Encoding: formatting.Hex,
	}, res, options...)
	if err != nil {
		return nil, err
	}
	return formatting.Decode(res.Encoding, res.Tx)
}

func (c *client) GetTxStatus(ctx context.Context, txID ids.ID, options ...rpc.Option) (*GetTxStatusResponse, error) {
	res := &GetTxStatusResponse{}
	err := c.requester.SendRequest(
		ctx,
		"platform.getTxStatus",
		&GetTxStatusArgs{
			TxID: txID,
		},
		res,
		options...,
	)
	return res, err
}

func (c *client) AwaitTxDecided(ctx context.Context, txID ids.ID, freq time.Duration, options ...rpc.Option) (*GetTxStatusResponse, error) {
	ticker := time.NewTicker(freq)
	defer ticker.Stop()

	for {
		res, err := c.GetTxStatus(ctx, txID, options...)
		if err == nil {
			switch res.Status {
			case status.Committed, status.Aborted, status.Dropped:
				return res, nil
			}
		}

		select {
		case <-ticker.C:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (c *client) GetStake(
	ctx context.Context,
	addrs []ids.ShortID,
	validatorsOnly bool,
	options ...rpc.Option,
) (map[ids.ID]uint64, [][]byte, error) {
	res := &GetStakeReply{}
	err := c.requester.SendRequest(ctx, "platform.getStake", &GetStakeArgs{
		JSONAddresses: api.JSONAddresses{
			Addresses: ids.ShortIDsToStrings(addrs),
		},
		ValidatorsOnly: validatorsOnly,
		Encoding:       formatting.Hex,
	}, res, options...)
	if err != nil {
		return nil, nil, err
	}

	staked := make(map[ids.ID]uint64, len(res.Stakeds))
	for assetID, amount := range res.Stakeds {
		staked[assetID] = uint64(amount)
	}

	outputs := make([][]byte, len(res.Outputs))
	for i, outputStr := range res.Outputs {
		output, err := formatting.Decode(res.Encoding, outputStr)
		if err != nil {
			return nil, nil, err
		}
		outputs[i] = output
	}
	return staked, outputs, err
}

func (c *client) GetMinStake(ctx context.Context, supernetID ids.ID, options ...rpc.Option) (uint64, uint64, error) {
	res := &GetMinStakeReply{}
	err := c.requester.SendRequest(ctx, "platform.getMinStake", &GetMinStakeArgs{
		SupernetID: supernetID,
	}, res, options...)
	return uint64(res.MinValidatorStake), uint64(res.MinDelegatorStake), err
}

func (c *client) GetTotalStake(ctx context.Context, supernetID ids.ID, options ...rpc.Option) (uint64, error) {
	res := &GetTotalStakeReply{}
	err := c.requester.SendRequest(ctx, "platform.getTotalStake", &GetTotalStakeArgs{
		SupernetID: supernetID,
	}, res, options...)
	var amount json.Uint64
	if supernetID == constants.PrimaryNetworkID {
		amount = res.Stake
	} else {
		amount = res.Weight
	}
	return uint64(amount), err
}

func (c *client) GetRewardUTXOs(ctx context.Context, args *api.GetTxArgs, options ...rpc.Option) ([][]byte, error) {
	res := &GetRewardUTXOsReply{}
	err := c.requester.SendRequest(ctx, "platform.getRewardUTXOs", args, res, options...)
	if err != nil {
		return nil, err
	}
	utxos := make([][]byte, len(res.UTXOs))
	for i, utxoStr := range res.UTXOs {
		utxoBytes, err := formatting.Decode(res.Encoding, utxoStr)
		if err != nil {
			return nil, err
		}
		utxos[i] = utxoBytes
	}
	return utxos, err
}

func (c *client) GetTimestamp(ctx context.Context, options ...rpc.Option) (time.Time, error) {
	res := &GetTimestampReply{}
	err := c.requester.SendRequest(ctx, "platform.getTimestamp", struct{}{}, res, options...)
	return res.Timestamp, err
}

func (c *client) GetValidatorsAt(
	ctx context.Context,
	supernetID ids.ID,
	height uint64,
	options ...rpc.Option,
) (map[ids.NodeID]*validators.GetValidatorOutput, error) {
	res := &GetValidatorsAtReply{}
	err := c.requester.SendRequest(ctx, "platform.getValidatorsAt", &GetValidatorsAtArgs{
		SupernetID: supernetID,
		Height:   json.Uint64(height),
	}, res, options...)
	return res.Validators, err
}

func (c *client) GetBlock(ctx context.Context, blockID ids.ID, options ...rpc.Option) ([]byte, error) {
	res := &api.FormattedBlock{}
	if err := c.requester.SendRequest(ctx, "platform.getBlock", &api.GetBlockArgs{
		BlockID:  blockID,
		Encoding: formatting.Hex,
	}, res, options...); err != nil {
		return nil, err
	}
	return formatting.Decode(res.Encoding, res.Block)
}

func (c *client) GetBlockByHeight(ctx context.Context, height uint64, options ...rpc.Option) ([]byte, error) {
	res := &api.FormattedBlock{}
	err := c.requester.SendRequest(ctx, "platform.getBlockByHeight", &api.GetBlockByHeightArgs{
		Height:   json.Uint64(height),
		Encoding: formatting.HexNC,
	}, res, options...)
	if err != nil {
		return nil, err
	}
	return formatting.Decode(res.Encoding, res.Block)
}
