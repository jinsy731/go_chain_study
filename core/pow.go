package core

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math"
	"math/big"
	"strconv"
)

// 난이도 값(여기서는 16bit, 즉 앞의 0이 16개)
// 실제로는 동적으로 조정되어야 함
const targetBits = 16

// Nonce의 최대값 (오버플로우 방지)
const maxNonce = math.MaxInt64

type ProofOfWork struct {
	block  *Block   // 검증할 블록
	target *big.Int // 목표값 (이 값보다 작은 해시를 찾아야 함)
}

func NewProofOfWork(b *Block) *ProofOfWork {
	target := big.NewInt(1) // 1로 초기화

	// target = 1 << (256 - targetBits)
	// targetBits만큼 왼쪽으로 시프트
	target.Lsh(target, uint(256-targetBits))

	pow := &ProofOfWork{b, target}
	return pow
}

// PoW를 위한 데이터 준비 (Block -> []byte)
// 해시 계산에는 Nonce와 Difficulty(targetBits)가 모두 포함되어야 함
func (pow *ProofOfWork) prepareData(nonce int) []byte {
	data := bytes.Join(
		[][]byte{
			pow.block.PrevBlockHash,
			pow.block.HashTransactions(),
			[]byte(strconv.FormatInt(pow.block.Timestamp, 10)),
			[]byte(strconv.FormatInt(int64(targetBits), 10)),
			[]byte(strconv.FormatInt(int64(nonce), 10)),
			[]byte(strconv.FormatInt(pow.block.Height, 10)),
		},
		[]byte{},
	)
	return data
}

func (pow *ProofOfWork) Run() (int, []byte) {
	var hashInt big.Int
	var hash [32]byte
	nonce := 0

	fmt.Printf(`Mining the block...`)

	for nonce < maxNonce {
		// nonce를 설정하여 데이터 준비
		data := pow.prepareData(nonce)
		// 데이터를 SHA-256 해싱
		hash = sha256.Sum256(data)

		// 해시([]byte)를 big.Int로 변환
		hashInt.SetBytes(hash[:])

		// Cmp: hashInt < pow.target 이면 -1
		if hashInt.Cmp(pow.target) == -1 {
			// PoW는 target보다 작은 해시를 찾는 과정이므로, 이 경우 PoW 작업이 완료
			fmt.Printf("Found! Hash: %x\n", hash)
			break
		} else {
			// 못 찾았으면 Nonce를 증가시켜 다시 진행
			nonce++
			fmt.Printf("\rHashing... %d (Target bits: %d)", nonce, targetBits)
		}
	}

	// 정답 논스와 해시를 반환
	return nonce, hash[:]
}

func (pow *ProofOfWork) Validate() bool {
	var hashInt big.Int

	// Nonce가 이미 블록에 설정되어 있다고 가정
	// Nonce -> Data -> Hash를 생성
	data := pow.prepareData(pow.block.Nonce)
	hash := sha256.Sum256(data)
	hashInt.SetBytes(hash[:])

	// Hash와 target을 비교
	isValid := hashInt.Cmp(pow.target) == -1

	return isValid
}
