// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package config

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/Juneo-io/juneogo/api/server"
	"github.com/Juneo-io/juneogo/chains"
	"github.com/Juneo-io/juneogo/genesis"
	"github.com/Juneo-io/juneogo/ids"
	"github.com/Juneo-io/juneogo/network"
	"github.com/Juneo-io/juneogo/network/dialer"
	"github.com/Juneo-io/juneogo/network/throttling"
	"github.com/Juneo-io/juneogo/node"
	"github.com/Juneo-io/juneogo/snow/consensus/snowball"
	"github.com/Juneo-io/juneogo/snow/networking/benchlist"
	"github.com/Juneo-io/juneogo/snow/networking/router"
	"github.com/Juneo-io/juneogo/snow/networking/tracker"
	"github.com/Juneo-io/juneogo/staking"
	"github.com/Juneo-io/juneogo/supernets"
	"github.com/Juneo-io/juneogo/trace"
	"github.com/Juneo-io/juneogo/utils/compression"
	"github.com/Juneo-io/juneogo/utils/constants"
	"github.com/Juneo-io/juneogo/utils/crypto/bls"
	"github.com/Juneo-io/juneogo/utils/ips"
	"github.com/Juneo-io/juneogo/utils/logging"
	"github.com/Juneo-io/juneogo/utils/perms"
	"github.com/Juneo-io/juneogo/utils/profiler"
	"github.com/Juneo-io/juneogo/utils/set"
	"github.com/Juneo-io/juneogo/utils/storage"
	"github.com/Juneo-io/juneogo/utils/timer"
	"github.com/Juneo-io/juneogo/version"
	"github.com/Juneo-io/juneogo/vms/platformvm/reward"
	"github.com/Juneo-io/juneogo/vms/proposervm"
)

const (
	chainConfigFileName  = "config"
	chainUpgradeFileName = "upgrade"
	supernetConfigFileExt  = ".json"

	keystoreDeprecationMsg = "keystore API is deprecated"
)

var (
	// Deprecated key --> deprecation message (i.e. which key replaces it)
	// TODO: deprecate "BootstrapIDsKey" and "BootstrapIPsKey"
	deprecatedKeys = map[string]string{
		KeystoreAPIEnabledKey: keystoreDeprecationMsg,
	}

	errConflictingACPOpinion                  = errors.New("supporting and objecting to the same ACP")
	errConflictingImplicitACPOpinion          = errors.New("objecting to enabled ACP")
	errSybilProtectionDisabledStakerWeights   = errors.New("sybil protection disabled weights must be positive")
	errSybilProtectionDisabledOnPublicNetwork = errors.New("sybil protection disabled on public network")
	errInvalidUptimeRequirement               = errors.New("uptime requirement must be in the range [0, 1]")
	errMinValidatorStakeAboveMax              = errors.New("minimum validator stake can't be greater than maximum validator stake")
	errInvalidMinDelegationFee                = errors.New("min delegation fee must be in the range [0, MaxDelegationFee]")
	errInvalidMaxDelegationFee                = errors.New("max delegation fee must be in the range [MinDelegationFee, 1,000,000]")
	errInvalidMinStakeDuration                = errors.New("min stake duration must be > 0")
	errMinStakeDurationAboveMax               = errors.New("max stake duration can't be less than min stake duration")
	errStakePeriodRewardShareZero             = errors.New("stake period reward share must be non-0")
	errStakePeriodRewardShareTooLarge         = fmt.Errorf("stake period reward share must be less than or equal to %d", reward.PercentDenominator)
	errStartRewardShareTooLarge               = fmt.Errorf("start reward share must be less than or equal to %d", reward.PercentDenominator)
	errStartRewardTimeZero                    = errors.New("start reward time must be non-0")
	errStartRewardTimeTooLarge                = fmt.Errorf("start reward time must be less than or equal to target reward time")
	errTargetRewardShareZero                  = errors.New("target reward share must be non-0")
	errTargetRewardShareTooLarge              = fmt.Errorf("target reward share must be less than or equal to start reward share")
	errStakeMintingPeriodBelowMin             = errors.New("stake minting period can't be less than max stake duration")
	errCannotTrackPrimaryNetwork              = errors.New("cannot track primary network")
	errStakingKeyContentUnset                 = fmt.Errorf("%s key not set but %s set", StakingTLSKeyContentKey, StakingCertContentKey)
	errStakingCertContentUnset                = fmt.Errorf("%s key set but %s not set", StakingTLSKeyContentKey, StakingCertContentKey)
	errMissingStakingSigningKeyFile           = errors.New("missing staking signing key file")
	errTracingEndpointEmpty                   = fmt.Errorf("%s cannot be empty", TracingEndpointKey)
	errPluginDirNotADirectory                 = errors.New("plugin dir is not a directory")
	errCannotReadDirectory                    = errors.New("cannot read directory")
	errUnmarshalling                          = errors.New("unmarshalling failed")
	errFileDoesNotExist                       = errors.New("file does not exist")
)

func getConsensusConfig(v *viper.Viper) snowball.Parameters {
	p := snowball.Parameters{
		K:                     v.GetInt(SnowSampleSizeKey),
		AlphaPreference:       v.GetInt(SnowPreferenceQuorumSizeKey),
		AlphaConfidence:       v.GetInt(SnowConfidenceQuorumSizeKey),
		Beta:                  v.GetInt(SnowCommitThresholdKey),
		ConcurrentRepolls:     v.GetInt(SnowConcurrentRepollsKey),
		OptimalProcessing:     v.GetInt(SnowOptimalProcessingKey),
		MaxOutstandingItems:   v.GetInt(SnowMaxProcessingKey),
		MaxItemProcessingTime: v.GetDuration(SnowMaxTimeProcessingKey),
	}
	if v.IsSet(SnowQuorumSizeKey) {
		p.AlphaPreference = v.GetInt(SnowQuorumSizeKey)
		p.AlphaConfidence = p.AlphaPreference
	}
	return p
}

func getLoggingConfig(v *viper.Viper) (logging.Config, error) {
	loggingConfig := logging.Config{}
	loggingConfig.Directory = GetExpandedArg(v, LogsDirKey)
	var err error
	loggingConfig.LogLevel, err = logging.ToLevel(v.GetString(LogLevelKey))
	if err != nil {
		return loggingConfig, err
	}
	logDisplayLevel := v.GetString(LogLevelKey)
	if v.IsSet(LogDisplayLevelKey) {
		logDisplayLevel = v.GetString(LogDisplayLevelKey)
	}
	loggingConfig.DisplayLevel, err = logging.ToLevel(logDisplayLevel)
	if err != nil {
		return loggingConfig, err
	}
	loggingConfig.LogFormat, err = logging.ToFormat(v.GetString(LogFormatKey), os.Stdout.Fd())
	loggingConfig.DisableWriterDisplaying = v.GetBool(LogDisableDisplayPluginLogsKey)
	loggingConfig.MaxSize = int(v.GetUint(LogRotaterMaxSizeKey))
	loggingConfig.MaxFiles = int(v.GetUint(LogRotaterMaxFilesKey))
	loggingConfig.MaxAge = int(v.GetUint(LogRotaterMaxAgeKey))
	loggingConfig.Compress = v.GetBool(LogRotaterCompressEnabledKey)

	return loggingConfig, err
}

