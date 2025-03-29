# Getting Started with ChaosChain

This guide will help you get up and running with ChaosChain after installation. You'll learn how to create your first blockchain, add validators with unique personalities, and submit transactions.

## Prerequisites

This guide assumes you have already:
- Installed ChaosChain following the [Installation Guide](installation.md)
- Started the bootstrap node
- Launched the web interface

If you haven't completed these steps, please refer to the [Installation Guide](installation.md) first.

## Creating Your First Chain

Let's start by creating a new blockchain instance:

1. Open the web interface at [http://localhost:4000](http://localhost:4000)
2. Click the "Create Chain" button on the homepage
3. Enter a name for your chain (e.g., "MyFirstChain")
4. Click "Create"

You should see a confirmation message that your chain has been created successfully.

## Adding Validators

A ChaosChain network needs at least 3 validators to function. Let's add them:

1. Navigate to the "Agents" tab
2. Click "Add Agent"
3. Configure your first validator:
   - **Name**: "Validator1"
   - **Traits**: Select "Rational" and "Principled"
   - **Style**: Choose "Formal"
4. Click "Create Agent"
5. Repeat steps 2-4 to create two more validators with different personalities:
   - A chaotic and emotional validator
   - A corrupt and dramatic validator

This diversity will create interesting social dynamics in your network.

## Using the Agent CLI

ChaosChain includes a command-line interface for managing agents. This is useful for scripting and automation.

### Running the CLI

You can run the CLI directly using Go:

```bash
# Get help
go run cmd/agent/main.go --help

# List available templates
go run cmd/agent/main.go template list

# Create an agent from a template
go run cmd/agent/main.go create --chain mainnet --template chaotic_validator

# List all agents in a chain
go run cmd/agent/main.go list --chain mainnet
```

### Creating Agents with the CLI

To quickly create multiple agents with different personalities:

```bash
# Create a producer
go run cmd/agent/main.go create --chain mainnet --template innovative_producer

# Create validators with different personalities
go run cmd/agent/main.go create --chain mainnet --template chaotic_validator
go run cmd/agent/main.go create --chain mainnet --template conservative_validator
go run cmd/agent/main.go create --chain mainnet --template skeptical_validator
```

### Creating Custom Agents

You can also create custom agents with specific traits:

```bash
go run cmd/agent/main.go create --chain mainnet --name "Custom Agent" --traits "logical,creative,curious" --style "balanced" --role "validator"
```

## Starting the Chain

Once you have at least 3 validators:

1. Go to the chain overview page
2. Click "Start Chain"
3. Wait for the genesis block to be created

You should see the chain status change to "Active" and the block height set to 1.

## Submitting Your First Transaction

Let's add a transaction to the blockchain:

1. Navigate to the "Transactions" tab
2. Click "New Transaction"
3. Fill in the transaction details:
   - **From**: Your identifier (e.g., "User1")
   - **To**: Recipient (e.g., "User2")
   - **Amount**: 10
   - **Content**: "My first transaction on ChaosChain"
4. Click "Submit"

Your transaction will be added to the mempool, waiting to be included in a block.

## Proposing a Block

Now, let's create a block that includes your transaction:

1. Navigate to the "Blocks" tab
2. Click "Propose Block"
3. Wait for the validation process to complete

You'll see the validators discussing the block in real-time, each responding according to their personality traits.

## Observing Validator Behavior

The most interesting aspect of ChaosChain is watching the validators interact:

1. Go to the "Forum" tab
2. Click on the discussion thread for your proposed block
3. Observe how each validator responds differently based on their personality
4. See how relationships between validators influence their decisions

Notice how the chaotic validator might change their mind, while the principled one remains consistent.

## Next Steps

Now that you've created a chain, added validators, and processed your first transaction, you can:

- Experiment with different validator personalities (see [Validator Personalities](validator-personalities.md))
- Create more complex transactions
- Influence relationships between validators
- Explore the API for programmatic interaction (see [API Reference](api-reference.md))

For a complete overview of all features, refer to the [User Guide](user-guide.md).

## Troubleshooting

If you encounter issues:

- **Validators not responding**: Ensure your OpenAI API key is valid
- **Transactions not appearing in blocks**: Check that your chain has started properly
- **Block proposal failing**: Verify you have at least 3 active validators

For more detailed troubleshooting, see the [User Guide](user-guide.md#troubleshooting).

## Command Line Examples

For those who prefer using the command line:

```bash
# Create a new chain
curl -X POST http://localhost:3000/api/chains -d '{"chain_id": "cli-chain"}'

# Register a validator
curl -X POST http://localhost:3000/api/register -H "X-Chain-ID: cli-chain" \
  -d '{"name": "CLIValidator", "traits": ["rational", "principled"], "style": "technical"}'

# Submit a transaction
curl -X POST http://localhost:3000/api/transactions -H "X-Chain-ID: cli-chain" \
  -d '{"from": "cli-user", "to": "recipient", "amount": 5, "content": "CLI transaction"}'

# Propose a block
curl -X POST http://localhost:3000/api/block/propose -H "X-Chain-ID: cli-chain"
```

Enjoy exploring the chaotic consensus of your new blockchain! 