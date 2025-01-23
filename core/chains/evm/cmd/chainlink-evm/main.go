package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/go-plugin"
	"github.com/pelletier/go-toml/v2"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/smartcontractkit/chainlink-common/pkg/logger"
	"github.com/smartcontractkit/chainlink-common/pkg/loop"
	"github.com/smartcontractkit/chainlink-common/pkg/sqlutil"
	"github.com/smartcontractkit/chainlink-common/pkg/types/core"
	"github.com/smartcontractkit/chainlink-common/pkg/utils/mailbox"
	evmcfg "github.com/smartcontractkit/chainlink-evm/pkg/config/toml"
	"github.com/smartcontractkit/chainlink-evm/pkg/keys"
	"github.com/smartcontractkit/chainlink/v2/core/chains/legacyevm"
	"github.com/smartcontractkit/chainlink/v2/core/services/relay/evm"
	clhttp "github.com/smartcontractkit/chainlink/v2/core/utils/http"
)

func main() {
	s := loop.MustNewStartedServer("PluginEVM")
	defer s.Stop()

	p := &pluginRelayer{EnvConfig: s.EnvConfig, Plugin: loop.Plugin{Logger: s.Logger}, DataSource: s.DataSource}
	defer s.Logger.ErrorIfFn(p.Close, "Failed to close")

	s.MustRegister(p)

	stopCh := make(chan struct{})
	defer close(stopCh)

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: loop.PluginRelayerHandshakeConfig(),
		Plugins: map[string]plugin.Plugin{
			loop.PluginRelayerName: &loop.GRPCPluginRelayer{
				PluginServer: p,
				BrokerConfig: loop.BrokerConfig{
					StopCh:   stopCh,
					Logger:   s.Logger,
					GRPCOpts: s.GRPCOpts,
				},
			},
		},
		GRPCServer: s.GRPCOpts.NewServer,
	})
}

type pluginRelayer struct {
	loop.EnvConfig
	loop.Plugin
	sqlutil.DataSource
}

func (c *pluginRelayer) NewRelayer(ctx context.Context, config string, keystore core.Keystore, capRegistry core.CapabilitiesRegistry) (loop.Relayer, error) {
	d := toml.NewDecoder(strings.NewReader(config))
	d.DisallowUnknownFields()
	var cfg struct {
		EVM evmcfg.EVMConfig
	}

	if err := d.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config toml: %w:\n\t%s", err, config)
	}

	evmKeystore := keys.NewChainStore(keystore, cfg.EVM.ChainID.ToInt())

	mailMon := mailbox.NewMonitor(c.AppID, logger.Named(c.Logger, "Mailbox"))
	c.SubService(mailMon)

	chain, err := legacyevm.NewTOMLChain(&cfg.EVM, legacyevm.ChainRelayOpts{
		Logger:   c.Logger,
		KeyStore: evmKeystore,
		ChainOpts: legacyevm.ChainOpts{
			ChainConfigs: evmcfg.EVMConfigs{&cfg.EVM},
			DatabaseConfig: &DatabaseConfig{
				defaultQueryTimeout: c.DatabaseQueryTimeout,
				logSQL:              c.DatabaseLogSQL,
			},
			FeatureConfig: &FeatureConfig{
				logPoller: c.FeatureLogPoller,
			},
			ListenerConfig: &ListenerConfig{
				fallbackPollInterval: c.DatabaseListenerFallbackPollInterval,
			},
			MailMon: mailMon,
			DS:      c.DataSource,
		},
	}, nil) // TODO client are not accessible - what breaks?
	if err != nil {
		return nil, fmt.Errorf("failed to create chain: %w", err)
	}

	//TODO do we need the "whole" relayer?
	ra, err := evm.NewRelayer(c.Logger, chain, evm.RelayerOpts{
		DS:                    c.DataSource,
		Registerer:            prometheus.DefaultRegisterer,
		EVMKeystore:           evmKeystore,
		CSAKeystore:           nil, //TODO csaKeystore
		MercuryPool:           nil,
		RetirementReportCache: nil,
		MercuryConfig:         nil,
		CapabilitiesRegistry:  capRegistry,
		HTTPClient:            clhttp.NewUnrestrictedHTTPClient(), //TODO core dependency
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create relayer: %w", err)
	}

	c.SubService(ra)

	return ra, nil
}

type DatabaseConfig struct {
	defaultQueryTimeout time.Duration
	logSQL              bool
}

func (d *DatabaseConfig) DefaultQueryTimeout() time.Duration {
	return d.defaultQueryTimeout
}

func (d *DatabaseConfig) LogSQL() bool {
	return d.logSQL
}

type FeatureConfig struct {
	logPoller bool
}

func (f *FeatureConfig) LogPoller() bool {
	return f.logPoller
}

type ListenerConfig struct {
	fallbackPollInterval time.Duration
}

func (l *ListenerConfig) FallbackPollInterval() time.Duration {
	return l.fallbackPollInterval
}
