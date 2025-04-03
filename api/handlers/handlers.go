package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"hash/crc32"

	"github.com/NethermindEth/chaoschain-launchpad/ai"
	"github.com/NethermindEth/chaoschain-launchpad/cmd/node"
	"github.com/NethermindEth/chaoschain-launchpad/communication"
	"github.com/NethermindEth/chaoschain-launchpad/core"
	da "github.com/NethermindEth/chaoschain-launchpad/da_layer"
	"github.com/NethermindEth/chaoschain-launchpad/registry"
	"github.com/NethermindEth/chaoschain-launchpad/utils"
	"github.com/NethermindEth/chaoschain-launchpad/validator"
	cfg "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/privval"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cometbft/cometbft/types"
)

// Add at the top with other types
type RelationshipUpdate struct {
	FromID   string  `json:"fromId"`
	TargetID string  `json:"targetId"`
	Score    float64 `json:"score"` // -1.0 to 1.0
}

// RegisterAgent - Registers a new AI agent (Producer or Validator)
func RegisterAgent(c *gin.Context) {
	chainID := c.GetString("chainID")
	var agent core.Agent
	if err := c.ShouldBindJSON(&agent); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent data"})
		return
	}

	// Register agent in registry
	registry.RegisterAgent(chainID, agent)

	// Assign specific ports based on agent ID
	basePort := 26656
	agentIDInt := int(crc32.ChecksumIEEE([]byte(agent.ID)))
	p2pPort := basePort + (agentIDInt % 10000)
	rpcPort := p2pPort + 1
	apiPort := p2pPort + 2

	if p2pPort == 26656 || rpcPort == 26657 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Agent port conflicts with Genesis node"})
		return
	}

	// Get genesis node ID from its node_key.json
	genesisNodeKeyFile := fmt.Sprintf("./data/%s/genesis/config/node_key.json", chainID)
	genesisNodeKey, err := p2p.LoadNodeKey(genesisNodeKeyFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load genesis node key"})
		return
	}

	// Create seed node string
	seedNode := fmt.Sprintf("%s@127.0.0.1:26656", genesisNodeKey.ID())

	log.Printf("This is the seed node with seed node %s", seedNode)

	// Create and start the node
	cmd := exec.Command(
		"./chaos-agent", // compiled agent binary
		"--chain", chainID,
		"--agent-id", agent.ID,
		"--p2p-port", fmt.Sprintf("%d", p2pPort),
		"--rpc-port", fmt.Sprintf("%d", rpcPort),
		"--genesis-node-id", seedNode,
		"--role", agent.Role, // Use the role flag with the agent's role value
		"--api-port", fmt.Sprintf("%d", apiPort),
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to start agent process: %v", err)})
		return
	}

	// Create a channel to receive process errors
	errCh := make(chan error, 1)
	go func() {
		errCh <- cmd.Wait()
	}()

	// Wait briefly for ports to be bound
	select {
	case err := <-errCh:
		// Process exited with error
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Agent process failed: %v", err)})
		return
	case <-time.After(3 * time.Second):
		// Process is still running after timeout, assume it's working
	}

	// Check if the process is still running
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Agent process exited unexpectedly"})
		return
	}

	registry.RegisterNode(chainID, agent.ID, registry.NodeInfo{
		IsGenesis: false,
		Name:      agent.ID,
		P2PPort:   p2pPort,
		RPCPort:   rpcPort,
		APIPort:   apiPort,
	})

	communication.BroadcastEvent(communication.EventAgentRegistered, agent)

	c.JSON(http.StatusOK, gin.H{
		"message": "Agent registered successfully",
		"agentID": agent.ID,
		"p2pPort": p2pPort,
		"rpcPort": rpcPort,
		"apiPort": apiPort,
	})
}

// GetBlock - Fetch a block by height
func GetBlock(c *gin.Context) {
	chainID := c.GetString("chainID")
	height, err := strconv.Atoi(c.Param("height"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid block height"})
		return
	}

	// Connect to the specific chain's node using chainID
	rpcPort, err := registry.GetRPCPortForChain(chainID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Chain not found: %v", err)})
		return
	}

	client, err := rpchttp.New(fmt.Sprintf("tcp://localhost:%d", rpcPort), "/websocket")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to connect to node: %v", err)})
		return
	}

	// Verify we're connected to the right chain
	status, err := client.Status(context.Background())
	if err != nil || status.NodeInfo.Network != chainID {
		c.JSON(http.StatusNotFound, gin.H{"error": "Chain not found"})
		return
	}

	// Get block at height
	heightPtr := new(int64)
	*heightPtr = int64(height)
	block, err := client.Block(context.Background(), heightPtr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Failed to get block: %v", err)})
		return
	}

	// Transform block data for response
	blockData := gin.H{
		"height":     block.Block.Height,
		"hash":       block.Block.Hash(),
		"timestamp":  block.Block.Time,
		"numTxs":     len(block.Block.Txs),
		"proposer":   block.Block.ProposerAddress,
		"validators": block.Block.LastCommit.Signatures,
	}

	c.JSON(http.StatusOK, gin.H{"block": blockData})
}

