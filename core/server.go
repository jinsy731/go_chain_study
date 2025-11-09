package core

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"time"
)

const protocol = "tcp"
const nodeVersion = 1
const commandLen = 12      // 명령어 길이 (12바이트로 고정)
const rpcPortOffset = 1000 // P2P + 1000 = RPC 포트

var (
	// 다운로드 중인 블록 큐
	// Server의 멤버 변수로 두고 Lock을 보호하는 것이 맞음.
	blocksInTransit  = [][]byte{}
	bootstrapAddress = "localhost:3000"
)

type Server struct {
	nodeAddress   string
	p2pPort       string
	rpcPort       string
	miningAddress string // 채굴 보상 주소 (설정된 경우에만 채굴)
	bc            *Blockchain
	mempool       *Mempool
	knownNodes    map[string]bool
}

type GetBlocks struct {
	AddrFrom string
}

type Inv struct {
	AddrFrom string
	Type     string   // "block" 또는 "tx"
	Items    [][]byte // 해시 목록
}

type GetData struct {
	AddrFrom string
	Type     string
	ID       []byte // 요청할 해시
}

type BlockMsg struct {
	AddrFrom string
	Block    []byte
}

// 메시지 구조체 (간소화한 버전)
type Version struct {
	Version    int64  // 블록체인 버전
	BestHeight int64  // 이 노드가 가진 블록의 최고 높이
	AddrFrom   string // 이 메시지를 보낸 노드의 주소
}

type TxMsg struct {
	AddrFrom    string
	Transaction []byte
}

func NewServer(port string, minerAddress string) *Server {
	nodeAddr := fmt.Sprintf("localhost:%s", port)
	rpcPortNum := (safeStringToInt(port) + rpcPortOffset)

	// 블록체인 로드
	bc := NewBlockchain(port)

	// 멤풀 생성
	mempool := NewMempool()

	// knownNodes 초기화
	knownNodesMap := make(map[string]bool)
	knownNodesMap["localhost:3000"] = true // 부트스트랩 노드는 하드코딩

	return &Server{
		nodeAddress:   nodeAddr,
		p2pPort:       port,
		rpcPort:       fmt.Sprintf("%d", rpcPortNum),
		miningAddress: minerAddress,
		bc:            bc,
		mempool:       mempool,
		knownNodes:    knownNodesMap,
	}
}

func (s *Server) Start() {
	fmt.Printf("Starting server on %s (RPC: %s)\n", s.nodeAddress, s.rpcPort)

	go s.startP2PListener()
	go s.startRPCListener()

	// 채굴자일 경우, 채굴 루프 시작
	if s.miningAddress != "" {
		fmt.Printf("Mining is enabled. Reward to: %s\n", s.miningAddress)
		go s.startMining()
	}

	// 부트스트랩 노드에 버전 전송
	go func() {
		time.Sleep(2 * time.Second)
		if s.nodeAddress != "localhost:3000" {
			s.sendVersion("localhost:3000")
		}
	}()

	// 메인 스레드 대기, 메인 스레드가 블로킹됨.
	select {}
}

func (s *Server) startP2PListener() {
	ln, err := net.Listen(protocol, s.nodeAddress)
	if err != nil {
		log.Panic(err)
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Panic(err)
		}
		go s.handleP2PConnection(conn)
	}
}

// 연결 처리 핸들러 (다른 노드에서 이 노드로 연결했을 때 핸들링)
func (s *Server) handleP2PConnection(conn net.Conn) {
	request := make([]byte, 0, 4096) // 4KB 버퍼
	tmp := make([]byte, 256)

	// conn으로부터 데이터 읽어들이기
	for {
		n, err := conn.Read(tmp) // n: 읽어들인 바이트 수
		if err != nil {
			// EOF Error 인 경우에는 에러로 처리할 필요없고, 그 외에만 에러로 처리
			if err != io.EOF {
				log.Panic(err)
			}
			break
		}
		request = append(request, tmp[:n]...)
	}

	// 메시지 파싱
	command := bytesToCommand(request[:commandLen])
	payload := request[commandLen:]

	fmt.Printf("Received command: %s\n", command)

	switch command {
	case "version":
		s.handleVersion(payload)
	case "getblocks":
		s.handleGetBlocks(payload)
	case "inv":
		s.handleInv(payload)
	case "getdata":
		s.handleGetData(payload)
	case "block":
		s.handleBlock(payload)
	case "tx":
		s.handleTx(payload)
	default:
		fmt.Println("Unknown command!")
	}
}

