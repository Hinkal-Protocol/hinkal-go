# Hinkal Go SDK

Native Go SDK for Hinkal Protocol.

The Hinkal Go SDK enables Go applications to integrate confidential transactions and settlement flows on public blockchains while remaining non-custodial. It is a native Go implementation of the Hinkal protocol, designed for backend services, payment infrastructure, exchanges, wallets, and financial applications that require on-chain privacy.

Unlike language bindings, the SDK implements protocol functionality directly in Go. UTXO management, Merkle tree synchronization, note decryption, transaction construction, and blockchain interactions all run natively without requiring Node.js or WebAssembly. Zero-knowledge proofs are generated securely through the Hinkal proving infrastructure.

---

## Features

- Native Go implementation
- No Node.js runtime required
- No WebAssembly dependency
- Private deposits and withdrawals
- Private wallet-to-wallet transfers
- Shielded balances
- Multi-chain support
- Non-custodial architecture
- Viewing keys for optional compliance and auditing
- Designed for backend services and payment infrastructure

---

## Supported Networks

- Ethereum
- Polygon
- Base
- Arbitrum
- Optimism
- BNB Chain
- Solana
- Tron
- Tempo
- Arc Testnet

---

## Requirements

| Component | Version |
| --------- | ------- |
| Go        | 1.25+   |
| Linux     | ✅      |
| macOS     | ✅      |
| amd64     | ✅      |
| arm64     | ✅      |

---

## Installation

Install the SDK:

```bash
go get github.com/Hinkal-Protocol/hinkal-go
```

Import the SDK:

```go
import hinkal "github.com/Hinkal-Protocol/hinkal-go"
```

---

## Quick Start

Create a configuration, initialize the SDK, and begin interacting with Hinkal.

```go
package main

import (
	hinkal "github.com/Hinkal-Protocol/hinkal-go"
)

func main() {
	cfg := hinkal.NewConfig(
		// configuration
	)

	sdk, err := hinkal.New(cfg)
	if err != nil {
		panic(err)
	}

	_ = sdk
}
```

For complete initialization examples, provider configuration, and transaction flows, see the documentation.

---

## Documentation

Complete documentation is available on GitBook.

- Getting Started
- Go SDK Guide
- API Reference
- Integration Guides
- Examples

> https://hinkal-team.gitbook.io/hinkal/hinkal-sdk/go-sdk

---

## What You Can Build

The Go SDK enables applications such as:

- Payment processors
- Wallet infrastructure
- Exchanges
- Treasury systems
- Settlement services
- Custodial and non-custodial platforms
- Privacy-preserving financial applications

---

## Architecture

The SDK is implemented entirely in Go and provides native support for Hinkal protocol functionality.

Core responsibilities include:

- Transaction construction
- Shielded balance management
- UTXO management
- Merkle tree synchronization
- Note encryption and decryption
- Blockchain interaction
- Transaction submission

Zero-knowledge proofs are generated through Hinkal's secure proving infrastructure.

---

## Examples

Additional examples and integration guides are available in the documentation:

> https://hinkal-team.gitbook.io/hinkal/hinkal-sdk/go-sdk

---

## Contributing

Contributions are welcome.

If you discover a bug or have a feature request, please open an issue or submit a pull request.
