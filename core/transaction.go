package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/btcsuite/btcd/btcec/v2"
	ecdsa "github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/mr-tron/base58"
)

const subsidy = 10 // 구현 간소화를 위해 10으로 고정

// 거래의 출력 (누가 얼마를 받았는가)
type TXOutput struct {
	Value      int
	PubKeyHash []byte // 주소에서 파생된 PubKeyHash로 잠금
	// ScriptPubKey string // 수신자의 주소 (잠금 스크립트, 지금은 간단히 문자열로 함)
}

// 거래의 입력 (과거의 어떤 돈을 쓸 것인가)
type TXInput struct {
	Txid      []byte // 참조할 과거 트랜잭션 ID
	Vout      int    // 참조할 과거 트랜잭션의 Output 인덱스
	Signature []byte // 실제 서명
	PubKey    []byte // 서명 검증에 사용할 공개키
	// ScriptSig string // 서명 (잠금 해제 스크립트, 지금은 간단히 문자열로 함)
}

type Transaction struct {
	ID   []byte      // 트랜잭션 해시 ID
	Vin  []*TXInput  // 입력 목록
	VOut []*TXOutput // 출력 목록
}

func init() {
	gob.Register(&Transaction{})
	gob.Register(&TXInput{})
	gob.Register(&TXOutput{})
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
		Txid: nil,
		Vout: -1,
	}

	txout := NewTXOutput(subsidy, to)

	tx := &Transaction{
		ID:   nil,
		Vin:  []*TXInput{txin},
		VOut: []*TXOutput{txout},
	}
	tx.SetID()

	return tx
}

// 금액과 주소를 받아 새 TXOutput 생성
// 주소를 통해 잠금
func NewTXOutput(value int, address string) *TXOutput {
	txo := &TXOutput{value, nil}
	//
	txo.Lock([]byte(address))
	return txo
}

func (out *TXOutput) Lock(address []byte) {
	// Base58 디코딩
	pubKeyHash, err := base58.Decode(string(address))
	if err != nil {
		log.Panic(err)
	}
	// 체크섬(4바이트)와 버전(1바이트)를 제외하고 실제 PubKeyHash(20바이트)만 추출
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-addressChecksumLen]
	out.PubKeyHash = pubKeyHash
}

func (out *TXOutput) IsLockedWithKey(pubKeyHash []byte) bool {
	return bytes.Equal(out.PubKeyHash, pubKeyHash)
}

func (in *TXInput) UsesKey(pubKeyHash []byte) bool {
	lockingHash := HashPubKey(in.PubKey)
	return bytes.Equal(lockingHash, pubKeyHash)
}

// 트랜잭션에 서명
func (tx *Transaction) Sign(privKey *btcec.PrivateKey, prevTXs map[string]*Transaction) {
	// 코인베이스 트랜잭션은 서명하지 않음
	if tx.IsCoinbase() {
		return
	}

	// 입력(Input)에 사용된 이전 트랜잭션(prevTX)이 유효한지 확인
	for _, vin := range tx.Vin {
		if prevTXs[hex.EncodeToString(vin.Txid)] == nil {
			log.Panic("ERROR: Previous transaction is not found")
		}
	}

	// 서명을 위한 복사본 생성
	txCopy := tx.TrimmedCopy()
	// 각 Input에 대해 서명 생성
	for inID, vin := range txCopy.Vin {
		prevTx := prevTXs[hex.EncodeToString(vin.Txid)]

		// ScriptSig를 비우고, 참조할 Output의 PubKeyHash로 채움
		// 이것이 서명할 "데이터"
		txCopy.Vin[inID].Signature = nil
		txCopy.Vin[inID].PubKey = prevTx.VOut[vin.Vout].PubKeyHash

		// 트랜잭션 복사본을 해시 (이것이 서명할 대상)
		txCopy.SetID() // SetID()는 해시 계산기 역할을 함
		dataToSign := txCopy.ID

		// 서명. btcec ecdsa 패키지가 deterministic ECDSA 서명을 제공
		signature := ecdsa.Sign(privKey, dataToSign)

		// 서명을 DER 인코딩된 바이트로 변환하여 저장
		tx.Vin[inID].Signature = signature.Serialize()

		// 서명 후 다시 PubKey를 비움
		txCopy.Vin[inID].PubKey = nil

		// 원본 트랜잭션의 Input에 서명과 공개키를 저장
		tx.Vin[inID].Signature = signature.Serialize()
		tx.Vin[inID].PubKey = privKey.PubKey().SerializeCompressed()
	}
}

// 서명을 위해 트랜잭션 복사본 생성
func (tx *Transaction) TrimmedCopy() *Transaction {
	var inputs []*TXInput

	for _, vin := range tx.Vin {
		// Signature와 PubKey가 비워진 Input 복사본
		inputs = append(inputs, &TXInput{Txid: vin.Txid, Vout: vin.Vout, Signature: nil, PubKey: nil})
	}

	return &Transaction{
		ID:   tx.ID,
		Vin:  inputs,
		VOut: tx.VOut,
	}
}

func (tx *Transaction) Verify(prevTXs map[string]*Transaction) bool {
	// 코인베이스 트랜잭션은 서명을 하지 않으므로 검증도 필요없음
	if tx.IsCoinbase() {
		return true
	}

	for _, vin := range tx.Vin {
		if prevTXs[hex.EncodeToString(vin.Txid)] == nil {
			log.Panic("ERROR: Previous transaction is not found")
		}
	}

	txCopy := tx.TrimmedCopy()

	for inID, vin := range tx.Vin {
		prevTx := prevTXs[hex.EncodeToString(vin.Txid)]

		txCopy.Vin[inID].Signature = nil
		txCopy.Vin[inID].PubKey = prevTx.VOut[vin.Vout].PubKeyHash
		txCopy.SetID()
		dataToVerify := txCopy.ID
		txCopy.Vin[inID].PubKey = nil

		// 서명을 []byte -> ecdsa.Signature로 역직렬화
		signature, err := ecdsa.ParseSignature(vin.Signature)
		if err != nil {
			log.Panic(err)
		}

		// 공개키를 []byte -> btcec.PublicKey로 역직렬화
		pubKey, err := btcec.ParsePubKey(vin.PubKey)
		if err != nil {
			log.Panic(err)
		}

		if !signature.Verify(dataToVerify, pubKey) {
			return false
		}
	}

	return true
}
