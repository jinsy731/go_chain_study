package core

import (
	"fmt"
	"log"
	"os"
	"time"

	"go.etcd.io/bbolt"
)

const dbFile = "blockchain.db"
const blocksBucket = "blocksBucket"
const genesisCoinbaseData = "The Times 03/Jan/2009 Chancellor on brink of second bailout for banks"

type Blockchain struct {
	tip []byte // 마지막 블록의 해시
	db  *bbolt.DB
}

func (bc *Blockchain) AddBlock(txs []*Transaction) {
	var lastHash []byte

	// DB에서 마지막 블록 해시(tip)를 가져옴 (read-only transaction: view)
	err := bc.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		lastHash = b.Get([]byte("l"))
		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	// 새 블록 생성
	newBlock := NewBlock(txs, lastHash)

	// Proof Of Work
	pow := NewProofOfWork(newBlock)
	nonce, hash := pow.Run()

	newBlock.Nonce = nonce
	newBlock.Hash = hash
	// 체인에 새 블록 추가 (DB에 새 블록 저장)
	// 새 블록 저장과 l키 업데이트는 원자적으로 이루어져야 함(같은 bbolt tx 내에서 작업)
	err = bc.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))

		// 새 블록 저장
		err := b.Put(newBlock.Hash, newBlock.Serialize())
		if err != nil {
			return err
		}

		// "l"키를 새 블록의 해시로 업데이트 (마지막 블록 해시 업데이트)
		err = b.Put([]byte("l"), newBlock.Hash)
		if err != nil {
			return err
		}

		bc.tip = newBlock.Hash
		return nil
	})

	if err != nil {
		log.Panic(err)
	}
	fmt.Println("Successfully added a new block!")
}

// 제네시스 블록으로 시작하는 새로운 블록체인 생성
// DB를 열고, 체인이 없으면 제네시스 블록을 생성
func NewBlockchain(address string) *Blockchain {
	// DB 파일이 존재하는지 확인
	// os.Stat으로 파일 상태정보를 가져옴. 파일이 없거나 접근할 수 없으면 error
	// os.IsNotExist(err)는 error가 파일이 존재하지 않아 발생한 것인지를 확인
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		fmt.Println("Blockchain database not found. Creating new one...")
	}

	db, err := bbolt.Open(dbFile, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.Panic(err)
	}

	var tip []byte

	// DB 트랜잭션 (Update = R/W)
	err = db.Update(func(tx *bbolt.Tx) error {
		// 버킷(테이블) 가져오기
		b := tx.Bucket([]byte(blocksBucket))

		// 버킷이 없으면
		if b == nil {
			fmt.Println("No existing blockchain found. Creating Genesis Block...")

			// 코인베이스 트랜잭션 생성
			// 아직 지갑이 없으므로 주소 대신 임시 문자열을 지정
			cbtx := NewCoinbaseTX(address, genesisCoinbaseData)
			genesisBlock := NewGenesisBlock(cbtx)

			// PoW
			pow := NewProofOfWork(genesisBlock)
			nonce, hash := pow.Run()
			genesisBlock.Hash = hash
			genesisBlock.Nonce = nonce

			b, err := tx.CreateBucket([]byte(blocksBucket))
			if err != nil {
				return err
			}

			// 제네시스 블록 직렬화 및 DB 저장
			err = b.Put(genesisBlock.Hash, genesisBlock.Serialize())
			if err != nil {
				return err
			}

			// 마지막 블록 해시(l키)를 제네시스 블록 해시로 저장
			err = b.Put([]byte("l"), genesisBlock.Hash)
			if err != nil {
				return err
			}
			tip = genesisBlock.Hash
		} else {
			// 버킷이 이미 존재하는 경우
			fmt.Println("Found existing blockchain.")
			// l키에서 마지막 블록 해시(tip)를 가져옴
			tip = b.Get([]byte("l"))
		}
		return nil
	})

	if err != nil {
		log.Panic(err)
	}

	// DB 인스턴스와 tip을 가진 Blockchain 구조체 포인터 반환
	return &Blockchain{tip, db}
}

// DB 연결 종료
func (bc *Blockchain) Close() {
	bc.db.Close()
}
