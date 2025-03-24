package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/aptos-labs/aptos-go-sdk"
	mcmsbind "github.com/smartcontractkit/chainlink-aptos/bindings/mcms"
	"github.com/smartcontractkit/mcms"
	aptosmcms "github.com/smartcontractkit/mcms/sdk/aptos"
	"github.com/smartcontractkit/mcms/types"
)

const (
	ValidUntilHours     = 72
	MCMSProposalVersion = "v1"
)

func GenerateProposal(
	client aptos.AptosRpcClient,
	mcmsContract mcmsbind.MCMS,
	chainSel uint64,
	operations []types.Operation,
	description string,
	opCount uint64,
) (*mcms.Proposal, uint64, error) {
	if opCount == 0 {
		// Create MCMS inspector
		inspector := aptosmcms.NewInspector(client)
		startingOpCount, err := inspector.GetOpCount(context.Background(), mcmsContract.Address.StringLong())
		if err != nil {
			return nil, 0, fmt.Errorf("failed to get starting op count: %w", err)
		}
		opCount = startingOpCount
	}

	// Create proposal builder
	validUntil := time.Now().Add(time.Hour * ValidUntilHours).Unix()
	proposalBuilder := mcms.NewProposalBuilder().
		SetVersion(MCMSProposalVersion).
		SetValidUntil(uint32(validUntil)).
		SetDescription(description).
		SetOverridePreviousRoot(true).
		AddChainMetadata(
			types.ChainSelector(chainSel),
			types.ChainMetadata{
				StartingOpCount: opCount,
				MCMAddress:      mcmsContract.Address.StringLong(),
			},
		)

	// Add operations and build
	for _, op := range operations {
		proposalBuilder.AddOperation(op)
	}
	proposal, err := proposalBuilder.Build()
	if err != nil {
		return nil, opCount, fmt.Errorf("failed to build proposal: %w", err)
	}

	return proposal, opCount + uint64(len(operations)), nil
}
