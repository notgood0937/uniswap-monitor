package main

import (
	"UniswapStalker/erc20"
	v2 "UniswapStalker/v2"
	v3 "UniswapStalker/v3"
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
	"log"
	"math"
	"math/big"
	"os"
	"strconv"
	"strings"
)

const (
	//main
	//infuraURL = "wss://eth-mainnet.g.alchemy.com/v2/XrljRQCnLOediLbLZ2jdByDj_LMcu73D"
	//uniswapv2Factory = "0x5C69bEe701ef814a2B6a3EDD4B1652CB9cc5aA6f"
	uniswapv3Factory = "0x1F98431c8aD98523631AE4a59f267346ea31F984"

	WrapETHAddress = "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"

	//base
	//infuraURL        = "wss://base-mainnet.g.alchemy.com/v2/XrljRQCnLOediLbLZ2jdByDj_LMcu73D"
	//uniswapv2Factory = "0x33128a8fC17869897dcE68Ed026d694621f6FDfD"
)

type Token struct {
	Address     common.Address
	Name        string
	Symbol      string
	Decimals    uint8
	TotalSupply *big.Int
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
	totalSupply, err := token0.TotalSupply(&bind.CallOpts{})
	if err != nil {
		panic(err)
	}
	tokenInfo.TotalSupply = totalSupply

	tokenInfo.Address = token
	return tokenInfo
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	// Connect to the Ethereum client
	provider := os.Getenv("INFURA_URL")
	botToken := os.Getenv("BOT_TOKEN")
	bot, err := telego.NewBot(botToken, telego.WithDefaultDebugLogger())
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	client, err := ethclient.Dial(provider)
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}
	defer client.Close()

	//监听uniswap swap事件日志
	//SwapEventListen(client)
	//监听uniswap交易对创建事件
	PairCreatedEventListen(client, bot)

}