func (s *Server) startMining() {
	fmt.Println("Mining loop started...")

	for {
		time.Sleep(10 * time.Second)
		// 멤풀에서 트랜잭션 수집
		txs := s.mempool.GetTxs()
		validTxs := []*Transaction{}
		// 트랜잭션 검증
		for _, tx := range txs {
			if s.bc.VerifyTransaction(tx) {
				validTxs = append(validTxs, tx)
			}
		}

		// 코인베이스 트랜잭션 추가
		coinbaseTx := NewCoinbaseTX(s.miningAddress, "")
		validTxs = append(validTxs, coinbaseTx) // 원래는 첫 번째 트랜잭션으로 포험되어야 함.

		// 채굴(PoW)
		tipHash, lastHeight := s.bc.GetTipInfo()
		newBlock := NewBlock(validTxs, tipHash, lastHeight+1)

		pow := NewProofOfWork(newBlock)
		nonce, hash := pow.Run()
		newBlock.Nonce = nonce
		newBlock.Hash = hash

		err := s.bc.AddBlock(newBlock)
		if err != nil {
			fmt.Printf("Error while mining (AddBlock failed): %v\n", err)
			continue // 포크가 발생했거나 유효하지 않은 tx가 껴있을 수 있음
		}

		// 블록에 포함된 트랜잭션들을 멤풀에서 비우기
		s.mempool.Clear(newBlock)

		// 새 블록 전파
		s.broadcastInv("block", [][]byte{newBlock.Hash})

	}
}

func (s *Server) broadcastInv(kind string, items [][]byte) {
	for nodeAddr := range s.knownNodes {
		if nodeAddr != s.nodeAddress {
			s.sendInv(nodeAddr, kind, items)
		}
	}
}

func commandToBytes(command string) []byte {
	var bytes [commandLen]byte

	for i, c := range command {
		bytes[i] = byte(c)
	}

	return bytes[:]
}

func bytesToCommand(bytes []byte) string {
	var command []byte

	// 0이 아닌 바이트만 추출
	for _, b := range bytes {
		if b != 0x0 {
			command = append(command, b)
		}
	}

	return string(command)
}

func sendData(addr string, data []byte) {
	conn, err := net.Dial(protocol, addr)
	if err != nil {
		fmt.Printf("%s is not available\n", addr)
		// 실제로는 knownNodes에서 제거하는 로직 필요
		return
	}
	defer conn.Close()

	_, err = io.Copy(conn, bytes.NewReader(data))
	if err != nil {
		log.Panic(err)
	}
}

// 'version' 메시지 전송
func (s *Server) sendVersion(addr string) {
	bestHeight := s.bc.GetBestHeight()

	ver := Version{
		Version:    nodeVersion,
		BestHeight: bestHeight,
		AddrFrom:   s.nodeAddress,
	}
	verMsg := append(commandToBytes("version"), gobEncode(ver)...)
	sendData(addr, verMsg)
}

// 'version' 메시지 처리
func (s *Server) handleVersion(payload []byte) {
	var buf bytes.Buffer
	var version Version

	buf.Write(payload)
	dec := gob.NewDecoder(&buf)
	if err := dec.Decode(&version); err != nil {
		log.Panic(err)
	}

	fmt.Printf("Received version: Height %d from %s\n", version.BestHeight, version.AddrFrom)

	myBestHeight := s.bc.GetBestHeight()
	opBestHeight := int64(version.BestHeight)

	// 상대방의 bestHeight가 나보다 높으면 getBlocks 메시지를 전송해서 받아오기
	if myBestHeight < int64(version.BestHeight) {
		s.sendGetBlocks(version.AddrFrom)
	} else if myBestHeight > opBestHeight {
		// 상대방의 bestHeight가 나보다 낮으면 내 version을 보내줌
		s.sendVersion(version.AddrFrom)
	}
	// 새로운 노드 주소를 knownNodes에 추가
	s.knownNodes[version.AddrFrom] = true
}

func (s *Server) sendTx(tx *Transaction, addr string) {
	var txBytes bytes.Buffer
	if err := gob.NewEncoder(&txBytes).Encode(tx); err != nil {
		log.Panic(err)
	}

	request := append(commandToBytes("tx"), txBytes.Bytes()...)
	sendData(addr, request)
}