// GetNetworkStatus - Returns the current status of ChaosChain
func GetNetworkStatus(c *gin.Context) {
	_ = c.GetString("chainID")

	client, err := rpchttp.New("tcp://localhost:26657", "/websocket")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to node"})
		return
	}

	// Get network info
	netInfo, err := client.NetInfo(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get network info"})
		return
	}

	networkStatus := gin.H{
		"netInfo": netInfo,
	}

	c.JSON(http.StatusOK, gin.H{"status": networkStatus})
}

// SubmitTransaction - Allows an agent to submit a transaction
func SubmitTransaction(c *gin.Context) {
	chainID := c.GetString("chainID")

	// Get port from Host header
	host := c.Request.Host
	apiPort := ""
	if i := strings.LastIndex(host, ":"); i != -1 {
		apiPort = host[i+1:]
	}

	// Get node info from API port
	_, nodeInfo, found := registry.GetNodeByAPIPort(chainID, apiPort)
	if !found {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Node not recognized"})
		return
	}

	var tx core.Transaction
	if err := c.ShouldBindJSON(&tx); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid transaction format"})
		return
	}

	// Set transaction type and data
	tx.Type = "discuss_transaction"

	// Connect to local RPC endpoint to get validator's public key
	client, err := rpchttp.New(fmt.Sprintf("tcp://localhost:%d", nodeInfo.RPCPort), "/websocket")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to connect to node: %v", err)})
		return
	}

	// Get validator's public key
	status, err := client.Status(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get node status: %v", err)})
		return
	}

	log.Printf("This is the status %+v", status)
	tx.Data = status.ValidatorInfo.PubKey.Bytes()

	// Encode transaction
	txBytes, err := tx.Marshal()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to encode transaction"})
		return
	}

	// Submit transaction to CometBFT
	result, err := client.BroadcastTxSync(context.Background(), txBytes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to broadcast tx: %v", err)})
		return
	}

	communication.BroadcastEvent(communication.EventNewTransaction, tx)

	c.JSON(http.StatusOK, gin.H{
		"message": "Transaction submitted successfully",
		"hash":    result.Hash.String(),
	})
}

// GetValidators - Returns the list of registered validators
func GetValidators(c *gin.Context) {
	chainID := c.GetString("chainID")

	// Get port from Host header
	host := c.Request.Host
	apiPort := ""
	if i := strings.LastIndex(host, ":"); i != -1 {
		apiPort = host[i+1:]
	}

	// Get node info from API port
	_, nodeInfo, found := registry.GetNodeByAPIPort(chainID, apiPort)
	if !found {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   fmt.Sprintf("Node not recognized for port %s", apiPort),
			"chainID": chainID,
			"apiPort": apiPort,
		})
		return
	}

	log.Printf("This is the node info %+v", nodeInfo)

	// Connect to the node using its RPC port
	client, err := rpchttp.New(fmt.Sprintf("tcp://localhost:%d", nodeInfo.RPCPort), "/websocket")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to connect to node: %v", err)})
		return
	}

	// Get validators from CometBFT
	result, err := client.Validators(context.Background(), nil, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get validators: %v", err)})
		return
	}

	validators := result.Validators

	c.JSON(http.StatusOK, gin.H{"validators": validators})
}

// GetSocialStatus - Retrieves an agent's social reputation
func GetSocialStatus(c *gin.Context) {
	agentID := c.Param("agentID")
	chainID := c.GetString("chainID")

	// Get consensus validator info from CometBF`T
	rpcPort, err := registry.GetRPCPortForChain(chainID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Chain not found: %v", err)})
		return
	}

	client, err := rpchttp.New(fmt.Sprintf("tcp://localhost:%d", rpcPort), "/websocket")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to connect: %v", err)})
		return
	}

	// Verify validator exists in CometBFT
	result, err := client.Validators(context.Background(), nil, nil, nil)
	if err != nil || !validatorExists(result.Validators, agentID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Validator not found in consensus set"})
		return
	}

	// Get social info from our registry
	socialVal := validator.GetSocialValidator(chainID, agentID)
	if socialVal == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Validator not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"agentID":       socialVal.ID,
		"name":          socialVal.Name,
		"mood":          socialVal.Mood,
		"relationships": socialVal.Relationships,
	})
}

