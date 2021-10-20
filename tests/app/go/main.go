package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/gateway"
)

const (
	ccID      = "samplecc"
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

	network, err := gw.GetNetwork("mychannel")
	if err != nil {
		fmt.Printf("Failed to get network: %v", err)
		os.Exit(1)
	}

	contract := network.GetContract("counter")

	var wg sync.WaitGroup
	start := time.Now()
	for i := 1; i <= 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			result, err := contract.SubmitTransaction("BatchIncrWithTimestamp", fmt.Sprint(i))
			if err != nil {
				panic(fmt.Sprintf("Failed to commit transaction: %v", err))
			} else {
				fmt.Println("Commit is successful")
			}
			fmt.Printf("The results is %v", result)
		}(i)
	}
	wg.Wait()
	fmt.Println("The time took is ", time.Now().Sub(start))
}

func main() {
	useWalletGateway()
}
