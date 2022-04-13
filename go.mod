module github.com/blockpane/cosmos-tx-exporter

go 1.16

require (
	github.com/cosmos/cosmos-sdk v0.45.1
	github.com/cosmos/ibc-go v1.4.0
	github.com/tendermint/tendermint v0.34.14
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
)

replace github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1
