# SimpleCoin

A learning project that explores a minimal UTXO-based blockchain implemented in Go.  
The CLI now supports multi-node experiments: each node keeps its own blockchain and wallet files keyed by the listening port, and a lightweight version message is exchanged when peers connect.

## Features

- Deterministic genesis block with a fixed coinbase to bootstrap nodes without mining.
- Port-scoped storage files (`blockchain_<port>.db`, `wallet_<port>.dat`) so multiple local nodes can coexist.
- CLI helpers to create wallets, query balances, send transactions, and rebuild the UTXO set.
- Experimental P2P server (`startnode`) that sends a version handshake to the bootstrap node (`localhost:3000`).

## Prerequisites

- Go 1.21+ (any recent Go toolchain should work)

Module dependencies (`bbolt`, `btcec`, Base58) are tracked in `go.mod`.  
Run `go mod tidy` if you need to download them again.

## Build

```bash
go build -o go-chain-study .
```

The examples below assume the binary is named `go-chain-study` and executed from the project root.

## Usage

### 1. Start a node

```bash
./go-chain-study startnode -port 3000
```

Port `3000` acts as the bootstrap node. When you launch another node (for example `3001`), it will dial the bootstrap node and exchange a version message.

### 2. Create a wallet for the node

```bash
./go-chain-study createwallet -port 3000
```

Wallets are stored in `wallet_<port>.dat`. Repeat the command for every node you run.

### 3. Check balances

```bash
./go-chain-study getbalance -address <ADDRESS> -port 3000
```

Each node maintains an independent UTXO set in `blockchain_<port>.db`. Omitting `-port` falls back to the default port (`3000`).

### 4. Send coins

```bash
./go-chain-study send -from <FROM_ADDRESS> -to <TO_ADDRESS> -amount 5 -port 3000
```

Transactions are verified and mined into a new block locally. The P2P layer does not yet broadcast blocks between nodes.

### 5. Rebuild the UTXO index

```bash
./go-chain-study reindexutxo -port 3000
```

This scans all blocks and rewrites the UTXO bucket for the specified node.

## Tips

- The genesis block is created on demand with a fixed reward address, so you no longer need a separate `createblockchain` step.
- Delete `blockchain_<port>.db` and `wallet_<port>.dat` if you want to reset a node.
- The networking layer currently handles only the version handshake; block or transaction propagation is still a work in progress.
