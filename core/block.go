package core

import (
	"bytes"
	"crypto/sha256"
	"strconv"
	"time"
)

type Block struct {
	Timestamp     int64  // 블록 생성 시간
	Data          []byte // 블록에 포함될 데이터 (여기서는 간단히 바이트 슬라이스로 구현)
	PrevBlockHash []byte // 이전 블록 해시
	Hash          []byte // 현재 블록 해시
	Nonce         int
}

// 블록의 해시 계산 함수
// 블록의 핵심 데이터(Timestamp, Data, PrevBlochHash)를 묶어 SHA-256 해시를 계산합니다.
func (b *Block) SetHash() {
	// Timestamp를 바이트 슬라이스로 변환
	timestamp := []byte(strconv.FormatInt(b.Timestamp, 10))

	// Nonce를 바이트 슬라이스로 변환
	nonce := []byte(strconv.FormatInt(int64(b.Nonce), 10))

	// 모든 데이터를 하나의 바이트 슬라이스로 결합
	headers := bytes.Join([][]byte{b.PrevBlockHash, b.Data, timestamp, nonce}, []byte{})

	// SHA-256 해시 계산
	hash := sha256.Sum256(headers)

	// [32]byte 배열을 슬라이스로 변환
	b.Hash = hash[:]
}

func NewBlock(data string, prevBlockHash []byte) *Block {
	block := &Block{
		Timestamp:     time.Now().Unix(),
		Data:          []byte(data), // data를 바이트 슬라이스로 변환
		PrevBlockHash: prevBlockHash,
		Hash:          []byte{},
		Nonce:         0,
	}

	// block.SetHash() // Hash는 PoW의 결과로 계산됨
	return block
}

func NewGenesisBlock() *Block {
	return NewBlock("Genesis Block", []byte{})
}
