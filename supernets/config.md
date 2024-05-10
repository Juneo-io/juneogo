---
tags: [Nodes]
description: Reference for all available Supernet config options and flags.
sidebar_label: Supernet Configs
pagination_label: Supernet Configs
sidebar_position: 2
---

# Supernet Configs

It is possible to provide parameters for a Supernet. Parameters here apply to all
chains in the specified Supernet.

AvalancheGo looks for files specified with `{supernetID}.json` under
`--supernet-config-dir` as documented
[here](/nodes/configure/avalanchego-config-flags.md#supernet-configs).

Here is an example of Supernet config file:

```json
{
  "validatorOnly": false,
  "consensusParameters": {
    "k": 25,
    "alpha": 18
  }
}
```

## Parameters

### Private Supernet

#### `validatorOnly` (bool)

If `true` this node does not expose Supernet blockchain contents to non-validators
via P2P messages. Defaults to `false`.

Avalanche Supernets are public by default. It means that every node can sync and
listen ongoing transactions/blocks in Supernets, even they're not validating the
listened Supernet.

Supernet validators can choose not to publish contents of blockchains via this
configuration. If a node sets `validatorOnly` to true, the node exchanges
messages only with this Supernet's validators. Other peers will not be able to
learn contents of this Supernet from this node.

:::tip

This is a node-specific configuration. Every validator of this Supernet has to use
this configuration in order to create a full private Supernet.

:::

#### `allowedNodes` (string list)

If `validatorOnly=true` this allows explicitly specified NodeIDs to be allowed
to sync the Supernet regardless of validator status. Defaults to be empty.

:::tip

This is a node-specific configuration. Every validator of this Supernet has to use
this configuration in order to properly allow a node in the private Supernet.

:::

#### `proposerMinBlockDelay` (duration)

The minimum delay performed when building snowman++ blocks. Default is set to 1 second.

As one of the ways to control network congestion, Snowman++ will only build a
block `proposerMinBlockDelay` after the parent block's timestamp. Some
high-performance custom VM may find this too strict. This flag allows tuning the
frequency at which blocks are built.

### Consensus Parameters

Supernet configs supports loading new consensus parameters. JSON keys are
different from their matching `CLI` keys. These parameters must be grouped under
`consensusParameters` key. The consensus parameters of a Supernet default to the
same values used for the Primary Network, which are given [CLI Snow Parameters](/nodes/configure/avalanchego-config-flags.md#snow-parameters).

| CLI Key                          | JSON Key              |
| :------------------------------- | :-------------------- |
| --snow-sample-size               | k                     |
| --snow-quorum-size               | alpha                 |
| --snow-commit-threshold          | `beta`                |
| --snow-concurrent-repolls        | concurrentRepolls     |
| --snow-optimal-processing        | `optimalProcessing`   |
| --snow-max-processing            | maxOutstandingItems   |
| --snow-max-time-processing       | maxItemProcessingTime |
| --snow-avalanche-batch-size      | `batchSize`           |
| --snow-avalanche-num-parents     | `parentSize`          |

### Gossip Configs

It's possible to define different Gossip configurations for each Supernet without
changing values for Primary Network. JSON keys of these
parameters are different from their matching `CLI` keys. These parameters
default to the same values used for the Primary Network. For more information
see [CLI Gossip Configs](/nodes/configure/avalanchego-config-flags.md#gossiping).

| CLI Key                                                 | JSON Key                               |
| :------------------------------------------------------ | :------------------------------------- |
| --consensus-accepted-frontier-gossip-validator-size     | gossipAcceptedFrontierValidatorSize    |
| --consensus-accepted-frontier-gossip-non-validator-size | gossipAcceptedFrontierNonValidatorSize |
| --consensus-accepted-frontier-gossip-peer-size          | gossipAcceptedFrontierPeerSize         |
| --consensus-on-accept-gossip-validator-size             | gossipOnAcceptValidatorSize            |
| --consensus-on-accept-gossip-non-validator-size         | gossipOnAcceptNonValidatorSize         |
| --consensus-on-accept-gossip-peer-size                  | gossipOnAcceptPeerSize                 |