func getHTTPConfig(v *viper.Viper) (node.HTTPConfig, error) {
	var (
		httpsKey  []byte
		httpsCert []byte
		err       error
	)
	switch {
	case v.IsSet(HTTPSKeyContentKey):
		rawContent := v.GetString(HTTPSKeyContentKey)
		httpsKey, err = base64.StdEncoding.DecodeString(rawContent)
		if err != nil {
			return node.HTTPConfig{}, fmt.Errorf("unable to decode base64 content: %w", err)
		}
	case v.IsSet(HTTPSKeyFileKey):
		httpsKeyFilepath := GetExpandedArg(v, HTTPSKeyFileKey)
		httpsKey, err = os.ReadFile(filepath.Clean(httpsKeyFilepath))
		if err != nil {
			return node.HTTPConfig{}, err
		}
	}

	switch {
	case v.IsSet(HTTPSCertContentKey):
		rawContent := v.GetString(HTTPSCertContentKey)
		httpsCert, err = base64.StdEncoding.DecodeString(rawContent)
		if err != nil {
			return node.HTTPConfig{}, fmt.Errorf("unable to decode base64 content: %w", err)
		}
	case v.IsSet(HTTPSCertFileKey):
		httpsCertFilepath := GetExpandedArg(v, HTTPSCertFileKey)
		httpsCert, err = os.ReadFile(filepath.Clean(httpsCertFilepath))
		if err != nil {
			return node.HTTPConfig{}, err
		}
	}

	return node.HTTPConfig{
		HTTPConfig: server.HTTPConfig{
			ReadTimeout:       v.GetDuration(HTTPReadTimeoutKey),
			ReadHeaderTimeout: v.GetDuration(HTTPReadHeaderTimeoutKey),
			WriteTimeout:      v.GetDuration(HTTPWriteTimeoutKey),
			IdleTimeout:       v.GetDuration(HTTPIdleTimeoutKey),
		},
		APIConfig: node.APIConfig{
			APIIndexerConfig: node.APIIndexerConfig{
				IndexAPIEnabled:      v.GetBool(IndexEnabledKey),
				IndexAllowIncomplete: v.GetBool(IndexAllowIncompleteKey),
			},
			AdminAPIEnabled:    v.GetBool(AdminAPIEnabledKey),
			InfoAPIEnabled:     v.GetBool(InfoAPIEnabledKey),
			KeystoreAPIEnabled: v.GetBool(KeystoreAPIEnabledKey),
			MetricsAPIEnabled:  v.GetBool(MetricsAPIEnabledKey),
			HealthAPIEnabled:   v.GetBool(HealthAPIEnabledKey),
		},
		HTTPHost:           v.GetString(HTTPHostKey),
		HTTPPort:           uint16(v.GetUint(HTTPPortKey)),
		HTTPSEnabled:       v.GetBool(HTTPSEnabledKey),
		HTTPSKey:           httpsKey,
		HTTPSCert:          httpsCert,
		HTTPAllowedOrigins: v.GetStringSlice(HTTPAllowedOrigins),
		HTTPAllowedHosts:   v.GetStringSlice(HTTPAllowedHostsKey),
		ShutdownTimeout:    v.GetDuration(HTTPShutdownTimeoutKey),
		ShutdownWait:       v.GetDuration(HTTPShutdownWaitKey),
	}, nil
}

func getRouterHealthConfig(v *viper.Viper, halflife time.Duration) (router.HealthConfig, error) {
	config := router.HealthConfig{
		MaxDropRate:            v.GetFloat64(RouterHealthMaxDropRateKey),
		MaxOutstandingRequests: int(v.GetUint(RouterHealthMaxOutstandingRequestsKey)),
		MaxOutstandingDuration: v.GetDuration(NetworkHealthMaxOutstandingDurationKey),
		MaxRunTimeRequests:     v.GetDuration(NetworkMaximumTimeoutKey),
		MaxDropRateHalflife:    halflife,
	}
	switch {
	case config.MaxDropRate < 0 || config.MaxDropRate > 1:
		return router.HealthConfig{}, fmt.Errorf("%q must be in [0,1]", RouterHealthMaxDropRateKey)
	case config.MaxOutstandingDuration <= 0:
		return router.HealthConfig{}, fmt.Errorf("%q must be positive", NetworkHealthMaxOutstandingDurationKey)
	case config.MaxRunTimeRequests <= 0:
		return router.HealthConfig{}, fmt.Errorf("%q must be positive", NetworkMaximumTimeoutKey)
	}
	return config, nil
}

func getAdaptiveTimeoutConfig(v *viper.Viper) (timer.AdaptiveTimeoutConfig, error) {
	config := timer.AdaptiveTimeoutConfig{
		InitialTimeout:     v.GetDuration(NetworkInitialTimeoutKey),
		MinimumTimeout:     v.GetDuration(NetworkMinimumTimeoutKey),
		MaximumTimeout:     v.GetDuration(NetworkMaximumTimeoutKey),
		TimeoutHalflife:    v.GetDuration(NetworkTimeoutHalflifeKey),
		TimeoutCoefficient: v.GetFloat64(NetworkTimeoutCoefficientKey),
	}
	switch {
	case config.MinimumTimeout < 1:
		return timer.AdaptiveTimeoutConfig{}, fmt.Errorf("%q must be positive", NetworkMinimumTimeoutKey)
	case config.MinimumTimeout > config.MaximumTimeout:
		return timer.AdaptiveTimeoutConfig{}, fmt.Errorf("%q must be >= %q", NetworkMaximumTimeoutKey, NetworkMinimumTimeoutKey)
	case config.InitialTimeout < config.MinimumTimeout || config.InitialTimeout > config.MaximumTimeout:
		return timer.AdaptiveTimeoutConfig{}, fmt.Errorf("%q must be in [%q, %q]", NetworkInitialTimeoutKey, NetworkMinimumTimeoutKey, NetworkMaximumTimeoutKey)
	case config.TimeoutHalflife <= 0:
		return timer.AdaptiveTimeoutConfig{}, fmt.Errorf("%q must > 0", NetworkTimeoutHalflifeKey)
	case config.TimeoutCoefficient < 1:
		return timer.AdaptiveTimeoutConfig{}, fmt.Errorf("%q must be >= 1", NetworkTimeoutCoefficientKey)
	}

	return config, nil
}

