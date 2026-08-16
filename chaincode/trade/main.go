// Chaincode "trade": a trading system for arbitrary goods on a Hyperledger Fabric
// network. Course project for PDASP 2024/25.
package main

import (
	"log"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
	"github.com/pdasp/trade/contract"
)

func main() {
	tradeContract := new(contract.TradeContract)
	tradeContract.Name = "trade"

	chaincode, err := contractapi.NewChaincode(tradeContract)
	if err != nil {
		log.Panicf("creating the 'trade' chaincode failed: %v", err)
	}

	if err := chaincode.Start(); err != nil {
		log.Panicf("starting the 'trade' chaincode failed: %v", err)
	}
}
