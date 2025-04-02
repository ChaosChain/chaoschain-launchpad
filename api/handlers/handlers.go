package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	"github.com/NethermindEth/chaoschain-launchpad/ai"
	"github.com/NethermindEth/chaoschain-launchpad/cmd/node"
	"github.com/NethermindEth/chaoschain-launchpad/communication"
	"github.com/NethermindEth/chaoschain-launchpad/consensus"
	"github.com/NethermindEth/chaoschain-launchpad/core"
	da "github.com/NethermindEth/chaoschain-launchpad/da_layer"
	"github.com/NethermindEth/chaoschain-launchpad/mempool"
	"github.com/NethermindEth/chaoschain-launchpad/p2p"
	"github.com/NethermindEth/chaoschain-launchpad/producer"
	"github.com/NethermindEth/chaoschain-launchpad/registry"
	"github.com/NethermindEth/chaoschain-launchpad/validator"
)

var (
	lastUsedPort         = 8080
	portMutex            sync.Mutex
	agentIdentitiesMutex sync.RWMutex
)

func findAvailablePort() int {
	portMutex.Lock()
	defer portMutex.Unlock()
	lastUsedPort++
	return lastUsedPort
}

func findAvailableAPIPort() int {
	portMutex.Lock()
	defer portMutex.Unlock()
	lastUsedPort++
	return lastUsedPort
}

// Add at the top with other types
type RelationshipUpdate struct {
	FromID   string  `json:"fromId"`
	TargetID string  `json:"targetId"`
	Score    float64 `json:"score"` // -1.0 to 1.0
}

// RegisterAgent - Registers a new AI agent (Producer or Validator)
func RegisterAgent(c *gin.Context) {
	chainID := c.GetString("chainID")
	chain := core.GetChain(chainID)
	if chain == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Chain not found"})
		return
	}

	var agent core.Agent
	if err := c.ShouldBindJSON(&agent); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent data"})
		return
	}

	// Assign a unique ID
	agent.ID = uuid.New().String()

	// Get bootstrap node's P2P instance
	var bootstrapNode *p2p.Node
	chain.NodesMu.RLock()
	for _, node := range chain.Nodes {
		bootstrapNode = node
		break // Get first node
	}
	chain.NodesMu.RUnlock()

	if bootstrapNode == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "No bootstrap node found for chain"})
		return
	}

	bootstrapPort := bootstrapNode.GetPort()
	if bootstrapPort == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Bootstrap node not ready"})
		return
	}

	log.Printf("Found bootstrap node at port: %d", bootstrapPort)

	// Create a new node for this agent
	newPort := findAvailablePort()
	agentNode := node.NewNode(node.NodeConfig{
		ChainConfig: p2p.ChainConfig{
			ChainID: chainID,
			P2PPort: newPort,
			APIPort: findAvailableAPIPort(),
		},
		BootstrapNode: fmt.Sprintf("localhost:%d", bootstrapPort),
	})

	if err := agentNode.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start agent node"})
		return
	}

	// Register the new node with the correct chain
	addr := fmt.Sprintf("localhost:%d", newPort)

	chain.RegisterNode(addr, agentNode.GetP2PNode())

	if agent.Role == "producer" {
		personality := ai.Personality{
			Name:   agent.Name,
			Traits: agent.Traits,
			Style:  agent.Style,
		}

		// Get mempool safely
		mempoolInterface := agentNode.GetMempool()
		if mempoolInterface == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get mempool"})
			return
		}

		// Create producer on its own node
		producerInstance := producer.NewProducer(mempoolInterface, personality, agentNode.GetP2PNode())

		// Register on the agent's node
		registry.RegisterProducer(chainID, agent.ID, producerInstance)

	} else if agent.Role == "validator" {
		validatorInstance := validator.NewValidator(
			agent.ID,
			agent.Name,
			agent.Traits,
			agent.Style,
			agent.Influences,
			agentNode.GetP2PNode(),
			chain.GenesisPrompt,
			chainID,
		)

		// Register on the agent's node
		registry.RegisterValidator(chainID, agent.ID, validatorInstance)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid agent role"})
		return
	}

	communication.BroadcastEvent(communication.EventAgentRegistered, agent)

	c.JSON(http.StatusOK, gin.H{
		"message": "Agent registered successfully",
		"agentID": agent.ID,
		"p2pPort": newPort,
		"apiPort": agentNode.GetAPIPort(),
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

	chain := core.GetChain(chainID)
	if chain == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Chain not found"})
		return
	}

	if height < 0 || height >= len(chain.Blocks) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Block not found"})
		return
	}
	block := chain.Blocks[height]

	c.JSON(http.StatusOK, gin.H{"block": block})
}

