package aptos

import (
	"fmt"

	"github.com/aptos-labs/aptos-go-sdk"
	"github.com/smartcontractkit/chainlink/deployment"
	"github.com/smartcontractkit/chainlink/deployment/ccip/changeset"
	"github.com/smartcontractkit/chainlink/deployment/ccip/changeset/aptos/operation"
	"github.com/smartcontractkit/mcms"
	mcmstypes "github.com/smartcontractkit/mcms/types"
)

var CsDeployAptosMCMS deployment.ChangeSetV2[DeployAptosMCMSConfig] = CsDeployAptosMCMSImpl{}

type deployAptosMCMSParams struct {
	env               deployment.Environment
	ab                *deployment.AddressBookMap
	chainSelector     uint64
	mcmsConfigs       mcmstypes.Config
	aptosOnChainState map[uint64]changeset.AptosCCIPChainState
	proposals         *[]mcms.Proposal
}

type CsDeployAptosMCMSImpl struct{}

func (c CsDeployAptosMCMSImpl) VerifyPreconditions(e deployment.Environment, config DeployAptosMCMSConfig) error {
	return nil
}

func (cs CsDeployAptosMCMSImpl) Apply(env deployment.Environment, c DeployAptosMCMSConfig) (deployment.ChangesetOutput, error) {
	state, err := changeset.LoadOnchainStateAptos(env)
	if err != nil {
		errRes := fmt.Errorf("failed to load existing onchain state: %w", err)
		env.Logger.Errorw(errRes.Error())
		return deployment.ChangesetOutput{}, errRes
	}

	newAddresses := deployment.NewMemoryAddressBook()
	proposals := []mcms.Proposal{}
	for chainSel, mcmsConfigs := range c.MCMSConfigPerChain {
		deployParams := deployAptosMCMSParams{
			env:               env,
			ab:                newAddresses,
			chainSelector:     chainSel,
			mcmsConfigs:       mcmsConfigs,
			aptosOnChainState: state,
			proposals:         &proposals,
		}
		err := deployMCMSContractsForAptosChain(&deployParams)
		if err != nil {
			errRes := fmt.Errorf("failed to deploy MCMS contracts for chain %d: %v", chainSel, err)
			env.Logger.Errorw(errRes.Error())
			return deployment.ChangesetOutput{AddressBook: newAddresses, MCMSProposals: proposals}, deployment.MaybeDataErr(errRes)
		}
	}

	return deployment.ChangesetOutput{
		AddressBook:   newAddresses,
		MCMSProposals: proposals,
	}, nil
}

func deployMCMSContractsForAptosChain(p *deployAptosMCMSParams) error {
	chainState, ok := p.aptosOnChainState[p.chainSelector]
	if !ok {
		return fmt.Errorf("chain %d not found on state", p.chainSelector)
	}
	aptosChain, ok := p.env.AptosChains[p.chainSelector]
	if !ok {
		return fmt.Errorf("chain %d not found in env", p.chainSelector)
	}

	ops := operation.MCMSDeploymentOperations{
		Env:         p.env,
		Ab:          p.ab,
		AptosChain:  aptosChain,
		MCMSConfigs: p.mcmsConfigs,
		Proposals:   p.proposals,
	}
	// Check if MCMS package is already deployed
	if (chainState.MCMSAddress != aptos.AccountAddress{}) {
		p.env.Logger.Infow("MCMS Package already deployed", "addr", chainState.MCMSAddress.String())
		return nil
	}
	// Deploy MCMS
	addressMCMS, contractMCMS, err := ops.DeployMCMS()
	if err != nil {
		return fmt.Errorf("failed to deploy MCMS contract: %w", err)
	}
	// Configure MCMS
	err = ops.ConfigureMCMS(addressMCMS)
	if err != nil {
		return fmt.Errorf("failed to configure MCMS contract: %w", err)
	}
	// Transfer ownership to self
	err = ops.TransferOwnershipToSelf(contractMCMS)
	if err != nil {
		return fmt.Errorf("failed to transfer ownership to self: %w", err)
	}
	// Generate proposal to transfer ownership to self
	// TODO: This returns nextOpCount, we should keep track of it when merging migrations
	proposal, _, err := ops.GenerateAcceptOwnershipProposal(addressMCMS, contractMCMS)
	if err != nil {
		return fmt.Errorf("failed to build AcceptOwnership proposal: %w", err)
	}
	*p.proposals = append(*p.proposals, *proposal)

	return nil
}
