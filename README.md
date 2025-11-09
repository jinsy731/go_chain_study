# Go Chain Study

A comprehensive blockchain implementation in Go featuring UTXO model, P2P networking, mining, and transaction propagation. This project demonstrates core blockchain concepts including proof-of-work consensus, distributed networking, and cryptographic transaction validation.

## Features

### Core Blockchain
- **UTXO Model**: Unspent Transaction Output system for efficient balance tracking
- **Proof of Work**: SHA-256 based mining with adjustable difficulty
- **Digital Signatures**: ECDSA signatures for transaction authorization
- **Merkle Trees**: Efficient transaction verification (structure in place)

### Networking & Distribution
- **P2P Protocol**: Custom binary protocol for node communication
- **Block Synchronization**: Automatic blockchain sync between nodes
- **Transaction Propagation**: Real-time transaction broadcasting across network
- **Mempool**: Transaction pool with validation and management

### Node Operations
- **Multi-node Support**: Independent node instances with port-based separation
- **Mining**: Configurable mining with coinbase rewards
- **RPC Interface**: Client-server communication for wallet operations
- **Persistent Storage**: BoltDB for blockchain and wallet data

### Wallet & Transactions
- **HD Wallets**: Hierarchical deterministic wallet generation
- **Address Validation**: Base58Check encoding with checksums
- **Transaction Creation**: UTXO-based transaction construction
- **Balance Queries**: Real-time balance calculation from UTXO set

## Architecture

```
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│   Node 3000     │  │   Node 3001     │  │   Node 3002     │
│  (Bootstrap)    │  │                 │  │                 │
├─────────────────┤  ├─────────────────┤  ├─────────────────┤
│ P2P Server :3000│◄─┤ P2P Client      │◄─┤ P2P Client      │
│ RPC Server :4000│  │ RPC Server :4001│  │ RPC Server :4002│
│ Miner (optional)│  │ Miner (optional)│  │ Miner (optional)│
└─────────────────┘  └─────────────────┘  └─────────────────┘
        │                      │                      │
        ▼                      ▼                      ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│ blockchain_3000 │  │ blockchain_3001 │  │ blockchain_3002 │
│ wallet_3000.dat │  │ wallet_3001.dat │  │ wallet_3002.dat │
└─────────────────┘  └─────────────────┘  └─────────────────┘
```

## Prerequisites

- **Go 1.21+**: Modern Go toolchain with module support
- **Network Access**: Open ports for P2P communication (default: 3000, 3001, etc.)

## Installation

1. **Clone the repository**:
```bash
git clone <repository-url>
cd go_chain_study
```

2. **Install dependencies**:
```bash
go mod tidy
```

3. **Build the binary**:
```bash
go build -o go-chain-study .
```

## Quick Start

### 1. Start the Bootstrap Node
```bash
# Terminal 1: Start bootstrap node with mining
./go-chain-study startnode -port 3000 -miner <MINER_ADDRESS>
```

### 2. Create Wallets
```bash
# Create wallet for node 3000
./go-chain-study createwallet -port 3000

# Note the generated address - you'll need it for mining rewards
```

### 3. Start Additional Nodes
```bash
# Terminal 2: Start second node
./go-chain-study startnode -port 3001

# Terminal 3: Start third node with mining
./go-chain-study createwallet -port 3001
./go-chain-study startnode -port 3001 -miner <NODE_3001_ADDRESS>
```

### 4. Monitor and Transact
```bash
# Check balance
./go-chain-study getbalance -address <ADDRESS> -port 3000

# Send transaction
./go-chain-study send -from <FROM_ADDRESS> -to <TO_ADDRESS> -amount 5 -port 3000

# Reindex UTXO set if needed
./go-chain-study reindexutxo -port 3000
```

## Command Reference

### Node Management
```bash
# Start a node (with optional mining)
./go-chain-study startnode -port <PORT> [-miner <MINING_ADDRESS>]

# Examples:
./go-chain-study startnode -port 3000                    # Bootstrap node
./go-chain-study startnode -port 3001 -miner <ADDRESS>   # Mining node
```

### Wallet Operations
```bash
# Create new wallet
./go-chain-study createwallet -port <PORT>

# Check balance
./go-chain-study getbalance -address <ADDRESS> -port <PORT>

# Send transaction
./go-chain-study send -from <FROM> -to <TO> -amount <AMOUNT> -port <PORT>
```

### Maintenance
```bash
# Rebuild UTXO index
./go-chain-study reindexutxo -port <PORT>
```

## Network Protocol

### P2P Messages
- **version**: Node capability and blockchain height exchange
- **getblocks**: Request block inventory from peer
- **inv**: Advertise available blocks or transactions
- **getdata**: Request specific block or transaction data
- **block**: Block data transmission
- **tx**: Transaction propagation

### RPC Interface
- **getbalance**: Query address balance via UTXO set
- **sendtx**: Create and broadcast new transaction

## File Structure

```
.
├── core/
│   ├── chain.go        # Blockchain core logic
│   ├── block.go        # Block structure and mining
│   ├── transaction.go  # Transaction handling and validation
│   ├── utxoset.go     # UTXO set management
│   ├── wallet.go      # Wallet operations
│   ├── server.go      # P2P networking
│   ├── rpc.go         # RPC server implementation
│   ├── mempool.go     # Transaction pool
│   └── cli.go         # Command line interface
├── main.go            # Application entry point
└── README.md          # This file
```

## Storage Format

### Database Layout (BoltDB)
- **blocks**: `hash -> block_data`
- **utxoBucket**: `tx_id -> utxo_list`
- **metadata**: `"l" -> last_block_hash`

### Wallet Format
- **File**: `wallet_<port>.dat` (Gob encoded)
- **Structure**: `address -> private_key_mapping`

## Mining Process

1. **Transaction Collection**: Gather valid transactions from mempool
2. **Block Creation**: Construct block with coinbase + transactions
3. **Proof of Work**: Find nonce satisfying difficulty target
4. **Block Addition**: Validate and add block to local chain
5. **UTXO Update**: Update unspent transaction output set
6. **Network Broadcast**: Propagate new block to peers

## Network Behavior

### Block Synchronization
1. New node connects to bootstrap node (localhost:3000)
2. Exchanges version messages with blockchain height
3. Requests missing blocks via getblocks/inv/getdata sequence
4. Downloads and validates blocks in chronological order
5. Updates local blockchain and UTXO set

### Transaction Flow
1. Transaction created via CLI send command
2. Added to local mempool after validation
3. Broadcasted to known peers via P2P network
4. Included in next mined block
5. Removed from mempool after block confirmation

## Development Notes

### Key Design Decisions
- **Port-based Isolation**: Each node uses `<port>` for P2P and `<port+1000>` for RPC
- **Deterministic Genesis**: Fixed genesis block prevents initialization inconsistencies
- **Incremental UTXO Updates**: Efficient balance tracking without full blockchain scan
- **Async Block Sync**: Non-blocking blockchain synchronization

### Known Limitations
- **Single Chain**: No fork resolution mechanism
- **Basic Difficulty**: Fixed difficulty target, no adjustment algorithm
- **Simple P2P**: Limited peer discovery and connection management
- **No Persistence**: Node restart requires resync from bootstrap

## Contributing

1. Fork the repository
2. Create a feature branch
3. Implement changes with tests
4. Submit a pull request

## License

This project is for educational purposes. See LICENSE file for details.