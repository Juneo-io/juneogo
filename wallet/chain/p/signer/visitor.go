// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package signer

import (
	"context"
	"errors"
	"fmt"

	"github.com/Juneo-io/juneogo/database"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/crypto/keychain"
	"github.com/Juneo-io/juneogo/utils/crypto/secp256k1"
	"github.com/Juneo-io/juneogo/utils/hashing"
	"github.com/Juneo-io/juneogo/vms/components/avax"
	"github.com/Juneo-io/juneogo/vms/components/verify"
	"github.com/Juneo-io/juneogo/vms/platformvm/stakeable"
	"github.com/Juneo-io/juneogo/vms/platformvm/txs"
	"github.com/Juneo-io/juneogo/vms/secp256k1fx"
)

var (
	_ txs.Visitor = (*visitor)(nil)

	ErrUnsupportedTxType     = errors.New("unsupported tx type")
	ErrUnknownInputType      = errors.New("unknown input type")
	ErrUnknownOutputType     = errors.New("unknown output type")
	ErrInvalidUTXOSigIndex   = errors.New("invalid UTXO signature index")
	ErrUnknownSupernetAuthType = errors.New("unknown supernet auth type")
	ErrUnknownOwnerType      = errors.New("unknown owner type")
	ErrUnknownCredentialType = errors.New("unknown credential type")

	emptySig [secp256k1.SignatureLen]byte
)

// visitor handles signing transactions for the signer
type visitor struct {
	kc      keychain.Keychain
	backend Backend
	ctx     context.Context
	tx      *txs.Tx
}

func (*visitor) AdvanceTimeTx(*txs.AdvanceTimeTx) error {
	return ErrUnsupportedTxType
}

func (*visitor) RewardValidatorTx(*txs.RewardValidatorTx) error {
	return ErrUnsupportedTxType
}

func (s *visitor) BaseTx(tx *txs.BaseTx) error {
	txSigners, err := s.getSigners(constants.PlatformChainID, tx.Ins)
	if err != nil {
		return err
	}
	return sign(s.tx, false, txSigners)
}

func (s *visitor) AddValidatorTx(tx *txs.AddValidatorTx) error {
	txSigners, err := s.getSigners(constants.PlatformChainID, tx.Ins)
	if err != nil {
		return err
	}
	return sign(s.tx, false, txSigners)
}

func (s *visitor) AddSupernetValidatorTx(tx *txs.AddSupernetValidatorTx) error {
	txSigners, err := s.getSigners(constants.PlatformChainID, tx.Ins)
	if err != nil {
		return err
	}
	supernetAuthSigners, err := s.getSupernetSigners(tx.SupernetValidator.Supernet, tx.SupernetAuth)
	if err != nil {
		return err
	}
	txSigners = append(txSigners, supernetAuthSigners)
	return sign(s.tx, false, txSigners)
}

func (s *visitor) AddDelegatorTx(tx *txs.AddDelegatorTx) error {
	txSigners, err := s.getSigners(constants.PlatformChainID, tx.Ins)
	if err != nil {
		return err
	}
	return sign(s.tx, false, txSigners)
}

func (s *visitor) CreateChainTx(tx *txs.CreateChainTx) error {
	txSigners, err := s.getSigners(constants.PlatformChainID, tx.Ins)
	if err != nil {
		return err
	}
	supernetAuthSigners, err := s.getSupernetSigners(tx.SupernetID, tx.SupernetAuth)
	if err != nil {
		return err
	}
	txSigners = append(txSigners, supernetAuthSigners)
	return sign(s.tx, false, txSigners)
}

func (s *visitor) CreateSupernetTx(tx *txs.CreateSupernetTx) error {
	txSigners, err := s.getSigners(constants.PlatformChainID, tx.Ins)
	if err != nil {
		return err
	}
	return sign(s.tx, false, txSigners)
}

