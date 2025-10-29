package main

import (
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/jinsy731/go-chain-study/core"
)

func main() {
	// 임시 주소(지갑 기능 생성 전까지)
	const myWalletAddress = "myWalletAddress"

	// DB에서 블록체인 로드 (없으면 생성)
	bc := core.NewBlockchain()
	defer bc.Close()

	// 실제로는 Mempool 에서 트랜잭션을 가져와야 하지만,
	// 지금은 "MinerA"에게 보상하는 코인베이스 트랜잭션만 포함시켜 테스트함.
	// 아직 사용자 간 트랜잭션은 구현 X
	// txs1 := []*core.Transaction{
	// 	core.NewCoinbaseTX("MinerA", "Block 1 Reward"),
	// }

	// txs2 := []*core.Transaction{
	// 	core.NewCoinbaseTX("MinerB", "Block 2 Reward"),
	// }

	// fmt.Println("Adding first block...")
	// bc.AddBlock(txs1)
	// fmt.Println("Adding second block...")
	// bc.AddBlock(txs2)

	bci := bc.Iterator()

	for {
		block := bci.Next()
		if block == nil {
			break
		}

		fmt.Printf("Prev. hash hex: %s\n", hex.EncodeToString(block.PrevBlockHash)) // byte slice -> hex string
		fmt.Printf("Hash: %s\n", hex.EncodeToString(block.Hash))

		fmt.Printf("Transactions:\n")
		for _, tx := range block.Transactions {
			fmt.Printf(" - TXID: %s\n", hex.EncodeToString(tx.ID))
		}

		// PoW 유효성 검증
		pow := core.NewProofOfWork(block)
		isValid := pow.Validate()
		fmt.Printf("PoW is valid: %s\n", strconv.FormatBool(isValid))
	}
}