// AddInfluence adds a new influence to a validator
func AddInfluence(c *gin.Context) {
	agentID := c.Param("agentID")
	chainID := c.GetString("chainID")
	var influence struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&influence); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid influence data"})
		return
	}

	v := validator.GetValidatorByID(chainID, agentID)
	if v == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Validator not found"})
		return
	}

	v.Influences = append(v.Influences, influence.Name)
	c.JSON(http.StatusOK, gin.H{"message": "Influence added successfully"})
}

// UpdateRelationship updates the relationship score between validators
func UpdateRelationship(c *gin.Context) {
	agentID := c.Param("agentID")
	chainID := c.GetString("chainID")
	var rel RelationshipUpdate
	if err := c.ShouldBindJSON(&rel); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid relationship data"})
		return
	}
	rel.FromID = agentID // Set the from ID

	v := validator.GetValidatorByID(chainID, agentID)
	if v == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Validator not found"})
		return
	}

	// Validate score range
	if rel.Score < -1.0 || rel.Score > 1.0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Score must be between -1.0 and 1.0"})
		return
	}

	v.Relationships[rel.TargetID] = rel.Score
	communication.BroadcastEvent(communication.EventAgentAlliance, rel)
	c.JSON(http.StatusOK, gin.H{"message": "Relationship updated successfully"})
}

// GetAllThreads returns all active discussion threads for monitoring.
func GetAllThreads(c *gin.Context) {
	threads := communication.GetAllThreads() // We'll implement this function in forum
	c.JSON(http.StatusOK, threads)
}

type CreateChainRequest struct {
	ChainID       string `json:"chain_id" binding:"required"`
	GenesisPrompt string `json:"genesis_prompt" binding:"required"`
}

func loadSampleAgents(genesisPrompt string) ([]core.Agent, error) {
	filename, err := ai.GenerateAgents(genesisPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate agents: %v", err)
	}
	filename = "examples/" + filename

	// Read the JSON file
	fileContent, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %v", filename, err)
	}

	var agents []core.Agent
	if err := json.Unmarshal(fileContent, &agents); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %v", filename, err)
	}

	return agents, nil
}

