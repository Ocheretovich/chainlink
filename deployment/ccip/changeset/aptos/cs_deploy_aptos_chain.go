package aptos

import (
	"fmt"

	"github.com/aptos-labs/aptos-go-sdk"

	"github.com/smartcontractkit/chainlink/deployment"
	"github.com/smartcontractkit/chainlink/deployment/ccip/changeset"
	"github.com/smartcontractkit/chainlink/deployment/ccip/changeset/aptos/operation"
	"github.com/smartcontractkit/mcms"
)

// CsDeployAptosChain deploys CCIP Package for Aptos chains
var CsDeployAptosChain deployment.ChangeSetV2[DeployAptosChainConfig] = CsDeployAptosChainImp{}

type CsDeployAptosChainImp struct{}

func (cs CsDeployAptosChainImp) VerifyPreconditions(env deployment.Environment, config DeployAptosChainConfig) error {
	// Validate configs
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid DeployAptosChainConfig: %w", err)
	}

	// Validate env and prerequisite contracts
	state, err := changeset.LoadOnchainStateAptos(env)
	if err != nil {
		return fmt.Errorf("failed to load existing onchain state: %w", err)
	}
	failedEnvChains := []uint64{}
	failedPrereqChains := []uint64{}
	for chainSel := range config.ContractParamsPerChain {
		if _, ok := env.AptosChains[chainSel]; !ok {
			failedEnvChains = append(failedEnvChains, chainSel)
		}
		chainState, ok := state[chainSel]
		if !ok || chainState.MCMSAddress == (aptos.AccountAddress{}) {
			failedPrereqChains = append(failedPrereqChains, chainSel)
		}
	}
	// If a chain is not in env it won't be in state, but these two checks are here to return clear errors
	if len(failedEnvChains) > 0 {
		return fmt.Errorf("env not found for chains: %v", failedEnvChains)
	}
	if len(failedPrereqChains) > 0 {
		return fmt.Errorf("MCMS contract not deployed for chains: %v", failedPrereqChains)
	}

	return nil
}

func (cs CsDeployAptosChainImp) Apply(env deployment.Environment, config DeployAptosChainConfig) (deployment.ChangesetOutput, error) {
	ab := deployment.NewMemoryAddressBook()
	proposals := &[]mcms.Proposal{}

	state, err := changeset.LoadOnchainStateAptos(env)
	if err != nil {
		return deployment.ChangesetOutput{}, fmt.Errorf("failed to load onchain state: %w", err)
	}

	// For each aptos chain in the config generate proposals
	for chainSel := range config.ContractParamsPerChain {
		chainState := state[chainSel]
		aptosChain := env.AptosChains[chainSel]

		ops := operation.CCIPDeploymentOperations{
			Env:          env,
			Ab:           ab,
			AptosChain:   aptosChain,
			OnChainState: chainState,
			Proposals:    proposals,
			MCMSOpCount:  0,
		}
		// Cleanup MCMS staging area
		err := ops.GenerateCleanupStagingProposal()
		if err != nil {
			err := fmt.Errorf("failed to generate cleanup staging proposal for chain %d : %w", chainSel, err)
			env.Logger.Error(err)
			return deployment.ChangesetOutput{AddressBook: ops.Ab}, err
		}
		// Generate proposals - Deploy CCIP package
		ccipObjectAddress, err := ops.GenerateDeployCCIPProposal()
		if err != nil {
			err := fmt.Errorf("failed to generate CCIP deploy proposal for chain %d : %w", chainSel, err)
			env.Logger.Error(err)
			return deployment.ChangesetOutput{AddressBook: ops.Ab}, err
		}
		// Generate proposals - Deploy Router package
		err = ops.GenerateDeployRouterProposal(ccipObjectAddress)
		if err != nil {
			err := fmt.Errorf("failed to generate Router deploy proposal for chain %d : %w", chainSel, err)
			env.Logger.Error(err)
			return deployment.ChangesetOutput{AddressBook: ops.Ab}, err
		}
		// TODO: Generate proposals - Initialize contracts
	}

	return deployment.ChangesetOutput{
		AddressBook:   ab,
		MCMSProposals: *proposals,
	}, nil
}