// 트랜잭션 데이터를 수신
func (s *Server) handleTx(payload []byte) {
	var txMsg TxMsg
	if err := gob.NewDecoder(bytes.NewReader(payload)).Decode(&txMsg); err != nil {
		log.Panic(err)
	}

	var tx Transaction
	if err := gob.NewDecoder(bytes.NewReader(txMsg.Transaction)).Decode(&tx); err != nil {
		log.Panic(err)
	}

	txID := hex.EncodeToString(tx.ID)

	// 멤풀에 이미 있는지 확인
	if _, ok := s.mempool.transactions[txID]; ok {
		return // 이미 있는 트랜잭션이면 처리할 필요 없음. 종료
	}

	// 트랜잭션 검증, 유효한 트랜잭션이면
	if !s.bc.VerifyTransaction(&tx) {
		fmt.Printf("[Tx] Invalid transaction %x received.\n", tx.ID)
		return
	}

	// 멤풀에 트랜잭션 추가
	if added := s.mempool.Add(&tx); added {
		fmt.Printf("[Tx] Added tx %x to mempool (size: %d)\n", tx.ID, len(s.mempool.transactions))
		// 이 트랜잭션을 다른 노드들에게도 전파
		s.broadcastTx(&tx, txMsg.AddrFrom)
	}

}

func (s *Server) broadcastTx(tx *Transaction, addrFrom string) {
	for node := range s.knownNodes {
		// 나와 이 메시지를 보낸 노드를 제외하고 보내기
		if node != s.nodeAddress && node != addrFrom {
			s.sendTx(tx, node)
		}
	}
}

// 'Inv' 메시지 전송
func (s *Server) sendInv(addr, kind string, items [][]byte) {
	inv := Inv{
		AddrFrom: s.nodeAddress,
		Type:     kind,
		Items:    items,
	}
	payload := gobEncode(inv)
	request := append(commandToBytes("inv"), payload...)

	sendData(addr, request)
}

// 'Inv' 메시지 처리
// 다른 노드의 블록 해시 목록을 받아서, 나한테 없는 블록을 요청
func (s *Server) handleInv(payload []byte) {
	var inv Inv

	if err := gob.NewDecoder(bytes.NewReader(payload)).Decode(&inv); err != nil {
		log.Panic(err)
	}

	fmt.Printf("Received inventory with %d %s items from %s\n", len(inv.Items), inv.Type, inv.AddrFrom)

	if inv.Type == "block" {
		// 내가 가진 블록 해시를 맵으로 만듦 (빠른 조회를 위해)
		myHashes := s.bc.GetBlockHashes()
		myHashesMap := make(map[string]bool)

		for _, hash := range myHashes {
			myHashesMap[hex.EncodeToString(hash)] = true
		}

		// 상대방이 보낸 해시 목록을 순회하며,
		// 내가 없는 해시만 요청할 블록 슬라이스에 추가
		var hashesToRequest [][]byte
		for _, hash := range inv.Items {
			if !myHashesMap[hex.EncodeToString(hash)] {
				hashesToRequest = append(hashesToRequest, hash)
			}
		}

		// hashesToRequest가 비어있으면,
		// 요청할 데이터가 없으므로 return
		if len(hashesToRequest) == 0 {
			fmt.Println("No new blocks to request. We are synced.")
			return
		}

		// 목록 뒤집기
		// inv.Items는 bc.Iterator()에 의해 만들어졌고, Iterator는 최근것부터 순회하므로
		// hashesToRequest에 있는 값은 최근 블록해시부터 순차적으로 쌓인 값이기 때문에 목록을 뒤집이서,
		// 가장 과거의 블록부터 요청할 수 있도록 해야함.
		for i, j := 0, len(hashesToRequest)-1; i < j; i, j = i+1, j-1 {
			hashesToRequest[i], hashesToRequest[j] = hashesToRequest[j], hashesToRequest[i]
		}

		// 뒤집힌 목록을 '다운로드 큐' 로 설정
		blocksInTransit = hashesToRequest

		hashToRequest := hashesToRequest[0]
		s.sendGetData(inv.AddrFrom, "block", hashToRequest)

		fmt.Printf("Requesting block %x from %s\n", hashToRequest, inv.AddrFrom)
	}

	if inv.Type == "tx" {
		// TODO
	}
}

func (s *Server) sendGetBlocks(addr string) {
	payload := gobEncode(GetBlocks{AddrFrom: s.nodeAddress})
	request := append(commandToBytes("getblocks"), payload...)
	sendData(addr, gobEncode(request))
}

func (s *Server) handleGetBlocks(payload []byte) {
	var getBlocks GetBlocks
	if err := gob.NewDecoder(bytes.NewReader(payload)).Decode(&getBlocks); err != nil {
		log.Panic(err)
	}

	blockHashes := s.bc.GetBlockHashes()

	s.sendInv(getBlocks.AddrFrom, "block", blockHashes)
}

