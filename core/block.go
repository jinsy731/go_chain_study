package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"log"
	"time"
)

type Block struct {
	Timestamp    int64 // 블록 생성 시간
	Transactions []*Transaction
	// Data          []byte // 블록에 포함될 데이터 (여기서는 간단히 바이트 슬라이스로 구현)
	PrevBlockHash []byte // 이전 블록 해시
	Hash          []byte // 현재 블록 해시
	Nonce         int
}

// 블록의 해시 계산 함수
// 블록의 핵심 데이터(Timestamp, Data, PrevBlochHash)를 묶어 SHA-256 해시를 계산합니다.
// deprecated: 이제 해시는 PoW 에 의해 계산됨
// func (b *Block) SetHash() {
// 	// Timestamp를 바이트 슬라이스로 변환
// 	timestamp := []byte(strconv.FormatInt(b.Timestamp, 10))

// 	// Nonce를 바이트 슬라이스로 변환
// 	nonce := []byte(strconv.FormatInt(int64(b.Nonce), 10))

// 	// 모든 데이터를 하나의 바이트 슬라이스로 결합
// 	headers := bytes.Join([][]byte{b.PrevBlockHash, b.Data, timestamp, nonce}, []byte{})

// 	// SHA-256 해시 계산
// 	hash := sha256.Sum256(headers)

// 	// [32]byte 배열을 슬라이스로 변환
// 	b.Hash = hash[:]
// }

func NewBlock(txs []*Transaction, prevBlockHash []byte) *Block {
	block := &Block{
		Timestamp:     time.Now().Unix(),
		Transactions:  txs,
		PrevBlockHash: prevBlockHash,
		Hash:          []byte{},
		Nonce:         0,
	}

	// block.SetHash() // Hash는 PoW의 결과로 계산됨
	return block
}

func NewGenesisBlock(coinbaseTx *Transaction) *Block {
	return NewBlock([]*Transaction{coinbaseTx}, []byte{})
}

// 블록의 모든 트랜잭션을 묶어 해시
// PoW에 사용되며, 머클 루트의 매우 단순화된 버전
func (b *Block) HashTransactions() []byte {
	var txHashes [][]byte
	var txHash [32]byte

	for _, tx := range b.Transactions {
		txHashes = append(txHashes, tx.ID)
	}

	txHash = sha256.Sum256(bytes.Join(txHashes, []byte{}))

	return txHash[:]
}

// 블록을 []byte로 직렬화
func (b *Block) Serialize() []byte {
	var encoded bytes.Buffer
	enc := gob.NewEncoder(&encoded)

	err := enc.Encode(b)
	if err != nil {
		log.Panic(err)
	}

	return encoded.Bytes()
}

// []byte를 Block 포인터로 역직렬화
func DeserializeBlock(bs []byte) *Block {
	var block Block
	dec := gob.NewDecoder(bytes.NewReader(bs))

	err := dec.Decode(&block)
	if err != nil {
		log.Panic(err)
	}

	return &block
}
