package tokens

import (
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink/deployment"
	"github.com/smartcontractkit/chainlink/deployment/datastore"
)

// DeployLinkTokenInput contains the selectors of the chains to which the Link Token contract
// should be deployed.
type DeployLinkTokenInput struct {
	ChainSelectors []uint64
}

var _ deployment.ChangeSetV2[DeployLinkTokenInput] = DeployLinkToken{}

// DeployLinkToken deploys Link Token contracts to the specified chains in the DeployLinkTokenInput.
type DeployLinkToken struct{}

// VerifyPreconditions ensures that all listed chain selectors are valid and available in the
// environment chains.
func (DeployLinkToken) VerifyPreconditions(
	e deployment.Environment, input DeployLinkTokenInput,
) error {
	return deployment.ValidateSelectorsInEnvironment(e, input.ChainSelectors)
}

func (DeployLinkToken) Apply(
	e deployment.Environment, input DeployLinkTokenInput,
) (deployment.ChangesetOutput, error) {
	var (
		out = deployment.ChangesetOutput{
			AddressBook: deployment.NewMemoryAddressBook(),
			DataStore:   datastore.NewMemoryDataStore[datastore.DefaultMetadata, datastore.DefaultMetadata](),
		}

		seqDeps = SeqDeployTokensDeps{
			Chains:    e.Chains,
			AddrBook:  out.AddressBook,
			Datastore: out.DataStore,
		}
		seqInput = SeqDeployTokensInput{
			ChainSelectors: input.ChainSelectors,
		}
	)

	_, err := operations.ExecuteSequence(
		e.OperationsBundle, SeqDeployTokens, seqDeps, seqInput,
	)

	return out, err
}