// GetNetworkStatus - Returns the current status of ChaosChain
func GetNetworkStatus(c *gin.Context) {
	chainID := c.GetString("chainID")
	bc := core.GetChain(chainID)
	if bc == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Chain not found"})
		return
	}

	// Get node count for this chain
	bc.NodesMu.RLock()
	nodeCount := len(bc.Nodes)
	bc.NodesMu.RUnlock()

	status := map[string]interface{}{
		"height":     len(bc.Blocks) - 1,
		"latestHash": bc.Blocks[len(bc.Blocks)-1].Hash(),
		"totalTxs":   len(bc.Blocks[len(bc.Blocks)-1].Txs),
		"nodeCount":  nodeCount,
	}

	c.JSON(http.StatusOK, gin.H{"status": status})
}

// SubmitTransaction - Allows an agent to submit a transaction
func SubmitTransaction(c *gin.Context) {
	chainID := c.GetString("chainID")

	var tx core.Transaction
	if err := c.ShouldBindJSON(&tx); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid transaction format"})
		return
	}

	// Set the chainID on the transaction
	tx.ChainID = chainID

	// In production, you would get the private key from secure storage
	privateKey, err := core.GenerateKeyPair()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate key"})
		return
	}

	// Sign the transaction
	if err := tx.SignTransaction(privateKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sign transaction"})
		return
	}

	// Process the signed transaction
	bc := core.GetChain(chainID)
	if bc == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Chain not found"})
		return
	}

	// Get the chain's mempool
	mp := mempool.GetMempool(chainID)
	if mp == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Mempool not found for chain"})
		return
	}

	if err := bc.ProcessTransaction(tx, mp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction failed: " + err.Error()})
		return
	}

	communication.BroadcastEvent(communication.EventNewTransaction, tx)

	c.JSON(http.StatusOK, gin.H{"message": "Transaction submitted successfully"})
}

// GetValidators - Returns the list of registered validators
func GetValidators(c *gin.Context) {
	chainID := c.GetString("chainID")
	validatorsList := validator.GetAllValidators(chainID)
	c.JSON(http.StatusOK, gin.H{"validators": validatorsList})
}

