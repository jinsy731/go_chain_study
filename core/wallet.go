package core

import (
	"bytes"
	"crypto/sha256"
	"log"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/ripemd160"
)

// 메인넷의 주소 버전
const version = byte(0x00)

// 주소 체크섬 길이 설정
const addressChecksumLen = 4

// 개인키와 공개키 쌍을 저장하는 Wallet 구조체
type Wallet struct {
	PrivateKey *btcec.PrivateKey
	PublicKey  []byte // 압축된 공개키
}

func NewWallet() *Wallet {
	// 새 개인키 생성 (secp256k1)
	privateKey, err := btcec.NewPrivateKey()
	if err != nil {
		log.Panic(err)
	}

	// 공개키 생성 (압축 형식 사용)
	// 공개키는 개인키로부터 파생
	// 압축 형식은 33바이트, 비압축 형식은 65바이트. 공간 절약을 위해 압축 사용
	publicKey := privateKey.PubKey().SerializeCompressed()

	return &Wallet{privateKey, publicKey}
}

// 지갑 주소 생성
// 비트코인의 주소 생성 표준 로직
func (w *Wallet) GetAddress() []byte {
	// 공개키 해시 (Pubkey -> SHA256 -> RIPEMD160)
	hash160PubKey := HashPubKey(w.PublicKey)

	// 버전 접두사 추가 (Version + PubkeyHash)
	versionedPayload := append([]byte{version}, hash160PubKey...)

	// 체크섬 계산
	checksum := checksum(versionedPayload)

	// Payload + Checksum 결합
	fullPayload := append(versionedPayload, checksum...)

	// Base58 인코딩
	address := base58.Encode(fullPayload)

	return []byte(address)
}

// 공개키를 HASH160 (SHA-256 후 RIPEMD160)
func HashPubKey(pubKey []byte) []byte {
	pubSHA256 := sha256.Sum256(pubKey)

	RIPEMD160Hasher := ripemd160.New()
	_, err := RIPEMD160Hasher.Write(pubSHA256[:])
	if err != nil {
		log.Panic(err)
	}

	return RIPEMD160Hasher.Sum(nil)
}

// 주소 검증을 위한 체크섬 생성 (Double SHA256)
func checksum(payload []byte) []byte {
	firstSHA := sha256.Sum256(payload)
	secondSHA := sha256.Sum256(firstSHA[:])

	return secondSHA[:addressChecksumLen]
}

func ValidateAddress(address string) bool {
	pubKeyHash, err := base58.Decode(address)
	if err != nil {
		return false
	}

	actualChecksum := pubKeyHash[len(pubKeyHash)-addressChecksumLen:]
	versionedPayload := pubKeyHash[:len(pubKeyHash)-addressChecksumLen]
	targetChecksum := checksum(versionedPayload)

	return bytes.Equal(actualChecksum, targetChecksum)
}
