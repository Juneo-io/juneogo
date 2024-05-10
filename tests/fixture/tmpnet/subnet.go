// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tmpnet

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/crypto/secp256k1"
	"github.com/Juneo-io/juneogo/utils/perms"
	"github.com/Juneo-io/juneogo/utils/set"
	"github.com/Juneo-io/juneogo/utils/units"
	"github.com/Juneo-io/juneogo/vms/platformvm"
	"github.com/Juneo-io/juneogo/vms/platformvm/txs"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
	"github.com/Juneo-io/juneogo/wallet/supernet/primary"
	"github.com/Juneo-io/juneogo/wallet/supernet/primary/common"
)

const defaultSupernetDirName = "supernets"

type Chain struct {
	// Set statically
	VMID    ids.ID
	Config  string
	Genesis []byte

	// Set at runtime
	ChainID      ids.ID
	PreFundedKey *secp256k1.PrivateKey
}

// Write the chain configuration to the specified directory.
func (c *Chain) WriteConfig(chainDir string) error {
	// TODO(marun) Ensure removal of an existing file if no configuration should be provided
	if len(c.Config) == 0 {
		return nil
	}

	chainConfigDir := filepath.Join(chainDir, c.ChainID.String())
	if err := os.MkdirAll(chainConfigDir, perms.ReadWriteExecute); err != nil {
		return fmt.Errorf("failed to create chain config dir: %w", err)
	}

	path := filepath.Join(chainConfigDir, defaultConfigFilename)
	if err := os.WriteFile(path, []byte(c.Config), perms.ReadWrite); err != nil {
		return fmt.Errorf("failed to write chain config: %w", err)
	}

	return nil
}

type Supernet struct {
	// A unique string that can be used to refer to the supernet across different temporary
	// networks (since the SupernetID will be different every time the supernet is created)
	Name string

	Config FlagsMap

	// The ID of the transaction that created the supernet
	SupernetID ids.ID

	// The private key that owns the supernet
	OwningKey *secp256k1.PrivateKey

	// IDs of the nodes responsible for validating the supernet
	ValidatorIDs []ids.NodeID

	Chains []*Chain
}

// Retrieves a wallet configured for use with the supernet
func (s *Supernet) GetWallet(ctx context.Context, uri string) (primary.Wallet, error) {
	keychain := secp256k1fx.NewKeychain(s.OwningKey)

	// Only fetch the supernet transaction if a supernet ID is present. This won't be true when
	// the wallet is first used to create the supernet.
	txIDs := set.Set[ids.ID]{}
	if s.SupernetID != ids.Empty {
		txIDs.Add(s.SupernetID)
	}

	return primary.MakeWallet(ctx, &primary.WalletConfig{
		URI:              uri,
		AVAXKeychain:     keychain,
		EthKeychain:      keychain,
		PChainTxsToFetch: txIDs,
	})
}

// Issues the supernet creation transaction and retains the result. The URI of a node is
// required to issue the transaction.
func (s *Supernet) Create(ctx context.Context, uri string) error {
	wallet, err := s.GetWallet(ctx, uri)
	if err != nil {
		return err
	}
	pWallet := wallet.P()

	supernetTx, err := pWallet.IssueCreateSupernetTx(
		&secp256k1fx.OutputOwners{
			Threshold: 1,
			Addrs: []ids.ShortID{
				s.OwningKey.Address(),
			},
		},
		common.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to create supernet %s: %w", s.Name, err)
	}
	s.SupernetID = supernetTx.ID()

	return nil
}

func (s *Supernet) CreateChains(ctx context.Context, w io.Writer, uri string) error {
	wallet, err := s.GetWallet(ctx, uri)
	if err != nil {
		return err
	}
	pWallet := wallet.P()

	if _, err := fmt.Fprintf(w, "Creating chains for supernet %q\n", s.Name); err != nil {
		return err
	}

	for _, chain := range s.Chains {
		createChainTx, err := pWallet.IssueCreateChainTx(
			s.SupernetID,
			chain.Genesis,
			chain.VMID,
			nil,
			"",
			common.WithContext(ctx),
		)
		if err != nil {
			return fmt.Errorf("failed to create chain: %w", err)
		}
		chain.ChainID = createChainTx.ID()

		if _, err := fmt.Fprintf(w, " created chain %q for VM %q on supernet %q\n", chain.ChainID, chain.VMID, s.Name); err != nil {
			return err
		}
	}
	return nil
}

// Add validators to the supernet
func (s *Supernet) AddValidators(ctx context.Context, w io.Writer, nodes ...*Node) error {
	apiURI := nodes[0].URI

	wallet, err := s.GetWallet(ctx, apiURI)
	if err != nil {
		return err
	}
	pWallet := wallet.P()

	// Collect the end times for current validators to reuse for supernet validators
	pvmClient := platformvm.NewClient(apiURI)
	validators, err := pvmClient.GetCurrentValidators(ctx, constants.PrimaryNetworkID, nil)
	if err != nil {
		return err
	}
	endTimes := make(map[ids.NodeID]uint64)
	for _, validator := range validators {
		endTimes[validator.NodeID] = validator.EndTime
	}

	startTime := time.Now().Add(DefaultValidatorStartTimeDiff)
	for _, node := range nodes {
		endTime, ok := endTimes[node.NodeID]
		if !ok {
			return fmt.Errorf("failed to find end time for %s", node.NodeID)
		}

		_, err := pWallet.IssueAddSupernetValidatorTx(
			&txs.SupernetValidator{
				Validator: txs.Validator{
					NodeID: node.NodeID,
					Start:  uint64(startTime.Unix()),
					End:    endTime,
					Wght:   units.Schmeckle,
				},
				Supernet: s.SupernetID,
			},
			common.WithContext(ctx),
		)
		if err != nil {
			return err
		}

		if _, err := fmt.Fprintf(w, " added %s as validator for supernet `%s`\n", node.NodeID, s.Name); err != nil {
			return err
		}
	}

	return nil
}