func getNetworkConfig(
	v *viper.Viper,
	networkID uint32,
	sybilProtectionEnabled bool,
	halflife time.Duration,
) (network.Config, error) {
	// Set the max number of recent inbound connections upgraded to be
	// equal to the max number of inbound connections per second.
	maxInboundConnsPerSec := v.GetFloat64(NetworkInboundThrottlerMaxConnsPerSecKey)
	upgradeCooldown := v.GetDuration(NetworkInboundConnUpgradeThrottlerCooldownKey)
	upgradeCooldownInSeconds := upgradeCooldown.Seconds()
	maxRecentConnsUpgraded := int(math.Ceil(maxInboundConnsPerSec * upgradeCooldownInSeconds))

	compressionType, err := compression.TypeFromString(v.GetString(NetworkCompressionTypeKey))
	if err != nil {
		return network.Config{}, err
	}

	allowPrivateIPs := !constants.ProductionNetworkIDs.Contains(networkID)
	if v.IsSet(NetworkAllowPrivateIPsKey) {
		allowPrivateIPs = v.GetBool(NetworkAllowPrivateIPsKey)
	}

	var supportedACPs set.Set[uint32]
	for _, acp := range v.GetIntSlice(ACPSupportKey) {
		if acp < 0 || acp > math.MaxInt32 {
			return network.Config{}, fmt.Errorf("invalid ACP: %d", acp)
		}
		supportedACPs.Add(uint32(acp))
	}

	var objectedACPs set.Set[uint32]
	for _, acp := range v.GetIntSlice(ACPObjectKey) {
		if acp < 0 || acp > math.MaxInt32 {
			return network.Config{}, fmt.Errorf("invalid ACP: %d", acp)
		}
		objectedACPs.Add(uint32(acp))
	}
	if supportedACPs.Overlaps(objectedACPs) {
		return network.Config{}, errConflictingACPOpinion
	}
	if constants.ScheduledACPs.Overlaps(objectedACPs) {
		return network.Config{}, errConflictingImplicitACPOpinion
	}

	// Because this node version has scheduled these ACPs, we should notify
	// peers that we support these upgrades.
	supportedACPs.Union(constants.ScheduledACPs)

	config := network.Config{
		ThrottlerConfig: network.ThrottlerConfig{
			MaxInboundConnsPerSec: maxInboundConnsPerSec,
			InboundConnUpgradeThrottlerConfig: throttling.InboundConnUpgradeThrottlerConfig{
				UpgradeCooldown:        upgradeCooldown,
				MaxRecentConnsUpgraded: maxRecentConnsUpgraded,
			},

			InboundMsgThrottlerConfig: throttling.InboundMsgThrottlerConfig{
				MsgByteThrottlerConfig: throttling.MsgByteThrottlerConfig{
					AtLargeAllocSize:    v.GetUint64(InboundThrottlerAtLargeAllocSizeKey),
					VdrAllocSize:        v.GetUint64(InboundThrottlerVdrAllocSizeKey),
					NodeMaxAtLargeBytes: v.GetUint64(InboundThrottlerNodeMaxAtLargeBytesKey),
				},
				BandwidthThrottlerConfig: throttling.BandwidthThrottlerConfig{
					RefillRate:   v.GetUint64(InboundThrottlerBandwidthRefillRateKey),
					MaxBurstSize: v.GetUint64(InboundThrottlerBandwidthMaxBurstSizeKey),
				},
				MaxProcessingMsgsPerNode: v.GetUint64(InboundThrottlerMaxProcessingMsgsPerNodeKey),
				CPUThrottlerConfig: throttling.SystemThrottlerConfig{
					MaxRecheckDelay: v.GetDuration(InboundThrottlerCPUMaxRecheckDelayKey),
				},
				DiskThrottlerConfig: throttling.SystemThrottlerConfig{
					MaxRecheckDelay: v.GetDuration(InboundThrottlerDiskMaxRecheckDelayKey),
				},
			},

			OutboundMsgThrottlerConfig: throttling.MsgByteThrottlerConfig{
				AtLargeAllocSize:    v.GetUint64(OutboundThrottlerAtLargeAllocSizeKey),
				VdrAllocSize:        v.GetUint64(OutboundThrottlerVdrAllocSizeKey),
				NodeMaxAtLargeBytes: v.GetUint64(OutboundThrottlerNodeMaxAtLargeBytesKey),
			},
		},

		HealthConfig: network.HealthConfig{
			Enabled:                      sybilProtectionEnabled,
			MaxTimeSinceMsgSent:          v.GetDuration(NetworkHealthMaxTimeSinceMsgSentKey),
			MaxTimeSinceMsgReceived:      v.GetDuration(NetworkHealthMaxTimeSinceMsgReceivedKey),
			MaxPortionSendQueueBytesFull: v.GetFloat64(NetworkHealthMaxPortionSendQueueFillKey),
			MinConnectedPeers:            v.GetUint(NetworkHealthMinPeersKey),
			MaxSendFailRate:              v.GetFloat64(NetworkHealthMaxSendFailRateKey),
			SendFailRateHalflife:         halflife,
		},

		ProxyEnabled:           v.GetBool(NetworkTCPProxyEnabledKey),
		ProxyReadHeaderTimeout: v.GetDuration(NetworkTCPProxyReadTimeoutKey),

		DialerConfig: dialer.Config{
			ThrottleRps:       v.GetUint32(NetworkOutboundConnectionThrottlingRpsKey),
			ConnectionTimeout: v.GetDuration(NetworkOutboundConnectionTimeoutKey),
		},

		TLSKeyLogFile: v.GetString(NetworkTLSKeyLogFileKey),

		TimeoutConfig: network.TimeoutConfig{
			PingPongTimeout:      v.GetDuration(NetworkPingTimeoutKey),
			ReadHandshakeTimeout: v.GetDuration(NetworkReadHandshakeTimeoutKey),
		},

		PeerListGossipConfig: network.PeerListGossipConfig{
			PeerListNumValidatorIPs: v.GetUint32(NetworkPeerListNumValidatorIPsKey),
			PeerListPullGossipFreq:  v.GetDuration(NetworkPeerListPullGossipFreqKey),
			PeerListBloomResetFreq:  v.GetDuration(NetworkPeerListBloomResetFreqKey),
		},

		DelayConfig: network.DelayConfig{
			MaxReconnectDelay:     v.GetDuration(NetworkMaxReconnectDelayKey),
			InitialReconnectDelay: v.GetDuration(NetworkInitialReconnectDelayKey),
		},

		MaxClockDifference:           v.GetDuration(NetworkMaxClockDifferenceKey),
		CompressionType:              compressionType,
		PingFrequency:                v.GetDuration(NetworkPingFrequencyKey),
		AllowPrivateIPs:              allowPrivateIPs,
		UptimeMetricFreq:             v.GetDuration(UptimeMetricFreqKey),
		MaximumInboundMessageTimeout: v.GetDuration(NetworkMaximumInboundTimeoutKey),

		SupportedACPs: supportedACPs,
		ObjectedACPs:  objectedACPs,

		RequireValidatorToConnect: v.GetBool(NetworkRequireValidatorToConnectKey),
		PeerReadBufferSize:        int(v.GetUint(NetworkPeerReadBufferSizeKey)),
		PeerWriteBufferSize:       int(v.GetUint(NetworkPeerWriteBufferSizeKey)),
	}

	switch {
	case config.HealthConfig.MaxTimeSinceMsgSent < 0:
		return network.Config{}, fmt.Errorf("%s must be >= 0", NetworkHealthMaxTimeSinceMsgSentKey)
	case config.HealthConfig.MaxTimeSinceMsgReceived < 0:
		return network.Config{}, fmt.Errorf("%s must be >= 0", NetworkHealthMaxTimeSinceMsgReceivedKey)
	case config.HealthConfig.MaxSendFailRate < 0 || config.HealthConfig.MaxSendFailRate > 1:
		return network.Config{}, fmt.Errorf("%s must be in [0,1]", NetworkHealthMaxSendFailRateKey)
	case config.HealthConfig.MaxPortionSendQueueBytesFull < 0 || config.HealthConfig.MaxPortionSendQueueBytesFull > 1:
		return network.Config{}, fmt.Errorf("%s must be in [0,1]", NetworkHealthMaxPortionSendQueueFillKey)
	case config.DialerConfig.ConnectionTimeout < 0:
		return network.Config{}, fmt.Errorf("%q must be >= 0", NetworkOutboundConnectionTimeoutKey)
	case config.PeerListPullGossipFreq < 0:
		return network.Config{}, fmt.Errorf("%s must be >= 0", NetworkPeerListPullGossipFreqKey)
	case config.PeerListBloomResetFreq < 0:
		return network.Config{}, fmt.Errorf("%s must be >= 0", NetworkPeerListBloomResetFreqKey)
	case config.ThrottlerConfig.InboundMsgThrottlerConfig.CPUThrottlerConfig.MaxRecheckDelay < constants.MinInboundThrottlerMaxRecheckDelay:
		return network.Config{}, fmt.Errorf("%s must be >= %d", InboundThrottlerCPUMaxRecheckDelayKey, constants.MinInboundThrottlerMaxRecheckDelay)
	case config.ThrottlerConfig.InboundMsgThrottlerConfig.DiskThrottlerConfig.MaxRecheckDelay < constants.MinInboundThrottlerMaxRecheckDelay:
		return network.Config{}, fmt.Errorf("%s must be >= %d", InboundThrottlerDiskMaxRecheckDelayKey, constants.MinInboundThrottlerMaxRecheckDelay)
	case config.MaxReconnectDelay < 0:
		return network.Config{}, fmt.Errorf("%s must be >= 0", NetworkMaxReconnectDelayKey)
	case config.InitialReconnectDelay < 0:
		return network.Config{}, fmt.Errorf("%s must be >= 0", NetworkInitialReconnectDelayKey)
	case config.MaxReconnectDelay < config.InitialReconnectDelay:
		return network.Config{}, fmt.Errorf("%s must be >= %s", NetworkMaxReconnectDelayKey, NetworkInitialReconnectDelayKey)
	case config.PingPongTimeout < 0:
		return network.Config{}, fmt.Errorf("%s must be >= 0", NetworkPingTimeoutKey)
	case config.PingFrequency < 0:
		return network.Config{}, fmt.Errorf("%s must be >= 0", NetworkPingFrequencyKey)
	case config.PingPongTimeout <= config.PingFrequency:
		return network.Config{}, fmt.Errorf("%s must be > %s", NetworkPingTimeoutKey, NetworkPingFrequencyKey)
	case config.ReadHandshakeTimeout < 0:
		return network.Config{}, fmt.Errorf("%s must be >= 0", NetworkReadHandshakeTimeoutKey)
	case config.MaxClockDifference < 0:
		return network.Config{}, fmt.Errorf("%s must be >= 0", NetworkMaxClockDifferenceKey)
	}
	return config, nil
}

func getBenchlistConfig(v *viper.Viper, consensusParameters snowball.Parameters) (benchlist.Config, error) {
	// AlphaConfidence is used here to ensure that benching can't cause a
	// liveness failure. If AlphaPreference were used, the benchlist may grow to
	// a point that committing would be extremely unlikely to happen.
	alpha := consensusParameters.AlphaConfidence
	k := consensusParameters.K
	config := benchlist.Config{
		Threshold:              v.GetInt(BenchlistFailThresholdKey),
		Duration:               v.GetDuration(BenchlistDurationKey),
		MinimumFailingDuration: v.GetDuration(BenchlistMinFailingDurationKey),
		MaxPortion:             (1.0 - (float64(alpha) / float64(k))) / 3.0,
	}
	switch {
	case config.Duration < 0:
		return benchlist.Config{}, fmt.Errorf("%q must be >= 0", BenchlistDurationKey)
	case config.MinimumFailingDuration < 0:
		return benchlist.Config{}, fmt.Errorf("%q must be >= 0", BenchlistMinFailingDurationKey)
	}
	return config, nil
}

