package core

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/mr-tron/base58"
)

type CLI struct{}

func (cli *CLI) printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  createwallet - Gerenates a new key-pair and saves it into the wallet file")
	fmt.Println("  getbalance -address ADDRESS - Get balance of ADDRESS")
	fmt.Println("  reindexutxo - Rebuilds the UTXO set")
	fmt.Println("  (추가 예정) send -from FROM -to TO -amount AMOUNT - Send AMOUNT of coins")
	fmt.Println("  (추가 예정) printchain - Print all the blocks of the blockchain")
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
	createblockchainCmd := flag.NewFlagSet("createblockchain", flag.ExitOnError)
	getBalanceCmd := flag.NewFlagSet("getbalance", flag.ExitOnError)
	reindexCmd := flag.NewFlagSet("reindexutxo", flag.ExitOnError)
	createWalletCmd := flag.NewFlagSet("createwallet", flag.ExitOnError)

	// getbalance 명령여의 하위 옵션
	getBalanceAddress := getBalanceCmd.String("address", "", "The address to get balance for")
	createBlockchainAddress := createblockchainCmd.String("address", "", "The address to send genesis block reward to")

	fmt.Println("os.Args[1] :", os.Args[1])
	fmt.Println("os.Args[2:] :", os.Args[2:])

	// 명령어 파싱
	// os.Args[1]	: 명령어
	// os.Args[2:]	: 옵션
	// $ executable-file getbalance -address xxxx...   -> Args[1]은 getbalance, Args[2:] 는 옵션
	switch os.Args[1] {
	case "createblockchain":
		err := createblockchainCmd.Parse(os.Args[2:])
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

	// createblockchain 명령어 실행 로직
	if createblockchainCmd.Parsed() {
		if *createBlockchainAddress == "" {
			createblockchainCmd.Usage()
			os.Exit(1)
		}
		// CreateBlockchain 호출
		bc := CreateBlockchain(*createBlockchainAddress)
		defer bc.Close()

		// 생성 직후, UTXO Set 인덱싱
		utxoSet := UTXOSet{bc}
		utxoSet.Reindex()
		fmt.Println("Done! Blockchain created and UTXO set indexed.")
	}

	// getbalance 명령어 실행 로직
	if getBalanceCmd.Parsed() {
		if *getBalanceAddress == "" {
			getBalanceCmd.Usage()
			os.Exit(1)
		}

		// 주소 유효성 검사
		if !ValidateAddress(*getBalanceAddress) {
			log.Panic("ERROR: Address is not valid")
		}

		// PubKeyHash 추출
		pubKeyHash := Base58Decode([]byte(*getBalanceAddress))
		pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-addressChecksumLen]

		// 블록체인 로드
		bc := NewBlockchain("") // 이제 NewBlockchain은 주소가 필요 없음 (CLI가 생성하므로)
		defer bc.Close()

		// UTXOSet 생성 및 잔액 조회
		utxoSet := UTXOSet{bc}
		balance := utxoSet.GetBalance(pubKeyHash)

		fmt.Printf("Balance of %s : %d\n", *getBalanceAddress, balance)
	}

	// reindexutxo 명령어 실행 로직
	if reindexCmd.Parsed() {
		bc := NewBlockchain("")
		defer bc.Close()

		utxoSet := UTXOSet{bc}
		utxoSet.Reindex()

		fmt.Println("Done! UTXO Set has been reindexed")
	}

	// createWallet 명령어 실행 로직
	if createWalletCmd.Parsed() {
		wallet := NewWallet()
		address := wallet.GetAddress()
		fmt.Printf("Your new address: %s\n", address)

		// 실제로는 wallet.dat 파일에 개인키를 저장해야 함. 지금은 주소만 출력
	}
}

func Base58Decode(input []byte) []byte {
	decode, err := base58.Decode(string(input))
	if err != nil {
		log.Panic(err)
	}
	return decode
}
