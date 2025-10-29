package core

import (
	"log"

	"go.etcd.io/bbolt"
)

// 블록체인을 순회하기 위한 구조체
type BlockchainIterator struct {
	currentHash []byte    // 현재 블록 해시(순회 기준점)
	db          *bbolt.DB // DB 커넥션
}

// Blockchain 구조체를 통해 Blockchain에 대한 Iterator를 생성
func (bc *Blockchain) Iterator() *BlockchainIterator {
	return &BlockchainIterator{bc.tip, bc.db}
}

func (i *BlockchainIterator) Next() *Block {
	var block *Block

	err := i.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		if i.currentHash == nil {
			return nil
		}
		// 현재 해시로 블록체인을 가져옴
		encodedBlock := b.Get(i.currentHash)
		// 블록 바이트스트림 역직렬화
		block = DeserializeBlock(encodedBlock)

		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	// 다음 Next() 호출을 위해 currentHash를 이전 블록 해시로 업데이트
	// 마지막 블록에서 역순으로 블록을 순회
	// 조회된 블록이 있는 경우에만 currentHash 업데이트.
	if block != nil {
		i.currentHash = block.PrevBlockHash
	}

	return block
}
