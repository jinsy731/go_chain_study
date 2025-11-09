package core

import (
	"bytes"
	"encoding/gob"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"github.com/mr-tron/base58"
)

const defaultPort = "3000"

type CLI struct{}

func (cli *CLI) printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  startnode -port PORT - Start a node")
	fmt.Println("  createwallet - Gerenates a new key-pair and saves it into the wallet file")
	fmt.Println("  getbalance -address ADDRESS - Get balance of ADDRESS")
	fmt.Println("  reindexutxo - Rebuilds the UTXO set")
	fmt.Println("  send -from FROM -to TO -amount AMOUNT - Send AMOUNT of coins")
}

func (cli *CLI) validateArgs() {
	if len(os.Args) < 2 {
		cli.printUsage()
		os.Exit(1)
	}
}

func (cli *CLI) Run() {
	cli.validateArgs()

	// 명령어 플래그
	getBalanceCmd := flag.NewFlagSet("getbalance", flag.ExitOnError)
	getBalanceAddress := getBalanceCmd.String("address", "", "The address to get balance for")
	getBalancePort := getBalanceCmd.String("port", defaultPort, "Node port")

	reindexCmd := flag.NewFlagSet("reindexutxo", flag.ExitOnError)
	reindexPort := reindexCmd.String("port", defaultPort, "Node port")

	createWalletCmd := flag.NewFlagSet("createwallet", flag.ExitOnError)
	createWalletPort := createWalletCmd.String("port", defaultPort, "Node port")

	sendCmd := flag.NewFlagSet("send", flag.ExitOnError)
	sendFrom := sendCmd.String("from", "", "Source wallet address")
	sendTo := sendCmd.String("to", "", "Destination wallet address")
	sendAmount := sendCmd.Int("amount", 0, "Amount to send")
	sendPort := sendCmd.String("port", defaultPort, "Node port")

	startnodeCmd := flag.NewFlagSet("startnode", flag.ExitOnError)
	startnodePort := startnodeCmd.String("port", defaultPort, "Node port to listen on")
	startnodeMiner := startnodeCmd.String("miner", "", "Minig reward address (optional)")

	// 명령어 파싱
	// os.Args[1]	: 명령어
	// os.Args[2:]	: 옵션
	// $ executable-file getbalance -address xxxx...   -> Args[1]은 getbalance, Args[2:] 는 옵션
	switch os.Args[1] {
	case "startnode":
		err := startnodeCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "send":
		err := sendCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "getbalance":
		err := getBalanceCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "reindexutxo":
		err := reindexCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "createwallet":
		err := createWalletCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	default:
		cli.printUsage()
		os.Exit(1)
	}

	// startnode 명령어 실행 로직
	if startnodeCmd.Parsed() {
		if *startnodePort == "" {
			startnodeCmd.Usage()
			os.Exit(1)
		}

		log.Println("[startnode] port: ", *startnodePort)
		log.Println("[startnode] miner: ", *startnodeMiner)

		server := NewServer(*startnodePort, *startnodeMiner)
		server.Start()
	}

	// (getbalance - RPC 클라이언트)
	if getBalanceCmd.Parsed() {
		if *getBalanceAddress == "" || *getBalancePort == "" {
			getBalanceCmd.Usage()
			os.Exit(1)
		}

		req := GetBalanceRequest{Address: *getBalanceAddress}
		resp, err := sendRPCRequest(*getBalancePort, rpcCmdGetBalance, req)
		if err != nil {
			log.Panic(err)
		}
		if !resp.Success {
			log.Panic(fmt.Errorf("GetBalance failed: %s", resp.Message))
		}

		// 응답 페이로드 디코딩
		var balanceResp GetBalanceResponse
		if err := gob.NewDecoder(bytes.NewBuffer(resp.Data)).Decode(&balanceResp); err != nil {
			log.Panic(err)
		}
		fmt.Printf("Balance for '%s' (via node %s): %d\n", *getBalanceAddress, *getBalancePort, balanceResp.Balance)
	}

	// (createwallet - 로컬 실행, RPC 불필요)
	if createWalletCmd.Parsed() {
		if *createWalletPort == "" {
			createWalletCmd.Usage()
			os.Exit(1)
		}
		wallets, _ := NewWallets(*createWalletPort)
		address := wallets.CreateWallet()
		wallets.SaveToFile(*createWalletPort)
		fmt.Printf("Wallet for node %s created. Address: %s\n", *createWalletPort, address)
	}

	// (send - RPC 클라이언트)
	if sendCmd.Parsed() {
		if *sendFrom == "" || *sendTo == "" || *sendAmount <= 0 || *sendPort == "" {
			sendCmd.Usage()
			os.Exit(1)
		}

		// (주소 유효성 검사는 서버가 하지만, 클라도 미리 하는 것이 좋음)
		if !ValidateAddress(*sendFrom) || !ValidateAddress(*sendTo) {
			log.Panic("ERROR: Addresses are not valid")
		}

		req := SendRequest{From: *sendFrom, To: *sendTo, Amount: *sendAmount}
		resp, err := sendRPCRequest(*sendPort, rpcCmdSend, req)
		if err != nil {
			log.Panic(err)
		}
		if !resp.Success {
			log.Panic(fmt.Errorf("Send failed: %s", resp.Message))
		}
		fmt.Println("Send successful:", resp.Message)
	}

	// reindexutxo 명령어 실행 로직
	if reindexCmd.Parsed() {
		bc := NewBlockchain(*reindexPort)
		defer bc.Close()

		utxoSet := UTXOSet{bc}
		utxoSet.Reindex()

		fmt.Println("Done! UTXO Set has been reindexed")
	}

	// createWallet 명령어 실행 로직
	if createWalletCmd.Parsed() {
		wallets, _ := NewWallets(*createWalletPort) // 파일에서 로드
		address := wallets.CreateWallet()           // 새 지갑 추가
		wallets.SaveToFile(*createWalletPort)       // 파일에 저장
		fmt.Printf("Your new address: %s\n", address)
	}
}

