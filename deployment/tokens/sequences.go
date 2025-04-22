package tokens

import (
	"github.com/Masterminds/semver/v3"
	chainsel "github.com/smartcontractkit/chain-selectors"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink/deployment"
	"github.com/smartcontractkit/chainlink/deployment/datastore"
)

type SeqDeployTokensDeps struct {
	Chains    map[uint64]deployment.Chain
	AddrBook  deployment.AddressBook
	Datastore datastore.MutableDataStore[
		datastore.DefaultMetadata,
		datastore.DefaultMetadata,
	]
}

type SeqDeployTokensInput struct {
	ChainSelectors []uint64
	Qualifier      string
	Labels         []string
}

type SeqDeployTokensOutput struct {
	Addresses []string `json:"address"`
}

// SeqDeployTokens is a sequence that deploys LINK token contracts across multiple chains.
var SeqDeployTokens = operations.NewSequence(
	"seq-deploy-tokens",
	semver.MustParse("1.0.0"),
	"Deploy LINK token contracts across multiple chains",
	func(b operations.Bundle, deps SeqDeployTokensDeps, input SeqDeployTokensInput) (SeqDeployTokensOutput, error) {
		out := SeqDeployTokensOutput{}

		for _, csel := range input.ChainSelectors {
			fam, err := chainsel.GetSelectorFamily(csel)
			if err != nil {
				return out, err
			}

			switch fam {
			case chainsel.FamilyEVM:
				chain := deps.Chains[csel]

				// Deploy the link token
				deployReport, err := operations.ExecuteOperation(b, OpEVMDeployLinkToken,
					OpEVMDeployLinkTokenDeps{
						Auth:        chain.DeployerKey,
						Backend:     chain.Client,
						ConfirmFunc: chain.Confirm,
					},
					OpEVMDeployLinkTokenInput{
						ChainSelector: csel,
						ChainName:     chain.String(),
					},
				)
				if err != nil {
					return out, err
				}

				// Store it in the legacy address book
				if _, err = operations.ExecuteOperation(b, OpAddAddrBookRecord,
					OpAddAddrBookRecordDeps{AddrBook: deps.AddrBook},
					OpAddAddrBookRecordInput{
						ChainSelector: csel,
						Address:       deployReport.Output.Address.String(),
						Type:          deployReport.Output.Type,
						Version:       deployReport.Output.Version,
						Labels:        input.Labels,
					},
				); err != nil {
					return out, err
				}

				// Store the address reference in the datastore
				if _, err := operations.ExecuteOperation(b, OpAddDatastoreAddrRef,
					OpAddDatastoreAddrRefDeps{Datastore: deps.Datastore},
					OpAddDatastoreAddrRefInput{
						ChainSelector: csel,
						Address:       deployReport.Output.Address.String(),
						Type:          deployReport.Output.Type,
						Version:       deployReport.Output.Version,
						Qualifier:     input.Qualifier,
						Labels:        input.Labels,
					},
				); err != nil {
					return out, err
				}
			}
		}

		return out, nil
	},
)
