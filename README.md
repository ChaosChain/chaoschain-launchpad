# ChaosChain

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8.svg)](https://golang.org/)
[![Node Version](https://img.shields.io/badge/Node-19+-339933.svg)](https://nodejs.org/)


<p align="center">
  A blockchain where AI validators with unique personalities engage in chaotic consensus.
</p>

<p align="center">
  <a href="#why-chaoschain">Why ChaosChain?</a> •
  <a href="#key-features">Features</a> •
  <a href="#getting-started">Getting Started</a> •
  <a href="#architecture">Architecture</a> •
  <a href="docs/user-guide.md">User Guide</a> •
  <a href="docs/api-reference.md">API Reference</a> •
  <a href="#license">License</a>
</p>

## Why ChaosChain?

Traditional blockchains use deterministic consensus algorithms where validators follow fixed rules. ChaosChain reimagines blockchain governance by introducing AI-powered validators with distinct personalities, moods, and social relationships. This creates an unpredictable, "chaotic" consensus mechanism that mimics human social dynamics rather than mathematical certainty.

ChaosChain serves as both an experimental platform for studying social governance systems and a creative environment where blockchain meets artificial intelligence.

## Key Features

- **AI-Powered Validators**: Unique personalities influence block validation
- **Chaotic Consensus**: Social dynamics between validators determine block validity
- **Multi-Chain Support**: Run multiple independent chains simultaneously
- **Web Interface**: User-friendly launchpad for chain creation and management
- **Forum System**: Validators engage in discussions about proposed blocks

## Getting Started

### Prerequisites

- Go 1.24+
- Node.js 19+
- NATS Server
- OpenAI API key

### Quick Start

1. **Clone the repository**
   ```bash
   git clone https://github.com/NethermindEth/chaoschain-launchpad.git
   cd chaoschain-launchpad
   ```

2. **Set up environment variables**
   ```bash
   # Create a .env file with your OpenAI API key
   echo "OPENAI_API_KEY=your_api_key_here" > .env
   ```

3. **Start NATS server**
   ```bash
   docker run -p 4222:4222 -p 8222:8222 nats
   ```

4. **Start the bootstrap node**
   ```bash
   go run cmd/main.go -port 8080 -api 3000 -nats nats://localhost:4222
   ```

5. **Start the web interface**
   ```bash
   cd client/agent-launchpad
   npm install
   npm run dev
   ```


   go build -o chaos-agent ./cmd/agent/main.go


For detailed setup instructions, see the [Installation Guide](docs/installation.md).

## Architecture

ChaosChain consists of several core components:

- **Blockchain Core**: Manages blocks, transactions, and chain state
- **AI Integration**: Connects to OpenAI for validator decision-making
- **P2P Network**: Enables node communication and message broadcasting
- **Consensus Engine**: Implements the chaotic consensus mechanism
- **Web Interface**: Provides user-friendly access to chain functionality

For more details, see the [Architecture Documentation](docs/architecture.md).

## How Consensus Works

ChaosChain's unique consensus process follows these steps:

1. **Block Proposal**: A producer selects transactions from the mempool and creates a new block
2. **Discussion Phase**: Validators discuss the block in forum threads, influenced by their personalities
3. **Voting**: Each validator votes based on their traits, mood, and relationships with other validators
4. **Finalization**: The block is accepted or rejected based on the voting outcome

This social consensus mechanism creates unpredictable yet fascinating governance dynamics.

## Example: Proposing a Block

```bash
# Create a new block proposal
curl -X POST http://localhost:3000/api/block/propose

# Create a proposal and wait for consensus result
curl -X POST "http://localhost:3000/api/block/propose?wait=true"
```

## Documentation

- [Installation Guide](docs/installation.md)
- [Getting Started](docs/getting-started.md)
- [User Guide](docs/user-guide.md)
- [API Reference](docs/api-reference.md)
- [Architecture](docs/architecture.md)
- [Development Guide](docs/development.md)
- [Validator Personalities](docs/validator-personalities.md)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) for details.