func Base58Decode(input []byte) []byte {
	decode, err := base58.Decode(string(input))
	if err != nil {
		log.Panic(err)
	}
	return decode
}

func sendRPCRequest(port string, cmd string, payload interface{}) (RPCResponse, error) {
	rpcPort := fmt.Sprintf("localhost:%d", safeStringToInt(port)+rpcPortOffset)

	conn, err := net.Dial(protocol, rpcPort)
	if err != nil {
		return RPCResponse{Success: false}, fmt.Errorf("Node at port %s is not running (RPC connection failed: %v)", port, err)
	}
	defer conn.Close()

	// 1. 요청 생성
	req := RPCRequest{
		Command: commandToBytes(cmd),
		Payload: gobEncode(payload),
	}

	// 2. 요청 직렬화 및 전송
	var reqBuff bytes.Buffer
	if err := gob.NewEncoder(&reqBuff).Encode(req); err != nil {
		return RPCResponse{Success: false}, err
	}
	if _, err := io.Copy(conn, &reqBuff); err != nil {
		return RPCResponse{Success: false}, err
	}

	// 3. 응답 수신
	// (단순화를 위해, 서버가 응답을 닫을 때까지 읽음)
	respBytes, err := io.ReadAll(conn)
	if err != nil {
		return RPCResponse{Success: false}, fmt.Errorf("Failed to read RPC response: %v", err)
	}

	if len(respBytes) == 0 {
		return RPCResponse{Success: false}, fmt.Errorf("Received empty RPC response from node %s", port)
	}

	// 4. 응답 역직렬화
	var resp RPCResponse
	if err := gob.NewDecoder(bytes.NewReader(respBytes)).Decode(&resp); err != nil {
		return RPCResponse{Success: false}, fmt.Errorf("Failed to decode RPC response: %v", err)
	}

	return resp, nil
}