func getStateSyncConfig(v *viper.Viper) (node.StateSyncConfig, error) {
	var (
		config       = node.StateSyncConfig{}
		stateSyncIPs = strings.Split(v.GetString(StateSyncIPsKey), ",")
		stateSyncIDs = strings.Split(v.GetString(StateSyncIDsKey), ",")
	)

	for _, ip := range stateSyncIPs {
		if ip == "" {
			continue
		}
		addr, err := ips.ToIPPort(ip)
		if err != nil {
			return node.StateSyncConfig{}, fmt.Errorf("couldn't parse state sync ip %s: %w", ip, err)
		}
		config.StateSyncIPs = append(config.StateSyncIPs, addr)
	}

	for _, id := range stateSyncIDs {
		if id == "" {
			continue
		}
		nodeID, err := ids.NodeIDFromString(id)
		if err != nil {
			return node.StateSyncConfig{}, fmt.Errorf("couldn't parse state sync peer id %s: %w", id, err)
		}
		config.StateSyncIDs = append(config.StateSyncIDs, nodeID)
	}

	lenIPs := len(config.StateSyncIPs)
	lenIDs := len(config.StateSyncIDs)
	if lenIPs != lenIDs {
		return node.StateSyncConfig{}, fmt.Errorf("expected the number of stateSyncIPs (%d) to match the number of stateSyncIDs (%d)", lenIPs, lenIDs)
	}

	return config, nil
}

func getBootstrapConfig(v *viper.Viper, networkID uint32) (node.BootstrapConfig, error) {
	config := node.BootstrapConfig{
		BootstrapBeaconConnectionTimeout:        v.GetDuration(BootstrapBeaconConnectionTimeoutKey),
		BootstrapMaxTimeGetAncestors:            v.GetDuration(BootstrapMaxTimeGetAncestorsKey),
		BootstrapAncestorsMaxContainersSent:     int(v.GetUint(BootstrapAncestorsMaxContainersSentKey)),
		BootstrapAncestorsMaxContainersReceived: int(v.GetUint(BootstrapAncestorsMaxContainersReceivedKey)),
	}

	// TODO: Add a "BootstrappersKey" flag to more clearly enforce ID and IP
	// length equality.
	ipsSet := v.IsSet(BootstrapIPsKey)
	idsSet := v.IsSet(BootstrapIDsKey)
	if ipsSet && !idsSet {
		return node.BootstrapConfig{}, fmt.Errorf("set %q but didn't set %q", BootstrapIPsKey, BootstrapIDsKey)
	}
	if !ipsSet && idsSet {
		return node.BootstrapConfig{}, fmt.Errorf("set %q but didn't set %q", BootstrapIDsKey, BootstrapIPsKey)
	}
	if !ipsSet && !idsSet {
		config.Bootstrappers = genesis.SampleBootstrappers(networkID, 5)
		return config, nil
	}

	bootstrapIPs := strings.Split(v.GetString(BootstrapIPsKey), ",")
	config.Bootstrappers = make([]genesis.Bootstrapper, 0, len(bootstrapIPs))
	for _, bootstrapIP := range bootstrapIPs {
		ip := strings.TrimSpace(bootstrapIP)
		if ip == "" {
			continue
		}

		addr, err := ips.ToIPPort(ip)
		if err != nil {
			return node.BootstrapConfig{}, fmt.Errorf("couldn't parse bootstrap ip %s: %w", ip, err)
		}
		config.Bootstrappers = append(config.Bootstrappers, genesis.Bootstrapper{
			// ID is populated below
			IP: ips.IPDesc(addr),
		})
	}

	bootstrapIDs := strings.Split(v.GetString(BootstrapIDsKey), ",")
	bootstrapNodeIDs := make([]ids.NodeID, 0, len(bootstrapIDs))
	for _, bootstrapID := range bootstrapIDs {
		id := strings.TrimSpace(bootstrapID)
		if id == "" {
			continue
		}

		nodeID, err := ids.NodeIDFromString(id)
		if err != nil {
			return node.BootstrapConfig{}, fmt.Errorf("couldn't parse bootstrap peer id %s: %w", id, err)
		}
		bootstrapNodeIDs = append(bootstrapNodeIDs, nodeID)
	}

	if len(config.Bootstrappers) != len(bootstrapNodeIDs) {
		return node.BootstrapConfig{}, fmt.Errorf("expected the number of bootstrapIPs (%d) to match the number of bootstrapIDs (%d)", len(config.Bootstrappers), len(bootstrapNodeIDs))
	}
	for i, nodeID := range bootstrapNodeIDs {
		config.Bootstrappers[i].ID = nodeID
	}

	return config, nil
}

func getIPConfig(v *viper.Viper) (node.IPConfig, error) {
	ipConfig := node.IPConfig{
		PublicIP:                  v.GetString(PublicIPKey),
		PublicIPResolutionService: v.GetString(PublicIPResolutionServiceKey),
		PublicIPResolutionFreq:    v.GetDuration(PublicIPResolutionFreqKey),
		ListenHost:                v.GetString(StakingHostKey),
		ListenPort:                uint16(v.GetUint(StakingPortKey)),
	}
	if ipConfig.PublicIPResolutionFreq <= 0 {
		return node.IPConfig{}, fmt.Errorf("%q must be > 0", PublicIPResolutionFreqKey)
	}
	if ipConfig.PublicIP != "" && ipConfig.PublicIPResolutionService != "" {
		return node.IPConfig{}, fmt.Errorf("only one of --%s and --%s can be given", PublicIPKey, PublicIPResolutionServiceKey)
	}
	return ipConfig, nil
}

func getProfilerConfig(v *viper.Viper) (profiler.Config, error) {
	config := profiler.Config{
		Dir:         GetExpandedArg(v, ProfileDirKey),
		Enabled:     v.GetBool(ProfileContinuousEnabledKey),
		Freq:        v.GetDuration(ProfileContinuousFreqKey),
		MaxNumFiles: v.GetInt(ProfileContinuousMaxFilesKey),
	}
	if config.Freq < 0 {
		return profiler.Config{}, fmt.Errorf("%s must be >= 0", ProfileContinuousFreqKey)
	}
	return config, nil
}

func getStakingTLSCertFromFlag(v *viper.Viper) (tls.Certificate, error) {
	stakingKeyRawContent := v.GetString(StakingTLSKeyContentKey)
	stakingKeyContent, err := base64.StdEncoding.DecodeString(stakingKeyRawContent)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("unable to decode base64 content: %w", err)
	}

	stakingCertRawContent := v.GetString(StakingCertContentKey)
	stakingCertContent, err := base64.StdEncoding.DecodeString(stakingCertRawContent)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("unable to decode base64 content: %w", err)
	}

	cert, err := staking.LoadTLSCertFromBytes(stakingKeyContent, stakingCertContent)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed creating cert: %w", err)
	}

	return *cert, nil
}

func getStakingTLSCertFromFile(v *viper.Viper) (tls.Certificate, error) {
	// Parse the staking key/cert paths and expand environment variables
	stakingKeyPath := GetExpandedArg(v, StakingTLSKeyPathKey)
	stakingCertPath := GetExpandedArg(v, StakingCertPathKey)

	// If staking key/cert locations are specified but not found, error
	if v.IsSet(StakingTLSKeyPathKey) || v.IsSet(StakingCertPathKey) {
		if _, err := os.Stat(stakingKeyPath); os.IsNotExist(err) {
			return tls.Certificate{}, fmt.Errorf("couldn't find staking key at %s", stakingKeyPath)
		} else if _, err := os.Stat(stakingCertPath); os.IsNotExist(err) {
			return tls.Certificate{}, fmt.Errorf("couldn't find staking certificate at %s", stakingCertPath)
		}
	} else {
		// Create the staking key/cert if [stakingKeyPath] and [stakingCertPath] don't exist
		if err := staking.InitNodeStakingKeyPair(stakingKeyPath, stakingCertPath); err != nil {
			return tls.Certificate{}, fmt.Errorf("couldn't generate staking key/cert: %w", err)
		}
	}

	// Load and parse the staking key/cert
	cert, err := staking.LoadTLSCertFromFiles(stakingKeyPath, stakingCertPath)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("couldn't read staking certificate: %w", err)
	}
	return *cert, nil
}

