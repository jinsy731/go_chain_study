package core

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"log"

	"go.etcd.io/bbolt"
)

const utxoBucket = "utxoBucket"

type UTXOSet struct {
	Blockchain *Blockchain
}

// 모든 블록을 스캔하여 현재의 UTXO Set을 만듦
func (u UTXOSet) Reindex() {
	db := u.Blockchain.db
	bucketName := []byte(utxoBucket)

	err := db.Update(func(tx *bbolt.Tx) error {
		// 기존 버킷 삭제
		err := tx.DeleteBucket(bucketName)
		if err != nil && err != bbolt.ErrBucketNotFound {
			log.Panic(err)
		}

		// 새 버킷 생성
		_, err = tx.CreateBucket(bucketName)
		if err != nil {
			log.Panic(err)
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	// 모든 블록을 스캔하여 모든 UTXO를 찾음
	// map[string][]*TXOutput
	// key: TXID, value: TXOutput Slice
	allUTXOs := u.Blockchain.FindAllUTXO()

	// 새 버킷에 UTXO 저장
	err = db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(bucketName)

		for txID, outs := range allUTXOs {
			// key 직렬화
			key, err := hex.DecodeString(txID)
			if err != nil {
				log.Panic(err)
			}

			// TXOutput 슬라이스 직렬화
			var outsData bytes.Buffer
			if err := gob.NewEncoder(&outsData).Encode(outs); err != nil {
				log.Panic(err)
			}

			err = b.Put(key, outsData.Bytes())
			if err != nil {
				log.Panic(err)
			}
		}

		return nil
	})
	if err != nil {
		log.Panic(err)
	}
}

// UTXOSet에서 특정 PubKeyHash의 모든 UTXO 찾기
func (u UTXOSet) FindUTXOs(pubKeyHash []byte) []*TXOutput {
	var UTXOs []*TXOutput
	db := u.Blockchain.db

	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(utxoBucket))
		if b == nil {
			log.Panic("UTXO Bucket nof found. Please reindex.")
		}

		c := b.Cursor() // 버킷을 순회하는 커서

		// utxoBucket을 처음부터 끝까지 스캔
		for k, v := c.First(); k != nil; k, v = c.Next() {

			// v(value)를 역직렬화 (TXOutput 슬라이스))
			var outs []*TXOutput
			if err := gob.NewDecoder(bytes.NewReader(v)).Decode(&outs); err != nil {
				log.Panic(err)
			}

			// Output 슬라이스를 순회
			for _, out := range outs {
				// 이 Output이 주어진 pubKeyHash로 잠겼는지 확인.
				if out.IsLockedWithKey(pubKeyHash) {
					UTXOs = append(UTXOs, out)
				}
			}
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	return UTXOs
}

func (u UTXOSet) GetBalance(pubKeyHash []byte) int {
	balance := 0
	utxos := u.FindUTXOs(pubKeyHash)

	for _, out := range utxos {
		balance += out.Value
	}

	return balance
}
