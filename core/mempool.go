package core

import (
	"encoding/hex"
	"sync"
)

type Mempool struct {
	transactions map[string]*Transaction // key: txID
	lock         sync.RWMutex
}

func NewMempool() *Mempool {
	return &Mempool{
		transactions: make(map[string]*Transaction),
	}
}

// mempool에 트랜잭션 추가 (유효성 검사는 서버가 수행)
// return bool : 새로 추가되었는지 여부
func (m *Mempool) Add(tx *Transaction) bool {
	m.lock.Lock()
	defer m.lock.Unlock()

	txID := hex.EncodeToString(tx.ID)
	if _, ok := m.transactions[txID]; ok {
		return false // 멤풀에 이미 존재하는 경우 false
	}

	m.transactions[txID] = tx
	return true
}

func (m *Mempool) Get(ID string) *Transaction {
	m.lock.Lock()
	defer m.lock.Unlock()

	if v, ok := m.transactions[ID]; ok {
		return v
	} else {
		return nil
	}
}

// 채굴을 위해 멤풀의 모든 트랜잭션을 반환
func (m *Mempool) GetTxs() []*Transaction {
	m.lock.Lock()
	defer m.lock.Unlock()

	var txs []*Transaction
	for _, tx := range m.transactions {
		txs = append(txs, tx)
	}
	return txs
}

func (m *Mempool) Exists(ID string) bool {
	m.lock.Lock()
	defer m.lock.Unlock()

	_, ok := m.transactions[ID]

	return ok
}

// 블록에 포함된 트랜잭션들을 멤풀에서 제거
func (m *Mempool) Clear(block *Block) {
	m.lock.Lock()
	defer m.lock.Unlock()

	for _, tx := range block.Transactions {
		txID := hex.EncodeToString(tx.ID)
		if _, ok := m.transactions[txID]; ok {
			delete(m.transactions, txID)
		}
	}
}
