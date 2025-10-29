package main

import (
	"encoding/hex"
	"fmt"

	"github.com/jinsy731/go-chain-study/core"
)

func main() {
	bc := core.NewBlockchain()

	bc.AddBlock("Send 1 BTC to Zin")
	bc.AddBlock("Send 2 BTC to Kim")

	for _, block := range bc.Blocks {
		fmt.Printf("Prev. hash hex: %s\n", hex.EncodeToString(block.PrevBlockHash)) // byte slice -> hex string
		fmt.Printf("Data: %s\n", string(block.Data))
		fmt.Printf("Hash: %s\n", hex.EncodeToString(block.Hash))
	}
}
