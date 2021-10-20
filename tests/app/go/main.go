package main

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/gateway"
)

const (
	ccID      = "counter"
	channelID = "mychannel"
	orgName   = "org1.example.com"
	orgAdmin  = "Admin"
)

/*
To run this app, make sure that one of the wallet files such as Admin.id from
vars/profiles/vscode/wallets directory is copied onto ./wallets directory,
then this example code will use the wallet file and connection file to make
connections to Fabric network
*/
func useWalletGateway() {
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

	contract := network.GetContract(ccID)
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
	time.Sleep(6 * time.Second)

	_, err = contract.SubmitTransaction("Commit", fmt.Sprint(now.Unix()))
	if err != nil {
		panic(fmt.Sprintf("Failed to commit transaction: commitTime=%v err=%v\n", now.Unix(), err))
	}

	afterCount, err := getCount(contract)
	if err != nil {
		panic(err)
	}
	if beforeCount+100 != afterCount {
		panic(fmt.Sprintf("%v != %v", beforeCount+100, afterCount))
	}
}

func getCount(contract *gateway.Contract) (int64, error) {
	v, err := contract.EvaluateTransaction("count")
	if err != nil {
		return 0, err
	}
	count, err := strconv.Atoi(string(v))
	if err != nil {
		return 0, err
	}
	return int64(count), nil
}

func main() {
	useWalletGateway()
}
