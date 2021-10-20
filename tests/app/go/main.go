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

	tick := time.NewTicker(time.Second)
	for i := 0; i < 10; i++ {
		<-tick.C
		msgTime := time.Now()
		commitTime := msgTime.Add(time.Second)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := contract.SubmitTransaction("Commit", fmt.Sprint(i), fmt.Sprint(commitTime.Unix()))
			if err != nil {
				fmt.Printf("Failed to commit transaction: %v\n", err)
				os.Exit(1)
			}
		}()
		go func() {
			defer wg.Done()
			_, err := contract.SubmitTransaction("BatchIncrWithTimestamp", "1", fmt.Sprint(msgTime.Unix()))
			if err == nil {
				fmt.Printf("Failed to commit transaction: %v\n", err)
				os.Exit(1)
			}
		}()
		wg.Wait()
	}
}

func main() {
	useWalletGateway()
}
