package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/NethermindEth/chaoschain-launchpad/ai"
	"github.com/NethermindEth/chaoschain-launchpad/api"
	"github.com/NethermindEth/chaoschain-launchpad/cmd/node"
	"github.com/NethermindEth/chaoschain-launchpad/core"
	"github.com/NethermindEth/chaoschain-launchpad/registry"
	"github.com/NethermindEth/chaoschain-launchpad/utils"
	cfg "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/privval"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize the agent registry
	registry.InitRegistry()

	// Initialize the AI
	ai.InitAI()

	// Parse command line flags
	agentID := flag.String("agent-id", "", "Agent ID")
	chainID := flag.String("chain", "mainnet", "Chain ID")
	p2pPort := flag.Int("p2p-port", 26656, "CometBFT P2P port")
	rpcPort := flag.Int("rpc-port", 26657, "CometBFT RPC port")
	apiPort := flag.Int("api-port", 0, "API server port (0 for auto-assign)")
	genesisNodeID := flag.String("genesis-node-id", "", "Genesis node ID")
	genesisP2PPort := flag.Int("genesis-p2p-port", 26656, "Genesis node P2P port")
	role := flag.String("role", "validator", "Node role (validator or producer)")
	flag.Parse()

	if *chainID == "" || *agentID == "" {
		log.Fatal("chain and agent-id are required")
	}

	rootDir := fmt.Sprintf("./data/%s/%s", *chainID, *agentID)

	config := cfg.DefaultConfig()

	config.Consensus.TimeoutPropose = 10 * time.Second
	config.Consensus.TimeoutPrevote = 10 * time.Second
	config.Consensus.TimeoutPrecommit = 10 * time.Second
	config.Consensus.TimeoutCommit = 15 * time.Second

	config.BaseConfig.RootDir = rootDir
	config.Moniker = *agentID
	config.P2P.ExternalAddress = fmt.Sprintf("tcp://127.0.0.1:%d", *p2pPort)
	config.P2P.ListenAddress = fmt.Sprintf("tcp://0.0.0.0:%d", *p2pPort)
	config.RPC.ListenAddress = fmt.Sprintf("tcp://0.0.0.0:%d", *rpcPort)

	config.P2P.SeedMode = false
	config.P2P.PexReactor = true
	config.P2P.AllowDuplicateIP = true
	config.P2P.AddrBookStrict = false

	// Get genesis node ID from its node_key.json if seed not provided
	if *genesisNodeID == "" {
		genesisNodeKeyFile := fmt.Sprintf("./data/%s/genesis/config/node_key.json", *chainID)
		if _, err := os.Stat(genesisNodeKeyFile); err == nil {
			genesisNodeKey, err := p2p.LoadNodeKey(genesisNodeKeyFile)
			if err == nil {
				*genesisNodeID = fmt.Sprintf("%s@127.0.0.1:%d", genesisNodeKey.ID(), *genesisP2PPort)
				log.Printf("Using genesis node as seed: %s", *genesisNodeID)
			}
		}
	}

	// Set seed node
	if *genesisNodeID != "" {
		config.P2P.Seeds = *genesisNodeID
		log.Printf("Using seed node: %s", *genesisNodeID)
	}

	address := ""

	// Set validator mode if specified
	if *role == "validator" {
		log.Printf("Starting node as validator")

		// Create required directories first
		if err := os.MkdirAll(rootDir+"/config", 0755); err != nil {
			log.Fatalf("Failed to create config directory: %v", err)
		}
		if err := os.MkdirAll(rootDir+"/data", 0755); err != nil {
			log.Fatalf("Failed to create data directory: %v", err)
		}

		// Copy genesis file from genesis node
		genesisFile := fmt.Sprintf("./data/%s/genesis/config/genesis.json", *chainID)
		if !utils.FileExists(genesisFile) {
			log.Fatalf("Genesis file not found at %s. Is genesis node running?", genesisFile)
		}

		// Read genesis file
		genesisBytes, err := os.ReadFile(genesisFile)
		if err != nil {
			log.Fatalf("Failed to read genesis file: %v", err)
		}

		// Write to new node's config directory
		newGenesisFile := fmt.Sprintf("%s/config/genesis.json", rootDir)
		if err := os.WriteFile(newGenesisFile, genesisBytes, 0644); err != nil {
			log.Fatalf("Failed to write genesis file: %v", err)
		}

		// Now generate/load validator key
		privValKeyFile := fmt.Sprintf("%s/config/priv_validator_key.json", rootDir)
		privValStateFile := fmt.Sprintf("%s/data/priv_validator_state.json", rootDir)
		if !utils.FileExists(privValKeyFile) {
			privVal := privval.GenFilePV(privValKeyFile, privValStateFile)
			privVal.Save()
		}

		// Load the validator's public key
		privVal := privval.LoadFilePV(privValKeyFile, privValStateFile)
		pubKey, err := privVal.GetPubKey()
		address = pubKey.Address().String()
		if err != nil {
			log.Fatalf("Failed to get validator public key: %v", err)
		}

		// Create a transaction to register the validator
		validatorTx := core.Transaction{
			Type:      "register_validator",
			From:      *agentID,
			To:        "",
			Amount:    0,
			Fee:       0,
			Content:   "",
			Timestamp: time.Now().Unix(),
			Signature: "",
			PublicKey: "", // optional: could be base64.StdEncoding.EncodeToString(pubKey.Bytes())
			ChainID:   *chainID,
			Hash:      nil,
			Data:      pubKey.Bytes(),
		}

		// Marshal the transaction
		txBytes, err := json.Marshal(validatorTx)
		if err != nil {
			log.Printf("Failed to marshal validator registration tx: %v", err)
		} else {
			// Connect to the genesis node's RPC
			client, err := rpchttp.New("tcp://localhost:26657", "/websocket")
			if err != nil {
				log.Printf("Warning: Failed to connect to genesis node: %v", err)
			} else {
				// Broadcast the transaction
				result, err := client.BroadcastTxSync(context.Background(), txBytes)
				if err != nil {
					log.Printf("Failed to broadcast validator registration tx: %v", err)
				} else {
					log.Printf("Registered validator tx: %s", result.Hash.String())

					if ok := registry.LinkAgentToValidator(*chainID, *agentID, address); !ok {
						log.Printf("Warning: Failed to link agent %s to validator %s", *agentID, address)
					}

					// Wait for validator registration to be committed
					log.Printf("Waiting for validator registration to be committed...")
					time.Sleep(10 * time.Second) // Give enough time for the next block to be committed

					// Start the node
					agentNode, err := node.NewNode(config, *chainID, address)
					if err != nil {
						log.Fatalf("Failed to start agent node: %v", err)
					}

					// Start API server for this agent node
					actualAPIPort := *apiPort
					if actualAPIPort == 0 {
						actualAPIPort = utils.FindAvailableAPIPort()
					}

					// Register node in handlers
					registry.RegisterNode(*chainID, *agentID, registry.NodeInfo{
						IsGenesis: false,
						Name:      *agentID,
						RPCPort:   *rpcPort,
						P2PPort:   *p2pPort,
						APIPort:   actualAPIPort,
					})

					// Start the node
					go func() {
						if err := agentNode.Start(context.Background()); err != nil {
							log.Printf("Failed to start agent node: %v", err)
						}
					}()

					// Setup and start API server
					router := gin.New()
					api.SetupRoutes(router, *chainID)
					log.Printf("Agent node [%s] started on P2P %d, RPC %d, API %d",
						*agentID, *p2pPort, *rpcPort, actualAPIPort)
					log.Fatal(router.Run(fmt.Sprintf(":%d", actualAPIPort)))
				}
			}
		}
	}
}
