package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/NethermindEth/chaoschain-launchpad/api"
	"github.com/NethermindEth/chaoschain-launchpad/cmd/node"
	_ "github.com/NethermindEth/chaoschain-launchpad/config" // Initialize config
	"github.com/NethermindEth/chaoschain-launchpad/core"
	da "github.com/NethermindEth/chaoschain-launchpad/da_layer"
	"github.com/NethermindEth/chaoschain-launchpad/mempool"
	"github.com/NethermindEth/chaoschain-launchpad/p2p"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Parse command line flags
	chainID := flag.String("chain", "mainnet", "Chain ID")
	port := flag.Int("port", 8080, "P2P port")
	apiPort := flag.Int("api", 3000, "API port")
	nats := flag.String("nats", "nats://localhost:4222", "NATS URL")
	flag.Parse()

	// Create and start node with chain configuration
	genesisNode := node.NewNode(node.NodeConfig{
		ChainConfig: p2p.ChainConfig{
			ChainID: *chainID,
			P2PPort: *port,
			APIPort: *apiPort,
		},
	})

	if err := genesisNode.Start(); err != nil {
		log.Fatalf("Failed to start node: %v", err)
	}

	// Initialize chain-specific components
	core.InitBlockchain(*chainID, mempool.GetMempool(*chainID))

	// Initialize EigenDA service
	log.Printf("Initializing EigenDA service with NATS URL: %s", *nats)
	if err := da.SetupGlobalDAService(*nats); err != nil {
		log.Printf("Warning: Failed to initialize EigenDA service: %v", err)
		// Continue without EigenDA service
	} else {
		log.Println("EigenDA service initialized successfully")
		defer da.CloseGlobalDAService()
	}

	// Register this node with the chain
	chain := core.GetChain(*chainID)
	addr := fmt.Sprintf("localhost:%d", *port)
	chain.RegisterNode(addr, genesisNode.GetP2PNode())

	// Start NATS messaging
	core.SetupNATS(*nats)
	defer core.CloseNATS()

	log.Printf("Chain %s started with P2P port %d and API port %d", *chainID, *port, *apiPort)

	// Start API server
	router := gin.New()
	api.SetupRoutes(router, *chainID)
	log.Fatal(router.Run(fmt.Sprintf(":%d", *apiPort)))

	// Load front-end port env variable
	err := godotenv.Load("../.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

}
