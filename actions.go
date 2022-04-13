package exporter

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/tendermint/tendermint/abci/types"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

func parseTxs(client *rpchttp.HTTP, transactions *Txs, resp *coretypes.ResultTxSearch) {
	for _, tx := range resp.Txs {
		if transactions.isDuplicate(hex.EncodeToString(tx.Tx.Hash())) {
			continue
		}
		blockTime, e := getBlockTime(client, tx.Height)
		if e != nil {
			log.Fatal("could not determine block time, this is fatal", e)
		}
		extraInfo := getEventDescription(tx)
		var skipNextSpend, wasDelegate bool
		for j, event := range tx.TxResult.Events {
			if event.Type != "coin_spent" && event.Type != "coin_received" {
				// no value transferred.
				continue
			}
			newEvent := &TxAction{
				Date:        blockTime,
				TxHash:      hex.EncodeToString(tx.Tx.Hash()),
				Index:       j,
				Description: event.Type + extraInfo,
				Height:      tx.Height,
			}
			wasDelegate = newEvent.addAttribute(extraInfo, event.Type, event.Attributes)
			// the first coin_spent on a delegate is the fee, don't count the second which is the delegated amount
			if wasDelegate && !skipNextSpend {
				skipNextSpend = true
			} else if skipNextSpend && event.Type == "coin_spent" {
				continue
			}
			if newEvent.hasValue() {
				if strings.Contains(extraInfo, "Withdraw") ||
					strings.Contains(extraInfo, "Delegate") ||
					strings.Contains(extraInfo, "BeginRedelegate") ||
					newEvent.Description == "coin_received - Authz Claim" ||
					newEvent.Description == "coin_spent - Vote" {

					if newEvent.RecievedAmount > 0 {
						newEvent.Label = "reward"
					} else if newEvent.SentAmount > 0 {
						newEvent.Label = "cost"
					}
				}
				transactions.Actions = append(transactions.Actions, newEvent)

				// special case for marble airdrop claims, this could get complex.
				// this will probably expand for others using airdrop contracts.
				if extraInfo == airdropSignature {
					droppedTokens, tokenName, isClaim := isAirdrop(tx)
					if isClaim {
						transactions.Actions = append(transactions.Actions, &TxAction{
							Date:             blockTime,
							TxHash:           hex.EncodeToString(tx.Tx.Hash()),
							Index:            99,
							Description:      "airdrop claim" + extraInfo,
							Height:           tx.Height,
							RecievedAmount:   droppedTokens,
							ReceivedCurrency: tokenName,
							Label:            "airdrop",
						})
					}
				}
			}
		}
	}
}

type Txs struct {
	sync.Mutex

	DeDup   map[string]bool
	Actions []*TxAction
}

func newTxs() *Txs {
	return &Txs{
		DeDup:   make(map[string]bool),
		Actions: make([]*TxAction, 0),
	}
}

func (txs *Txs) isDuplicate(txid string) (dup bool) {
	txs.Lock()
	defer txs.Unlock()
	if txs.DeDup[txid] {
		return true
	}
	txs.DeDup[txid] = true
	return
}

func (txs *Txs) sort() {
	sort.Slice(txs.Actions, func(i, j int) bool {
		if txs.Actions[i].Height == txs.Actions[j].Height {
			return txs.Actions[i].Index < txs.Actions[j].Index
		}
		return txs.Actions[i].Height < txs.Actions[j].Height
	})
}

func getEventDescription(tx *coretypes.ResultTx) (actions string) {
	actionList := make(map[string]bool)
	var isAuthz bool
	for _, event := range tx.TxResult.Events {
		for _, attribute := range event.Attributes {
			if string(attribute.Key) == "action" && string(attribute.Value) == "/cosmos.authz.v1beta1.MsgExec" {
				isAuthz = true
			} else if string(attribute.Key) == "action" {
				actionSplit := strings.Split(string(attribute.Value), ".")
				if len(actionSplit) > 0 {
					actionList[strings.TrimPrefix(actionSplit[len(actionSplit)-1], "Msg")] = true
				}
			} else if isAuthz && string(attribute.Key) == "module" && string(attribute.Value) == "distribution" {
				actionList["Authz Claim"] = true
			}
		}
	}
	for k := range actionList {
		actions += " - " + k
	}
	return
}

type TxAction struct {
	Date             time.Time
	SentAmount       float64
	SentCurrency     string
	RecievedAmount   float64
	ReceivedCurrency string
	FeeAmount        float64
	FeeCurrency      string
	Label            string
	Description      string
	TxHash           string

	Index  int   // each tx has multiple events, track their order
	Height int64 // needed to sort at the end
}

func (t *TxAction) addAttribute(extraInfo string, evtType string, attrs []types.EventAttribute) (skipSent bool) {
	var tmpAmount string
	var add bool
	switch evtType {
	case "coin_spent":
		for _, attr := range attrs {
			if strings.Contains(extraInfo, "Authz Claim") {
				// this account didn't pay any fees for ReStake: determined by authz vs distribution module.
				break
			}
			if string(attr.Key) == "spender" && string(attr.Value) == account {
				add = true
			} else if string(attr.Key) == "amount" {
				tmpAmount = string(attr.Value)
			}
			// on a delegate event, the first spend is the fee, second is the delegation, don't count the delegation as a spend!
			if add && tmpAmount != "" && strings.Contains(extraInfo, "Delegate") {
				skipSent = true
				break
			}
		}
		if add {
			amt, token, _, e := parseAmount(tmpAmount)
			if e != nil {
				log.Println(e)
				return
			}
			t.Label = "withdrawal"
			t.SentAmount = amt
			t.SentCurrency = token
			return
		}

	case "coin_received":
		for _, attr := range attrs {
			if string(attr.Key) == "receiver" && string(attr.Value) == account {
				add = true
			} else if string(attr.Key) == "amount" {
				tmpAmount = string(attr.Value)
			}
		}
		if add {
			amt, token, _, e := parseAmount(tmpAmount)
			if e != nil {
				log.Println(e)
				return
			}
			t.Label = "deposit"
			t.RecievedAmount = amt
			t.ReceivedCurrency = token
			return
		}

	}

	return
}

func (t *TxAction) hasValue() (ok bool) {
	if t.SentAmount == 0 && t.FeeAmount == 0 && t.RecievedAmount == 0 {
		return false
	}
	ok = true
	if t.SentAmount == 0 && t.RecievedAmount == 0 {
		// tx with only a fee needs to send or koinly will reject it
		t.SentAmount = t.FeeAmount
		t.FeeAmount = 0
		t.SentCurrency = t.FeeCurrency
		t.FeeCurrency = ""
	}
	return
}

func (t TxAction) toCsv() []byte {
	return []byte(fmt.Sprintf(
		`"%s","%g","%s","%g","%s","%g","%s",,,"%s","%s","%s-%d"`,
		t.Date.UTC().Format(time.RFC822),
		t.SentAmount,
		t.SentCurrency,
		t.RecievedAmount,
		t.ReceivedCurrency,
		t.FeeAmount,
		t.FeeCurrency,
		t.Label,
		t.Description,
		t.TxHash,
		t.Index,
	) + "\n")
}

func getBlockTime(client *rpchttp.HTTP, height int64) (time.Time, error) {
	block, err := client.Block(context.Background(), &height)
	if err != nil {
		return time.Time{}, err
	}
	return block.Block.Time, nil
}
