package tokens

import (
	"github.com/Masterminds/semver/v3"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/smartcontractkit/chainlink-deployments-framework/operations"
	"github.com/smartcontractkit/chainlink-evm/gethwrappers/shared/generated/link_token"
	"github.com/smartcontractkit/chainlink/deployment"
	"github.com/smartcontractkit/chainlink/deployment/datastore"
)

var (
	// LinkToken is the burn/mint link token which is now used in all new deployments.
	// https://github.com/smartcontractkit/chainlink/blob/develop/core/gethwrappers/shared/generated/link_token/link_token.go#L34
	LinkTokenTypeAndVersion = deployment.NewTypeAndVersion(
		"LinkToken",
		*semver.MustParse("1.0.0"),
	)
)

// OpEVMDeployLinkTokenDeps defines the dependencies to perform the Link Token
// deployment operation.
type OpEVMDeployLinkTokenDeps struct {
	Auth        *bind.TransactOpts
	Backend     bind.ContractBackend
	ConfirmFunc func(tx *types.Transaction) (uint64, error)
}

type OpEVMDeployLinkTokenInput struct {
	ChainSelector uint64
	ChainName     string
}

type OpEvmDeployLinkTokenOutput struct {
	Address       common.Address `json:"address"`
	ChainSelector uint64         `json:"chainSelector"`
	ChainName     string         `json:"chainName"`
	Type          string         `json:"type"`
	Version       string         `json:"version"`
}

var OpEVMDeployLinkToken = operations.NewOperation(
	"evm-deploy-link-token",
	semver.MustParse("1.0.0"),
	"Deploy EVM LINK Contract Operation",
	func(b operations.Bundle, deps OpEVMDeployLinkTokenDeps, input OpEVMDeployLinkTokenInput) (OpEvmDeployLinkTokenOutput, error) {
		out := OpEvmDeployLinkTokenOutput{}

		// Deploy the link token
		addr, tx, _, err := link_token.DeployLinkToken(
			deps.Auth,
			deps.Backend,
		)
		if err != nil {
			b.Logger.Errorw("Failed to deploy link token",
				"chainSelector", input.ChainSelector,
				"chainName", input.ChainName,
				"err", err,
			)

			return out, err
		}

		// Confirm the transaction
		if _, err = deps.ConfirmFunc(tx); err != nil {
			b.Logger.Errorw("Failed to confirm deployment",
				"chainSelector", input.ChainSelector,
				"chainName", input.ChainName,
				"contractAddr", addr.String(),
				"err", err,
			)

			return out, err
		}

		return OpEvmDeployLinkTokenOutput{
			Address:       addr,
			ChainSelector: input.ChainSelector,
			ChainName:     input.ChainName,
			Type:          LinkTokenTypeAndVersion.Type.String(),
			Version:       LinkTokenTypeAndVersion.Version.String(),
		}, nil
	})

type OpAddAddrBookRecordInput struct {
	ChainSelector uint64
	Address       string
	Type          string
	Version       string
	Labels        []string
}

type OpAddAddrBookRecordOutput struct {
	ChainSelector  uint64 `json:"chainSelector"`
	Address        string `json:"address"`
	TypeAndVersion string `json:"typeAndVersion"`
}

type OpAddAddrBookRecordDeps struct {
	AddrBook deployment.AddressBook
}

var OpAddAddrBookRecord = operations.NewOperation(
	"add-address-book-record",
	semver.MustParse("1.0.0"),
	"Adds an address record to address book",
	func(b operations.Bundle, deps OpAddAddrBookRecordDeps, in OpAddAddrBookRecordInput) (OpAddAddrBookRecordOutput, error) {
		out := OpAddAddrBookRecordOutput{}

		tv := deployment.NewTypeAndVersion(
			deployment.ContractType(in.Type),
			*semver.MustParse(in.Version),
		)

		for _, label := range in.Labels {
			tv.AddLabel(label)
		}

		if err := deps.AddrBook.Save(in.ChainSelector, in.Address, tv); err != nil {
			return out, err
		}

		return OpAddAddrBookRecordOutput{
			ChainSelector:  in.ChainSelector,
			Address:        in.Address,
			TypeAndVersion: tv.String(),
		}, nil
	})

type OpAddDatastoreAddrRefInput struct {
	ChainSelector uint64
	Address       string
	Qualifier     string
	Type          string
	Version       string
	Labels        []string
}

type OpAddDatastoreAddrRefOutput struct {
	ChainSelector uint64 `json:"chainSelector"`
	Address       string `json:"address"`
}

type OpAddDatastoreAddrRefDeps struct {
	Datastore datastore.MutableDataStore[
		datastore.DefaultMetadata,
		datastore.DefaultMetadata,
	]
}

// OpAddDatastoreAddrRef adds a new address reference to the datastore.
var OpAddDatastoreAddrRef = operations.NewOperation(
	"add-datastore-address-reference",
	semver.MustParse("1.0.0"),
	"Adds an address reference to the datastore",
	func(b operations.Bundle, deps OpAddDatastoreAddrRefDeps, input OpAddDatastoreAddrRefInput) (OpAddDatastoreAddrRefOutput, error) {
		out := OpAddDatastoreAddrRefOutput{}

		labels := make(datastore.LabelSet, len(input.Labels))
		for _, label := range input.Labels {
			labels[label] = struct{}{}
		}

		if err := deps.Datastore.Addresses().Add(
			datastore.AddressRef{
				ChainSelector: input.ChainSelector,
				Address:       input.Address,
				Type:          datastore.ContractType(input.Type),
				Version:       semver.MustParse(input.Version),
				Qualifier:     input.Qualifier,
				Labels:        labels,
			},
		); err != nil {
			return out, err
		}

		return OpAddDatastoreAddrRefOutput{
			ChainSelector: input.ChainSelector,
			Address:       input.Address,
		}, nil
	})
