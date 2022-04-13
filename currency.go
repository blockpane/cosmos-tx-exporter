package exporter

import (
	"context"
	"errors"
	"log"
	"math"
	"math/big"
	"regexp"
	"strings"
	"sync"

	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	ibctypes "github.com/cosmos/ibc-go/modules/apps/transfer/types"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
)

type currency struct {
	Precision   int    `yaml:"precision"`
	TokenSymbol string `yaml:"token_symbol"`
	Chain       string `yaml:"chain"`
}

type currencies struct {
	sync.Mutex

	Denoms map[string]*currency `yaml:"denoms"`
}


func (c *currencies) lookup(denom string) (precision int, token string, chain string, found bool) {
	c.Lock()
	defer c.Unlock()

	if strings.HasPrefix(denom, "ibc/") {
		denom = ibcMap.get(strings.TrimPrefix(denom, "ibc/"))
	}
	// guess based on prefix, not optimal so signal a guess using the bool
	if c.Denoms[denom] == nil {
		switch denom[0] {
		case 'u':
			precision = 6
			token = strings.ToUpper(denom[1:])
		case 'n':
			precision = 9
			token = strings.ToUpper(denom[1:])
		case 'a':
			precision = 18
			token = strings.ToUpper(denom[1:])
		default:
			precision = 6
			token = strings.ToUpper(denom)
		}
		chain = token
		return
	}

	return c.Denoms[denom].Precision, c.Denoms[denom].TokenSymbol, c.Denoms[denom].Chain, true
}

type ibcCurrency struct {
	sync.Mutex

	hash map[string]string
}

func (ibc *ibcCurrency) add(hash string, denom string) error {
	if hash == "" || denom == "" {
		return errors.New("hash or denom empty, skipping: "+hash)
	}
	ibc.Lock()
	defer ibc.Unlock()
	ibc.hash[strings.TrimPrefix(hash, "ibc/")] = denom
	return nil
}

func (ibc *ibcCurrency) get(hash string) string {
	ibc.Lock()
	defer ibc.Unlock()
	s := ibc.hash[strings.TrimPrefix(hash, "ibc/")]
	if s == "" {
		s = "unknown"
	}
	return s
}

func discoverIbcCurrencies(client *rpchttp.HTTP) (*ibcCurrency, error) {
	resp, err := client.ABCIQuery(context.Background(), `/cosmos.bank.v1beta1.Query/TotalSupply`, []byte{ 10, 0})
	if err != nil {
		return nil, err
	}

	coins := &banktypes.QueryTotalSupplyResponse{}
	err = coins.Unmarshal(resp.Response.GetValue())
	if err != nil {
		return nil, err
	}
	ibcs := &ibcCurrency{
		hash: make(map[string]string),
	}

	for _, coin := range coins.Supply {
		if strings.HasPrefix(coin.Denom, `ibc/`) {
			func() {
				denomHash := strings.TrimPrefix(coin.Denom, `ibc/`)
				q := []byte{10, 64}
				q = append(q, []byte(denomHash)...)
				traceResp, e := client.ABCIQuery(context.Background(), `/ibc.applications.transfer.v1.Query/DenomTrace`, q)
				if e != nil {
					log.Println("error getting DenomTrace for: ", coin.Denom, e)
					return
				}
				denomTrace := &ibctypes.QueryDenomTraceResponse{}
				e = denomTrace.Unmarshal(traceResp.Response.GetValue())
				if e != nil {
					log.Println("error decoding DenomTrace for: ", coin.Denom, e)
					return
				}
				if denomTrace.DenomTrace.GetBaseDenom() == "" {
					return
				}
				_ = ibcs.add(denomHash, denomTrace.DenomTrace.GetBaseDenom())
			}()
		}
	}

	return ibcs, nil
}

var amtRex = regexp.MustCompile(`^\d+`)
var denomRex = regexp.MustCompile(`[a-zA-Z/]+$`)
var ibcRex = regexp.MustCompile(`ibc/\w+$`)

func parseAmount(amt string) (amount float64, token string, chain string, err error) {
	denom := ibcRex.FindString(amt)
	if denom == "" {
		denom = denomRex.FindString(amt)
		if denom == "" {
			err = errors.New("denom is empty string")
			return
		}
	}

	var precision int
	var found bool
	precision, token, chain, found = currencyMap.lookup(denom)
	if !found {
		log.Printf("warning: could not lookup token %s. Using guessed precision of %d, and token name %s", denom, precision, token)
	}

	// have to use big because of those chains using 18 digits :(
	bigAmount, _, err := new(big.Float).Parse(amtRex.FindString(amt), 10)
	if err != nil {
		return
	}
	a, _ := bigAmount.Float64()
	if a == 0 {
		err = errors.New("amount is 0")
	}

	var div float64 = 1
	if precision > 0 {
		div = math.Pow(10.0, float64(precision))
	}

	amount, _ = new(big.Float).Quo(bigAmount, big.NewFloat(div)).Float64()
	return
}
