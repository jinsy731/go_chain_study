package core

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"net"
)

const (
	rpcCmdGetBalance    = "getbalance"
	rpcCmdSend          = "sendtx"
	rpcCmdGetBestHeight = "getbestheight"
)

type RPCRequest struct {
	Command []byte // 12바이트
	Payload []byte // GOB
}

type GetBalanceRequest struct {
	Address string
}

type SendRequest struct {
	From   string
	To     string
	Amount int
}

type RPCResponse struct {
	Success bool
	Message string
	Data    []byte // GOB
}

type GetBalanceResponse struct {
	Balance int
}

func (s *Server) startRPCListener() {
	ln, err := net.Listen(protocol, fmt.Sprintf("localhost:%s", s.rpcPort))
	if err != nil {
		log.Panic(err)
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Panic(err)
		}
		go s.handleRPCConnection(conn)
	}
}

func (s *Server) handleRPCConnection(conn net.Conn) {
	defer conn.Close()

	var req RPCRequest
	if err := gob.NewDecoder(conn).Decode(&req); err != nil {
		if err != io.EOF {
			log.Printf("Failed to decode RPC request: %v\n", err)
		}
		return
	}

	command := bytesToCommand(req.Command)
	payload := req.Payload

	fmt.Printf("Received RPC command: %s\n", command)

	var response RPCResponse

	switch command {
	case rpcCmdGetBalance:
		response = s.rpcGetBalance(payload)
	case rpcCmdSend:
		response = s.rpcSend(payload)
	default:
		response = RPCResponse{Success: false, Message: "Unknown RPC command"}
	}

	if err := gob.NewEncoder(conn).Encode(response); err != nil {
		log.Printf("Failed to send RPC response: %v\n", err)
	}
}

func (s *Server) rpcGetBalance(payload []byte) RPCResponse {
	var req GetBalanceRequest

	if err := gob.NewDecoder(bytes.NewReader(payload)).Decode(&req); err != nil {
		log.Panic(err)
	}

	if !ValidateAddress(req.Address) {
		return RPCResponse{
			Success: false,
			Message: "Invalid address",
		}
	}
	// address -> base58 decode -> version, checksum 부분 제거 -> pubKeyHash
	pubKeyHash := Base58Decode([]byte(req.Address))
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-addressChecksumLen]

	// UTXO Set에서 balance 조회
	utxoSet := UTXOSet{Blockchain: s.bc}
	balance := utxoSet.GetBalance(pubKeyHash)

	resData := gobEncode(GetBalanceResponse{
		Balance: balance,
	})

	return RPCResponse{
		Success: true,
		Data:    resData,
	}
}

// 거래 생성 및 전파 요청
func (s *Server) rpcSend(payload []byte) RPCResponse {
	var req SendRequest
	if err := gob.NewDecoder(bytes.NewReader(payload)).Decode(&req); err != nil {
		log.Panic(err)
	}

	// 서버가 자신의 포트(p2p port)로 지갑 파일 로드
	wallets, err := NewWallets(s.p2pPort)
	if err != nil {
		return RPCResponse{Success: false, Message: "Server wallet file not found"}
	}

	wallet, ok := wallets.GetWallet(req.From)
	if !ok {
		return RPCResponse{Success: false, Message: "Sender wallet not found in this node's wallet file"}
	}
	// 유효성 검사
	if !ValidateAddress(req.From) || !ValidateAddress(req.To) || req.Amount <= 0 {
		return RPCResponse{Success: false, Message: "Invalid send request parameters"}
	}

	// 트랜잭션 생성
	tx, err := s.bc.NewTransaction(wallet, req.To, req.Amount)
	if err != nil {
		return RPCResponse{Success: false, Message: fmt.Sprintf("TX creation failed: %v", err)}
	}

	// 멤풀에 추가 및 다른 노드에 전파
	if added := s.mempool.Add(tx); !added {

	}
	// TODO: broadcast 로직 추가
	s.broadcastTx(tx, s.nodeAddress)

	return RPCResponse{Success: true, Message: fmt.Sprintf("TX %x sent to mempool.", tx.ID)}
}
