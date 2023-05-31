# Supernets

The Avalanche network consists of the Primary Network and a collection of
sub-networks (supernets).

## Supernet Creation

Supernets are created by issuing a *CreateSupernetTx*. After a *CreateSupernetTx* is
accepted, a new supernet will exist with the *SupernetID* equal to the *TxID* of the
*CreateSupernetTx*. The *CreateSupernetTx* creates a permissioned supernet. The
*Owner* field in *CreateSupernetTx* specifies who can modify the state of the
supernet.

## Permissioned Supernets

A permissioned supernet can be modified by a few different transactions.

- CreateChainTx
  - Creates a new chain that will be validated by all validators of the supernet.
- AddSupernetValidatorTx
  - Adds a new validator to the supernet with the specified *StartTime*,
    *EndTime*, and *Weight*.
- RemoveSupernetValidatorTx
  - Removes a validator from the supernet.
- TransformSupernetTx
  - Converts the permissioned supernet into a permissionless supernet.
  - Specifies all of the staking parameters.
    - AVAX is not allowed to be used as a staking token. In general, it is not
      advisable to have multiple supernets using the same staking token.
  - After becoming a permissionless supernet, previously added permissioned
    validators will remain to finish their staking period.
  - No more chains will be able to be added to the supernet.

### Permissionless Supernets

Currently, nothing can be performed on a permissionless supernet.
