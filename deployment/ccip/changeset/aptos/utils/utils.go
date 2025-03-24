package utils

import (
	"fmt"
	"strings"

	"github.com/aptos-labs/aptos-go-sdk"
	"github.com/smartcontractkit/chainlink/deployment"
)

// TODO: This function will be used directly, but it need to be a parameter of AptosChain
// to be consistent to Evm/Solana pattern
func ConfirmTx(chain deployment.AptosChain, txHash string) error {
	userTx, err := chain.Client.WaitForTransaction(txHash)
	if err != nil {
		return err
	}
	if !userTx.Success {
		return fmt.Errorf("transaction failed: %s", userTx.VmStatus)
	}
	return nil
}

func IsMCMSStagingAreaClean(client aptos.AptosRpcClient, aptosMCMSObjAddr aptos.AccountAddress) (bool, error) {
	resources, err := client.AccountResources(aptosMCMSObjAddr)
	if err != nil {
		return false, err
	}
	for _, resource := range resources {
		if strings.Contains(resource.Type, "StagingArea") {
			return false, nil
		}
	}
	return true, nil
}
