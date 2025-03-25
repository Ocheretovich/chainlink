package aptos_test

import (
	"testing"

	"github.com/aptos-labs/aptos-go-sdk"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"

	mcmsbind "github.com/smartcontractkit/chainlink-aptos/bindings/mcms"

	"github.com/smartcontractkit/chainlink/deployment"
	"github.com/smartcontractkit/chainlink/deployment/ccip/changeset"
	ccipChangesetAptos "github.com/smartcontractkit/chainlink/deployment/ccip/changeset/aptos"
	"github.com/smartcontractkit/chainlink/deployment/common/proposalutils"
	"github.com/smartcontractkit/chainlink/deployment/environment/memory"
	"github.com/smartcontractkit/chainlink/v2/core/logger"
	mcmstypes "github.com/smartcontractkit/mcms/types"
)

// TestDeployAptosMCMS verifies that the deployment of MCMS packages to Aptos chains
func TestDeployAptosMCMS(t *testing.T) {
	t.Parallel()
	lggr := logger.TestLogger(t)

	// Setup memory environment with 1 Aptos chain
	e := memory.NewMemoryEnvironment(t, lggr, zapcore.InfoLevel, memory.MemoryEnvironmentConfig{
		AptosChains: 1,
	})

	// Get chain selectors
	aptosChainSelectors := e.AllChainSelectorsAptos()
	require.Equal(t, 1, len(aptosChainSelectors), "Expected exactly 1 Aptos chain")

	// Create MCMS configuration
	mcmsConfig := proposalutils.SingleGroupMCMSV2(t)

	// Deploy MCMS to Aptos chain
	output, err := ccipChangesetAptos.CsDeployAptosMCMS.Apply(e, ccipChangesetAptos.DeployAptosMCMSConfig{
		MCMSConfigPerChain: map[uint64]mcmstypes.Config{
			aptosChainSelectors[0]: mcmsConfig,
		},
	})
	require.NoError(t, err)

	// Merge output addresses into existing AB
	err = e.ExistingAddresses.Merge(output.AddressBook)
	require.NoError(t, err)

	// Verify MCMS was deployed successfully
	verifyMCMSDeployment(t, e, aptosChainSelectors[0])
}

// verifyMCMSDeployment checks that MCMS was correctly deployed to the specified chain
func verifyMCMSDeployment(t *testing.T, e deployment.Environment, chainSelector uint64) {
	// Get the Aptos client for the chain
	client := e.AptosChains[chainSelector].Client
	require.NotNil(t, client, "Aptos client not found for chain selector")

	// Get current MCMS state
	state, err := changeset.LoadOnchainStateAptos(e)
	require.NoError(t, err)
	require.NotNil(t, state[chainSelector], "No state found for chain")

	mcmsAddr := state[chainSelector].MCMSAddress
	require.NotEmpty(t, mcmsAddr, "MCMS address should not be empty")

	// Bind MCMS contract
	mcmsContract := mcmsbind.Bind(mcmsAddr, client)
	ownerAddr, err := mcmsContract.MCMSAccount().Owner(nil)
	require.NoError(t, err)
	require.NotEqual(t, aptos.AccountAddress{}, ownerAddr)
}