func PairCreatedEventListen(client *ethclient.Client, bot *telego.Bot) {
	pairCreatedSigHash := common.HexToHash("0x0d3648bd0f6ba80134a33ba9275ac585d9d315f0ad8355cddefde31afa28d0e9")
	factoryAddress := "0x5C69bEe701ef814a2B6a3EDD4B1652CB9cc5aA6f"
	v2FactoryAddress := common.HexToAddress(factoryAddress)
	v2Factory, err := v2.NewV2Factory(v2FactoryAddress, client)
	senderAddress := ""
	senderBalance := new(big.Float)
	if err != nil {
		log.Fatalf("Failed to create instance of factory contract: %v", err)
	}
	query := ethereum.FilterQuery{
		Addresses: []common.Address{v2FactoryAddress},
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
			switch vLog.Topics[0].Hex() {
			case pairCreatedSigHash.Hex():
				event, err := v2Factory.ParsePairCreated(vLog)
				if err != nil {
					log.Fatalf("Failed to parse pair created event: %v", err)
				}
				//fmt.Println(event)
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
							//fmt.Println("sender:", from.Hex())
							senderAddress = from.Hex()
							fmt.Println("sender:", senderAddress)
							balance, err := client.BalanceAt(context.Background(), from, nil)
							if err != nil {
								log.Fatal(err)
							}
							b1 := new(big.Float)
							b1.SetString(balance.String())
							senderBalance = new(big.Float).Quo(b1, big.NewFloat(math.Pow10(18)))

						}
					}
				}
				//只打印weth交易对
				weth := common.HexToAddress(WrapETHAddress).Hex()
				linkPreviewOptions := &telego.LinkPreviewOptions{IsDisabled: true}
				if strings.Contains(common.HexToAddress(event.Token0.Hex()).Hex(), weth) || strings.Contains(common.HexToAddress(event.Token1.Hex()).Hex(), weth) {
					if strings.Contains(common.HexToAddress(event.Token0.Hex()).Hex(), weth) {
						Token1 := getTokenInfo(client, event.Token1)
						t1 := new(big.Float)
						t1.SetString(Token1.TotalSupply.String())
						ts := new(big.Float).Quo(t1, big.NewFloat(math.Pow10(int(Token1.Decimals))))
						//message :=fmt.Printf("PairCreated event:\n Token: name: %s address: %s totalsuppy:%f  pair: %s\n", Token1.Symbol, event.Token1.Hex(), ts, event.Pair)

						message := fmt.Sprintf("*新币部署*\n\n代币:%s\\(%s\\)\n合约:%s总量:%d\\(精度%d\\)\n创建人地址:%s\n", Token1.Symbol, Token1.Name, event.Token1.Hex(), ts, Token1.Decimals, senderAddress)
						fmt.Println(message)
						_, err2 := bot.SendMessage(
							tu.Message(
								tu.ID(-1002228256981),
								"<b>✨ 新币部署 ✨\n"+"\n"+
									"代币:"+fmt.Sprintf("<a href='https://etherscan.io/token/%s'>%s(%s)</a>", event.Token1.Hex(), Token1.Symbol, Token1.Name)+"\n"+
									"合约:<code>"+event.Token1.Hex()+"</code>\n"+
									"总量:"+ts.String()+" 精度"+strconv.Itoa(int(Token1.Decimals))+"\n"+
									"创建人:\n\t-地址:<code>"+senderAddress+"</code>\n"+
									"\t-余额🚰:"+senderBalance.String()+" ETH"+
									"</b>",
							).WithParseMode(telego.ModeHTML).WithLinkPreviewOptions(linkPreviewOptions),
						)
						if err2 != nil {
							log.Fatalf("send message err")
						}
						go MarketOpenEventListen(client, event.Pair, bot, Token1)

					} else {
						Token0 := getTokenInfo(client, event.Token0)
						t0 := new(big.Float)
						t0.SetString(Token0.TotalSupply.String())
						ts := new(big.Float).Quo(t0, big.NewFloat(math.Pow10(int(Token0.Decimals))))
						//fmt.Printf("PairCreated event:\n Token: name: %s address: %s  totalsuppy:%f  pair: %s\n", Token0.Symbol, event.Token0, ts, event.Pair)

						message := fmt.Sprintf("*新币部署*\n\n代币:%s\\(%s\\)\n合约:%s\n总量:%d\\(精度%d\\)\n创建人地址:%s\n", Token0.Symbol, Token0.Name, event.Token0.Hex(), ts, Token0.Decimals, senderAddress)
						fmt.Println(message)
						_, err2 := bot.SendMessage(
							tu.Message(
								tu.ID(-1002228256981),
								"<b>✨ 新币部署 ✨\n"+"\n"+
									"代币:"+fmt.Sprintf("<a href='https://etherscan.io/token/%s'>%s(%s)</a>", event.Token0.Hex(), Token0.Symbol, Token0.Name)+"\n"+
									"合约:<code>"+event.Token0.Hex()+"</code>\n"+
									"总量:"+ts.String()+" 精度"+strconv.Itoa(int(Token0.Decimals))+"\n"+
									"创建人:\n\t-地址:<code>"+senderAddress+"</code>\n"+
									"\t-余额🚰:"+senderBalance.String()+" ETH"+
									"</b>",
							).WithParseMode(telego.ModeHTML).WithLinkPreviewOptions(linkPreviewOptions),
						)
						if err2 != nil {
							log.Fatalf("send message err")
						}
						go MarketOpenEventListen(client, event.Pair, bot, Token0)
					}
				}

				//todo
				//	若pair address在pool内发生swap事件，则认为该币开盘

			}

		}
	}

}

