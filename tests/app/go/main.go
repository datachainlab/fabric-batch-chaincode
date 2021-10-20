package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric-sdk-go/pkg/client/channel/invoke"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/errors/status"
	"github.com/hyperledger/fabric-sdk-go/pkg/common/providers/fab"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/gateway"
	"github.com/pkg/errors"
)

const (
	ccID      = "counter"
	channelID = "mychannel"
	orgName   = "org1.example.com"
	orgAdmin  = "Admin"
)

// this test case ensures:
// - succeed to submit Msgs concurrently
// - theses Msgs are committed and its counter matches expected value
func testConcurrentSubmitMsg() {
	contract := getContract()
	beforeCount, err := getCount(contract)
	if err != nil {
		panic(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err = contract.SubmitTransaction("BatchIncrWithTimestamp", "1", fmt.Sprint(time.Now().Unix()))
			if err != nil {
				panic(fmt.Sprintf("Failed to commit transaction %v", err))
			}
		}()
	}
	wg.Wait()
	now := time.Now()
	time.Sleep(time.Second)

	countBytes, err := contract.SubmitTransaction("Commit", fmt.Sprint(now.Unix()))
	if err != nil {
		panic(fmt.Sprintf("Failed to commit transaction: commitTime=%v err=%v\n", now.Unix(), err))
	}
	num := mustParseInt64(string(countBytes))
	if num != 100 {
		panic(fmt.Sprintf("unexpected commit number: %v", num))
	}

	afterCount, err := getCount(contract)
	if err != nil {
		panic(err)
	}
	if beforeCount+100 != afterCount {
		panic(fmt.Sprintf("%v != %v", beforeCount+100, afterCount))
	}
}

// this test case ensures:
// - failed to submit a Msg referencing a committed time-range
func testCommitSafety() {
	contract := getContract()

	beforeCount, err := getCount(contract)
	if err != nil {
		panic(err)
	}
	msgTime := time.Now().Add(-time.Second)

	tx, err := contract.CreateTransaction("BatchIncrWithTimestamp")
	if err != nil {
		panic(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := tx.SimulateWithHandler(delaySubmitHandler{5 * time.Second}, "1", fmt.Sprint(msgTime.Unix()))
			if err == nil {
				panic("must be failed")
			} else if !strings.Contains(err.Error(), "PHANTOM_READ_CONFLICT") {
				panic(err)
			}
		}()
	}
	commitTime := msgTime
	countBytes, err := contract.SubmitTransaction("Commit", fmt.Sprint(commitTime.Unix()))
	if err != nil {
		panic(fmt.Sprintf("Failed to commit transaction: commitTime=%v err=%v\n", commitTime.Unix(), err))
	}
	num := mustParseInt64(string(countBytes))
	if num != 0 {
		panic(fmt.Sprintf("unexpected commit number: %v", num))
	}
	wg.Wait()

	afterCount, err := getCount(contract)
	if err != nil {
		panic(err)
	}
	if beforeCount != afterCount {
		panic(fmt.Sprintf("%v != %v", beforeCount, afterCount))
	}
}

func getContract() *gateway.Contract {
	wallet, err := gateway.NewFileSystemWallet("./wallets")
	if err != nil {
		fmt.Printf("Failed to create wallet: %s\n", err)
		os.Exit(1)
	}

	if !wallet.Exists("Admin") {
		fmt.Println("Failed to get Admin from wallet")
		os.Exit(1)
	}

	gw, err := gateway.Connect(
		gateway.WithConfig(config.FromFile("./connection.json")),
		gateway.WithIdentity(wallet, "Admin"),
	)

	if err != nil {
		fmt.Printf("Failed to connect: %v", err)
		os.Exit(1)
	}

	if gw == nil {
		fmt.Println("Failed to create gateway")
		os.Exit(1)
	}

	network, err := gw.GetNetwork(channelID)
	if err != nil {
		fmt.Printf("Failed to get network: %v", err)
		os.Exit(1)
	}
	return network.GetContract(ccID)
}

func getCount(contract *gateway.Contract) (int64, error) {
	var v []byte
	var err error
	for i := 0; i < 5; i++ {
		// if err is not nil, the endorsers may not have synchronized their states yet.
		v, err = contract.EvaluateTransaction("count")
		if err == nil {
			return mustParseInt64(string(v)), nil
		}
		time.Sleep(time.Second)
	}
	return 0, err
}

func mustParseInt64(s string) int64 {
	count, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return int64(count)
}

type delaySubmitHandler struct {
	delay time.Duration
}

func (h delaySubmitHandler) Handle(requestContext *invoke.RequestContext, clientContext *invoke.ClientContext) {
	txnID := requestContext.Response.TransactionID

	//Register Tx event
	reg, statusNotifier, err := clientContext.EventService.RegisterTxStatusEvent(string(txnID)) // TODO: Change func to use TransactionID instead of string
	if err != nil {
		requestContext.Error = errors.Wrap(err, "error registering for TxStatus event")
		return
	}
	defer clientContext.EventService.Unregister(reg)
	time.Sleep(h.delay)
	_, err = createAndSendTransaction(clientContext.Transactor, requestContext.Response.Proposal, requestContext.Response.Responses)
	if err != nil {
		requestContext.Error = errors.Wrap(err, "CreateAndSendTransaction failed")
		return
	}

	select {
	case txStatus := <-statusNotifier:
		requestContext.Response.TxValidationCode = txStatus.TxValidationCode

		if txStatus.TxValidationCode != peer.TxValidationCode_VALID {
			requestContext.Error = status.New(status.EventServerStatus, int32(txStatus.TxValidationCode),
				"received invalid transaction", nil)
			return
		}
	case <-requestContext.Ctx.Done():
		requestContext.Error = status.New(status.ClientStatus, status.Timeout.ToInt32(),
			"Execute didn't receive block event", nil)
		return
	}
}

func createAndSendTransaction(sender fab.Sender, proposal *fab.TransactionProposal, resps []*fab.TransactionProposalResponse) (*fab.TransactionResponse, error) {
	txnRequest := fab.TransactionRequest{
		Proposal:          proposal,
		ProposalResponses: resps,
	}
	tx, err := sender.CreateTransaction(txnRequest)
	if err != nil {
		return nil, errors.WithMessage(err, "CreateTransaction failed")
	}
	transactionResponse, err := sender.SendTransaction(tx)
	if err != nil {
		return nil, errors.WithMessage(err, "SendTransaction failed")

	}
	return transactionResponse, nil
}

func main() {
	testConcurrentSubmitMsg()
	testCommitSafety()
}