// CreateChain creates a new blockchain instance
func CreateChain(c *gin.Context) {
	var req CreateChainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Check if chain already exists in our registry
	if _, err := registry.GetRPCPortForChain(req.ChainID); err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Chain already exists"})
		return
	}

	// Create CometBFT config for genesis node
	config := cfg.DefaultConfig()
	config.BaseConfig.RootDir = "./data/" + req.ChainID
	config.Moniker = "genesis-node"
	config.P2P.ListenAddress = "tcp://0.0.0.0:0"
	config.RPC.ListenAddress = "tcp://0.0.0.0:0"

	// Get genesis node ID from its node_key.json
	genesisNodeKeyFile := fmt.Sprintf("./data/%s/genesis/config/node_key.json", req.ChainID)
	genesisNodeKey, err := p2p.LoadNodeKey(genesisNodeKeyFile)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load genesis node key"})
		return
	}

	// Make addresses routable
	config.P2P.AllowDuplicateIP = true
	config.P2P.AddrBookStrict = false
	// We'll get the actual port after the node starts

	// Use genesis node as seed for peer discovery
	peerString := fmt.Sprintf("%s@127.0.0.1:26656", genesisNodeKey.ID())
	config.P2P.Seeds = peerString

	// Additional P2P settings
	config.P2P.PexReactor = true        // Enable peer exchange
	config.P2P.MaxNumInboundPeers = 100 // Increase limits
	config.P2P.MaxNumOutboundPeers = 30
	config.P2P.AddrBookStrict = false  // Allow same IP different ports
	config.P2P.AllowDuplicateIP = true // Important for local testing

	// Additional settings for better peer connections
	config.P2P.HandshakeTimeout = 20 * time.Second
	config.P2P.DialTimeout = 3 * time.Second
	config.P2P.FlushThrottleTimeout = 10 * time.Millisecond

	// Create required directories
	if err := os.MkdirAll(config.BaseConfig.RootDir+"/config", 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create config directory: %v", err)})
		return
	}

	// Initialize config files and validator keys
	if err := os.MkdirAll(config.BaseConfig.RootDir+"/config", 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create config directory: %v", err)})
		return
	}

	// Initialize validator key files
	privValKeyFile := config.PrivValidatorKeyFile()
	privValStateFile := config.PrivValidatorStateFile()
	if !utils.FileExists(privValKeyFile) {
		privVal := privval.GenFilePV(privValKeyFile, privValStateFile)
		privVal.Save()
	}

	// Initialize node key file
	nodeKeyFile := config.NodeKeyFile()
	if !utils.FileExists(nodeKeyFile) {
		if _, err := p2p.LoadOrGenNodeKey(nodeKeyFile); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to generate node key: %v", err)})
			return
		}
	}

	// Initialize genesis.json if it doesn't exist
	genesisFile := config.GenesisFile()
	if !utils.FileExists(genesisFile) {
		// Get the validator's public key
		privVal := privval.LoadFilePV(privValKeyFile, privValStateFile)
		pubKey, err := privVal.GetPubKey()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get validator public key: %v", err)})
			return
		}

		// Create genesis validator directly
		genValidator := types.GenesisValidator{
			PubKey: pubKey,
			Power:  1000000, // Increase validator power significantly
			Name:   "genesis",
		}

		genDoc := types.GenesisDoc{
			ChainID:         req.ChainID,
			GenesisTime:     time.Now(),
			ConsensusParams: types.DefaultConsensusParams(),
			Validators:      []types.GenesisValidator{genValidator},
		}

		// Validate genesis doc before saving
		if err := genDoc.ValidateAndComplete(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to validate genesis doc: %v", err)})
			return
		}

		if err := genDoc.SaveAs(genesisFile); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create genesis file: %v", err)})
			return
		}
	}

	// Create and start the genesis node
	genesisNode, err := node.NewNode(config, req.ChainID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to create genesis node: %v", err)})
		return
	}

	if err := genesisNode.Start(context.Background()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start bootstrap node"})
		return
	}

	// Register chain in our registry
	registry.RegisterNode(req.ChainID, "genesis", registry.NodeInfo{
		IsGenesis: true,
		Name:      "genesis",
		RPCPort:   func() int { p, _ := strconv.Atoi(config.RPC.ListenAddress[10:]); return p }(),
		P2PPort:   func() int { p, _ := strconv.Atoi(config.P2P.ListenAddress[10:]); return p }(),
	})

	communication.BroadcastEvent(communication.EventChainCreated, map[string]interface{}{
		"chainId":   req.ChainID,
		"timestamp": time.Now(),
	})

	// TODO: Register sample agents based on the genesis prompt
	// agents, err := loadSampleAgents(req.GenesisPrompt)
	// if err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load sample agents"})
	// 	return
	// }

	// log.Printf("Loaded %d sample agents", len(agents))

	// // Register agents synchronously
	// for _, agent := range agents {
	// 	// Add a small delay between registrations for better UX
	// 	time.Sleep(500 * time.Millisecond)

	// 	if err := registerAgent(req.ChainID, agent, p2pPort); err != nil {
	// 		log.Printf("Failed to register agent %s: %v", agent.ID, err)
	// 		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to register agent %s", agent.ID)})
	// 		return
	// 	}
	// 	log.Printf("Successfully registered agent: %s (%s)", agent.Name, agent.ID)
	// }

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Chain created successfully",
		"chain_id": req.ChainID,
		"genesis_node": map[string]int{
			"p2p_port": func() int { p, _ := strconv.Atoi(config.P2P.ListenAddress[10:]); return p }(),
			"rpc_port": func() int { p, _ := strconv.Atoi(config.RPC.ListenAddress[10:]); return p }(),
		},
		// "registered_agents": len(agents),
	})
}

// ListChains returns all available chains
func ListChains(c *gin.Context) {
	chains := core.GetAllChains()
	c.JSON(http.StatusOK, gin.H{
		"chains": chains,
	})
}

func validatorExists(validators []*types.Validator, agentID string) bool {
	for _, v := range validators {
		if v.Address.String() == agentID {
			return true
		}
	}
	return false
}

// GetBlockDiscussions returns the discussions for a specific block by hash
func GetBlockDiscussions(c *gin.Context) {
	chainID := c.GetString("chainID")
	blockHash := c.Param("blockHash")

	// Get the blob reference for this block
	ref, found := da.GetBlobReferenceByBlockHash(chainID, blockHash)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "No discussions found for this block"})
		return
	}

	// Retrieve the data from EigenDA
	offchainData, err := da.GetOffchainData(ref.BlobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to retrieve discussions: %v", err)})
		return
	}

	// Format timestamps for better readability in the response
	formattedDiscussions := make([]map[string]interface{}, len(offchainData.Discussions))
	for i, d := range offchainData.Discussions {
		formattedDiscussions[i] = map[string]interface{}{
			"id":          d.ID,
			"validatorId": d.ValidatorID,
			"message":     d.Message,
			"timestamp":   d.Timestamp.Format(time.RFC3339),
			"type":        d.Type,
			"round":       d.Round,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"blockHash":   blockHash,
		"blockHeight": ref.BlockHeight,
		"discussions": formattedDiscussions,
		"votes":       offchainData.Votes,
		"outcome":     offchainData.Outcome,
		"agents":      offchainData.AgentIdentities,
		"timestamp":   time.Unix(offchainData.Timestamp, 0).Format(time.RFC3339),
	})
}

