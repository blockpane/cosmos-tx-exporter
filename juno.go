package exporter

import (
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"strconv"
)

const (
	marbleAirdropContract = "juno1unjfruscnz39mh42dtekak489s7mnzyh7ry20t80errfkf5j3spqetewaq"
	airdropSignature      = " - ExecuteContract - claim - transfer"
)

func isAirdrop(tx *coretypes.ResultTx) (amount float64, token string, isClaim bool) {
	for _, event := range tx.TxResult.Events {
		var contractAddress, action, addr, claimAmount string
		for _, attribute := range event.Attributes {
			switch string(attribute.Key) {
			case "_contract_address":
				contractAddress = string(attribute.Value)
			case "action":
				action = string(attribute.Value)
			case "address":
				addr = string(attribute.Value)
			case "amount":
				claimAmount = string(attribute.Value)
			}
		}
		switch "" {
		case contractAddress, action, addr, claimAmount:
			continue
		}
		if addr == account && contractAddress == marbleAirdropContract {
			var err error
			amount, err = strconv.ParseFloat(claimAmount, 64)
			if err != nil {
				return
			}
			token = "MARBLE"
			isClaim = true
			return
		}
	}
	return
}