// GetSocialStatus - Retrieves an agent's social reputation
func GetSocialStatus(c *gin.Context) {
	agentID := c.Param("agentID")
	chainID := c.GetString("chainID")

	validator := validator.GetValidatorByID(chainID, agentID)
	if validator == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Validator not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"agentID":       validator.ID,
		"name":          validator.Name,
		"mood":          validator.Mood,
		"relationships": validator.Relationships,
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

// ProposeBlock creates a new block and starts consensus
func ProposeBlock(c *gin.Context) {
	log.Printf("Proposing block")
	chainID := c.GetString("chainID")
	if chainID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing chain ID"})
		return
	}

	// Get optional reward amount from query parameters
	rewardAmountStr := c.DefaultQuery("reward", "0")
	rewardAmount, err := strconv.ParseFloat(rewardAmountStr, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reward amount"})
		return
	}

	waitForConsensus := c.DefaultQuery("wait", "false") == "true"

	bc := core.GetChain(chainID)
	if bc == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Chain not found"})
		return
	}

	// If a reward is specified, check if it's valid
	if rewardAmount > 0 {
		// Get chain funds
		chainFunds := core.GetChainFunds(chainID)
		if chainFunds == nil {
			// Initialize with the chain's reward pool if not already initialized
			chainFunds = core.InitializeChainFunds(chainID, float64(bc.RewardPool))
		}

		// Check if there are enough funds in the pool
		if rewardAmount > chainFunds.TotalFunds {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Insufficient funds in reward pool. Available: %.2f, Requested: %.2f", chainFunds.TotalFunds, rewardAmount)})
			return
		}

		// If we have a reward and funds are available, create a reward transaction
		proposerID := c.DefaultQuery("proposer", "SYSTEM")

		// By default, all reward goes to the proposer
		recipients := make(map[string]float64)
		recipients[proposerID] = rewardAmount

		rewardTx := core.CreateRewardTransaction(proposerID, chainID, rewardAmount, recipients)

		// Add to mempool
		mp := mempool.GetMempool(chainID)
		if mp != nil {
			mp.AddTransaction(*rewardTx)
			log.Printf("Added reward transaction of %.2f to mempool", rewardAmount)
		}
	}

	block, err := bc.CreateBlock()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Immediately create the discussion thread for visualization.
	// The thread ID is derived from the block's hash.
	threadID := block.Hash()
	producerName := "ProducerAgent"
	title := fmt.Sprintf("Block Proposal %s", threadID)
	communication.CreateThread(threadID, title, producerName)

	// Set up subscriptions before starting consensus
	mp := mempool.GetMempool(chainID)
	if mp != nil {
		// Create a unique subject for this block's discussions
		// blockSubject := fmt.Sprintf("BLOCK_DISCUSSION_TRIGGER.%s", threadID)
		voteSubject := fmt.Sprintf("AGENT_VOTE.%s", threadID)

		// Create subscription cleanup function
		var subs []*nats.Subscription
		defer func() {
			for _, sub := range subs {
				if sub != nil {
					sub.Unsubscribe()
				}
			}
		}()

		// Subscribe to discussions
		discussionSub, err := core.NatsBrokerInstance.Subscribe("BLOCK_DISCUSSION_TRIGGER", func(m *nats.Msg) {
			var discussion consensus.Discussion
			if err := json.Unmarshal(m.Data, &discussion); err != nil {
				log.Printf("Error unmarshalling discussion: %v", err)
				return
			}

			mp.EphemeralVotes = append(mp.EphemeralVotes, mempool.EphemeralVote{
				ID:           discussion.ID,
				AgentID:      discussion.ValidatorID,
				VoteDecision: discussion.Type,
				Timestamp:    discussion.Timestamp.Unix(),
			})

			// Store agent identity if not already stored
			agentIdentitiesMutex.Lock()
			if _, exists := mp.EphemeralAgentIdentities[discussion.ValidatorID]; !exists {
				v := validator.GetValidatorByID(chainID, discussion.ValidatorID)
				if v != nil {
					mp.EphemeralAgentIdentities[discussion.ValidatorID] = v.Name
				} else {
					mp.EphemeralAgentIdentities[discussion.ValidatorID] = discussion.ValidatorID
				}
			}
			agentIdentitiesMutex.Unlock()
		})

		if err != nil {
			log.Printf("Warning: Failed to subscribe to discussions: %v", err)
		} else {
			subs = append(subs, discussionSub)
		}

		// Subscribe to votes
		voteSub, err := core.NatsBrokerInstance.Subscribe(voteSubject, func(m *nats.Msg) {
			var vote consensus.Discussion
			if err := json.Unmarshal(m.Data, &vote); err != nil {
				log.Printf("Error unmarshalling vote: %v", err)
				return
			}

			mp.EphemeralVotes = append(mp.EphemeralVotes, mempool.EphemeralVote{
				AgentID:      vote.ValidatorID,
				VoteDecision: vote.Type,
				Timestamp:    vote.Timestamp.Unix(),
			})

			agentIdentitiesMutex.Lock()
			if _, exists := mp.EphemeralAgentIdentities[vote.ValidatorID]; !exists {
				v := validator.GetValidatorByID(chainID, vote.ValidatorID)
				if v != nil {
					mp.EphemeralAgentIdentities[vote.ValidatorID] = v.Name
				} else {
					mp.EphemeralAgentIdentities[vote.ValidatorID] = vote.ValidatorID
				}
			}
			agentIdentitiesMutex.Unlock()
		})

		if err != nil {
			log.Printf("Warning: Failed to subscribe to votes: %v", err)
		} else {
			subs = append(subs, voteSub)
		}
	}

	// Get the consensus manager before setting up subscriptions
	cm := consensus.GetConsensusManager(chainID)
	if cm == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get consensus manager"})
		return
	}

	// Start consensus
	if err := cm.ProposeBlock(block); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to start consensus: " + err.Error()})
		return
	}

	if !waitForConsensus {
		c.JSON(http.StatusOK, gin.H{
			"message":   "Block proposed successfully, consensus started",
			"block":     block,
			"thread_id": threadID,
		})
		return
	}

	result := make(chan consensus.ConsensusResult)
	cm.SubscribeResult(int64(block.Height), result)

	// Calculate total expected time
	totalTime := time.Duration(consensus.DiscussionRounds+1)*consensus.RoundDuration +
		60*time.Second + // Buffer time
		10*time.Second // Safety margin

	select {
	case consensusResult := <-result:
		log.Printf("hereeeeee")
		// Store offchain data to EigenDA and clear temporary mempool data
		if mp := mempool.GetMempool(chainID); mp != nil {
			log.Printf("yeeeeeee")
			var discussions []consensus.Discussion
			activeConsensus := cm.GetActiveConsensus()
			if activeConsensus != nil {
				discussions = activeConsensus.GetDiscussions()
				log.Printf("hereeeeee 1")
			}

			votes := make([]da.Vote, len(mp.EphemeralVotes))
			for i, ev := range mp.EphemeralVotes {
				votes[i] = da.Vote{
					AgentID:      ev.AgentID,
					VoteDecision: ev.VoteDecision,
					Timestamp:    ev.Timestamp,
				}
				log.Printf("hereeeeee 3")
			}

			if len(discussions) == 0 && len(votes) == 0 {
				log.Printf("Warning: No discussions or votes collected for block %d", block.Height)
			}

			offchain := da.OffchainData{
				ChainID:     chainID,
				BlockHash:   threadID,
				BlockHeight: block.Height,
				Discussions: discussions,
				Votes:       votes,
				Outcome: func() string {
					if consensusResult.State == consensus.Accepted {
						return "accepted"
					}
					return "rejected"
				}(),
				AgentIdentities: mp.EphemeralAgentIdentities,
				Timestamp:       time.Now().Unix(),
			}

			log.Printf("hereeeeee 4")

			if id, err := da.SaveOffchainData(offchain); err != nil {
				log.Printf("Error saving offchain data: %v", err)
			} else {
				log.Printf("Offchain data saved with id: %s", id)
			}

			log.Printf("Consensus completed $s", consensusResult.State)
			log.Printf("Consensus support $s", consensusResult.Support)
			log.Printf("Consensus Oppose $s", consensusResult.Oppose)

			// Send response to client before clearing data
			c.JSON(http.StatusOK, gin.H{
				"message":   "Consensus completed",
				"block":     block,
				"accepted":  consensusResult.State == consensus.Accepted,
				"support":   consensusResult.Support,
				"oppose":    consensusResult.Oppose,
				"thread_id": threadID,
			})

			// If consensus was accepted, trigger task breakdown and delegation process
			if consensusResult.State == consensus.Accepted {
				// Process all transactions in the block, including rewards
				if err := core.ProcessBlockTransactions(block); err != nil {
					log.Printf("Warning: Error processing block transactions: %v", err)
				} else {
					log.Printf("Successfully processed all transactions in block %d", block.Height)
				}

				// Extract transaction information for analysis
				txCount := len(block.Txs)

				// Only proceed with task breakdown if there are transactions to analyze
				if txCount > 0 {
					log.Printf("Starting collaborative task breakdown process for block %d with %d transaction(s)", block.Height, txCount)

					// Format transaction data for task breakdown
					var transactionDetails string
					if txCount == 1 {
						tx := block.Txs[0]
						transactionDetails = fmt.Sprintf("Transaction details:\n- Type: %s\n- Content: %s\n- From: %s\n- To: %s\n- Amount: %.2f\n- Timestamp: %d",
							tx.Type, tx.Content, tx.From, tx.To, tx.Amount, tx.Timestamp)
					} else {
						transactionDetails = "Transactions:\n"
						for i, tx := range block.Txs {
							transactionDetails += fmt.Sprintf("\nTransaction %d:\n- Type: %s\n- Content: %s\n- From: %s\n- To: %s\n- Amount: %.2f\n- Timestamp: %d\n",
								i+1, tx.Type, tx.Content, tx.From, tx.To, tx.Amount, tx.Timestamp)
						}
					}

					// Initialize task breakdown discussion thread
					taskBreakdownID := fmt.Sprintf("task-breakdown-%s", block.Hash())
					communication.CreateThread(taskBreakdownID, fmt.Sprintf("Task Breakdown for Block %d", block.Height), "System")

					// Start collaborative task breakdown process
					taskBreakdownResults := validator.StartCollaborativeTaskBreakdown(chainID, block, transactionDetails)

					// After task breakdown is complete, start task delegation process
					if taskBreakdownResults != nil && len(taskBreakdownResults.FinalSubtasks) > 0 {
						log.Printf("Task breakdown completed with %d subtasks. Starting task delegation process.",
							len(taskBreakdownResults.FinalSubtasks))

						// Initialize task delegation discussion thread
						taskDelegationID := fmt.Sprintf("task-delegation-%s", block.Hash())
						communication.CreateThread(taskDelegationID, fmt.Sprintf("Task Delegation for Block %d", block.Height), "System")

						// Start collaborative task delegation process
						delegationResults := validator.StartCollaborativeTaskDelegation(chainID, taskBreakdownResults)

						if delegationResults != nil {
							log.Printf("Task delegation completed. %d subtasks delegated to validators.",
								len(delegationResults.Assignments))

							// Store the results in the blockchain or a persistence layer
							// (This would be implemented in a real system)

							// Notify the assigned validators of their tasks
							validator.NotifyAssignedValidators(chainID, delegationResults)
						} else {
							log.Printf("Task delegation process failed or produced no assignments")
						}
					} else {
						log.Printf("Task breakdown process failed or produced no subtasks")
					}
				} else {
					log.Printf("No transactions in block %d, skipping task breakdown and delegation", block.Height)
				}
			}

			// Clear temporary data after response is sent
			mp.ClearTemporaryData()
		} else {
			c.JSON(http.StatusOK, gin.H{
				"message":   "Consensus completed",
				"block":     block,
				"accepted":  consensusResult.State == consensus.Accepted,
				"support":   consensusResult.Support,
				"oppose":    consensusResult.Oppose,
				"thread_id": threadID,
			})
		}

	case <-time.After(totalTime):
		log.Printf("nooooooo")
		c.JSON(http.StatusGatewayTimeout, gin.H{
			"error":     "Consensus timed out",
			"block":     block,
			"thread_id": threadID,
		})
	}
}

