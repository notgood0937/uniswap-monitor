package main

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
	"log"
	"math"
	"math/big"
	"os"
)

func main() {
	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	// Connect to the Ethereum client
	provider := os.Getenv("INFURA_URL")
	client, err := ethclient.Dial(provider)
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}
	defer client.Close()
	account := common.HexToAddress("0x71c7656ec7ab88b098defb751b7401b5f6d8976f")
	balance, err := client.BalanceAt(context.Background(), account, nil)
	if err != nil {
		log.Fatal(err)
	}
	b1 := new(big.Float)
	b1.SetString(balance.String())
	ts := new(big.Float).Quo(b1, big.NewFloat(math.Pow10(18)))
	fmt.Println("账户余额:", ts)
}
