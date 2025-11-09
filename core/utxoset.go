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
			log.Panic("UTXO Bucket not found. Please reindex.")
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

// amount만큼 보낼 수 있는 UTXO 찾기
func (u UTXOSet) FindSpendableOutputs(pubKeyHash []byte, amount int) (int, map[string][]int) {
	spendableOutputs := make(map[string][]int) // (Key: TXID, Value: Output 인덱스 슬라이스)
	accumulated := 0
	db := u.Blockchain.db

	err := db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(utxoBucket))
		c := b.Cursor()

		// UTXO 버킷 스캔
		// key: TXID, value: []*TXOutput
		for k, v := c.First(); k != nil; k, v = c.Next() {
			txID := hex.EncodeToString(k)

			var outs []*TXOutput
			if err := gob.NewDecoder(bytes.NewReader(v)).Decode(&outs); err != nil {
				log.Panic(err)
			}

			for outIdx, out := range outs {
				// 내 PubKeyHash로 잠겨있고, 아직 다 안모였으면
				if out.IsLockedWithKey(pubKeyHash) && accumulated < amount {
					accumulated += out.Value
					spendableOutputs[txID] = append(spendableOutputs[txID], outIdx)
				}
			}
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	return accumulated, spendableOutputs
}

// 블록이 추가될 때 UTXO Set을 업데이트
func (u UTXOSet) Update(block *Block) {
	db := u.Blockchain.db

	err := db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(utxoBucket))

		// 사용된 Output을 UTXO Set에서 제거
		for _, transaction := range block.Transactions {
			if !transaction.IsCoinbase() {

				for _, vin := range transaction.Vin {
					remainingOuts := []*TXOutput{}
					rawOuts := b.Get(vin.Txid)
					var outs []*TXOutput

					if err := gob.NewDecoder(bytes.NewReader(rawOuts)).Decode(&outs); err != nil {
						log.Panic(err)
					}

					// 사용된 Output(vin의 vout index와 일치하는 index의 output)을 제외한 Output 목록을 만듦
					for outIdx, out := range outs {
						if outIdx != vin.Vout {
							remainingOuts = append(remainingOuts, out)
						}
					}

					// 어떤 txID의 UTXO를 모두 소진한 경우
					if len(remainingOuts) == 0 {
						// 해당 txID를 Key로 하는 데이터를 UTXO 버킷에서 제거
						if err := b.Delete(vin.Txid); err != nil {
							log.Panic(err)
						}
					} else { // 일부만 소진된 경우
						var remainingOutsBin bytes.Buffer
						if err := gob.NewEncoder(&remainingOutsBin).Encode(remainingOuts); err != nil {
							log.Panic(err)
						}
						// 남은 Output으로 덮어쓰기
						if err := b.Put(vin.Txid, remainingOutsBin.Bytes()); err != nil {
							log.Panic(err)
						}
					}
				}
			}
			// 새로 생성된 Output을 UTXO Set에 추가
			var newOuts []*TXOutput
			var newOutsBin bytes.Buffer
			newOuts = append(newOuts, transaction.VOut...)

			if err := gob.NewEncoder(&newOutsBin).Encode(newOuts); err != nil {
				log.Panic(err)
			}
			if err := b.Put(transaction.ID, newOutsBin.Bytes()); err != nil {
				log.Panic(err)
			}
		}

		return nil
	})
	if err != nil {
		log.Panic(err)
	}
}
