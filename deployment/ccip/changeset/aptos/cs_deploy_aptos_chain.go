package aptos

import (
	"errors"
	"fmt"

	"github.com/aptos-labs/aptos-go-sdk"

	"github.com/smartcontractkit/chainlink/deployment"
	"github.com/smartcontractkit/chainlink/deployment/ccip/changeset"
	"github.com/smartcontractkit/chainlink/deployment/ccip/changeset/aptos/config"
	"github.com/smartcontractkit/chainlink/deployment/ccip/changeset/aptos/operation"
	"github.com/smartcontractkit/chainlink/deployment/operations"
	"github.com/smartcontractkit/mcms"
	mcmstypes "github.com/smartcontractkit/mcms/types"
)

var _ deployment.ChangeSetV2[config.DeployAptosChainConfig] = DeployAptosChain{}

// DeployAptosChain deploys Aptos chain packages and modules
type DeployAptosChain struct{}

func (cs DeployAptosChain) VerifyPreconditions(env deployment.Environment, config config.DeployAptosChainConfig) error {
	// Validate env and prerequisite contracts
	state, err := changeset.LoadOnchainStateAptos(env)
	if err != nil {
		return fmt.Errorf("failed to load existing Aptos onchain state: %w", err)
	}
	var errs []error
	for chainSel := range config.ContractParamsPerChain {
		if err := config.Validate(); err != nil {
			errs = append(errs, fmt.Errorf("invalid config for Aptos chain %d: %w", chainSel, err))
			continue
		}
		if _, ok := env.AptosChains[chainSel]; !ok {
			errs = append(errs, fmt.Errorf("aptos chain %d not found in env", chainSel))
		}
		chainState, ok := state[chainSel]
		if !ok {
			errs = append(errs, fmt.Errorf("aptos chain %d not found in state", chainSel))
			continue
		}
		if chainState.MCMSAddress == aptos.AccountZero {
			mcmsConfig := config.MCMSConfigPerChain[chainSel]
			if err := mcmsConfig.Validate(); err != nil {
				errs = append(errs, fmt.Errorf("invalid mcms configs for Aptos chain %d: %w", chainSel, err))
			}
		}
	}

	return errors.Join(errs...)
}

func (cs DeployAptosChain) Apply(env deployment.Environment, config config.DeployAptosChainConfig) (deployment.ChangesetOutput, error) {
	state, err := changeset.LoadOnchainStateAptos(env)
	if err != nil {
		return deployment.ChangesetOutput{}, fmt.Errorf("failed to load Aptos onchain state: %w", err)
	}

	ab := deployment.NewMemoryAddressBook()
	proposals := []mcms.Proposal{}
	seqReports := make([]operations.Report[any, any], 0)

	// Deploy CCIP on each Aptos chain in config
	for chainSel := range config.ContractParamsPerChain {
		chainState := state[chainSel]
		aptosChain := env.AptosChains[chainSel]

		deps := operation.AptosDeps{
			AB:           ab,
			AptosChain:   aptosChain,
			OnChainState: chainState,
		}

		// MCMS Deploy operations
		mcmsSeqReport, err := operations.ExecuteSequence(env.OperationsBundle, DeployMCMSSequence, deps, config.MCMSConfigPerChain[chainSel])
		if err != nil {
			return deployment.ChangesetOutput{}, err
		}
		seqReports = append(seqReports, mcmsSeqReport.ExecutionReports...)
		proposals = append(proposals, *mcmsSeqReport.Output.MCMSProposal)

		// CCIP Deploy operations
		ccipSeqInput := DeployCCIPSeqInput{
			MCMSAddress: mcmsSeqReport.Output.MCMSAddress,
			MCMSOpCount: mcmsSeqReport.Output.NextOpCount,
			CCIPConfig:  config.ContractParamsPerChain[chainSel],
		}
		ccipSeqReport, err := operations.ExecuteSequence(env.OperationsBundle, DeployCCIPSequence, deps, ccipSeqInput)
		if err != nil {
			return deployment.ChangesetOutput{}, fmt.Errorf("failed to deploy CCIP for Aptos chain %d: %w", chainSel, err)
		}
		seqReports = append(seqReports, ccipSeqReport.ExecutionReports...)
		for _, proposal := range ccipSeqReport.Output.MCMSProposals {
			proposals = append(proposals, *proposal)
		}
	}

	return deployment.ChangesetOutput{
		AddressBook:   ab,
		MCMSProposals: proposals,
		Reports:       seqReports,
	}, nil
}

// Deploy MCMS Sequence
type DeployMCMSSeqOutput struct {
	MCMSAddress  aptos.AccountAddress
	MCMSProposal *mcms.Proposal
	NextOpCount  uint64
}

var DeployMCMSSequence = operations.NewSequence(
	"deploy-aptos-mcms-sequence",
	operation.Version1_0_0,
	"Deploy Aptos MCMS contract and configure it",
	deployMCMSSequence,
)

