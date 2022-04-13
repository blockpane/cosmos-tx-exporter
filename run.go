package exporter

import (
	"context"
	"fmt"
	rpchttp "github.com/tendermint/tendermint/rpc/client/http"
	"log"
	"time"
)

func Run() {
	defer f.Close()
	client, _ := rpchttp.New(endpoint, "/websocket")
	err := client.Start()
	if err != nil {
		log.Fatal(err)
	}
	defer client.Stop()
	//ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ibcMap, err = discoverIbcCurrencies(client)
	if err != nil {
		log.Fatal(err)
	}

	transactions := newTxs()

	go func() {
		for {
			time.Sleep(10 * time.Second)
			log.Printf("Processed %d transactions\n", len(transactions.Actions))
		}
	}()

	perPage := 100
	more := true
	for page := 1; more; page++ {
		resp, e := client.TxSearch(ctx, fmt.Sprintf("coin_received.receiver = '%s'", account), false, &page, &perPage, "desc")
		if e != nil {
			log.Fatal(e)
		}
		parseTxs(client, transactions, resp)
		more = resp.TotalCount > perPage*page
	}
	more = true
	for page := 1; more; page++ {
		resp, e := client.TxSearch(ctx, fmt.Sprintf("coin_spent.spender = '%s'", account), false, &page, &perPage, "desc")
		if e != nil {
			log.Fatal(e)
		}
		parseTxs(client, transactions, resp)
		more = resp.TotalCount > perPage*page
	}

	log.Println("Collection finished, sorting transactions...")
	transactions.sort()
	for i := range transactions.Actions {
		_, err = f.Write(transactions.Actions[i].toCsv())
		if err != nil {
			log.Fatal(err)
		}
	}
	log.Printf("Done, writing file %s", outFile)
}