// GetBlockDiscussionsByHeight returns the discussions for a specific block by height
func GetBlockDiscussionsByHeight(c *gin.Context) {
	chainID := c.GetString("chainID")
	heightStr := c.Param("height")

	height, err := strconv.Atoi(heightStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid block height"})
		return
	}

	// Get the blob reference for this block
	ref, found := da.GetBlobReferenceByHeight(chainID, height)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "No discussions found for this block height"})
		return
	}

	// Retrieve the data from EigenDA
	offchainData, err := da.GetOffchainData(ref.BlobID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to retrieve discussions: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"blockHash":   ref.BlockHash,
		"blockHeight": height,
		"discussions": offchainData.Discussions,
		"votes":       offchainData.Votes,
		"outcome":     offchainData.Outcome,
		"agents":      offchainData.AgentIdentities,
		"timestamp":   offchainData.Timestamp,
	})
}

// ListBlockDiscussions returns a list of all blocks with discussions for a chain
func ListBlockDiscussions(c *gin.Context) {
	chainID := c.GetString("chainID")

	// Get all blob references for this chain
	refs := da.GetBlobReferencesForChain(chainID)
	if len(refs) == 0 {
		c.JSON(http.StatusOK, gin.H{"blocks": []interface{}{}})
		return
	}

	// Create a summary for each block
	blocks := make([]map[string]interface{}, len(refs))
	for i, ref := range refs {
		blocks[i] = map[string]interface{}{
			"blockHash":   ref.BlockHash,
			"blockHeight": ref.BlockHeight,
			"outcome":     ref.Outcome,
			"timestamp":   ref.Timestamp,
			"blobId":      ref.BlobID,
		}
	}

	c.JSON(http.StatusOK, gin.H{"blocks": blocks})
}

// Add validator directly to genesis file
func AddValidatorToGenesis(chainID string, agent core.Agent) bool {

	// Set up data directory paths
	dataDir := fmt.Sprintf("./data/%s/%s", chainID, agent.ID)
	genesisFile := fmt.Sprintf("./data/%s/genesis/config/genesis.json", chainID)

	// Create required directories
	if err := os.MkdirAll(dataDir+"/config", 0755); err != nil {
		return false
	}
	if err := os.MkdirAll(dataDir+"/data", 0755); err != nil {
		return false
	}

	// Generate validator key
	privValKeyFile := fmt.Sprintf("%s/config/priv_validator_key.json", dataDir)
	privValStateFile := fmt.Sprintf("%s/data/priv_validator_state.json", dataDir)
	privVal := privval.GenFilePV(privValKeyFile, privValStateFile)
	pubKey, _ := privVal.GetPubKey()

	// Read genesis file
	genesisBytes, err := os.ReadFile(genesisFile)
	if err != nil {
		return false
	}

	// Parse genesis file
	var genDoc types.GenesisDoc
	if err := json.Unmarshal(genesisBytes, &genDoc); err != nil {
		return false
	}

	// Add validator to genesis
	validator := types.GenesisValidator{
		Address: pubKey.Address(),
		PubKey:  pubKey,
		Power:   10,
		Name:    agent.ID,
	}
	genDoc.Validators = append(genDoc.Validators, validator)

	// Write updated genesis file
	updatedGenesisBytes, err := json.MarshalIndent(genDoc, "", "  ")
	if err != nil {
		return false
	}

	if err := os.WriteFile(genesisFile, updatedGenesisBytes, 0644); err != nil {
		return false
	}

	// Copy updated genesis to new node
	newGenesisFile := fmt.Sprintf("%s/config/genesis.json", dataDir)
	if err := os.WriteFile(newGenesisFile, updatedGenesisBytes, 0644); err != nil {
		return false
	}

	return true
}

// GetAllAgents returns all registered agents for a chain
func GetAllAgents(c *gin.Context) {
	chainID := c.GetString("chainID")

	agents := registry.GetAllAgents(chainID)

	c.JSON(http.StatusOK, gin.H{
		"agents": agents,
	})
}