func (s *visitor) ImportTx(tx *txs.ImportTx) error {
	txSigners, err := s.getSigners(constants.PlatformChainID, tx.Ins)
	if err != nil {
		return err
	}
	txImportSigners, err := s.getSigners(tx.SourceChain, tx.ImportedInputs)
	if err != nil {
		return err
	}
	txSigners = append(txSigners, txImportSigners...)
	return sign(s.tx, false, txSigners)
}

func (s *visitor) ExportTx(tx *txs.ExportTx) error {
	txSigners, err := s.getSigners(constants.PlatformChainID, tx.Ins)
	if err != nil {
		return err
	}
	return sign(s.tx, false, txSigners)
}

func (s *visitor) RemoveSupernetValidatorTx(tx *txs.RemoveSupernetValidatorTx) error {
	txSigners, err := s.getSigners(constants.PlatformChainID, tx.Ins)
	if err != nil {
		return err
	}
	supernetAuthSigners, err := s.getSupernetSigners(tx.Supernet, tx.SupernetAuth)
	if err != nil {
		return err
	}
	txSigners = append(txSigners, supernetAuthSigners)
	return sign(s.tx, true, txSigners)
}

func (s *visitor) TransferSupernetOwnershipTx(tx *txs.TransferSupernetOwnershipTx) error {
	txSigners, err := s.getSigners(constants.PlatformChainID, tx.Ins)
	if err != nil {
		return err
	}
	supernetAuthSigners, err := s.getSupernetSigners(tx.Supernet, tx.SupernetAuth)
	if err != nil {
		return err
	}
	txSigners = append(txSigners, supernetAuthSigners)
	return sign(s.tx, true, txSigners)
}

func (s *visitor) TransformSupernetTx(tx *txs.TransformSupernetTx) error {
	txSigners, err := s.getSigners(constants.PlatformChainID, tx.Ins)
	if err != nil {
		return err
	}
	supernetAuthSigners, err := s.getSupernetSigners(tx.Supernet, tx.SupernetAuth)
	if err != nil {
		return err
	}
	txSigners = append(txSigners, supernetAuthSigners)
	return sign(s.tx, true, txSigners)
}

func (s *visitor) AddPermissionlessValidatorTx(tx *txs.AddPermissionlessValidatorTx) error {
	txSigners, err := s.getSigners(constants.PlatformChainID, tx.Ins)
	if err != nil {
		return err
	}
	return sign(s.tx, true, txSigners)
}

func (s *visitor) AddPermissionlessDelegatorTx(tx *txs.AddPermissionlessDelegatorTx) error {
	txSigners, err := s.getSigners(constants.PlatformChainID, tx.Ins)
	if err != nil {
		return err
	}
	return sign(s.tx, true, txSigners)
}

func (s *visitor) getSigners(sourceChainID ids.ID, ins []*avax.TransferableInput) ([][]keychain.Signer, error) {
	txSigners := make([][]keychain.Signer, len(ins))
	for credIndex, transferInput := range ins {
		inIntf := transferInput.In
		if stakeableIn, ok := inIntf.(*stakeable.LockIn); ok {
			inIntf = stakeableIn.TransferableIn
		}

		input, ok := inIntf.(*secp256k1fx.TransferInput)
		if !ok {
			return nil, ErrUnknownInputType
		}

		inputSigners := make([]keychain.Signer, len(input.SigIndices))
		txSigners[credIndex] = inputSigners

		utxoID := transferInput.InputID()
		utxo, err := s.backend.GetUTXO(s.ctx, sourceChainID, utxoID)
		if err == database.ErrNotFound {
			// If we don't have access to the UTXO, then we can't sign this
			// transaction. However, we can attempt to partially sign it.
			continue
		}
		if err != nil {
			return nil, err
		}

		outIntf := utxo.Out
		if stakeableOut, ok := outIntf.(*stakeable.LockOut); ok {
			outIntf = stakeableOut.TransferableOut
		}

		out, ok := outIntf.(*secp256k1fx.TransferOutput)
		if !ok {
			return nil, ErrUnknownOutputType
		}

		for sigIndex, addrIndex := range input.SigIndices {
			if addrIndex >= uint32(len(out.Addrs)) {
				return nil, ErrInvalidUTXOSigIndex
			}

			addr := out.Addrs[addrIndex]
			key, ok := s.kc.Get(addr)
			if !ok {
				// If we don't have access to the key, then we can't sign this
				// transaction. However, we can attempt to partially sign it.
				continue
			}
			inputSigners[sigIndex] = key
		}
	}
	return txSigners, nil
}