func getStakingTLSCert(v *viper.Viper) (tls.Certificate, error) {
	if v.GetBool(StakingEphemeralCertEnabledKey) {
		// Use an ephemeral staking key/cert
		cert, err := staking.NewTLSCert()
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("couldn't generate ephemeral staking key/cert: %w", err)
		}
		return *cert, nil
	}

	switch {
	case v.IsSet(StakingTLSKeyContentKey) && !v.IsSet(StakingCertContentKey):
		return tls.Certificate{}, errStakingCertContentUnset
	case !v.IsSet(StakingTLSKeyContentKey) && v.IsSet(StakingCertContentKey):
		return tls.Certificate{}, errStakingKeyContentUnset
	case v.IsSet(StakingTLSKeyContentKey) && v.IsSet(StakingCertContentKey):
		return getStakingTLSCertFromFlag(v)
	default:
		return getStakingTLSCertFromFile(v)
	}
}

func getStakingSigner(v *viper.Viper) (*bls.SecretKey, error) {
	if v.GetBool(StakingEphemeralSignerEnabledKey) {
		key, err := bls.NewSecretKey()
		if err != nil {
			return nil, fmt.Errorf("couldn't generate ephemeral signing key: %w", err)
		}
		return key, nil
	}

	if v.IsSet(StakingSignerKeyContentKey) {
		signerKeyRawContent := v.GetString(StakingSignerKeyContentKey)
		signerKeyContent, err := base64.StdEncoding.DecodeString(signerKeyRawContent)
		if err != nil {
			return nil, fmt.Errorf("unable to decode base64 content: %w", err)
		}
		key, err := bls.SecretKeyFromBytes(signerKeyContent)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse signing key: %w", err)
		}
		return key, nil
	}

	signingKeyPath := GetExpandedArg(v, StakingSignerKeyPathKey)
	_, err := os.Stat(signingKeyPath)
	if !errors.Is(err, fs.ErrNotExist) {
		signingKeyBytes, err := os.ReadFile(signingKeyPath)
		if err != nil {
			return nil, err
		}
		key, err := bls.SecretKeyFromBytes(signingKeyBytes)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse signing key: %w", err)
		}
		return key, nil
	}

	if v.IsSet(StakingSignerKeyPathKey) {
		return nil, errMissingStakingSigningKeyFile
	}

	key, err := bls.NewSecretKey()
	if err != nil {
		return nil, fmt.Errorf("couldn't generate new signing key: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(signingKeyPath), perms.ReadWriteExecute); err != nil {
		return nil, fmt.Errorf("couldn't create path for signing key at %s: %w", signingKeyPath, err)
	}

	keyBytes := bls.SecretKeyToBytes(key)
	if err := os.WriteFile(signingKeyPath, keyBytes, perms.ReadWrite); err != nil {
		return nil, fmt.Errorf("couldn't write new signing key to %s: %w", signingKeyPath, err)
	}
	if err := os.Chmod(signingKeyPath, perms.ReadOnly); err != nil {
		return nil, fmt.Errorf("couldn't restrict permissions on new signing key at %s: %w", signingKeyPath, err)
	}
	return key, nil
}

func getStakingConfig(v *viper.Viper, networkID uint32) (node.StakingConfig, error) {
	config := node.StakingConfig{
		SybilProtectionEnabled:        v.GetBool(SybilProtectionEnabledKey),
		SybilProtectionDisabledWeight: v.GetUint64(SybilProtectionDisabledWeightKey),
		PartialSyncPrimaryNetwork:     v.GetBool(PartialSyncPrimaryNetworkKey),
		StakingKeyPath:                GetExpandedArg(v, StakingTLSKeyPathKey),
		StakingCertPath:               GetExpandedArg(v, StakingCertPathKey),
		StakingSignerPath:             GetExpandedArg(v, StakingSignerKeyPathKey),
	}
	if !config.SybilProtectionEnabled && config.SybilProtectionDisabledWeight == 0 {
		return node.StakingConfig{}, errSybilProtectionDisabledStakerWeights
	}

	if !config.SybilProtectionEnabled && (networkID == constants.MainnetID || networkID == constants.TestnetID) {
		return node.StakingConfig{}, errSybilProtectionDisabledOnPublicNetwork
	}

	var err error
	config.StakingTLSCert, err = getStakingTLSCert(v)
	if err != nil {
		return node.StakingConfig{}, err
	}
	config.StakingSigningKey, err = getStakingSigner(v)
	if err != nil {
		return node.StakingConfig{}, err
	}
	if networkID != constants.MainnetID && networkID != constants.TestnetID {
		config.UptimeRequirement = v.GetFloat64(UptimeRequirementKey)
		config.MinValidatorStake = v.GetUint64(MinValidatorStakeKey)
		config.MaxValidatorStake = v.GetUint64(MaxValidatorStakeKey)
		config.MinDelegatorStake = v.GetUint64(MinDelegatorStakeKey)
		config.MinStakeDuration = v.GetDuration(MinStakeDurationKey)
		config.MaxStakeDuration = v.GetDuration(MaxStakeDurationKey)
		config.RewardConfig.MinStakePeriod = v.GetDuration(MinStakeDurationKey)
		config.RewardConfig.MaxStakePeriod = v.GetDuration(MaxStakeDurationKey)
		config.RewardConfig.StakePeriodRewardShare = v.GetUint64(StakePeriodRewardShareKey)
		config.RewardConfig.StartRewardShare = v.GetUint64(StakeStartRewardShareKey)
		config.RewardConfig.StartRewardTime = v.GetUint64(StakeStartRewardTimeKey)
		config.RewardConfig.TargetRewardShare = v.GetUint64(StakeTargetRewardShareKey)
		config.RewardConfig.TargetRewardTime = v.GetUint64(StakeTargetRewardTimeKey)
		config.MinDelegationFee = v.GetUint32(MinDelegatorFeeKey)
		config.MaxDelegationFee = v.GetUint32(MaxDelegatorFeeKey)
		switch {
		case config.UptimeRequirement < 0 || config.UptimeRequirement > 1:
			return node.StakingConfig{}, errInvalidUptimeRequirement
		case config.MinValidatorStake > config.MaxValidatorStake:
			return node.StakingConfig{}, errMinValidatorStakeAboveMax
		case config.MaxDelegationFee > 1_000_000:
			return node.StakingConfig{}, errInvalidMaxDelegationFee
		case config.MinDelegationFee > config.MaxDelegationFee:
			return node.StakingConfig{}, errInvalidMinDelegationFee
		case config.MinStakeDuration <= 0:
			return node.StakingConfig{}, errInvalidMinStakeDuration
		case config.MaxStakeDuration < config.MinStakeDuration:
			return node.StakingConfig{}, errMinStakeDurationAboveMax
		case config.RewardConfig.StakePeriodRewardShare == 0:
			return node.StakingConfig{}, errStakePeriodRewardShareZero
		case config.RewardConfig.StakePeriodRewardShare > reward.PercentDenominator:
			return node.StakingConfig{}, errStakePeriodRewardShareTooLarge
		case config.RewardConfig.StartRewardShare > reward.PercentDenominator:
			return node.StakingConfig{}, errStartRewardShareTooLarge
		case config.RewardConfig.StartRewardTime == 0:
			return node.StakingConfig{}, errStartRewardTimeZero
		case config.RewardConfig.StartRewardTime > config.RewardConfig.TargetRewardTime:
			return node.StakingConfig{}, errStartRewardTimeTooLarge
		case config.RewardConfig.TargetRewardShare == 0:
			return node.StakingConfig{}, errTargetRewardShareZero
		case config.RewardConfig.TargetRewardShare > config.RewardConfig.StartRewardShare:
			return node.StakingConfig{}, errTargetRewardShareTooLarge
		}
	} else {
		config.StakingConfig = genesis.GetStakingConfig(networkID)
	}
	return config, nil
}