// GetAllThreads returns all active discussion threads for monitoring.
func GetAllThreads(c *gin.Context) {
	threads := communication.GetAllThreads() // We'll implement this function in forum
	c.JSON(http.StatusOK, threads)
}

type CreateChainRequest struct {
	ChainID       string `json:"chain_id" binding:"required"`
	GenesisPrompt string `json:"genesis_prompt" binding:"required"`
	RewardPool    int    `json:"reward_pool" binding:"required"`
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

func registerAgent(chainID string, agent core.Agent, bootstrapPort int) error {
	// Create a new node for this agent
	newPort := findAvailablePort()
	agentNode := node.NewNode(node.NodeConfig{
		ChainConfig: p2p.ChainConfig{
			ChainID: chainID,
			P2PPort: newPort,
			APIPort: findAvailableAPIPort(),
		},
		BootstrapNode: fmt.Sprintf("localhost:%d", bootstrapPort),
	})

	if err := agentNode.Start(); err != nil {
		return fmt.Errorf("failed to start agent node: %v", err)
	}

	// Register the new node with the chain
	chain := core.GetChain(chainID)
	addr := fmt.Sprintf("localhost:%d", newPort)
	chain.RegisterNode(addr, agentNode.GetP2PNode())

	if agent.Role == "validator" {
		validatorInstance := validator.NewValidator(
			agent.ID,
			agent.Name,
			agent.Traits,
			agent.Style,
			agent.Influences,
			agentNode.GetP2PNode(),
			chain.GenesisPrompt,
			chainID,
		)

		// Register validator
		registry.RegisterValidator(chainID, agent.ID, validatorInstance)

		// Broadcast WebSocket event
		communication.BroadcastEvent(communication.EventAgentRegistered, map[string]interface{}{
			"agent":     agent,
			"chainId":   chainID,
			"nodePort":  newPort,
			"timestamp": time.Now(),
		})
	}

	return nil
}

// CreateChain creates a new blockchain instance
func CreateChain(c *gin.Context) {
	var req CreateChainRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Check if chain already exists
	if core.GetChain(req.ChainID) != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Chain already exists"})
		return
	}

	// Find available ports for the bootstrap node
	p2pPort := findAvailablePort()
	apiPort := findAvailableAPIPort()

	// Create bootstrap node for the new chain
	bootstrapNode := node.NewNode(node.NodeConfig{
		ChainConfig: p2p.ChainConfig{
			ChainID: req.ChainID,
			P2PPort: p2pPort,
			APIPort: apiPort,
		},
		// No bootstrap node address as this will be the bootstrap node
	})

	if err := bootstrapNode.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start bootstrap node"})
		return
	}

	// Initialize new chain with its own mempool
	mp := mempool.NewMempool(req.ChainID)
	core.InitBlockchain(req.ChainID, mp, req.GenesisPrompt, req.RewardPool)

	// Register the bootstrap node with the chain
	chain := core.GetChain(req.ChainID)
	addr := fmt.Sprintf("localhost:%d", p2pPort)
	chain.RegisterNode(addr, bootstrapNode.GetP2PNode())

	communication.BroadcastEvent(communication.EventChainCreated, map[string]interface{}{
		"chainId":   req.ChainID,
		"timestamp": time.Now(),
	})

	// Register sample agents based on the genesis prompt
	agents, err := loadSampleAgents(req.GenesisPrompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load sample agents"})
		return
	}

	log.Printf("Loaded %d sample agents", len(agents))

	// Register agents synchronously
	for _, agent := range agents {
		// Add a small delay between registrations for better UX
		time.Sleep(500 * time.Millisecond)

		if err := registerAgent(req.ChainID, agent, p2pPort); err != nil {
			log.Printf("Failed to register agent %s: %v", agent.ID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to register agent %s", agent.ID)})
			return
		}
		log.Printf("Successfully registered agent: %s (%s)", agent.Name, agent.ID)
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Chain created successfully",
		"chain_id": req.ChainID,
		"bootstrap_node": map[string]int{
			"p2p_port": p2pPort,
			"api_port": apiPort,
		},
		"registered_agents": len(agents),
	})
}