func (s *Server) sendGetData(addr, kind string, id []byte) {
	getData := GetData{
		AddrFrom: s.nodeAddress,
		Type:     kind,
		ID:       id,
	}

	request := append(commandToBytes("getdata"), gobEncode(getData)...)
	sendData(addr, request)
}

// 'getData' 요청을 처리
// 'getData'를 보낸 노드에게, 'block' 메시지로 응답
func (s *Server) handleGetData(payload []byte) {
	var getData GetData

	if err := gob.NewDecoder(bytes.NewReader(payload)).Decode(&getData); err != nil {
		log.Panic(err)
	}

	if getData.Type == "block" {
		block, err := s.bc.GetBlock(getData.ID)
		if err != nil {
			fmt.Printf("[GetData] Block %x not found\n", getData.ID)
			return
		}
		s.sendBlock(getData.AddrFrom, block)
	}

	if getData.Type == "tx" {
		tx := s.mempool.Get(hex.EncodeToString(getData.ID))
		if tx == nil {
			fmt.Printf("[GetData] TX %x not found in mempool\n", getData.ID)
			return
		}
		s.sendTx(tx, getData.AddrFrom)
	}
}

// 'block' 메시지 전송
// 블록 데이터를 전달
func (s *Server) sendBlock(addr string, block *Block) {
	blockData := block.Serialize()
	blockMsg := BlockMsg{
		AddrFrom: s.nodeAddress,
		Block:    blockData,
	}
	request := append(commandToBytes("block"), gobEncode(blockMsg)...)
	sendData(addr, request)
}

// 'block' 메시지를 처리
// 블록을 검증하고, DB에 블록을 추가
func (s *Server) handleBlock(payload []byte) {
	var blockMsg BlockMsg

	if err := gob.NewDecoder(bytes.NewReader(payload)).Decode(&blockMsg); err != nil {
		log.Panic(err)
	}

	block := DeserializeBlock(blockMsg.Block)
	fmt.Printf("Received a new block! Hash: %x, Height: %d\n", block.Hash, block.Height)

	err := s.bc.AddBlock(block)
	// 블록 추가 실패시
	if err != nil {
		fmt.Printf("Error adding block %x: %v\n", block.Hash, err)

		// 실패한 블록이 우리가 요청한 블록이면, 큐를 비워 이 피어로부터의 동기화를 중단
		// 블록 추가에 실패했다는건, 해당 피어가 보낸 블록이 가짜이거나, 깨졌거나, 우리 체인과 호환되지 않는 블록이라는 뜻
		// 따라서, 해당 피어로부터 계속 블록을 받는 것은 시간과 대역폭 낭비이고, 해당 피어가 보낸 해시 목록(Inv)도 신뢰할 수 없기 때문에
		// blockInTransit도 비움 (fail-fast)
		if len(blocksInTransit) > 0 && bytes.Equal(block.Hash, blocksInTransit[0]) {
			blocksInTransit = [][]byte{} // 큐 비우기
		}

		return
	}

	// 블록 추가에 성공하면 해당 블록에 있는 트랜잭션을 멤풀에서 제거
	s.mempool.Clear(block)

	// 블록 추가 성공시
	if len(blocksInTransit) > 0 {
		// 방금 추가된 블록이 큐의 첫번째 항목(요청했던 블륵)인지 확인
		if bytes.Equal(block.Hash, blocksInTransit[0]) {
			// 큐에서 첫번째 항목 제거(pop)
			blocksInTransit = blocksInTransit[1:]

			// 아직 동기화해야 할 블록이 남아있으면,
			if len(blocksInTransit) > 0 {
				nextHash := blocksInTransit[0]
				s.sendGetData(blockMsg.AddrFrom, "block", nextHash)
				fmt.Printf("Requesting next block %x from %s\n", nextHash, blockMsg.AddrFrom)
			} else {
				// 큐가 비었다면
				fmt.Println("Block sync complete.")
				fmt.Println("Re-indexing UTXO set for consistency...")
				// 대규모 동기화 후에는 UTXO Set을 Reindex 하여 상태를 확실히 맞추는 것이 안전함.
				utxoSet := UTXOSet{s.bc}
				utxoSet.Reindex()
				fmt.Println("UTXO set re-indexed.")
			}

		}
	}
}

// gob encoding 헬퍼 함수
func gobEncode(data any) []byte {
	var buf bytes.Buffer

	if err := gob.NewEncoder(&buf).Encode(data); err != nil {
		log.Panic(err)
	}

	return buf.Bytes()
}

func safeStringToInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		log.Panic(err)
	}
	return i
}
