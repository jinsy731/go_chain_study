package core

import (
	"encoding/hex"
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
	err = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		tip = b.Get([]byte("l"))
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	// DB 인스턴스와 tip을 가진 Blockchain 구조체 포인터 반환
	return &Blockchain{tip, db}
}

func CreateBlockchain(address string) *Blockchain {
	if _, err := os.Stat(dbFile); !os.IsNotExist(err) {
		fmt.Println("Blockchain already exists.")
		os.Exit(1)
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

// 전체 블록체인을 스캔하여 현재의 UTXO Map을 반환
func (bc *Blockchain) FindAllUTXO() map[string][]*TXOutput {
	UTXO := make(map[string][]*TXOutput)
	// key: txID, value: 사용된 Output 인덱스
	spentTXOs := make(map[string][]int)

	bci := bc.Iterator()

	// 블록을 순회
	for {
		block := bci.Next()

		// 블록 안의 트랜잭션을 순회
		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)

		OutputsLoop:
			// 트랜잭션 안의 Outputs를 순회
			for outIdx, out := range tx.VOut {
				// txID의 Outputs 중 사용된 Output이 있는 경우
				if spentTXOs[txID] != nil {
					// 해당 txID의 Outputs를 순회하며, 이미 사용된 Output인 경우 아래 과정을 Skip (UTXO 맵에 저장하는 과정을 스킵)
					for _, spentOutIdx := range spentTXOs[txID] {
						if spentOutIdx == outIdx {
							continue OutputsLoop
						}
					}
				}

				// 사용되지 않았다면, UTXO 맵에 추가
				outs := UTXO[txID]
				outs = append(outs, out)
				UTXO[txID] = outs
			}

			// Inputs을 순회하며 사용된 Output을 찾아서 spentTXOs 에 추가
			// Coinbase 트랜잭션이 아닌 경우에만 사용된 Input이 있음.
			if !tx.IsCoinbase() {
				for _, in := range tx.Vin {
					inTxID := hex.EncodeToString(in.Txid)
					spentTXOs[inTxID] = append(spentTXOs[inTxID], in.Vout)
				}
			}
		}

		// 더 이상 순회할 블록이 없으면 for loop break
		if len(block.PrevBlockHash) == 0 {
			break
		}
	}

	return UTXO
}

// DB 연결 종료
func (bc *Blockchain) Close() {
	bc.db.Close()
}
