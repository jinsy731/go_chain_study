package core

type Blockchain struct {
	Blocks []*Block
}

func (bc *Blockchain) AddBlock(txs []*Transaction) {
	// 이전 블록 가져오기
	prevBlock := bc.Blocks[len(bc.Blocks)-1]

	// 새 블록 생성
	newBlock := NewBlock(txs, prevBlock.Hash)

	// Proof Of Work
	pow := NewProofOfWork(newBlock)
	nonce, hash := pow.Run()

	newBlock.Nonce = nonce
	newBlock.Hash = hash

	// 체인에 새 블록 추가
	bc.Blocks = append(bc.Blocks, newBlock)
}

// 제네시스 블록으로 시작하는 새로운 블록체인 생성
func NewBlockchain() *Blockchain {
	// 코인베이스 트랜잭션 생성
	// 아직 지갑이 없으므로 주소 대신 임시 문자열을 지정
	cbtx := NewCoinbaseTX("GenesisOwner", "genesisBlock")
	genesisBlock := NewGenesisBlock(cbtx)

	pow := NewProofOfWork(genesisBlock)
	nonce, hash := pow.Run()

	genesisBlock.Nonce = nonce
	genesisBlock.Hash = hash

	return &Blockchain{[]*Block{genesisBlock}}
}