func MarketOpenEventListen(client *ethclient.Client, pool common.Address, bot *telego.Bot, token *Token) {
	v2SwapSigHash := common.HexToHash("0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822")
	v2PairAddress := common.HexToAddress("0xC75650fe4D14017b1e12341A97721D5ec51D5340")
	linkPreviewOptions := &telego.LinkPreviewOptions{IsDisabled: true}
	tgGroupId := os.Getenv("TG_GROUP_ID")
	id, err := strconv.ParseInt(tgGroupId, 10, 64)
	if err != nil {
		fmt.Println("转换错误:", err)
		return
	}
	v2Pair, err := v2.NewV2Pair(v2PairAddress, client)
	if err != nil {
		log.Fatalf("Failed to create instance of factory contract: %v", err)
	}
	query := ethereum.FilterQuery{
		Addresses: []common.Address{pool},
	}
	logs := make(chan types.Log)
	sub, err := client.SubscribeFilterLogs(context.Background(), query, logs)
	if err != nil {
		log.Fatalf("Failed to subscribe to logs: %v", err)
	}
	defer sub.Unsubscribe()
	done := make(chan bool)
	for {
		fmt.Println(done)
		select {
		case <-done:
			log.Println("goroutine exiting")
			fmt.Println("goroutine exiting")
			break
		case err := <-sub.Err():
			log.Fatalf("Error while listening for logs: %v", err)
		case vLog := <-logs:
			switch vLog.Topics[0].Hex() {
			case v2SwapSigHash.Hex():
				event, err := v2Pair.ParseSwap(vLog)
				if err != nil {
					log.Fatalf("Failed to parse log: %v", err)
				}
				fmt.Println("market open --------")
				fmt.Println("pool:", pool.Hex())
				fmt.Println(event)
				inlineKeyboard := tu.InlineKeyboard(
					tu.InlineKeyboardRow( // Row 2
						tu.InlineKeyboardButton("Trading on UniSwap🐴").WithURL("https://app.uniswap.org/explore/tokens/ethereum/" + token.Address.Hex()), // Column 1
					),
				)
				_, err2 := bot.SendMessage(
					tu.Message(
						tu.ID(id),
						"<b>🚀 新币开盘 🚀\n"+"\n"+
							"代币:"+fmt.Sprintf("<a href='https://etherscan.io/token/%s'>%s(%s)</a>", token.Address.Hex(), token.Symbol, token.Name)+"\n"+
							"合约:<code>"+token.Address.Hex()+"</code>\n"+
							"Lp:<code>"+pool.Hex()+"</code>\n\n"+
							"交易对:"+token.Symbol+"/WETH\n"+
							"DEX:"+"Uniswap-v2\n"+
							"</b>",
					).WithParseMode(telego.ModeHTML).WithLinkPreviewOptions(linkPreviewOptions).WithReplyMarkup(inlineKeyboard),
				)
				if err2 != nil {
					log.Fatalf("send message err")
				}
				done <- true
				//sub.Unsubscribe()
			}
		}
	}
	return
}

func SwapEventListen(client *ethclient.Client) {
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
				//判断那个是weth

				//if event.Amount1.Cmp(big.NewInt(0)) < 0 {
				//	f0balance := new(big.Float)
				//	f0balance.SetString(event.Amount0.String())
				//	f1balance := new(big.Float)
				//	f1balance.SetString(event.Amount1.String())
				//	token0Value := new(big.Float).Quo(f0balance, big.NewFloat(math.Pow10(int(token1Info.Decimals))))
				//	token1Value := new(big.Float).Quo(f1balance, big.NewFloat(math.Pow10(int(token0Info.Decimals))))
				//	fmt.Println("Swap event:Buy")
				//	fmt.Printf("buy %f %s use %f %s\n", new(big.Float).Abs(token1Value), token0Info.Symbol, new(big.Float).Abs(token0Value), token1Info.Symbol)
				//	fmt.Println(event.Tick)
				//} else {
				//	f0balance := new(big.Float)
				//	f0balance.SetString(event.Amount0.String())
				//	f1balance := new(big.Float)
				//	f1balance.SetString(event.Amount1.String())
				//	token0Value := new(big.Float).Quo(f0balance, big.NewFloat(math.Pow10(int(token1Info.Decimals))))
				//	token1Value := new(big.Float).Quo(f1balance, big.NewFloat(math.Pow10(int(token0Info.Decimals))))
				//	fmt.Println("Swap event:Sell")
				//	fmt.Printf("sell %f %s to %f %s\n", new(big.Float).Abs(token1Value), token0Info.Symbol, new(big.Float).Abs(token0Value), token1Info.Symbol)
				//	fmt.Println(event.Tick)
				//}
				//todo 计算价格
				//可以根据 event.SqrtPriceX96
				fmt.Printf("Recipient: %s\nSender: %s\nAmount0: %s\nAmount1: %s\n", event.Recipient.Hex(), event.Sender.Hex(), event.Amount0, event.Amount1)
			default:
				fmt.Println("other:")
				fmt.Println(vLog.Topics[0])
				// Log the event details
			}
		}
	}
}

//func SqrtPriceX96ToPrice(sqrtPriceX96 *big.Int) float64 {
//	// 2^96
//	twoPow96 := big.NewInt(2).Exp(big.NewInt(2), big.NewInt(96), nil)
//
//	// 计算 price
//	price := new(big.Float).Quo(new(big.Float).SetInt(sqrtPriceX96), new(big.Float).SetInt(twoPow96))
//	price.Sqrt(price)
//
//	// 转换为浮点数
//	priceFloat, _ := price.Float64()
//
//	return priceFloat
//}
