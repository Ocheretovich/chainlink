package aptos

import (
	"testing"

	"github.com/aptos-labs/aptos-go-sdk"
	"github.com/smartcontractkit/chainlink/deployment"
	"github.com/smartcontractkit/chainlink/deployment/ccip/changeset"
	commonchangeset "github.com/smartcontractkit/chainlink/deployment/common/changeset"
	"github.com/smartcontractkit/chainlink/deployment/common/proposalutils"
	"github.com/smartcontractkit/chainlink/deployment/environment/memory"
	"github.com/smartcontractkit/chainlink/v2/core/logger"
	mcmstypes "github.com/smartcontractkit/mcms/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"

	ccipbind "github.com/smartcontractkit/chainlink-aptos/bindings/ccip"
)

func TestCsDeployAptosChainImp_VerifyPreconditions(t *testing.T) {
	tests := []struct {
		name      string
		env       deployment.Environment
		config    DeployAptosChainConfig
		wantErrRe string
		wantErr   bool
	}{
		{
			name: "success - valid config and state",
			env: deployment.Environment{
				Name:   "test",
				Logger: logger.TestLogger(t),
				AptosChains: map[uint64]deployment.AptosChain{
					743186221051783445:  {},
					4457093679053095497: {},
				},
				ExistingAddresses: getTestAddressBook(
					map[uint64]map[string]deployment.TypeAndVersion{
						4457093679053095497: {
							mockMCMSAddress: {Type: changeset.AptosMCMSType},
						},
						743186221051783445: {
							mockMCMSAddress: {Type: changeset.AptosMCMSType},
						},
					},
				),
			},
			config: DeployAptosChainConfig{
				ContractParamsPerChain: map[uint64]ChainContractParams{
					4457093679053095497: getMockChainContractParams(t, 4457093679053095497),
					743186221051783445:  getMockChainContractParams(t, 743186221051783445),
				},
			},
			wantErr: false,
		},
		{
			name: "error - chain has no env",
			env: deployment.Environment{
				Name:   "test",
				Logger: logger.TestLogger(t),
				AptosChains: map[uint64]deployment.AptosChain{
					4457093679053095497: {},
				},
				ExistingAddresses: getTestAddressBook(
					map[uint64]map[string]deployment.TypeAndVersion{
						4457093679053095497: {
							mockMCMSAddress: {Type: changeset.AptosMCMSType},
						},
						743186221051783445: {
							mockMCMSAddress: {Type: changeset.AptosMCMSType},
						},
					},
				),
			},
			config: DeployAptosChainConfig{
				ContractParamsPerChain: map[uint64]ChainContractParams{
					4457093679053095497: getMockChainContractParams(t, 4457093679053095497),
					743186221051783445:  getMockChainContractParams(t, 743186221051783445),
				},
			},
			wantErrRe: `env not found for chains: \[743186221051783445\]`,
			wantErr:   true,
		},
		{
			name: "error - invalid config - chainSelector",
			env: deployment.Environment{
				Name:              "test",
				Logger:            logger.TestLogger(t),
				ExistingAddresses: deployment.NewMemoryAddressBook(),
				AptosChains:       map[uint64]deployment.AptosChain{},
			},
			config: DeployAptosChainConfig{
				ContractParamsPerChain: map[uint64]ChainContractParams{
					1: {},
				},
			},
			wantErrRe: "invalid DeployAptosChainConfig:",
			wantErr:   true,
		},
		{
			name: "error - missing MCMS contract for 2 chains",
			env: deployment.Environment{
				Name:   "test",
				Logger: logger.TestLogger(t),
				AptosChains: map[uint64]deployment.AptosChain{
					743186221051783445:  {},
					4457093679053095497: {},
				},
				ExistingAddresses: getTestAddressBook(
					map[uint64]map[string]deployment.TypeAndVersion{
						4457093679053095497: {
							mockAddress: {Type: "testType"},
						},
						743186221051783445: {
							mockAddress: {Type: "testType"},
						},
					},
				),
			},
			config: DeployAptosChainConfig{
				ContractParamsPerChain: map[uint64]ChainContractParams{
					4457093679053095497: getMockChainContractParams(t, 4457093679053095497),
					743186221051783445:  getMockChainContractParams(t, 743186221051783445),
				},
			},
			wantErrRe: "MCMS contract not deployed for chains:.*(4457093679053095497.*743186221051783445|743186221051783445.*4457093679053095497).*",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := CsDeployAptosChainImp{}
			err := cs.VerifyPreconditions(tt.env, tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Regexp(t, tt.wantErrRe, err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCsDeployAptosChain_Apply(t *testing.T) {
	t.Parallel()
	lggr := logger.TestLogger(t)

	// Setup memory environment with 1 Aptos chain
	e := memory.NewMemoryEnvironment(t, lggr, zapcore.InfoLevel, memory.MemoryEnvironmentConfig{
		AptosChains: 1,
	})

	// Get chain selectors
	aptosChainSelectors := e.AllChainSelectorsAptos()
	require.Equal(t, 1, len(aptosChainSelectors), "Expected exactly 1 Aptos chain")
	chainSelector := aptosChainSelectors[0]
	t.Log("Deployer: ", e.AptosChains[chainSelector].DeployerSigner)

	// Deploy MCMS
	mcmsConfig := proposalutils.SingleGroupMCMSV2(t)
	mcmsDeployConfig := DeployAptosMCMSConfig{
		MCMSConfigPerChain: map[uint64]mcmstypes.Config{
			chainSelector: mcmsConfig,
		},
	}

	e, err := commonchangeset.ApplyChangesetsV2(t, e, []commonchangeset.ConfiguredChangeSet{
		commonchangeset.Configure(CsDeployAptosMCMS, mcmsDeployConfig),
	})
	require.NoError(t, err)

	// Deploy CCIP to Aptos chain
	ccipConfig := DeployAptosChainConfig{
		ContractParamsPerChain: map[uint64]ChainContractParams{
			chainSelector: getMockChainContractParams(t, chainSelector),
		},
	}

	e, err = commonchangeset.ApplyChangesetsV2(t, e, []commonchangeset.ConfiguredChangeSet{
		commonchangeset.Configure(CsDeployAptosChain, ccipConfig),
	})
	require.NoError(t, err)

	// Verify CCIP deployment state by binding ccip contract and checking if it's deployed
	state, err := changeset.LoadOnchainStateAptos(e)
	require.NoError(t, err)
	require.NotNil(t, state[chainSelector], "No state found for chain")

	ccipAddr := state[chainSelector].CCIPAddress
	require.NotEmpty(t, ccipAddr, "CCIP address should not be empty")

	// Bind CCIP contract
	ccipContract := ccipbind.Bind(ccipAddr, e.AptosChains[chainSelector].Client)
	ownerAddr, err := ccipContract.Auth().Owner(nil)
	require.NoError(t, err)
	require.NotEqual(t, aptos.AccountAddress{}, ownerAddr, "MCMS must own CCIP contract")
}