func getTxFeeConfig(v *viper.Viper, networkID uint32) genesis.TxFeeConfig {
	if networkID != constants.MainnetID && networkID != constants.TestnetID {
		return genesis.TxFeeConfig{
			TxFee:                         v.GetUint64(TxFeeKey),
			CreateAssetTxFee:              v.GetUint64(CreateAssetTxFeeKey),
			CreateSupernetTxFee:             v.GetUint64(CreateSupernetTxFeeKey),
			TransformSupernetTxFee:          v.GetUint64(TransformSupernetTxFeeKey),
			CreateBlockchainTxFee:         v.GetUint64(CreateBlockchainTxFeeKey),
			AddPrimaryNetworkValidatorFee: v.GetUint64(AddPrimaryNetworkValidatorFeeKey),
			AddPrimaryNetworkDelegatorFee: v.GetUint64(AddPrimaryNetworkDelegatorFeeKey),
			AddSupernetValidatorFee:         v.GetUint64(AddSupernetValidatorFeeKey),
			AddSupernetDelegatorFee:         v.GetUint64(AddSupernetDelegatorFeeKey),
		}
	}
	return genesis.GetTxFeeConfig(networkID)
}

func getGenesisData(v *viper.Viper, networkID uint32, stakingCfg *genesis.StakingConfig) ([]byte, ids.ID, error) {
	// try first loading genesis content directly from flag/env-var
	if v.IsSet(GenesisFileContentKey) {
		genesisData := v.GetString(GenesisFileContentKey)
		return genesis.FromFlag(networkID, genesisData, stakingCfg)
	}

	// if content is not specified go for the file
	if v.IsSet(GenesisFileKey) {
		genesisFileName := GetExpandedArg(v, GenesisFileKey)
		return genesis.FromFile(networkID, genesisFileName, stakingCfg)
	}

	// finally if file is not specified/readable go for the predefined config
	config := genesis.GetConfig(networkID)
	return genesis.FromConfig(config)
}

func getTrackedSupernets(v *viper.Viper) (set.Set[ids.ID], error) {
	trackSupernetsStr := v.GetString(TrackSupernetsKey)
	trackSupernetsStrs := strings.Split(trackSupernetsStr, ",")
	trackedSupernetIDs := set.NewSet[ids.ID](len(trackSupernetsStrs))
	for _, supernet := range trackSupernetsStrs {
		if supernet == "" {
			continue
		}
		supernetID, err := ids.FromString(supernet)
		if err != nil {
			return nil, fmt.Errorf("couldn't parse supernetID %q: %w", supernet, err)
		}
		if supernetID == constants.PrimaryNetworkID {
			return nil, errCannotTrackPrimaryNetwork
		}
		trackedSupernetIDs.Add(supernetID)
	}
	return trackedSupernetIDs, nil
}

func getDatabaseConfig(v *viper.Viper, networkID uint32) (node.DatabaseConfig, error) {
	var (
		configBytes []byte
		err         error
	)
	if v.IsSet(DBConfigContentKey) {
		dbConfigContent := v.GetString(DBConfigContentKey)
		configBytes, err = base64.StdEncoding.DecodeString(dbConfigContent)
		if err != nil {
			return node.DatabaseConfig{}, fmt.Errorf("unable to decode base64 content: %w", err)
		}
	} else if v.IsSet(DBConfigFileKey) {
		path := GetExpandedArg(v, DBConfigFileKey)
		configBytes, err = os.ReadFile(path)
		if err != nil {
			return node.DatabaseConfig{}, err
		}
	}

	return node.DatabaseConfig{
		Name:     v.GetString(DBTypeKey),
		ReadOnly: v.GetBool(DBReadOnlyKey),
		Path: filepath.Join(
			GetExpandedArg(v, DBPathKey),
			constants.NetworkName(networkID),
		),
		Config: configBytes,
	}, nil
}

func getAliases(v *viper.Viper, name string, contentKey string, fileKey string) (map[ids.ID][]string, error) {
	var fileBytes []byte
	if v.IsSet(contentKey) {
		var err error
		aliasFlagContent := v.GetString(contentKey)
		fileBytes, err = base64.StdEncoding.DecodeString(aliasFlagContent)
		if err != nil {
			return nil, fmt.Errorf("unable to decode base64 content for %s: %w", name, err)
		}
	} else {
		aliasFilePath := filepath.Clean(GetExpandedArg(v, fileKey))
		exists, err := storage.FileExists(aliasFilePath)
		if err != nil {
			return nil, err
		}

		if !exists {
			if v.IsSet(fileKey) {
				return nil, fmt.Errorf("%w: %s", errFileDoesNotExist, aliasFilePath)
			}
			return nil, nil
		}

		fileBytes, err = os.ReadFile(aliasFilePath)
		if err != nil {
			return nil, err
		}
	}

	aliasMap := make(map[ids.ID][]string)
	if err := json.Unmarshal(fileBytes, &aliasMap); err != nil {
		return nil, fmt.Errorf("%w on %s: %w", errUnmarshalling, name, err)
	}
	return aliasMap, nil
}

func getVMAliases(v *viper.Viper) (map[ids.ID][]string, error) {
	return getAliases(v, "vm aliases", VMAliasesContentKey, VMAliasesFileKey)
}

func getChainAliases(v *viper.Viper) (map[ids.ID][]string, error) {
	return getAliases(v, "chain aliases", ChainAliasesContentKey, ChainAliasesFileKey)
}

// getPathFromDirKey reads flag value from viper instance and then checks the folder existence
func getPathFromDirKey(v *viper.Viper, configKey string) (string, error) {
	configDir := GetExpandedArg(v, configKey)
	cleanPath := filepath.Clean(configDir)
	ok, err := storage.FolderExists(cleanPath)
	if err != nil {
		return "", err
	}
	if ok {
		return cleanPath, nil
	}
	if v.IsSet(configKey) {
		// user specified a config dir explicitly, but dir does not exist.
		return "", fmt.Errorf("%w: %s", errCannotReadDirectory, cleanPath)
	}
	return "", nil
}

func getChainConfigsFromFlag(v *viper.Viper) (map[string]chains.ChainConfig, error) {
	chainConfigContentB64 := v.GetString(ChainConfigContentKey)
	chainConfigContent, err := base64.StdEncoding.DecodeString(chainConfigContentB64)
	if err != nil {
		return nil, fmt.Errorf("unable to decode base64 content: %w", err)
	}

	chainConfigs := make(map[string]chains.ChainConfig)
	if err := json.Unmarshal(chainConfigContent, &chainConfigs); err != nil {
		return nil, fmt.Errorf("could not unmarshal JSON: %w", err)
	}
	return chainConfigs, nil
}

func getChainConfigsFromDir(v *viper.Viper) (map[string]chains.ChainConfig, error) {
	chainConfigPath, err := getPathFromDirKey(v, ChainConfigDirKey)
	if err != nil {
		return nil, err
	}

	if len(chainConfigPath) == 0 {
		return make(map[string]chains.ChainConfig), nil
	}

	return readChainConfigPath(chainConfigPath)
}

// getChainConfigs reads & puts chainConfigs to node config
func getChainConfigs(v *viper.Viper) (map[string]chains.ChainConfig, error) {
	if v.IsSet(ChainConfigContentKey) {
		return getChainConfigsFromFlag(v)
	}
	return getChainConfigsFromDir(v)
}

// readChainConfigPath reads chain config files from static directories and returns map with contents,
// if successful.
func readChainConfigPath(chainConfigPath string) (map[string]chains.ChainConfig, error) {
	chainDirs, err := filepath.Glob(filepath.Join(chainConfigPath, "*"))
	if err != nil {
		return nil, err
	}
	chainConfigMap := make(map[string]chains.ChainConfig)
	for _, chainDir := range chainDirs {
		dirInfo, err := os.Stat(chainDir)
		if err != nil {
			return nil, err
		}

		if !dirInfo.IsDir() {
			continue
		}

		// chainconfigdir/chainId/config.*
		configData, err := storage.ReadFileWithName(chainDir, chainConfigFileName)
		if err != nil {
			return chainConfigMap, err
		}

		// chainconfigdir/chainId/upgrade.*
		upgradeData, err := storage.ReadFileWithName(chainDir, chainUpgradeFileName)
		if err != nil {
			return chainConfigMap, err
		}

		chainConfigMap[dirInfo.Name()] = chains.ChainConfig{
			Config:  configData,
			Upgrade: upgradeData,
		}
	}
	return chainConfigMap, nil
}

// getSupernetConfigs reads supernet configs from the correct place
// (flag or file) and returns a non-nil map.
func getSupernetConfigs(v *viper.Viper, supernetIDs []ids.ID) (map[ids.ID]supernets.Config, error) {
	if v.IsSet(SupernetConfigContentKey) {
		return getSupernetConfigsFromFlags(v, supernetIDs)
	}
	return getSupernetConfigsFromDir(v, supernetIDs)
}

