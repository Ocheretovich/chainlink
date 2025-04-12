package aptos

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	aptosfeequoter "github.com/smartcontractkit/chainlink-aptos/bindings/ccip/fee_quoter"
	"github.com/smartcontractkit/chainlink/deployment"
	"github.com/smartcontractkit/chainlink/deployment/ccip/changeset"
	"github.com/smartcontractkit/chainlink/deployment/ccip/changeset/aptos/config"
	"github.com/smartcontractkit/chainlink/deployment/ccip/changeset/testhelpers"
	"github.com/smartcontractkit/chainlink/deployment/ccip/changeset/v1_6"
	commonchangeset "github.com/smartcontractkit/chainlink/deployment/common/changeset"
	"github.com/smartcontractkit/chainlink/v2/core/capabilities/ccip/ccipevm"
)

func TestAddAptosLanes_Apply(t *testing.T) {
	// Setup environment and config
	deployedEnvironment, _ := testhelpers.NewMemoryEnvironment(t)
	env := deployedEnvironment.Env

	emvSelector := env.AllChainSelectors()[0]
	aptosSelector := uint64(4457093679053095497)

	// TODO: Some mocks for now
	env.AptosChains = map[uint64]deployment.AptosChain{
		aptosSelector: {
			Selector:       aptosSelector,
			Client:         nil,
			DeployerSigner: nil,
			URL:            "",
		},
	}
	typeAndVersion := deployment.NewTypeAndVersion(changeset.AptosCCIPType, deployment.Version1_6_0)
	env.ExistingAddresses.Save(aptosSelector, "0x368c3297ab04693970e2b58445110a0c3845b3710115403d9f4fbab9a75e414d", typeAndVersion)

	cfg := config.UpdateAptosLanesConfig{
		MCMSConfig: nil,
		Lanes: []config.LaneConfig{
			{
				Source: config.AptosChainDefinition{
					Selector:                 aptosSelector,
					GasPrice:                 big.NewInt(1e17),
					FeeQuoterDestChainConfig: aptosTestDestFeeQuoterConfig(t),
				},
				Dest: config.EVMChainDefinition{
					ChainDefinition: v1_6.ChainDefinition{
						Selector:                 emvSelector,
						GasPrice:                 big.NewInt(1e17),
						TokenPrices:              map[common.Address]*big.Int{},
						FeeQuoterDestChainConfig: v1_6.DefaultFeeQuoterDestChainConfig(true),
					},
				},
				IsDisabled: false,
			},
			{
				Source: config.EVMChainDefinition{
					ChainDefinition: v1_6.ChainDefinition{
						Selector:                 emvSelector,
						GasPrice:                 big.NewInt(1e17),
						TokenPrices:              map[common.Address]*big.Int{},
						FeeQuoterDestChainConfig: v1_6.DefaultFeeQuoterDestChainConfig(true),
					},
				},
				Dest: config.AptosChainDefinition{
					Selector:                 aptosSelector,
					GasPrice:                 big.NewInt(1e17),
					FeeQuoterDestChainConfig: aptosTestDestFeeQuoterConfig(t),
				},
				IsDisabled: false,
			},
		},
		TestRouter: true,
	}

	// Apply the changeset
	env, err := commonchangeset.ApplyChangesetsV2(t, env, []commonchangeset.ConfiguredChangeSet{
		commonchangeset.Configure(AddAptosLanes{}, cfg),
	})
	require.NoError(t, err)
}

// TODO: Deduplicate these test helpers
func aptosTestDestFeeQuoterConfig(t *testing.T) aptosfeequoter.DestChainConfig {
	return aptosfeequoter.DestChainConfig{
		IsEnabled:                         true,
		MaxNumberOfTokensPerMsg:           11,
		MaxDataBytes:                      40_000,
		MaxPerMsgGasLimit:                 4_000_000,
		DestGasOverhead:                   ccipevm.DestGasOverhead,
		DefaultTokenFeeUsdCents:           30,
		DestGasPerPayloadByteBase:         ccipevm.CalldataGasPerByteBase,
		DestGasPerPayloadByteHigh:         ccipevm.CalldataGasPerByteHigh,
		DestGasPerPayloadByteThreshold:    ccipevm.CalldataGasPerByteThreshold,
		DestDataAvailabilityOverheadGas:   700,
		DestGasPerDataAvailabilityByte:    17,
		DestDataAvailabilityMultiplierBps: 2,
		DefaultTokenDestGasOverhead:       100_000,
		DefaultTxGasLimit:                 100_000,
		GasMultiplierWeiPerEth:            12e17,
		NetworkFeeUsdCents:                20,
		ChainFamilySelector:               hexMustDecode(t, v1_6.AptosFamilySelector),
		EnforceOutOfOrder:                 false,
		GasPriceStalenessThreshold:        2,
	}
}

func hexMustDecode(t *testing.T, s string) []byte {
	b, err := hex.DecodeString(s)
	require.NoError(t, err)
	return b
}
