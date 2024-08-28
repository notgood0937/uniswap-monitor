package main

import (
	"UniswapStalker/erc20"
	v3 "UniswapStalker/v3"
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"log"
	"math/big"
)

const (
	//main
	infuraURL = "wss://eth-mainnet.g.alchemy.com/v2/XrljRQCnLOediLbLZ2jdByDj_LMcu73D"
	//uniswapv2Factory = "0x5C69bEe701ef814a2B6a3EDD4B1652CB9cc5aA6f"
	uniswapv3Factory = "0x1F98431c8aD98523631AE4a59f267346ea31F984"

	//base
	//infuraURL        = "wss://base-mainnet.g.alchemy.com/v2/XrljRQCnLOediLbLZ2jdByDj_LMcu73D"
	//uniswapv2Factory = "0x33128a8fC17869897dcE68Ed026d694621f6FDfD"
)

type Token struct {
	Name     string
	Symbol   string
	Decimals uint8
}

func getTokenInfo(client *ethclient.Client, token common.Address) *Token {
	token0, err := erc20.NewErc20(token, client)
	var tokenInfo = new(Token)
	if err != nil {
		panic(err)
	}
	// 查询代币信息
	name, err := token0.Name(&bind.CallOpts{})

	if err != nil {
		panic(err)
	}
	tokenInfo.Name = name
	symbol, err := token0.Symbol(&bind.CallOpts{})

	if err != nil {
		panic(err)
	}
	tokenInfo.Symbol = symbol
	decimals, err := token0.Decimals(&bind.CallOpts{})
	if err != nil {
		panic(err)
	}
	tokenInfo.Decimals = decimals
	return tokenInfo
}

func main() {
	// Connect to the Ethereum client
	client, err := ethclient.Dial(infuraURL)
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}
	defer client.Close()

	// Replace with the actual address of the Uniswap V2 factory contract
	Token0Address := common.HexToAddress("0xEE2a03Aa6Dacf51C18679C516ad5283d8E7C2637")
	wethAddress := common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")
	v3FactoryAddress := common.HexToAddress(uniswapv3Factory)

	logSwapSigHash := common.HexToHash("0xc42079f94a6350d7e6235f29174924f928cc2ac818eb64fed8004e115fbcca67")
	//fmt.Println(logSwapSigHash)
	factory, err := v3.NewV3(v3FactoryAddress, client)

	if err != nil {
		log.Fatalf("Failed to create instance of factory contract: %v", err)
	}

	token0Info := getTokenInfo(client, Token0Address)
	token1Info := getTokenInfo(client, wethAddress)
	fmt.Printf("token0 name: %s\nsymbol: %s\ndecimals: %d\n", token0Info.Name, token0Info.Symbol, token0Info.Decimals)
	fmt.Printf("token1 name: %s\nsymbol: %s\ndecimals: %d\n", token1Info.Name, token1Info.Symbol, token1Info.Decimals)
	fee := big.NewInt(3000)
	pool, err := factory.GetPool(&bind.CallOpts{}, Token0Address, wethAddress, fee)
	if err != nil {
		log.Fatalf("Failed1 %v", err)
	}
	// Create an instance of the factory contract
	routerAddress := pool
	fmt.Println(routerAddress.Hex())
	router, err := v3.NewV3Router(routerAddress, client)
	if err != nil {
		log.Fatalf("Failed to create instance of factory contract: %v", err)
	}

	// Set up a filter query to listen for PairCreated events
	query := ethereum.FilterQuery{
		Addresses: []common.Address{routerAddress},
	}

	// Subscribe to the PairCreated events
	logs := make(chan types.Log)
	sub, err := client.SubscribeFilterLogs(context.Background(), query, logs)
	if err != nil {
		log.Fatalf("Failed to subscribe to logs: %v", err)
	}
	defer sub.Unsubscribe()

	fmt.Println("Listening for PairCreated events...")

	for {
		select {
		case err := <-sub.Err():
			log.Fatalf("Error while listening for logs: %v", err)
		case vLog := <-logs:
			// Debugging: print the raw log
			fmt.Printf("Raw log: %+v\n", vLog.TxHash.Hex())
			block, err := client.BlockByNumber(context.Background(), big.NewInt(int64(vLog.BlockNumber)))
			if err != nil {
				log.Fatal(err)
			}
			for _, tx := range block.Transactions() {
				if tx.Hash() == vLog.TxHash {
					chainID, err := client.NetworkID(context.Background())
					if err != nil {
						log.Fatal(err)
					}
					if from, err := types.Sender(types.NewLondonSigner(chainID), tx); err == nil {
						fmt.Println("sender:", from.Hex())
					}
				}
			}
			switch vLog.Topics[0].Hex() {
			case logSwapSigHash.Hex():
				event, err := router.ParseSwap(vLog)
				//Parse the log data
				if err != nil {
					log.Fatalf("Failed to parse log: %v", err)
				}
				if event.Amount1.Cmp(big.NewInt(0)) < 0 {
					fmt.Println("Swap event:Buy")
					fmt.Printf("buy %s %s use %s %s\n", event.Amount1, token0Info.Symbol, event.Amount0, token1Info.Symbol)
				} else {
					fmt.Println("Swap event:Sell")
					fmt.Printf("sell %s %s to %s %s\n", event.Amount1, token0Info.Symbol, event.Amount0, token1Info.Symbol)
				}
				//todo 计算价格
				//可以根据 event.SqrtPriceX96

				//fmt.Printf("Recipient: %s\nSender: %s\nAmount0: %s\nAmount1: %s\n", event.Recipient.Hex(), event.Sender.Hex(), event.Amount0, event.Amount1)
			default:
				fmt.Println("other:")
				fmt.Println(vLog.Topics[0])
				// Log the event details
			}
		}
	}
}