// Write the supernet configuration to disk
func (s *Supernet) Write(supernetDir string, chainDir string) error {
	if err := os.MkdirAll(supernetDir, perms.ReadWriteExecute); err != nil {
		return fmt.Errorf("failed to create supernet dir: %w", err)
	}
	tmpnetConfigPath := filepath.Join(supernetDir, s.Name+".json")

	// Since supernets are expected to be serialized for the first time
	// without their chains having been created (i.e. chains will have
	// empty IDs), use the absence of chain IDs as a prompt for a
	// supernet name uniqueness check.
	if len(s.Chains) > 0 && s.Chains[0].ChainID == ids.Empty {
		_, err := os.Stat(tmpnetConfigPath)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err == nil {
			return fmt.Errorf("a supernet with name %s already exists", s.Name)
		}
	}

	// Write supernet configuration for tmpnet
	bytes, err := DefaultJSONMarshal(s)
	if err != nil {
		return fmt.Errorf("failed to marshal tmpnet supernet %s: %w", s.Name, err)
	}
	if err := os.WriteFile(tmpnetConfigPath, bytes, perms.ReadWrite); err != nil {
		return fmt.Errorf("failed to write tmpnet supernet config %s: %w", s.Name, err)
	}

	// The supernet and chain configurations for avalanchego can only be written once
	// they have been created since the id of the creating transaction must be
	// included in the path.
	if s.SupernetID == ids.Empty {
		return nil
	}

	// TODO(marun) Ensure removal of an existing file if no configuration should be provided
	if len(s.Config) > 0 {
		// Write supernet configuration for avalanchego
		bytes, err = DefaultJSONMarshal(s.Config)
		if err != nil {
			return fmt.Errorf("failed to marshal avalanchego supernet config %s: %w", s.Name, err)
		}

		avgoConfigDir := filepath.Join(supernetDir, s.SupernetID.String())
		if err := os.MkdirAll(avgoConfigDir, perms.ReadWriteExecute); err != nil {
			return fmt.Errorf("failed to create avalanchego supernet config dir: %w", err)
		}

		avgoConfigPath := filepath.Join(avgoConfigDir, defaultConfigFilename)
		if err := os.WriteFile(avgoConfigPath, bytes, perms.ReadWrite); err != nil {
			return fmt.Errorf("failed to write avalanchego supernet config %s: %w", s.Name, err)
		}
	}

	for _, chain := range s.Chains {
		if err := chain.WriteConfig(chainDir); err != nil {
			return err
		}
	}

	return nil
}

// HasChainConfig indicates whether at least one of the supernet's
// chains have explicit configuration. This can be used to determine
// whether validator restart is required after chain creation to
// ensure that chains are configured correctly.
func (s *Supernet) HasChainConfig() bool {
	for _, chain := range s.Chains {
		if len(chain.Config) > 0 {
			return true
		}
	}
	return false
}

func waitForActiveValidators(
	ctx context.Context,
	w io.Writer,
	pChainClient platformvm.Client,
	supernet *Supernet,
) error {
	ticker := time.NewTicker(DefaultPollingInterval)
	defer ticker.Stop()

	if _, err := fmt.Fprintf(w, "Waiting for validators of supernet %q to become active\n", supernet.Name); err != nil {
		return err
	}

	if _, err := fmt.Fprint(w, " "); err != nil {
		return err
	}

	for {
		if _, err := fmt.Fprint(w, "."); err != nil {
			return err
		}
		validators, err := pChainClient.GetCurrentValidators(ctx, supernet.SupernetID, nil)
		if err != nil {
			return err
		}
		validatorSet := set.NewSet[ids.NodeID](len(validators))
		for _, validator := range validators {
			validatorSet.Add(validator.NodeID)
		}
		allActive := true
		for _, validatorID := range supernet.ValidatorIDs {
			if !validatorSet.Contains(validatorID) {
				allActive = false
			}
		}
		if allActive {
			if _, err := fmt.Fprintf(w, "\n saw the expected active validators of supernet %q\n", supernet.Name); err != nil {
				return err
			}
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("failed to see the expected active validators of supernet %q before timeout", supernet.Name)
		case <-ticker.C:
		}
	}
}

// Reads supernets from [network dir]/supernets/[supernet name].json
func readSupernets(supernetDir string) ([]*Supernet, error) {
	if _, err := os.Stat(supernetDir); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(supernetDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read supernet dir: %w", err)
	}

	supernets := []*Supernet{}
	for _, entry := range entries {
		if entry.IsDir() {
			// Looking only for files
			continue
		}
		if filepath.Ext(entry.Name()) != ".json" {
			// Supernet files should have a .json extension
			continue
		}

		supernetPath := filepath.Join(supernetDir, entry.Name())
		bytes, err := os.ReadFile(supernetPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read supernet file %s: %w", supernetPath, err)
		}
		supernet := &Supernet{}
		if err := json.Unmarshal(bytes, supernet); err != nil {
			return nil, fmt.Errorf("failed to unmarshal supernet from %s: %w", supernetPath, err)
		}
		supernets = append(supernets, supernet)
	}

	return supernets, nil
}
