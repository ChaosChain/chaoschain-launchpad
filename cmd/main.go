package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/NethermindEth/chaoschain-launchpad/api"
	"github.com/NethermindEth/chaoschain-launchpad/cmd/node"
	"github.com/NethermindEth/chaoschain-launchpad/registry"

	"github.com/NethermindEth/chaoschain-launchpad/core"
	cfg "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/privval"
	"github.com/cometbft/cometbft/types"
	"github.com/gin-gonic/gin"
)

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

func main() {
	// Parse command line flags
	chainID := flag.String("chain", "mainnet", "Chain ID")
	nodeID := flag.String("node-id", "genesis", "Node ID")
	p2pPort := flag.Int("p2p-port", 26656, "CometBFT P2P port")
	rpcPort := flag.Int("rpc-port", 26657, "CometBFT RPC port")
	apiPort := flag.Int("api-port", 3000, "API server port")
	nats := flag.String("nats", "nats://localhost:4222", "NATS URL")
	flag.Parse()

	// before creating data directory, check if it exists and delete it
	if _, err := os.Stat(fmt.Sprintf("./data/%s", *chainID)); err == nil {
		os.RemoveAll(fmt.Sprintf("./data/%s", *chainID))
		os.Remove("./config/addrbook.json")
		// Also remove any other persistent state files
		os.RemoveAll("./config/priv_validator_state.json")
	}

	// Create data directory for the chain
	dataDir := fmt.Sprintf("./data/%s/%s", *chainID, *nodeID)
	if err := os.MkdirAll(dataDir+"/data", 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Create CometBFT config
	config := cfg.DefaultConfig()
	config.BaseConfig.RootDir = dataDir
	config.Moniker = *chainID
	config.P2P.ListenAddress = fmt.Sprintf("tcp://0.0.0.0:%d", *p2pPort)
	config.RPC.ListenAddress = fmt.Sprintf("tcp://0.0.0.0:%d", *rpcPort)

	// Initialize config files and validator keys
	if err := os.MkdirAll(config.BaseConfig.RootDir+"/config", 0755); err != nil {
		log.Fatalf("Failed to create config directory: %v", err)
	}

	// Create data directory for validator state
	if err := os.MkdirAll(config.BaseConfig.RootDir+"/data", 0755); err != nil {
		log.Fatalf("Failed to create validator data directory: %v", err)
	}

	// Initialize validator key files
	privValKeyFile := config.PrivValidatorKeyFile()
	privValStateFile := config.PrivValidatorStateFile()
	if !fileExists(privValKeyFile) {
		privVal := privval.GenFilePV(privValKeyFile, privValStateFile)
		// Initialize with empty state
		privVal.Save()
	} else {
		// Load existing validator and ensure state file exists
		privVal := privval.LoadFilePV(privValKeyFile, privValStateFile)
		if !fileExists(privValStateFile) {
			// Initialize with empty state if missing
			privVal.Save()
		}
	}

	// Initialize node key file
	nodeKeyFile := config.NodeKeyFile()
	if !fileExists(nodeKeyFile) {
		if _, err := p2p.LoadOrGenNodeKey(nodeKeyFile); err != nil {
			log.Fatalf("Failed to generate node key: %v", err)
		}
	}

	// Initialize genesis.json if it doesn't exist
	genesisFile := config.GenesisFile()
	// Force regeneration of genesis file to ensure validators are included
	if err := os.Remove(genesisFile); err != nil && !os.IsNotExist(err) {
		log.Printf("Failed to remove existing genesis file: %v", err)
	}

	if !fileExists(genesisFile) {
		// Get the validator's public key
		privVal := privval.LoadFilePV(privValKeyFile, privValStateFile)
		pubKey, err := privVal.GetPubKey()
		if err != nil {
			log.Fatalf("Failed to get validator public key: %v", err)
		}

		// Create genesis validator directly
		genValidator := types.GenesisValidator{
			PubKey: pubKey,
			Power:  1000000, // Increase validator power significantly
			Name:   "genesis",
		}

		genDoc := types.GenesisDoc{
			ChainID:         *chainID,
			GenesisTime:     time.Now(),
			ConsensusParams: types.DefaultConsensusParams(),
			Validators:      []types.GenesisValidator{genValidator},
		}

		// Validate genesis doc before saving
		if err := genDoc.ValidateAndComplete(); err != nil {
			log.Fatalf("Failed to validate genesis doc: %v", err)
		}

		// Ensure the validator was correctly added
		if len(genDoc.Validators) == 0 {
			log.Fatalf("No validators in genesis document after validation")
		}

		if err := genDoc.SaveAs(genesisFile); err != nil {
			log.Fatalf("Failed to create genesis file: %v", err)
		}
	}

	// Genesis node settings
	config.P2P.AllowDuplicateIP = true
	config.P2P.AddrBookStrict = false
	config.P2P.ExternalAddress = fmt.Sprintf("tcp://127.0.0.1:%d", *p2pPort)
	config.P2P.ListenAddress = fmt.Sprintf("tcp://0.0.0.0:%d", *p2pPort)

	// Additional settings for better peer connections
	config.P2P.HandshakeTimeout = 20 * time.Second
	config.P2P.DialTimeout = 3 * time.Second
	config.P2P.FlushThrottleTimeout = 10 * time.Millisecond
	config.P2P.MaxNumInboundPeers = 40
	config.P2P.MaxNumOutboundPeers = 10

	config.P2P.SeedMode = true // Only helps discover peers, doesn't try to dial anyone
	config.P2P.PexReactor = true

	genesisNode, err := node.NewNode(config, *chainID)
	if err != nil {
		log.Fatalf("main: Failed to create node: %v", err)
	}

	err = genesisNode.Start(context.Background())
	if err != nil {
		log.Fatalf("Failed to start node: %v", err)
	}

	registry.RegisterNode(*chainID, *nodeID, registry.NodeInfo{
		IsGenesis: true,
		RPCPort:   *rpcPort,
		P2PPort:   *p2pPort,
		APIPort:   *apiPort,
	})

	// Start NATS messaging
	core.SetupNATS(*nats)
	defer core.CloseNATS()

	log.Printf("Genesis node for chain %s started with P2P port %d, RPC port %d, and API port %d",
		*chainID, *p2pPort, *rpcPort, *apiPort)

	// Start API server
	router := gin.New()
	api.SetupRoutes(router, *chainID)
	log.Fatal(router.Run(fmt.Sprintf(":%d", *apiPort)))
}
