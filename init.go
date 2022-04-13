package exporter

import (
	_ "embed"
	"flag"
	"os"

	"gopkg.in/yaml.v3"
	"log"
)

var (
	account     string
	endpoint    string
	outFile     string
	f           *os.File
	currencyMap = &currencies{}
	ibcMap      = &ibcCurrency{}
)

const csvHeader = `Date,Sent Amount,Sent Currency,Received Amount,Received Currency,Fee Amount,Fee Currency,Net Worth Amount,Net Worth Currency,Label,Description,TxHash`

//go:embed currencies.yaml
var denoms []byte

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	err := yaml.Unmarshal(denoms, currencyMap)
	if err != nil {
		log.Fatal(err)
	}

	flag.StringVar(&account, "account", "", "required: account to query")
	flag.StringVar(&endpoint, "node", "http://127.0.0.1:26657", "required: tendermint RPC endpoing")
	flag.StringVar(&outFile, "out", "history.csv", "optional: output file name")
	flag.Parse()

	if account == "" || endpoint == "" {
		flag.PrintDefaults()
		log.Fatal("required flag missing.")
	}

	f, err = os.OpenFile(outFile, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	_, err = f.WriteString(csvHeader)
	if err != nil {
		log.Fatal(err)
	}
}
