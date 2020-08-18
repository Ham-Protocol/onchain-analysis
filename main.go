package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/big"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/joho/godotenv"
)

type Token struct {
	address string
	symbol  string
}

type TokenOwner struct {
	address string
	amount  *big.Int
	percent float64
}

type Transaction struct {
	BlockNumber     string
	TimeStamp       string
	Hash            string
	Nonce           string
	BlockHash       string
	From            string
	ContractAddress string
	To              string
	Value           string
	TokenName       string
	TokenSymbol     string
}

type Response struct {
	Status  string
	Message string
	Result  []*Transaction
}

var oneGwei = big.NewInt(int64(math.Pow(10, 9)))
var decimals = big.NewInt(0).Mul(oneGwei, oneGwei)

var (
	yamDeployBlock = 10636698
	yamFirstRebase = 1061000
	preYCRVPool    = 10645800
	//peakYamBlock = 10648400
	url       = fmt.Sprintf("https://api.etherscan.io/api?module=account&action=tokentx&address=%%s&startblock=%d&endblock=%d&sort=asc&apikey=%%s", yamDeployBlock, preYCRVPool)
	poolAddrs = []*Token{
		{"0x6009a344c7f993b16eba2c673fefd2e07f9be5fd", "LEND"},
		{"0xFDC28897A1E32B595f1f4f1D3aE0Df93B1eee452", "LINK"},
		{"0xcFe1E539AcB2D489a651cA011a6eB93d32f97E23", "MKR"},
		{"0x6c3FC1FFDb14D92394f40eeC91D9Ce8B807f132D", "SNX"},
		{"0x587A07cE5c265A38Dd6d42def1566BA73eeb06F5", "WETH"},
		{"0xc5B6488c7D5BeD173B76Bd5DCA712f45fB9EaEaB", "YFI"},
		{"0x8538e5910c6f80419cd3170c26073ff238048c9e", "COMP"},
		//{"0xaddbcd6a68bfeb6e312e82b30ce1eb4a54497f4c", "UNI-V2"}, // YAM_YCRV_UNI_LP.
		//{"0x9EbB67687FEE2d265D7b824714DF13622D90E663", "UNI-V2"}, // AMPL_ETH_UNI_LP.
	}
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	etherscanAPI := os.Getenv("ETHERSCAN_API")
	for _, pool := range poolAddrs {
		resp, err := http.Get(fmt.Sprintf(url, pool.address, etherscanAPI))
		if err != nil {
			fmt.Printf("%s", err)
			os.Exit(1)
		}
		contents, err := ioutil.ReadAll(resp.Body)
		if err != nil {
		}
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("%s", err)
			os.Exit(1)
		}
		var respObj Response
		if err := json.Unmarshal(contents, &respObj); err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}
		// Get map of holders from etherscan response.
		holders := holdersFromResponse(respObj.Result, pool)
		// Accumulate all deposits and turn holders into an array for easy sorting.
		holderArray, totalTokens := totalAndArrayFromMap(holders)

		fmt.Printf("\nDistribution for %s out of total %d tokens", pool.symbol, totalTokens.Uint64())
		formatStats(holderArray)
	}
}

func holdersFromResponse(transactions []*Transaction, pool *Token) map[string]*big.Int {
	holders := make(map[string]*big.Int, 3000)
	for _, tx := range transactions {
		if tx.TokenSymbol != pool.symbol {
			continue
		}
		value, ok := big.NewInt(0).SetString(tx.Value, 10)
		if !ok {
			fmt.Printf("could not convert %s\n", tx.Value)
			os.Exit(1)
		}

		// Etherscan gives us all token transactions to and from an address, so we
		// build up user balances using the transactions.
		toAddr := strings.ToLower(tx.To)
		fromAddr := strings.ToLower(tx.From)
		if toAddr == strings.ToLower(pool.address) {
			currentBal := big.NewInt(0)
			if holders[fromAddr] != nil {
				currentBal = holders[fromAddr]
			}
			holders[fromAddr] = currentBal.Add(currentBal, value)
		} else if fromAddr == strings.ToLower(pool.address) {
			currentBal := big.NewInt(0)
			if holders[toAddr] != nil {
				currentBal = holders[toAddr]
			}
			holders[toAddr] = currentBal.Sub(currentBal, value)
		}
	}
	return holders
}

func totalAndArrayFromMap(holders map[string]*big.Int) ([]*TokenOwner, *big.Int) {
	var holderArray []*TokenOwner
	totalTokens := big.NewInt(0)
	// Accumulate tokens and add each address to the array for sorting.
	for addr, tokens := range holders {
		humanTokens := tokens.Div(tokens, decimals)
		totalTokens.Add(totalTokens, humanTokens)
		holderArray = append(holderArray, &TokenOwner{address: addr, amount: humanTokens})
	}
	// Sort by balance in pool.
	sort.SliceStable(holderArray, func(i, j int) bool {
		return holderArray[i].amount.Cmp(holderArray[j].amount) > 0
	})
	// Calculate the percentage per holder.
	for i, holder := range holderArray {
		holderArray[i].percent = float64(holder.amount.Int64()) / float64(totalTokens.Int64()) * 100
	}
	return holderArray, totalTokens
}

func formatStats(holders []*TokenOwner) {
	var accumulatedPercent float64
	var accumulatedAmount uint64
	for i, holder := range holders {
		accumulatedPercent += holder.percent
		accumulatedAmount += holder.amount.Uint64()
		if i == 0 {
			fmt.Printf("\nThe top 1 holder has %.4f%% of pool (%d tokens)", accumulatedPercent, accumulatedAmount)
		} else if i == 4 {
			fmt.Printf("\nThe top 5 holders have %.4f%% of pool (%d tokens)", accumulatedPercent, accumulatedAmount)
		} else if i == 9 {
			fmt.Printf("\nThe top 10 holders have %.4f%% of pool (%d tokens)", accumulatedPercent, accumulatedAmount)
		} else if i == 24 {
			fmt.Printf("\nThe top 25 holders have %.4f%% of pool (%d tokens)", accumulatedPercent, accumulatedAmount)
		} else if i == 49 {
			fmt.Printf("\nThe top 50 holders have %.4f%% of pool (%d tokens)", accumulatedPercent, accumulatedAmount)
		} else if i == 99 {
			fmt.Printf("\nThe top 100 holders have %.4f%% of pool (%d tokens)", accumulatedPercent, accumulatedAmount)
		} else if i == 249 {
			fmt.Printf("\nThe top 250 holders have %.4f%% of pool (%d tokens)", accumulatedPercent, accumulatedAmount)
		} else if i == len(holders)-1 {
			fmt.Printf("\n%d holders in total (%d tokens)", len(holders), accumulatedAmount)
		}
	}
}
