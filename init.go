package exporter

import (
	_ "embed"

	"gopkg.in/yaml.v3"
	"log"
)

var (
	account   = `juno1sgghjqdrj9kujkx38q04d99qsljwfd6mee4lt0`
	endpoint  = "http://fsn3:46657"
	currencyMap = &currencies{}
	ibcMap = &ibcCurrency{}
)

const csvHeader = `Date,Sent Amount,Sent Currency,Received Amount,Received Currency,Fee Amount,Fee Currency,Net Worth Amount,Net Worth Currency,Label,Description,TxHash`

//go:embed currencies.yaml
var denoms []byte

func init()  {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	err := yaml.Unmarshal(denoms, currencyMap)
	if err != nil {
		log.Fatal(err)
	}
}
