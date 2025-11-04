package core

import (
	"bytes"
	"encoding/gob"
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
	default:
		fmt.Println("Unknown command!")
	}
}

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
