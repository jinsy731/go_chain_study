package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"log"
)

const subsidy = 10 // 구현 간소화를 위해 10으로 고정

// 거래의 출력 (누가 얼마를 받았는가)
type TXOutput struct {
	Value        int
	ScriptPubKey string // 수신자의 주소 (잠금 스크립트, 지금은 간단히 문자열로 함)
}

// 거래의 입력 (과거의 어떤 돈을 쓸 것인가)
type TXInput struct {
	Txid      []byte // 참조할 과거 트랜잭션 ID
	Vout      int    // 참조할 과거 트랜잭션의 Output 인덱스
	ScriptSig string // 서명 (잠금 해제 스크립트, 지금은 간단히 문자열로 함)
}

type Transaction struct {
	ID   []byte      // 트랜잭션 해시 ID
	Vin  []*TXInput  // 입력 목록
	VOut []*TXOutput // 출력 목록
}

// 트랜잭션의 해시 ID를 계산하고 설정
func (tx *Transaction) SetID() {
	var encoded bytes.Buffer
	var hash [32]byte

	// Go 객체 <-> Byte Stream 인코딩, 디코딩을 위한 gob
	enc := gob.NewEncoder(&encoded)
	// Transaction 객체를 직렬화해서 버퍼(encoded) 저장
	err := enc.Encode(tx)

	if err != nil {
		log.Panic(err)
	}

	// Transaction 객체를 직렬화한 바이트 스트림을 SHA-256 해싱
	hash = sha256.Sum256(encoded.Bytes())
	tx.ID = hash[:]
}

// 트랜잭션이 코인베이스 트랜잭션인지 확인
func (tx *Transaction) IsCoinbase() bool {
	return len(tx.Vin) == 1 && tx.Vin[0].Txid == nil && tx.Vin[0].Vout == -1
}

// 채굴 보상을 위한 코인베이스 트랜잭션 생성
func NewCoinbaseTX(to, data string) *Transaction {
	// 코인베이스 트랜잭션의 데이터는 자유롭게 생성
	if data == "" {
		data = fmt.Sprintf("Reward to '%s'", to)
	}

	// 코인베이스는 참조할 Output이 없으므로, Txid=nil, Vout=-1
	txin := &TXInput{
		Txid:      nil,
		Vout:      -1,
		ScriptSig: data,
	}

	// 보상을 받는 사람에게 subsidy 만큼 지급
	txout := &TXOutput{
		Value:        subsidy,
		ScriptPubKey: to,
	}

	tx := &Transaction{
		ID:   nil,
		Vin:  []*TXInput{txin},
		VOut: []*TXOutput{txout},
	}
	tx.SetID()

	return tx
}

// ScriptSig가 ScriptPubKey를 해제할 수 있는지 확인
func (in *TXInput) CanUnlockOutputWith(unlockingData string) bool {
	return in.ScriptSig == unlockingData
}

// ScriptPubKey가 unlockingData로 해제될 수 있는지 확인
func (out *TXOutput) CanBeUnlockedWith(unlockingData string) bool {
	return out.ScriptPubKey == unlockingData
}
