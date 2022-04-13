package exporter

import (
	"context"
	"fmt"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	"log"
	"time"
)

func Run() {

	client, _ := rpchttp.New(endpoint, "/websocket")
	err := client.Start()
	if err != nil {
		log.Fatal(err)
	}
	defer client.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ibcMap, err = discoverIbcCurrencies(client)
	if err != nil {
		log.Fatal(err)
	}

	transactions := newTxs()
	// TODO: loop correctly!
	page, perPage := 1, 100
	resp, err := client.TxSearch(ctx, fmt.Sprintf("coin_received.receiver = '%s'", account), false, &page, &perPage, "desc")
	if err != nil {
		log.Fatal(err)
	}
	parseTxs(client, transactions, resp)
	resp, err = client.TxSearch(ctx, fmt.Sprintf("coin_spent.spender = '%s'", account), false, &page, &perPage, "desc")
	if err != nil {
		log.Fatal(err)
	}
	parseTxs(client, transactions, resp)

	fmt.Println(csvHeader)
	transactions.sort()
	for i := range transactions.Actions {
		fmt.Print(string(transactions.Actions[i].toCsv()))
	}
}