func (s *visitor) getSupernetSigners(supernetID ids.ID, supernetAuth verify.Verifiable) ([]keychain.Signer, error) {
	supernetInput, ok := supernetAuth.(*secp256k1fx.Input)
	if !ok {
		return nil, ErrUnknownSupernetAuthType
	}

	ownerIntf, err := s.backend.GetSupernetOwner(s.ctx, supernetID)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to fetch supernet owner for %q: %w",
			supernetID,
			err,
		)
	}
	owner, ok := ownerIntf.(*secp256k1fx.OutputOwners)
	if !ok {
		return nil, ErrUnknownOwnerType
	}

	authSigners := make([]keychain.Signer, len(supernetInput.SigIndices))
	for sigIndex, addrIndex := range supernetInput.SigIndices {
		if addrIndex >= uint32(len(owner.Addrs)) {
			return nil, ErrInvalidUTXOSigIndex
		}

		addr := owner.Addrs[addrIndex]
		key, ok := s.kc.Get(addr)
		if !ok {
			// If we don't have access to the key, then we can't sign this
			// transaction. However, we can attempt to partially sign it.
			continue
		}
		authSigners[sigIndex] = key
	}
	return authSigners, nil
}

// TODO: remove [signHash] after the ledger supports signing all transactions.
func sign(tx *txs.Tx, signHash bool, txSigners [][]keychain.Signer) error {
	unsignedBytes, err := txs.Codec.Marshal(txs.CodecVersion, &tx.Unsigned)
	if err != nil {
		return fmt.Errorf("couldn't marshal unsigned tx: %w", err)
	}
	unsignedHash := hashing.ComputeHash256(unsignedBytes)

	if expectedLen := len(txSigners); expectedLen != len(tx.Creds) {
		tx.Creds = make([]verify.Verifiable, expectedLen)
	}

	sigCache := make(map[ids.ShortID][secp256k1.SignatureLen]byte)
	for credIndex, inputSigners := range txSigners {
		credIntf := tx.Creds[credIndex]
		if credIntf == nil {
			credIntf = &secp256k1fx.Credential{}
			tx.Creds[credIndex] = credIntf
		}

		cred, ok := credIntf.(*secp256k1fx.Credential)
		if !ok {
			return ErrUnknownCredentialType
		}
		if expectedLen := len(inputSigners); expectedLen != len(cred.Sigs) {
			cred.Sigs = make([][secp256k1.SignatureLen]byte, expectedLen)
		}

		for sigIndex, signer := range inputSigners {
			if signer == nil {
				// If we don't have access to the key, then we can't sign this
				// transaction. However, we can attempt to partially sign it.
				continue
			}
			addr := signer.Address()
			if sig := cred.Sigs[sigIndex]; sig != emptySig {
				// If this signature has already been populated, we can just
				// copy the needed signature for the future.
				sigCache[addr] = sig
				continue
			}

			if sig, exists := sigCache[addr]; exists {
				// If this key has already produced a signature, we can just
				// copy the previous signature.
				cred.Sigs[sigIndex] = sig
				continue
			}

			var sig []byte
			if signHash {
				sig, err = signer.SignHash(unsignedHash)
			} else {
				sig, err = signer.Sign(unsignedBytes)
			}
			if err != nil {
				return fmt.Errorf("problem signing tx: %w", err)
			}
			copy(cred.Sigs[sigIndex][:], sig)
			sigCache[addr] = cred.Sigs[sigIndex]
		}
	}

	signedBytes, err := txs.Codec.Marshal(txs.CodecVersion, tx)
	if err != nil {
		return fmt.Errorf("couldn't marshal tx: %w", err)
	}
	tx.SetBytes(unsignedBytes, signedBytes)
	return nil
}
