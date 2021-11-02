package main

import (
	"fmt"

	"github.com/datachainlab/fabric-batch-chaincode/_examples/counter/chaincode"
	"github.com/datachainlab/fabric-batch-chaincode/batch"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

// An example for application

func main() {
	batchcc := &batch.BatchContract{
		FnRegistry:    map[string]batch.Fn{},
		Authenticator: batch.DefaultAuthenticator,

		TotalQueryLimit:        100000,
		MsgTimeGapAllowance:    16,
		CommitTimeGapAllowance: 0, // for test
		GetPeerTime:            batch.DefaultGetPeerTime,
	}
	contract := chaincode.NewSmartContract(batchcc)
	chaincode, err := contractapi.NewChaincode(contract)
	if err != nil {
		fmt.Printf("Error create batch chaincode: %s", err.Error())
		return
	}
	if err := chaincode.Start(); err != nil {
		fmt.Printf("Error starting batch chaincode: %s", err.Error())
	}
}