func getSupernetConfigsFromFlags(v *viper.Viper, supernetIDs []ids.ID) (map[ids.ID]supernets.Config, error) {
	supernetConfigContentB64 := v.GetString(SupernetConfigContentKey)
	supernetConfigContent, err := base64.StdEncoding.DecodeString(supernetConfigContentB64)
	if err != nil {
		return nil, fmt.Errorf("unable to decode base64 content: %w", err)
	}

	// partially parse configs to be filled by defaults later
	supernetConfigs := make(map[ids.ID]json.RawMessage, len(supernetIDs))
	if err := json.Unmarshal(supernetConfigContent, &supernetConfigs); err != nil {
		return nil, fmt.Errorf("could not unmarshal JSON: %w", err)
	}

	res := make(map[ids.ID]supernets.Config)
	for _, supernetID := range supernetIDs {
		if rawSupernetConfigBytes, ok := supernetConfigs[supernetID]; ok {
			config := getDefaultSupernetConfig(v)
			if err := json.Unmarshal(rawSupernetConfigBytes, &config); err != nil {
				return nil, err
			}

			if config.ConsensusParameters.Alpha != nil {
				config.ConsensusParameters.AlphaPreference = *config.ConsensusParameters.Alpha
				config.ConsensusParameters.AlphaConfidence = config.ConsensusParameters.AlphaPreference
			}

			if err := config.Valid(); err != nil {
				return nil, err
			}

			res[supernetID] = config
		}
	}
	return res, nil
}

// getSupernetConfigs reads SupernetConfigs to node config map
func getSupernetConfigsFromDir(v *viper.Viper, supernetIDs []ids.ID) (map[ids.ID]supernets.Config, error) {
	supernetConfigPath, err := getPathFromDirKey(v, SupernetConfigDirKey)
	if err != nil {
		return nil, err
	}

	supernetConfigs := make(map[ids.ID]supernets.Config)
	if len(supernetConfigPath) == 0 {
		// supernet config path does not exist but not explicitly specified, so ignore it
		return supernetConfigs, nil
	}

	// reads supernet config files from a path and given supernetIDs and returns a map.
	for _, supernetID := range supernetIDs {
		filePath := filepath.Join(supernetConfigPath, supernetID.String()+supernetConfigFileExt)
		fileInfo, err := os.Stat(filePath)
		switch {
		case errors.Is(err, os.ErrNotExist):
			// this supernet config does not exist, move to the next one
			continue
		case err != nil:
			return nil, err
		case fileInfo.IsDir():
			return nil, fmt.Errorf("%q is a directory, expected a file", fileInfo.Name())
		}

		// supernetConfigDir/supernetID.json
		file, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}

		config := getDefaultSupernetConfig(v)
		if err := json.Unmarshal(file, &config); err != nil {
			return nil, fmt.Errorf("%w: %w", errUnmarshalling, err)
		}

		if config.ConsensusParameters.Alpha != nil {
			config.ConsensusParameters.AlphaPreference = *config.ConsensusParameters.Alpha
			config.ConsensusParameters.AlphaConfidence = config.ConsensusParameters.AlphaPreference
		}

		if err := config.Valid(); err != nil {
			return nil, err
		}

		supernetConfigs[supernetID] = config
	}

	return supernetConfigs, nil
}

func getDefaultSupernetConfig(v *viper.Viper) supernets.Config {
	return supernets.Config{
		ConsensusParameters:         getConsensusConfig(v),
		ValidatorOnly:               false,
		ProposerMinBlockDelay:       proposervm.DefaultMinBlockDelay,
		ProposerNumHistoricalBlocks: proposervm.DefaultNumHistoricalBlocks,
	}
}

func getCPUTargeterConfig(v *viper.Viper) (tracker.TargeterConfig, error) {
	vdrAlloc := v.GetFloat64(CPUVdrAllocKey)
	maxNonVdrUsage := v.GetFloat64(CPUMaxNonVdrUsageKey)
	maxNonVdrNodeUsage := v.GetFloat64(CPUMaxNonVdrNodeUsageKey)
	switch {
	case vdrAlloc < 0:
		return tracker.TargeterConfig{}, fmt.Errorf("%q (%f) < 0", CPUVdrAllocKey, vdrAlloc)
	case maxNonVdrUsage < 0:
		return tracker.TargeterConfig{}, fmt.Errorf("%q (%f) < 0", CPUMaxNonVdrUsageKey, maxNonVdrUsage)
	case maxNonVdrNodeUsage < 0:
		return tracker.TargeterConfig{}, fmt.Errorf("%q (%f) < 0", CPUMaxNonVdrNodeUsageKey, maxNonVdrNodeUsage)
	default:
		return tracker.TargeterConfig{
			VdrAlloc:           vdrAlloc,
			MaxNonVdrUsage:     maxNonVdrUsage,
			MaxNonVdrNodeUsage: maxNonVdrNodeUsage,
		}, nil
	}
}

func getDiskSpaceConfig(v *viper.Viper) (requiredAvailableDiskSpace uint64, warningThresholdAvailableDiskSpace uint64, err error) {
	requiredAvailableDiskSpace = v.GetUint64(SystemTrackerRequiredAvailableDiskSpaceKey)
	warningThresholdAvailableDiskSpace = v.GetUint64(SystemTrackerWarningThresholdAvailableDiskSpaceKey)
	switch {
	case warningThresholdAvailableDiskSpace < requiredAvailableDiskSpace:
		return 0, 0, fmt.Errorf("%q (%d) < %q (%d)", SystemTrackerWarningThresholdAvailableDiskSpaceKey, warningThresholdAvailableDiskSpace, SystemTrackerRequiredAvailableDiskSpaceKey, requiredAvailableDiskSpace)
	default:
		return requiredAvailableDiskSpace, warningThresholdAvailableDiskSpace, nil
	}
}

func getDiskTargeterConfig(v *viper.Viper) (tracker.TargeterConfig, error) {
	vdrAlloc := v.GetFloat64(DiskVdrAllocKey)
	maxNonVdrUsage := v.GetFloat64(DiskMaxNonVdrUsageKey)
	maxNonVdrNodeUsage := v.GetFloat64(DiskMaxNonVdrNodeUsageKey)
	switch {
	case vdrAlloc < 0:
		return tracker.TargeterConfig{}, fmt.Errorf("%q (%f) < 0", DiskVdrAllocKey, vdrAlloc)
	case maxNonVdrUsage < 0:
		return tracker.TargeterConfig{}, fmt.Errorf("%q (%f) < 0", DiskMaxNonVdrUsageKey, maxNonVdrUsage)
	case maxNonVdrNodeUsage < 0:
		return tracker.TargeterConfig{}, fmt.Errorf("%q (%f) < 0", DiskMaxNonVdrNodeUsageKey, maxNonVdrNodeUsage)
	default:
		return tracker.TargeterConfig{
			VdrAlloc:           vdrAlloc,
			MaxNonVdrUsage:     maxNonVdrUsage,
			MaxNonVdrNodeUsage: maxNonVdrNodeUsage,
		}, nil
	}
}

func getTraceConfig(v *viper.Viper) (trace.Config, error) {
	enabled := v.GetBool(TracingEnabledKey)
	if !enabled {
		return trace.Config{
			Enabled: false,
		}, nil
	}

	exporterTypeStr := v.GetString(TracingExporterTypeKey)
	exporterType, err := trace.ExporterTypeFromString(exporterTypeStr)
	if err != nil {
		return trace.Config{}, err
	}

	endpoint := v.GetString(TracingEndpointKey)
	if endpoint == "" {
		return trace.Config{}, errTracingEndpointEmpty
	}

	return trace.Config{
		ExporterConfig: trace.ExporterConfig{
			Type:     exporterType,
			Endpoint: endpoint,
			Insecure: v.GetBool(TracingInsecureKey),
			Headers:  v.GetStringMapString(TracingHeadersKey),
		},
		Enabled:         true,
		TraceSampleRate: v.GetFloat64(TracingSampleRateKey),
		AppName:         constants.AppName,
		Version:         version.Current.String(),
	}, nil
}