// ListChains returns all available chains
func ListChains(c *gin.Context) {
	chains := core.GetAllChains()
	c.JSON(http.StatusOK, gin.H{
		"chains": chains,
	})
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

// SubmitTask initiates task delegation discussion among validators
func SubmitTask(c *gin.Context) {
	var request struct {
		ChainID string `json:"chain_id" binding:"required"`
		Task    string `json:"task" binding:"required"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create a transaction for the task
	bc := core.GetChain(request.ChainID)
	if bc == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Chain not found"})
		return
	}

	// Create a block with this transaction
	tx := core.Transaction{
		From:      "system",
		To:        "validators",
		Content:   request.Task,
		Timestamp: time.Now().Unix(),
		ChainID:   request.ChainID,
		Type:      "TASK_BREAKDOWN",
	}

	// Create a block with this transaction for collaborative breakdown
	block := &core.Block{
		Height:    bc.Blocks[len(bc.Blocks)-1].Height + 1,
		PrevHash:  bc.Blocks[len(bc.Blocks)-1].Hash(),
		Txs:       []core.Transaction{tx},
		Timestamp: time.Now().Unix(),
		Proposer:  "system",
		ChainID:   request.ChainID,
	}

	// Get all validators to update their memory with the current block
	validators := validator.GetAllValidators(request.ChainID)
	for _, v := range validators {
		if v.Memory != nil {
			v.Memory.SetCurrentBlock(block)
		}
	}

	// Start collaborative task breakdown process
	log.Printf("Starting collaborative task breakdown process for task: %s", request.Task)
	breakdownResults := StartCollaborativeTaskBreakdown(request.ChainID, block, request.Task)

	if breakdownResults == nil || len(breakdownResults.FinalSubtasks) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to break down the task",
		})
		return
	}

	// Start collaborative task delegation process
	log.Printf("Starting collaborative task delegation process for %d subtasks", len(breakdownResults.FinalSubtasks))
	delegationResults := StartCollaborativeTaskDelegation(request.ChainID, breakdownResults)

	if delegationResults == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delegate tasks",
		})
		return
	}

	// Notify assigned validators
	NotifyAssignedValidators(request.ChainID, delegationResults)

	// Add metrics about decision strategy used
	decisionMetrics := map[string]interface{}{
		"breakdown_strategy":  breakdownResults.SelectedStrategy.Name,
		"delegation_strategy": delegationResults.Strategy.Name,
		"consensus_score":     breakdownResults.ConsensusScore,
		"total_subtasks":      len(breakdownResults.FinalSubtasks),
		"validators_involved": len(validators),
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Task submitted and delegated successfully",
		"breakdown_results": gin.H{
			"subtasks":        breakdownResults.FinalSubtasks,
			"strategy":        breakdownResults.SelectedStrategy.Name,
			"consensus_score": breakdownResults.ConsensusScore,
		},
		"delegation_results": gin.H{
			"assignments": delegationResults.Assignments,
			"strategy":    delegationResults.Strategy.Name,
		},
		"metrics": decisionMetrics,
	})
}

// SubmitWorkReview submits completed work for review
func SubmitWorkReview(c *gin.Context) {
	chainID := c.Param("chainId")
	var work struct {
		TaskID      string `json:"task_id"`
		Content     string `json:"content"`
		SubmittedBy string `json:"submitted_by"`
	}

	if err := c.BindJSON(&work); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create transaction for work review
	tx := core.Transaction{
		Type:    "WORK_REVIEW",
		Content: work.Content,
		ChainID: chainID,
		From:    work.SubmittedBy,
	}

	// Get chain and add to mempool
	chain := core.GetChain(chainID)
	if chain == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Chain not found"})
		return
	}

	chain.Mempool.AddTransaction(tx)

	// Broadcast to all validators through P2P
	p2p.GetP2PNode().BroadcastMessage(p2p.Message{
		Type: "WORK_REVIEW",
		Data: tx,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Work submitted for review"})
}

// ProposeRewardDistribution proposes how to distribute rewards for completed work
func ProposeRewardDistribution(c *gin.Context) {
	chainID := c.Param("chainId")
	var proposal struct {
		TaskID       string   `json:"task_id"`
		TotalReward  float64  `json:"total_reward"`
		Contributors []string `json:"contributors"`
	}

	if err := c.BindJSON(&proposal); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create transaction for reward distribution
	tx := core.Transaction{
		Type:    "REWARD_DISTRIBUTION",
		Content: fmt.Sprintf("Task: %s, Reward: %f", proposal.TaskID, proposal.TotalReward),
		ChainID: chainID,
	}

	// Get chain and add to mempool
	chain := core.GetChain(chainID)
	if chain == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Chain not found"})
		return
	}

	chain.Mempool.AddTransaction(tx)

	// Broadcast to all validators through P2P
	p2p.GetP2PNode().BroadcastMessage(p2p.Message{
		Type: "REWARD_DISTRIBUTION",
		Data: map[string]interface{}{
			"transaction":  tx,
			"contributors": proposal.Contributors,
			"totalReward":  proposal.TotalReward,
		},
	})

	c.JSON(http.StatusOK, gin.H{"message": "Reward distribution proposed"})
}

// StartCollaborativeTaskBreakdown starts a collaborative task breakdown process
func StartCollaborativeTaskBreakdown(chainID string, block *core.Block, transactionDetails string) *validator.TaskBreakdownResults {
	log.Printf("Starting collaborative task breakdown for %s", chainID)
	return validator.StartCollaborativeTaskBreakdown(chainID, block, transactionDetails)
}

// StartCollaborativeTaskDelegation starts a collaborative task delegation process
func StartCollaborativeTaskDelegation(chainID string, taskBreakdown *validator.TaskBreakdownResults) *validator.TaskDelegationResults {
	log.Printf("Starting collaborative task delegation for %s", chainID)
	return validator.StartCollaborativeTaskDelegation(chainID, taskBreakdown)
}

// NotifyAssignedValidators notifies validators of their assigned tasks
func NotifyAssignedValidators(chainID string, delegationResults *validator.TaskDelegationResults) {
	log.Printf("Notifying assigned validators for %s", chainID)
	validator.NotifyAssignedValidators(chainID, delegationResults)
}

// GetValidatorBalance returns the current balance for a validator
func GetValidatorBalance(c *gin.Context) {
	chainID := c.GetString("chainID")
	agentID := c.Param("agentID")

	// Check if validator exists
	validator := validator.GetValidatorByID(chainID, agentID)
	if validator == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Validator not found"})
		return
	}

	// Get chain funds
	chainFunds := core.GetChainFunds(chainID)
	if chainFunds == nil {
		// If not initialized yet, return zero balance
		c.JSON(http.StatusOK, gin.H{
			"validator_id": agentID,
			"name":         validator.Name,
			"balance":      0.0,
		})
		return
	}

	// Get validator balance
	balance := chainFunds.GetBalance(agentID)

	c.JSON(http.StatusOK, gin.H{
		"validator_id": agentID,
		"name":         validator.Name,
		"balance":      balance,
	})
}
