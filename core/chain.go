package core

type Blockchain struct {
	Blocks []*Block
}

func (bc *Blockchain) AddBlock(data string) {
	// 이전 블록 가져오기
	prevBlock := bc.Blocks[len(bc.Blocks)-1]

	// 새 블록 생성
	newBlock := NewBlock(data, prevBlock.Hash)

	// 체인에 새 블록 추가
	bc.Blocks = append(bc.Blocks, newBlock)
}

func NewBlockchain() *Blockchain {
	return &Blockchain{[]*Block{NewGenesisBlock()}}
}