// Returns the path to the directory that contains VM binaries.
func getPluginDir(v *viper.Viper) (string, error) {
	pluginDir := GetExpandedString(v, v.GetString(PluginDirKey))

	if v.IsSet(PluginDirKey) {
		// If the flag was given, assert it exists and is a directory
		info, err := os.Stat(pluginDir)
		if err != nil {
			return "", fmt.Errorf("plugin dir %q not found: %w", pluginDir, err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("%w: %q", errPluginDirNotADirectory, pluginDir)
		}
	} else {
		// If the flag wasn't given, make sure the default location exists.
		if err := os.MkdirAll(pluginDir, perms.ReadWriteExecute); err != nil {
			return "", fmt.Errorf("failed to create plugin dir at %s: %w", pluginDir, err)
		}
	}

	return pluginDir, nil
}

func GetNodeConfig(v *viper.Viper) (node.Config, error) {
	var (
		nodeConfig node.Config
		err        error
	)

	nodeConfig.PluginDir, err = getPluginDir(v)
	if err != nil {
		return node.Config{}, err
	}

	nodeConfig.ConsensusShutdownTimeout = v.GetDuration(ConsensusShutdownTimeoutKey)
	if nodeConfig.ConsensusShutdownTimeout < 0 {
		return node.Config{}, fmt.Errorf("%q must be >= 0", ConsensusShutdownTimeoutKey)
	}

	// Gossiping
	nodeConfig.FrontierPollFrequency = v.GetDuration(ConsensusFrontierPollFrequencyKey)
	if nodeConfig.FrontierPollFrequency < 0 {
		return node.Config{}, fmt.Errorf("%s must be >= 0", ConsensusFrontierPollFrequencyKey)
	}

	// App handling
	nodeConfig.ConsensusAppConcurrency = int(v.GetUint(ConsensusAppConcurrencyKey))
	if nodeConfig.ConsensusAppConcurrency <= 0 {
		return node.Config{}, fmt.Errorf("%s must be > 0", ConsensusAppConcurrencyKey)
	}

	nodeConfig.UseCurrentHeight = v.GetBool(ProposerVMUseCurrentHeightKey)

	// Logging
	nodeConfig.LoggingConfig, err = getLoggingConfig(v)
	if err != nil {
		return node.Config{}, err
	}

	// Network ID
	nodeConfig.NetworkID, err = constants.NetworkID(v.GetString(NetworkNameKey))
	if err != nil {
		return node.Config{}, err
	}

	// Database
	nodeConfig.DatabaseConfig, err = getDatabaseConfig(v, nodeConfig.NetworkID)
	if err != nil {
		return node.Config{}, err
	}

	// IP configuration
	nodeConfig.IPConfig, err = getIPConfig(v)
	if err != nil {
		return node.Config{}, err
	}

	// Staking
	nodeConfig.StakingConfig, err = getStakingConfig(v, nodeConfig.NetworkID)
	if err != nil {
		return node.Config{}, err
	}

	// Tracked Supernets
	nodeConfig.TrackedSupernets, err = getTrackedSupernets(v)
	if err != nil {
		return node.Config{}, err
	}

	// HTTP APIs
	nodeConfig.HTTPConfig, err = getHTTPConfig(v)
	if err != nil {
		return node.Config{}, err
	}

	// Health
	nodeConfig.HealthCheckFreq = v.GetDuration(HealthCheckFreqKey)
	if nodeConfig.HealthCheckFreq < 0 {
		return node.Config{}, fmt.Errorf("%s must be positive", HealthCheckFreqKey)
	}
	// Halflife of continuous averager used in health checks
	healthCheckAveragerHalflife := v.GetDuration(HealthCheckAveragerHalflifeKey)
	if healthCheckAveragerHalflife <= 0 {
		return node.Config{}, fmt.Errorf("%s must be positive", HealthCheckAveragerHalflifeKey)
	}

	// Router
	nodeConfig.RouterHealthConfig, err = getRouterHealthConfig(v, healthCheckAveragerHalflife)
	if err != nil {
		return node.Config{}, err
	}

	// Metrics
	nodeConfig.MeterVMEnabled = v.GetBool(MeterVMsEnabledKey)

	// Adaptive Timeout Config
	nodeConfig.AdaptiveTimeoutConfig, err = getAdaptiveTimeoutConfig(v)
	if err != nil {
		return node.Config{}, err
	}

	// Network Config
	nodeConfig.NetworkConfig, err = getNetworkConfig(
		v,
		nodeConfig.NetworkID,
		nodeConfig.SybilProtectionEnabled,
		healthCheckAveragerHalflife,
	)
	if err != nil {
		return node.Config{}, err
	}

	// Supernet Configs
	supernetConfigs, err := getSupernetConfigs(v, nodeConfig.TrackedSupernets.List())
	if err != nil {
		return node.Config{}, fmt.Errorf("couldn't read supernet configs: %w", err)
	}

	primaryNetworkConfig := getDefaultSupernetConfig(v)
	if err := primaryNetworkConfig.Valid(); err != nil {
		return node.Config{}, fmt.Errorf("invalid consensus parameters: %w", err)
	}
	supernetConfigs[constants.PrimaryNetworkID] = primaryNetworkConfig

	nodeConfig.SupernetConfigs = supernetConfigs

	// Benchlist
	nodeConfig.BenchlistConfig, err = getBenchlistConfig(v, primaryNetworkConfig.ConsensusParameters)
	if err != nil {
		return node.Config{}, err
	}

	// File Descriptor Limit
	nodeConfig.FdLimit = v.GetUint64(FdLimitKey)

	// Tx Fee
	nodeConfig.TxFeeConfig = getTxFeeConfig(v, nodeConfig.NetworkID)

	// Genesis Data
	genesisStakingCfg := nodeConfig.StakingConfig.StakingConfig
	nodeConfig.GenesisBytes, nodeConfig.AvaxAssetID, err = getGenesisData(v, nodeConfig.NetworkID, &genesisStakingCfg)
	if err != nil {
		return node.Config{}, fmt.Errorf("unable to load genesis file: %w", err)
	}

	// StateSync Configs
	nodeConfig.StateSyncConfig, err = getStateSyncConfig(v)
	if err != nil {
		return node.Config{}, err
	}

	// Bootstrap Configs
	nodeConfig.BootstrapConfig, err = getBootstrapConfig(v, nodeConfig.NetworkID)
	if err != nil {
		return node.Config{}, err
	}

	// Chain Configs
	nodeConfig.ChainConfigs, err = getChainConfigs(v)
	if err != nil {
		return node.Config{}, fmt.Errorf("couldn't read chain configs: %w", err)
	}

	// Profiler
	nodeConfig.ProfilerConfig, err = getProfilerConfig(v)
	if err != nil {
		return node.Config{}, err
	}

	// VM Aliases
	nodeConfig.VMAliases, err = getVMAliases(v)
	if err != nil {
		return node.Config{}, err
	}
	// Chain aliases
	nodeConfig.ChainAliases, err = getChainAliases(v)
	if err != nil {
		return node.Config{}, err
	}

	nodeConfig.SystemTrackerFrequency = v.GetDuration(SystemTrackerFrequencyKey)
	nodeConfig.SystemTrackerProcessingHalflife = v.GetDuration(SystemTrackerProcessingHalflifeKey)
	nodeConfig.SystemTrackerCPUHalflife = v.GetDuration(SystemTrackerCPUHalflifeKey)
	nodeConfig.SystemTrackerDiskHalflife = v.GetDuration(SystemTrackerDiskHalflifeKey)

	nodeConfig.RequiredAvailableDiskSpace, nodeConfig.WarningThresholdAvailableDiskSpace, err = getDiskSpaceConfig(v)
	if err != nil {
		return node.Config{}, err
	}

	nodeConfig.CPUTargeterConfig, err = getCPUTargeterConfig(v)
	if err != nil {
		return node.Config{}, err
	}

	nodeConfig.DiskTargeterConfig, err = getDiskTargeterConfig(v)
	if err != nil {
		return node.Config{}, err
	}

	nodeConfig.TraceConfig, err = getTraceConfig(v)
	if err != nil {
		return node.Config{}, err
	}

	nodeConfig.ChainDataDir = GetExpandedArg(v, ChainDataDirKey)

	nodeConfig.ProcessContextFilePath = GetExpandedArg(v, ProcessContextFileKey)

	nodeConfig.ProvidedFlags = providedFlags(v)
	return nodeConfig, nil
}

func providedFlags(v *viper.Viper) map[string]interface{} {
	settings := v.AllSettings()
	customSettings := make(map[string]interface{}, len(settings))
	for key, val := range settings {
		if v.IsSet(key) {
			customSettings[key] = val
		}
	}
	return customSettings
}