func deployMCMSSequence(b operations.Bundle, deps operation.AptosDeps, configMCMS mcmstypes.Config) (DeployMCMSSeqOutput, error) {
	// Check if MCMS package is already deployed
	if deps.OnChainState.MCMSAddress != aptos.AccountZero {
		b.Logger.Infow("MCMS Package already deployed", "addr", deps.OnChainState.MCMSAddress.String())
		return DeployMCMSSeqOutput{}, nil
	}
	// Deploy MCMS
	deployMCMSReport, err := operations.ExecuteOperation(b, operation.DeployMCMSOp, deps, operations.EmptyInput{})
	if err != nil {
		return DeployMCMSSeqOutput{}, err
	}
	// Configure MCMS
	configureMCMSInput := operation.ConfigureMCMSInput{
		AddressMCMS: deployMCMSReport.Output.AddressMCMS,
		MCMSConfigs: configMCMS,
	}
	_, err = operations.ExecuteOperation(b, operation.ConfigureMCMSOp, deps, configureMCMSInput)
	if err != nil {
		return DeployMCMSSeqOutput{}, err
	}
	// Transfer ownership to self
	_, err = operations.ExecuteOperation(b, operation.TransferOwnershipToSelfOp, deps, deployMCMSReport.Output.ContractMCMS)
	if err != nil {
		return DeployMCMSSeqOutput{}, err
	}
	// Generate proposal to accept ownership
	generateAcceptOwnershipProposalInput := operation.GenerateAcceptOwnershipProposalInput{
		AddressMCMS:  deployMCMSReport.Output.AddressMCMS,
		ContractMCMS: deployMCMSReport.Output.ContractMCMS,
	}
	gaopReport, err := operations.ExecuteOperation(b, operation.GenerateAcceptOwnershipProposalOp, deps, generateAcceptOwnershipProposalInput)
	if err != nil {
		return DeployMCMSSeqOutput{}, err
	}

	return DeployMCMSSeqOutput{
		MCMSAddress:  deployMCMSReport.Output.AddressMCMS,
		MCMSProposal: gaopReport.Output.MCMSProposal,
		NextOpCount:  gaopReport.Output.NextOpCount,
	}, nil
}

// Deploy CCIP Sequence
type DeployCCIPSeqInput struct {
	MCMSAddress aptos.AccountAddress
	MCMSOpCount uint64
	CCIPConfig  config.ChainContractParams
}

type DeployCCIPSeqOutput struct {
	CCIPAddress   aptos.AccountAddress
	MCMSProposals []*mcms.Proposal
	NextOpCount   uint64
}

var DeployCCIPSequence = operations.NewSequence(
	"deploy-aptos-ccip-sequence",
	operation.Version1_0_0,
	"Deploy Aptos CCIP contracts and initialize them",
	deployCCIPSequence,
)

func deployCCIPSequence(b operations.Bundle, deps operation.AptosDeps, in DeployCCIPSeqInput) (DeployCCIPSeqOutput, error) {
	var proposals []*mcms.Proposal
	mcmsOpCount := in.MCMSOpCount

	// Cleanup staging area
	cleanupInput := operation.CleanupStagingAreaInput{
		MCMSAddress: in.MCMSAddress,
		MCMSOpCount: mcmsOpCount,
	}
	cleanupReport, err := operations.ExecuteOperation(b, operation.CleanupStagingAreaOp, deps, cleanupInput)
	if err != nil {
		return DeployCCIPSeqOutput{}, err
	}
	if cleanupReport.Output.MCMSProposal != nil {
		proposals = append(proposals, cleanupReport.Output.MCMSProposal)
		mcmsOpCount = cleanupReport.Output.NextOpCount
	}

	// Generate proposal to deploy CCIP package
	deployCCIPInput := operation.DeployCCIPInput{
		MCMSAddress: in.MCMSAddress,
		MCMSOpCount: mcmsOpCount,
	}
	deployCCIPReport, err := operations.ExecuteOperation(b, operation.GenerateDeployCCIPProposalOp, deps, deployCCIPInput)
	if err != nil {
		return DeployCCIPSeqOutput{}, err
	}
	ccipAddress := deployCCIPReport.Output.CCIPAddress
	if deployCCIPReport.Output.MCMSProposal != nil {
		proposals = append(proposals, deployCCIPReport.Output.MCMSProposal)
		mcmsOpCount = deployCCIPReport.Output.NextOpCount
	}

	// Generate proposal to deploy Router module
	deployRouterInput := operation.DeployRouterInput{
		MCMSAddress: in.MCMSAddress,
		CCIPAddress: ccipAddress,
		MCMSOpCount: mcmsOpCount,
	}
	deployRouterReport, err := operations.ExecuteOperation(b, operation.GenerateDeployRouterProposalOp, deps, deployRouterInput)
	if err != nil {
		return DeployCCIPSeqOutput{}, err
	}
	if deployRouterReport.Output.MCMSProposal != nil {
		proposals = append(proposals, deployRouterReport.Output.MCMSProposal)
		mcmsOpCount = deployRouterReport.Output.NextOpCount
	}

	// Generate proposal to initialize CCIP
	initCCIPInput := operation.InitializeCCIPInput{
		MCMSAddress: in.MCMSAddress,
		CCIPAddress: ccipAddress,
		CCIPConfig:  in.CCIPConfig,
		MCMSOpCount: mcmsOpCount,
	}
	initCCIPReport, err := operations.ExecuteOperation(b, operation.InitializeCCIPOp, deps, initCCIPInput)
	if err != nil {
		return DeployCCIPSeqOutput{}, err
	}
	if initCCIPReport.Output.MCMSProposal != nil {
		proposals = append(proposals, initCCIPReport.Output.MCMSProposal)
		mcmsOpCount = initCCIPReport.Output.NextOpCount
	}

	return DeployCCIPSeqOutput{
		CCIPAddress:   ccipAddress,
		MCMSProposals: proposals,
		NextOpCount:   mcmsOpCount,
	}, nil
}
