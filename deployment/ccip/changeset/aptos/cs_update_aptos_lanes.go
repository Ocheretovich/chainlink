package aptos

import (
	"github.com/smartcontractkit/chainlink/deployment"
	config "github.com/smartcontractkit/chainlink/deployment/ccip/changeset/aptos/config"
	"github.com/smartcontractkit/chainlink/deployment/ccip/changeset/v1_6"
	"github.com/smartcontractkit/mcms"
)

var _ deployment.ChangeSetV2[config.UpdateAptosLanesConfig] = AddAptosLanes{}

// AddAptosLane implements adding a new lane to an existing Aptos CCIP deployment
type AddAptosLanes struct{}

func (cs AddAptosLanes) VerifyPreconditions(env deployment.Environment, cfg config.UpdateAptosLanesConfig) error {
	// TODO: Implement verification logic - check chain selector validity, MCMS configuration, etc.
	// Placeholder implementation to show expected structure

	// This EVM specific changeset will be called from within this Aptos changeset, hence, we're verifying it here
	// TODO: this is an anti-pattern, change this once EVM changesets are refactored as Operations
	evmUpdateCfg := config.ToEVMUpdateLanesConfig(cfg)
	err := v1_6.UpdateLanesPrecondition(env, evmUpdateCfg)
	if err != nil {
		return err
	}
	return nil
}

func (cs AddAptosLanes) Apply(env deployment.Environment, cfg config.UpdateAptosLanesConfig) (deployment.ChangesetOutput, error) {
	timeLockProposals := []mcms.TimelockProposal{}

	// Add lane on EVM chains
	// TODO: applying a changeset within another changeset is an anti-pattern. Using it here until EVM is refactored into Operations
	out, err := v1_6.UpdateLanesLogic(env, cfg.MCMSConfig, config.ToEVMUpdateLanesConfig(cfg))
	if err != nil {
		return deployment.ChangesetOutput{}, err
	}
	timeLockProposals = append(timeLockProposals, out.MCMSTimelockProposals...)

	// Add lane on Aptos chains
	return deployment.ChangesetOutput{}, nil
}
