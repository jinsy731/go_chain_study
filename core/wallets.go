package core

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"os"

	"github.com/btcsuite/btcd/btcec/v2"
)

const walletFile = "wallet.dat"

type Wallets struct {
	Wallets map[string]*Wallet // key: address
}

type walletRecord struct {
	PrivateKey []byte
	PublicKey  []byte
}

// wallet.dat 파일에서 지갑들을 불러옴(Load)
func NewWallets() (*Wallets, error) {
	// wallet.dat 파일이 있는지 확인하고 없으면, 새로운 Wallets 구조체를 반환
	if _, err := os.Stat(walletFile); os.IsNotExist(err) {
		wallets := &Wallets{}
		wallets.Wallets = make(map[string]*Wallet)
		return wallets, err
	}

	// 파일이 있으면 파일 읽기
	fileContent, err := os.ReadFile(walletFile)
	if err != nil {
		log.Panic(err)
	}

	// gob으로 역직렬화
	var stored map[string]walletRecord
	decoder := gob.NewDecoder(bytes.NewReader(fileContent))
	err = decoder.Decode(&stored)
	if err != nil {
		log.Panic(err)
	}

	wallets := &Wallets{Wallets: make(map[string]*Wallet)}
	for address, record := range stored {
		if len(record.PrivateKey) == 0 {
			continue
		}

		privKey, _ := btcec.PrivKeyFromBytes(record.PrivateKey)
		wallets.Wallets[address] = &Wallet{
			PrivateKey: privKey,
			PublicKey:  append([]byte(nil), record.PublicKey...),
		}
	}

	return wallets, nil
}

// 새 지갑을 생성하고 맵에 추가
func (ws *Wallets) CreateWallet() string {
	wallet := NewWallet()
	address := string(wallet.GetAddress())
	ws.Wallets[address] = wallet

	fmt.Printf("New wallet created. Address: %s\n", address)
	return address
}

// 주소로 지갑 찾기
func (ws *Wallets) GetWallet(address string) Wallet {
	return *ws.Wallets[address]
}

// 지갑 맵을 파일에 GOB으로 저장(Save)
func (ws *Wallets) SaveToFile() {
	var content bytes.Buffer

	if ws.Wallets == nil {
		ws.Wallets = make(map[string]*Wallet)
	}

	stored := make(map[string]walletRecord)
	for address, wallet := range ws.Wallets {
		if wallet == nil || wallet.PrivateKey == nil {
			continue
		}

		stored[address] = walletRecord{
			PrivateKey: wallet.PrivateKey.Serialize(),
			PublicKey:  append([]byte(nil), wallet.PublicKey...),
		}
	}

	encoder := gob.NewEncoder(&content)
	err := encoder.Encode(stored)
	if err != nil {
		log.Panic(err)
	}

	err = os.WriteFile(walletFile, content.Bytes(), 0644)
	if err != nil {
		log.Panic(err)
	}
}
