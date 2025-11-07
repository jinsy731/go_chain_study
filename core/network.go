package core

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
)

const protocol = "tcp"
const nodeVersion = 1
const commandLen = 12 // 명령어 길이 (12바이트로 고정)

// 노드의 주소 (여기선 간단히 문자열 배열로 관리)
var knownNodes = []string{"localhost:3000"} // 3000번 포트가 부트스트랩 노드(첫 노드)
var nodeAddress string                      // 현재 노드의 주소 (예: localhost:3001)

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
	Version    int    // 블록체인 버전
	BestHeight int    // 이 노드가 가진 블록의 최고 높이
	AddrFrom   string // 이 메시지를 보낸 노드의 주소
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

	return fmt.Sprintf("%s", command)
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

// 연결 처리 핸들러 (다른 노드에서 이 노드로 연결했을 때 핸들링)
func handleConnection(conn net.Conn, bc *Blockchain) {
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
		handleVersion(payload, bc)
	case "getblocks":
		handleGetBlocks(payload, bc)
	case "inv":
		handleInv(payload, bc)
	case "getdata":
		handleGetData(payload, bc)
	case "block":
		handleBlock(payload, bc)
	default:
		fmt.Println("Unknown command!")
	}
}

// 'version' 메시지 전송
func sendVersion(addr string, bc *Blockchain) {
	bestHeight := 0 // DB에서 실제 마지막 블록 높이를 가져오는 로직 필요

	ver := Version{
		Version:    nodeVersion,
		BestHeight: bestHeight,
		AddrFrom:   nodeAddress,
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(ver); err != nil {
		log.Panic(err)
	}
	verMsg := append(commandToBytes("version"), buf.Bytes()...)
	sendData(addr, verMsg)
}

// 'version' 메시지 처리
func handleVersion(payload []byte, bc *Blockchain) {
	var buf bytes.Buffer
	var payloadData Version

	buf.Write(payload)
	dec := gob.NewDecoder(&buf)
	if err := dec.Decode(&payloadData); err != nil {
		log.Panic(err)
	}

	fmt.Printf("Received version: Height %d from %s\n", payloadData.BestHeight, payloadData.AddrFrom)

	// 상대방의 bestHeight가 나보다 높으면 getBlocks 메시지를 전송해서 받아오기
	// 상대방의 bestHeight가 나보다 낮으면 내 version을 보내줌
	// 새로운 노드 주소를 knownNodes에 추가
	// if !nodeIsKnown(payloadData.AddrFrom) {
	// 	knownNodes = append(knownNodes, payloadData.AddrFrom)
	// }
}

var blocksInTransit = [][]byte{}

// 'Inv' 메시지 전송
func sendInv(addr, kind string, items [][]byte) {
	inv := Inv{
		AddrFrom: nodeAddress,
		Type:     kind,
		Items:    items,
	}
	payload := gobEncode(inv)
	request := append(commandToBytes("inv"), payload...)

	sendData(addr, request)
}

// 'Inv' 메시지 처리
// 다른 노드의 블록 해시 목록을 받아서, 나한테 없는 블록을 요청
func handleInv(payload []byte, bc *Blockchain) {
	var inv Inv

	if err := gob.NewDecoder(bytes.NewReader(payload)).Decode(&inv); err != nil {
		log.Panic(err)
	}

	fmt.Printf("Received inventory with %d %s items from %s\n", len(inv.Items), inv.Type, inv.AddrFrom)

	if inv.Type == "block" {
		// 내가 가진 블록 해시를 맵으로 만듦 (빠른 조회를 위해)
		myHashes := bc.GetBlockHashes()
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
		sendGetData(inv.AddrFrom, "block", hashToRequest)

		fmt.Printf("Requesting block %x from %s\n", hashToRequest, inv.AddrFrom)
	}

	if inv.Type == "tx" {
		// TODO
	}
}

func sendGetBlocks(addr string) {
	payload := gobEncode(GetBlocks{AddrFrom: nodeAddress})
	request := append(commandToBytes("getblocks"), payload...)
	sendData(addr, gobEncode(request))
}

func handleGetBlocks(payload []byte, bc *Blockchain) {
	var getBlocks GetBlocks
	if err := gob.NewDecoder(bytes.NewReader(payload)).Decode(&getBlocks); err != nil {
		log.Panic(err)
	}

	blockHashes := bc.GetBlockHashes()

	sendInv(getBlocks.AddrFrom, "block", blockHashes)
}

func sendGetData(addr, kind string, id []byte) {
	getData := GetData{
		AddrFrom: nodeAddress,
		Type:     kind,
		ID:       id,
	}

	sendData(addr, gobEncode(getData))
}

// 'getData' 요청을 처리
// 'getData'를 보낸 노드에게, 'block' 메시지로 응답
func handleGetData(payload []byte, bc *Blockchain) {
	var getData GetData

	if err := gob.NewDecoder(bytes.NewReader(payload)).Decode(&getData); err != nil {
		log.Panic(err)
	}

}

// 'block' 메시지 전송
// 블록 데이터를 전달
func sendBlock(addr string, block *Block) {
	blockData := block.Serialize()
	blockMsg := BlockMsg{
		AddrFrom: nodeAddress,
		Block:    blockData,
	}

	sendData(addr, gobEncode(blockMsg))
}

// 'block' 메시지를 처리
// 블록을 검증하고, DB에 블록을 추가
func handleBlock(payload []byte, bc *Blockchain) {
	var blockMsg BlockMsg

	if err := gob.NewDecoder(bytes.NewReader(payload)).Decode(&blockMsg); err != nil {
		log.Panic(err)
	}

	block := DeserializeBlock(blockMsg.Block)
	fmt.Printf("Received a new block! Hash: %x, Height: %d\n", block.Hash, block.Height)

	err := bc.AddBlock(block)
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

	// 블록 추가 성공시
	if len(blocksInTransit) > 0 {
		// 방금 추가된 블록이 큐의 첫번째 항목(요청했던 블륵)인지 확인
		if bytes.Equal(block.Hash, blocksInTransit[0]) {
			// 큐에서 첫번째 항목 제거(pop)
			blocksInTransit = blocksInTransit[1:]

			// 아직 동기화해야 할 블록이 남아있으면,
			if len(blocksInTransit) > 0 {
				nextHash := blocksInTransit[0]
				sendGetData(blockMsg.AddrFrom, "block", nextHash)
				fmt.Printf("Requesting next block %x from %s\n", nextHash, blockMsg.AddrFrom)
			} else {
				// 큐가 비었다면
				fmt.Println("Block sync complete.")
				fmt.Println("Re-indexing UTXO set for consistency...")
				// 대규모 동기화 후에는 UTXO Set을 Reindex 하여 상태를 확실히 맞추는 것이 안전함.
				utxoSet := UTXOSet{bc}
				utxoSet.Reindex()
				fmt.Println("UTXO set re-indexed.")
			}

		}
	}
}

// P2P 서버 시작
func StartServer(port string, bc *Blockchain) {
	nodeAddress = fmt.Sprintf("localhost:%s", port)

	ln, err := net.Listen(protocol, nodeAddress)
	if err != nil {
		log.Panic(err)
	}
	defer ln.Close()

	fmt.Printf("Node is listening on %s\n", nodeAddress)

	// IMPORTANT: 첫 노드(3000)가 아니라면, 첫 노드에게 'version' 메시지 전송
	if nodeAddress != knownNodes[0] {
		sendVersion(knownNodes[0], bc) // 부트스트랩 노드에 접속 요청
	}

	// 새로운 연결 수락 (무한 루프)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Panic(err)
		}

		// 각 연결을 별도의 고루틴으로 처리
		go handleConnection(conn, bc)
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
